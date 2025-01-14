// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	ctrlerror "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	"github.com/go-logr/logr"
	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/v2/openstack/image/v2/images"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/layer3/floatingips"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/security/groups"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/security/rules"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/ports"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/client"

	openstackapi "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
)

// bastionEndpoints holds the endpoints the bastion host provides
type bastionEndpoints struct {
	// private is the private endpoint of the bastion. It is required when opening a port on the worker node to allow SSH access from the bastion
	private *corev1.LoadBalancerIngress
	//  public is the public endpoint where the enduser connects to establish the SSH connection.
	public *corev1.LoadBalancerIngress
}

// ready returns true if both public and private interfaces each have either
// an IP or a hostname or both.
func (be *bastionEndpoints) ready() bool {
	return be != nil && IngressReady(be.private) && IngressReady(be.public)
}

func (a *actuator) Reconcile(ctx context.Context, log logr.Logger, bastion *extensionsv1alpha1.Bastion, cluster *controller.Cluster) error {
	opt, err := DetermineOptions(bastion, cluster)
	if err != nil {
		return err
	}

	credentials, err := openstack.GetCredentials(ctx, a.client, opt.SecretReference, false)
	if err != nil {
		return fmt.Errorf("could not get Openstack credentials: %w", err)
	}

	openstackClientFactory, err := a.openstackClientFactory.NewFactory(credentials)
	if err != nil {
		return util.DetermineError(fmt.Errorf("could not create Openstack client factory: %w", err), helper.KnownCodes)
	}

	// TODO(hebelsan) Remove bastionConfig via helm chart config map in future release
	if useBastionControllerConfig(a.bastionConfig) {
		imageClient, err := openstackClientFactory.Images()
		if err != nil {
			return util.DetermineError(err, helper.KnownCodes)
		}

		imageRes, err := imageClient.ListImages(ctx, images.ListOpts{
			ID:         a.bastionConfig.ImageRef,
			Visibility: "all",
		})
		if err != nil {
			log.Info("image not found by id")
		}
		// we didn't find any image by ID. We will try to find by name.
		if len(imageRes) == 0 {
			imageRes, err = imageClient.ListImages(ctx, images.ListOpts{
				Name:       a.bastionConfig.ImageRef,
				Visibility: "all",
			})
			if err != nil {
				return err
			}
		}
		if len(imageRes) == 0 {
			return fmt.Errorf("imageRef: '%s' not found neither by id or name", a.bastionConfig.ImageRef)
		}
		image := &imageRes[0]

		opt.machineType = a.bastionConfig.FlavorRef
		opt.imageID = image.ID
	}

	computeClient, err := openstackClientFactory.Compute()
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	networkingClient, err := openstackClientFactory.Networking()
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	infraStatus, err := getInfrastructureStatus(ctx, a.client, cluster)
	if err != nil {
		return err
	}

	securityGroup, err := ensureSecurityGroup(ctx, log, networkingClient, opt)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	err = ensureSecurityGroupRules(ctx, log, networkingClient, bastion, opt, infraStatus, securityGroup.ID)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	err = ensureShootWorkerSecurityGroupRules(ctx, log, networkingClient, opt, infraStatus, securityGroup.ID)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	instance, err := ensureComputeInstance(ctx, log, computeClient, infraStatus, opt)
	if err != nil || instance == nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	infrastructureConfig := &openstackapi.InfrastructureConfig{}

	if cluster.Shoot.Spec.Provider.InfrastructureConfig.Raw == nil {
		return errors.New("infrastructureConfig raw must not be empty")
	}

	if _, _, err := a.decoder.Decode(cluster.Shoot.Spec.Provider.InfrastructureConfig.Raw, nil, infrastructureConfig); err != nil {
		return fmt.Errorf("could not decode InfrastructureConfig of cluster Profile': %w", err)
	}

	fipid, err := ensurePublicIPAddress(ctx, opt, log, networkingClient, infraStatus)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	err = ensureAssociateFIPWithInstance(ctx, networkingClient, instance, fipid)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	// refresh instance after public ip attached/created
	instances, err := getBastionInstance(ctx, computeClient, opt.BastionInstanceName)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	if len(instances) == 0 {
		return errors.New("instances must not be empty")
	}

	// check if the instance already exists and has an IP
	endpoints, err := getInstanceEndpoints(&instances[0], opt)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	if !endpoints.ready() {
		return &ctrlerror.RequeueAfterError{
			// requeue rather soon, so that the user (most likely gardenctl eventually)
			// doesn't have to wait too long for the public endpoint to become available
			RequeueAfter: 5 * time.Second,
			Cause:        errors.New("bastion instance has no public/private endpoints yet"),
		}
	}

	// once a public endpoint is available, publish the endpoint on the
	// Bastion resource to notify upstream about the ready instance
	patch := client.MergeFrom(bastion.DeepCopy())
	bastion.Status.Ingress = endpoints.public
	return a.client.Status().Patch(ctx, bastion, patch)
}

func ensurePublicIPAddress(ctx context.Context, opt *Options, log logr.Logger, client openstackclient.Networking, infraStatus *openstackapi.InfrastructureStatus) (*floatingips.FloatingIP, error) {
	fips, err := getFipByName(ctx, client, opt.BastionInstanceName)
	if err != nil {
		return nil, err
	}

	if len(fips) != 0 && fips[0].Status == "ACTIVE" {
		return &fips[0], nil
	}

	log.Info("creating new bastion Public IP")

	if infraStatus.Networks.FloatingPool.ID == "" {
		return nil, errors.New("floatingPool must not be empty")
	}

	if infraStatus.Networks.Router.ID == "" {
		return nil, errors.New("router must not be empty")
	}

	router, err := client.GetRouterByID(ctx, infraStatus.Networks.Router.ID)
	if err != nil {
		return nil, err
	}

	if router == nil {
		return nil, fmt.Errorf("router with ID %s was not found", infraStatus.Networks.Router.ID)
	}

	if len(router.GatewayInfo.ExternalFixedIPs) == 0 {
		return nil, errors.New("no external fixed IPs detected on the router")
	}

	createOpts := floatingips.CreateOpts{
		Description:       opt.BastionInstanceName,
		FloatingNetworkID: infraStatus.Networks.FloatingPool.ID,
		SubnetID:          router.GatewayInfo.ExternalFixedIPs[0].SubnetID,
	}

	fip, err := createFloatingIP(ctx, client, createOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to get (create) public ip address: %w", err)
	}

	return fip, nil
}

func ensureComputeInstance(ctx context.Context, log logr.Logger, client openstackclient.Compute, infraStatus *openstackapi.InfrastructureStatus, opt *Options) (*servers.Server, error) {
	instances, err := getBastionInstance(ctx, client, opt.BastionInstanceName)
	if openstackclient.IgnoreNotFoundError(err) != nil {
		return nil, err
	}

	if len(instances) != 0 {
		return &instances[0], nil
	}

	log.Info("Creating new bastion compute instance")

	if infraStatus.Networks.ID == "" {
		return nil, errors.New("network id not found")
	}

	flavorID, err := client.FindFlavorID(ctx, opt.machineType)
	if err != nil {
		return nil, err
	}

	if flavorID == "" {
		return nil, errors.New("flavorID not found")
	}

	createOpts := servers.CreateOpts{
		Name:           opt.BastionInstanceName,
		FlavorRef:      flavorID,
		ImageRef:       opt.imageID,
		SecurityGroups: []string{opt.SecurityGroup},
		Networks:       []servers.Network{{UUID: infraStatus.Networks.ID}},
		UserData:       opt.UserData,
	}

	instance, err := createBastionInstance(ctx, client, createOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create bastion compute instance: %w ", err)
	}

	return instance, nil
}

func getInstanceEndpoints(instance *servers.Server, opt *Options) (*bastionEndpoints, error) {
	if instance == nil {
		return nil, errors.New("compute instance can't be nil")
	}

	if instance.Status != "ACTIVE" {
		return nil, errors.New("compute instance not active yet")
	}

	endpoints := &bastionEndpoints{}

	privateIP, externalIP, err := GetIPs(instance, opt)
	if err != nil {
		return nil, fmt.Errorf("no IP found: %w", err)
	}

	if ingress := addressToIngress(nil, &privateIP); ingress != nil {
		endpoints.private = ingress
	}

	if ingress := addressToIngress(nil, &externalIP); ingress != nil {
		endpoints.public = ingress
	}
	return endpoints, nil
}

// IngressReady returns true if either an IP or a hostname or both are set.
func IngressReady(ingress *corev1.LoadBalancerIngress) bool {
	return ingress != nil && (ingress.Hostname != "" || ingress.IP != "")
}

// addressToIngress converts the IP address into a
// corev1.LoadBalancerIngress resource. If both arguments are nil, then
// nil is returned.
func addressToIngress(dnsName *string, ipAddress *string) *corev1.LoadBalancerIngress {
	var ingress *corev1.LoadBalancerIngress

	if ipAddress != nil || dnsName != nil {
		ingress = &corev1.LoadBalancerIngress{}
		if dnsName != nil {
			ingress.Hostname = *dnsName
		}

		if ipAddress != nil {
			ingress.IP = *ipAddress
		}
	}

	return ingress
}

func ensureAssociateFIPWithInstance(ctx context.Context, networkClient openstackclient.Networking, instance *servers.Server, floatingIP *floatingips.FloatingIP) error {
	instancePorts, err := networkClient.GetInstancePorts(ctx, instance.ID)
	if err != nil {
		return err
	}

	if len(instancePorts) == 0 {
		return fmt.Errorf("no ports found for instance id %s", instance.ID)
	}

	// maybe we need to refine which port to use
	var activePort *ports.Port
	for _, port := range instancePorts {
		if port.Status == "ACTIVE" {
			activePort = &port
			break
		}
	}

	if activePort == nil {
		return fmt.Errorf("no active port found for instance id %s", instance.ID)
	}

	// check if floating ip is already associated with the instance
	fip, err := networkClient.GetFloatingIP(ctx, floatingips.ListOpts{PortID: activePort.ID})
	if err != nil {
		return err
	}

	if fip.ID == "" {
		err := networkClient.UpdateFIPWithPort(ctx, floatingIP.ID, activePort.ID)
		if err != nil {
			return err
		}
		fip = *floatingIP
	}

	if fip.ID != "" {
		return nil
	}

	if fip.Status != "ACTIVE" || instance.Status != "ACTIVE" {
		return fmt.Errorf("instance or floating ip address not ready yet")
	}

	return networkClient.UpdateFIPWithPort(ctx, floatingIP.ID, activePort.ID)
}

func ensureSecurityGroupRules(ctx context.Context, log logr.Logger, client openstackclient.Networking, bastion *extensionsv1alpha1.Bastion, opt *Options, infraStatus *openstackapi.InfrastructureStatus, secGroupID string) error {
	ingressPermissions, err := ingressPermissions(bastion)
	if err != nil {
		return err
	}

	// The assumption is that the shoot only has one security group
	if len(infraStatus.SecurityGroups) == 0 {
		return errors.New("shoot security groups not found")
	}

	// apply security rules in bastion security group
	var wantedRules []rules.CreateOpts
	for _, ingressPermission := range ingressPermissions {
		wantedRules = append(wantedRules,
			IngressAllowSSH(opt, ingressPermission.EtherType, secGroupID, ingressPermission.CIDR, ""),
		)
	}
	wantedRules = append(wantedRules, EgressAllowSSHToWorker(opt, secGroupID, infraStatus.SecurityGroups[0].ID))

	currentRules, err := listRules(ctx, client, secGroupID)
	if err != nil {
		return fmt.Errorf("failed to list rules: %w", err)
	}

	rulesToAdd, rulesToDelete := rulesSymmetricDifference(wantedRules, currentRules)

	for _, rule := range rulesToAdd {
		if err := createSecurityGroupRuleIfNotExist(ctx, log, client, rule); err != nil {
			return fmt.Errorf("failed to add security group rule %s: %w", rule.Description, err)
		}
	}

	for _, rule := range rulesToDelete {
		if err := deleteRule(ctx, client, rule.ID); err != nil {
			if openstackclient.IsNotFoundError(err) {
				continue
			}
			return fmt.Errorf("failed to delete security group rule %s (%s): %w", rule.Description, rule.ID, err)
		}
		log.Info("Unwanted security group rule deleted", "rule", rule.Description, "ruleID", rule.ID)
	}

	return nil
}

func rulesSymmetricDifference(wantedRules []rules.CreateOpts, currentRules []rules.SecGroupRule) ([]rules.CreateOpts, []rules.SecGroupRule) {
	var rulesToDelete []rules.SecGroupRule
	for _, currentRule := range currentRules {
		found := false
		for _, wantedRule := range wantedRules {
			if ruleEqual(wantedRule, currentRule) {
				found = true
				break
			}
		}

		if !found {
			rulesToDelete = append(rulesToDelete, currentRule)
		}
	}

	var rulesToAdd []rules.CreateOpts
	for _, wantedRule := range wantedRules {
		found := false
		for _, currentRule := range currentRules {
			if ruleEqual(wantedRule, currentRule) {
				found = true
				break
			}
		}

		if !found {
			rulesToAdd = append(rulesToAdd, wantedRule)
		}
	}

	return rulesToAdd, rulesToDelete
}

func ruleEqual(a rules.CreateOpts, b rules.SecGroupRule) bool {
	if !equality.Semantic.DeepEqual(string(a.Direction), b.Direction) {
		return false
	}

	if !equality.Semantic.DeepEqual(a.Description, b.Description) {
		return false
	}

	if !equality.Semantic.DeepEqual(string(a.EtherType), b.EtherType) {
		return false
	}

	if !equality.Semantic.DeepEqual(a.SecGroupID, b.SecGroupID) {
		return false
	}

	if !equality.Semantic.DeepEqual(a.PortRangeMin, b.PortRangeMin) || !equality.Semantic.DeepEqual(a.PortRangeMax, b.PortRangeMax) {
		return false
	}

	if !equality.Semantic.DeepEqual(string(a.Protocol), b.Protocol) {
		return false
	}

	if !equality.Semantic.DeepEqual(a.RemoteGroupID, b.RemoteGroupID) {
		return false
	}

	if !equality.Semantic.DeepEqual(a.RemoteIPPrefix, b.RemoteIPPrefix) {
		return false
	}

	return true
}

func createSecurityGroupRuleIfNotExist(ctx context.Context, log logr.Logger, client openstackclient.Networking, createOpts rules.CreateOpts) error {
	if _, err := createRules(ctx, client, createOpts); err != nil {
		if gophercloud.ResponseCodeIs(err, http.StatusConflict) {
			return nil
		}
		return fmt.Errorf("failed to create Security Group rule %s: %w", createOpts.Description, err)
	}
	log.Info("Security Group Rule created", "rule", createOpts.Description)
	return nil
}

func ensureSecurityGroup(ctx context.Context, log logr.Logger, client openstackclient.Networking, opt *Options) (groups.SecGroup, error) {
	securityGroups, err := getSecurityGroups(ctx, client, opt.SecurityGroup)
	if err != nil {
		return groups.SecGroup{}, err
	}

	if len(securityGroups) != 0 {
		return securityGroups[0], nil
	}

	result, err := createSecurityGroup(ctx, client, groups.CreateOpts{
		Name:        opt.SecurityGroup,
		Description: opt.SecurityGroup,
	})
	if err != nil {
		return groups.SecGroup{}, err
	}

	log.Info("Security Group created", "security group", result.Name)
	return *result, nil
}

func ensureShootWorkerSecurityGroupRules(ctx context.Context, log logr.Logger, client openstackclient.Networking, opt *Options, infraStatus *openstackapi.InfrastructureStatus, secGroupID string) error {
	if len(infraStatus.SecurityGroups) == 0 {
		return errors.New("shoot security groups not found")
	}

	allowSSHRule := IngressAllowSSH(opt, rules.EtherType4, infraStatus.SecurityGroups[0].ID, "", secGroupID)
	if err := createSecurityGroupRuleIfNotExist(ctx, log, client, allowSSHRule); err != nil {
		return fmt.Errorf("failed to add shoot worker security group rule for %s: %w", allowSSHRule.Description, err)
	}
	return nil
}

func getInfrastructureStatus(ctx context.Context, c client.Client, cluster *controller.Cluster) (*openstackapi.InfrastructureStatus, error) {
	worker := &extensionsv1alpha1.Worker{}
	err := c.Get(ctx, client.ObjectKey{Namespace: cluster.ObjectMeta.Name, Name: cluster.Shoot.Name}, worker)
	if err != nil {
		return nil, err
	}

	if worker == nil || worker.Spec.InfrastructureProviderStatus == nil {
		return nil, errors.New("infrastructure provider status must be not empty for worker")
	}

	return helper.InfrastructureStatusFromRaw(worker.Spec.InfrastructureProviderStatus)
}
