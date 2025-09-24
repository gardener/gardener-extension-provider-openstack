// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
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
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/layer3/routers"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/security/groups"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/security/rules"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/ports"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/utils/net"
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
	opts, err := NewOpts(bastion, cluster, log)
	if err != nil {
		return err
	}

	credentials, err := openstack.GetCredentials(ctx, a.client, opts.SecretReference, false)
	if err != nil {
		return fmt.Errorf("could not get Openstack credentials: %w", err)
	}

	openstackClientFactory, err := a.openstackClientFactory.NewFactory(ctx, credentials)
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

		opts.MachineType = a.bastionConfig.FlavorRef
		opts.ImageID = image.ID
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

	securityGroup, err := ensureSecurityGroup(ctx, networkingClient, opts)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	err = ensureSecurityGroupRules(ctx, networkingClient, bastion, opts, infraStatus, securityGroup.ID)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	err = ensureShootWorkerSecurityGroupRules(ctx, networkingClient, opts, infraStatus, securityGroup.ID)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	instance, err := ensureComputeInstance(ctx, computeClient, infraStatus, opts)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	fipID, err := ensurePublicIPAddress(ctx, opts, networkingClient, infraStatus)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	err = ensureAssociateFIPWithInstance(ctx, networkingClient, instance, fipID)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	// check if the instance already exists and has an IP
	endpoints, err := ensureEndpoints(instance, opts)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	// once a public endpoint is available, publish the endpoint on the
	// Bastion resource to notify upstream about the ready instance
	patch := client.MergeFrom(bastion.DeepCopy())
	bastion.Status.Ingress = endpoints.public
	return a.client.Status().Patch(ctx, bastion, patch)
}

// Retry performs a function with retries, delay, and a max number of attempts
func Retry(maxRetries int, delay time.Duration, log logr.Logger, fn func() error) error {
	var err error
	for i := 0; i < maxRetries; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		log.Info(fmt.Sprintf("Attempt %d failed, retrying in %v: %v", i+1, delay, err))
		time.Sleep(delay)
	}
	return err
}

func ensurePublicIPAddress(ctx context.Context, opts Options, client openstackclient.Networking, infraStatus *openstackapi.InfrastructureStatus) (floatingips.FloatingIP, error) {
	opts.Logr.Info("Ensuring public IP address for bastion instance", "name", opts.BastionInstanceName)

	fips, err := findFipByName(ctx, client, opts.BastionInstanceName)
	if err != nil {
		return floatingips.FloatingIP{}, err
	}

	if len(fips) > 1 {
		return floatingips.FloatingIP{}, fmt.Errorf("bastion instance has more than one public IP address")
	}

	if len(fips) == 1 {
		if fips[0].Status == "ACTIVE" {
			opts.Logr.Info("Found existing public IP address for bastion instance", "name", opts.BastionInstanceName, "ip", fips[0].FloatingIP)
			return fips[0], nil
		}
		return floatingips.FloatingIP{}, fmt.Errorf("public IP address for %s is not active, status: %s", opts.BastionInstanceName, fips[0].Status)
	}

	if infraStatus.Networks.FloatingPool.ID == "" {
		return floatingips.FloatingIP{}, fmt.Errorf("floatingPool must not be empty")
	}

	if infraStatus.Networks.Router.ID == "" {
		return floatingips.FloatingIP{}, fmt.Errorf("router must not be empty")
	}

	router, err := client.GetRouterByID(ctx, infraStatus.Networks.Router.ID)
	if err != nil {
		return floatingips.FloatingIP{}, err
	}

	if router == nil {
		return floatingips.FloatingIP{}, fmt.Errorf("router with ID %s was not found", infraStatus.Networks.Router.ID)
	}

	if len(router.GatewayInfo.ExternalFixedIPs) == 0 {
		return floatingips.FloatingIP{}, fmt.Errorf("no external fixed IPs detected on the router")
	}

	if router.Status != "ACTIVE" {
		opts.Logr.Info("Router not active, retrying until it becomes ACTIVE", "routerID", router.ID, "currentStatus", router.Status)
		if err := Retry(30, 6*time.Second, opts.Logr, func() error {
			router, err = client.GetRouterByID(ctx, infraStatus.Networks.Router.ID)
			if err != nil {
				return err
			}
			if router == nil {
				return fmt.Errorf("router with ID %s was not found", infraStatus.Networks.Router.ID)
			}
			if router.Status != "ACTIVE" {
				return fmt.Errorf("router not active yet, status: %s", router.Status)
			}
			return nil
		}); err != nil {
			return floatingips.FloatingIP{}, err
		}
	}

	// locate the first ipv4 address.
	idx := slices.IndexFunc(router.GatewayInfo.ExternalFixedIPs, func(s routers.ExternalFixedIP) bool {
		return net.IsIPv4String(s.IPAddress)
	})
	if idx == -1 {
		return floatingips.FloatingIP{}, fmt.Errorf("failed to locate a suitable ipv4 address in the router external fixed IPs")
	}

	createOpts := floatingips.CreateOpts{
		Description:       opts.BastionInstanceName,
		FloatingNetworkID: infraStatus.Networks.FloatingPool.ID,
		SubnetID:          router.GatewayInfo.ExternalFixedIPs[idx].SubnetID,
	}

	opts.Logr.Info("Creating public IP address", "name", opts.BastionInstanceName, "subnetID", createOpts.SubnetID, "floatingNetworkID", createOpts.FloatingNetworkID)
	fip, err := createFloatingIP(ctx, client, createOpts)
	if err != nil {
		return floatingips.FloatingIP{}, fmt.Errorf("failed to create public ip address: %w", err)
	}
	opts.Logr.Info("Public IP address created", "name", opts.BastionInstanceName, "ip", fip.FloatingIP)

	// wait until floating IP is active
	err = Retry(30, 5*time.Second, opts.Logr, func() error {
		fip, err = client.GetFloatingIP(ctx, floatingips.ListOpts{ID: fip.ID})
		if err != nil {
			return err
		}

		if fip.Status != "ACTIVE" {
			return fmt.Errorf("fip not active yet, status: %s", fip.Status)
		}

		return nil
	})

	return fip, err
}

func ensureComputeInstance(ctx context.Context, client openstackclient.Compute, infraStatus *openstackapi.InfrastructureStatus, opts Options) (servers.Server, error) {
	opts.Logr.Info("Ensuring bastion compute instance", "name", opts.BastionInstanceName)

	instances, err := findBastionInstances(ctx, client, opts.BastionInstanceName)
	if openstackclient.IgnoreNotFoundError(err) != nil {
		return servers.Server{}, err
	}

	if len(instances) == 1 {
		opts.Logr.Info("Found existing bastion compute instance", "name", opts.BastionInstanceName, "id", instances[0].ID)
		return instances[0], nil
	}

	if infraStatus.Networks.ID == "" {
		return servers.Server{}, errors.New("network id not found")
	}

	flavorID, err := client.FindFlavorID(ctx, opts.MachineType)
	if err != nil {
		return servers.Server{}, err
	}

	if flavorID == "" {
		return servers.Server{}, errors.New("flavorID not found")
	}

	createOpts := servers.CreateOpts{
		Name:           opts.BastionInstanceName,
		FlavorRef:      flavorID,
		ImageRef:       opts.ImageID,
		SecurityGroups: []string{opts.SecurityGroup},
		Networks:       []servers.Network{{UUID: infraStatus.Networks.ID}},
		UserData:       opts.UserData,
	}

	opts.Logr.Info("Starting bastion compute instance creation", "name", opts.BastionInstanceName)
	instance, err := createBastionInstance(ctx, client, createOpts)
	if err != nil {
		return servers.Server{}, fmt.Errorf("failed to create bastion compute instance: %w ", err)
	}

	// wait until instance and floatingIP are ready
	err = Retry(60, 10*time.Second, opts.Logr, func() error {
		// refresh bastion instance
		instance, err = getBastionInstance(ctx, client, opts.BastionInstanceName)
		if err != nil {
			return util.DetermineError(err, helper.KnownCodes)
		}

		if instance.Status != "ACTIVE" {
			return fmt.Errorf("bastion instance not active yet, status: %s, progress %d", instance.Status, instance.Progress)
		}

		return nil
	})

	return instance, err
}

func getInstanceEndpoints(instance servers.Server, opts Options) (bastionEndpoints, error) {
	if instance.Status != "ACTIVE" {
		return bastionEndpoints{}, errors.New("compute instance not active yet")
	}

	endpoints := bastionEndpoints{}

	privateIP, externalIP, err := GetIPs(instance, opts)
	if err != nil {
		return bastionEndpoints{}, fmt.Errorf("no IP found: %w", err)
	}

	if ingress := addressToIngress("", privateIP); ingress != nil {
		endpoints.private = ingress
	}

	if ingress := addressToIngress("", externalIP); ingress != nil {
		endpoints.public = ingress
	}
	return endpoints, nil
}

// IngressReady returns true if either an IP or a hostname or both are set.
func IngressReady(ingress *corev1.LoadBalancerIngress) bool {
	return ingress != nil && (ingress.Hostname != "" || ingress.IP != "")
}

// addressToIngress converts the IP address into a corev1.LoadBalancerIngress resource.
// If both arguments are nil, then nil is returned.
func addressToIngress(dnsName string, ipAddress string) *corev1.LoadBalancerIngress {
	if dnsName == "" && ipAddress == "" {
		return nil
	}
	ingress := &corev1.LoadBalancerIngress{}
	if dnsName != "" {
		ingress.Hostname = dnsName
	}
	if ipAddress != "" {
		ingress.IP = ipAddress
	}
	return ingress
}

func ensureAssociateFIPWithInstance(ctx context.Context, networkClient openstackclient.Networking, instance servers.Server, floatingIP floatingips.FloatingIP) error {
	// the floatingIP is already associated
	if floatingIP.PortID != "" {
		return nil
	}

	return associateFIPWithInstance(ctx, networkClient, instance, floatingIP)
}

func associateFIPWithInstance(ctx context.Context, networkClient openstackclient.Networking, instance servers.Server, floatingIP floatingips.FloatingIP) error {
	instancePorts, err := networkClient.GetInstancePorts(ctx, instance.ID)
	if err != nil {
		return err
	}

	if len(instancePorts) == 0 {
		return fmt.Errorf("no ports found for instance id %s", instance.ID)
	}

	// maybe we need to refine which port to use
	var instancePort *ports.Port
	for _, port := range instancePorts {
		if port.Status == "ACTIVE" {
			instancePort = &port
			break
		}
	}

	if instancePort == nil {
		return fmt.Errorf("no active port found for instance id %s", instance.ID)
	}

	return networkClient.UpdateFIPWithPort(ctx, floatingIP.ID, instancePort.ID)
}

func ensureSecurityGroupRules(ctx context.Context, client openstackclient.Networking, bastion *extensionsv1alpha1.Bastion, opts Options, infraStatus *openstackapi.InfrastructureStatus, secGroupID string) error {
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
			IngressAllowSSH(opts, ingressPermission.EtherType, secGroupID, ingressPermission.CIDR, ""),
		)
	}
	wantedRules = append(wantedRules, EgressAllowSSHToWorker(opts, secGroupID, infraStatus.SecurityGroups[0].ID))

	currentRules, err := listRules(ctx, client, secGroupID)
	if err != nil {
		return fmt.Errorf("failed to list rules: %w", err)
	}

	rulesToAdd, rulesToDelete := rulesSymmetricDifference(wantedRules, currentRules)

	for _, rule := range rulesToAdd {
		opts.Logr.Info("Creating security group rule", "rule", rule.Description)
		if err := createSecurityGroupRuleIfNotExist(ctx, opts.Logr, client, rule); err != nil {
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
		opts.Logr.Info("Unwanted security group rule deleted", "rule", rule.Description, "ruleID", rule.ID)
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
			log.Info("Security Group Rule already exists", "rule", createOpts.Description)
			return nil
		}
		return fmt.Errorf("failed to create Security Group rule %s: %w", createOpts.Description, err)
	}
	log.Info("Security Group Rule created", "rule", createOpts.Description)
	return nil
}

func ensureSecurityGroup(ctx context.Context, client openstackclient.Networking, opts Options) (groups.SecGroup, error) {
	opts.Logr.Info("Ensuring security group for bastion", "name", opts.SecurityGroup)

	securityGroups, err := getSecurityGroups(ctx, client, opts.SecurityGroup)
	if err != nil {
		return groups.SecGroup{}, err
	}

	if len(securityGroups) > 1 {
		return groups.SecGroup{}, fmt.Errorf("found more than one security group with name %s", opts.SecurityGroup)
	}

	if len(securityGroups) == 1 {
		opts.Logr.Info("Security Group already exists", "name", securityGroups[0].Name, "id", securityGroups[0].ID)
		return securityGroups[0], nil
	}

	opts.Logr.Info("Creating new security group", "security group", opts.SecurityGroup)
	result, err := createSecurityGroup(ctx, client, groups.CreateOpts{
		Name:        opts.SecurityGroup,
		Description: opts.SecurityGroup,
	})
	if err != nil {
		return groups.SecGroup{}, err
	}
	opts.Logr.Info("Security Group created", "security group", result.Name)

	return *result, nil
}

func ensureShootWorkerSecurityGroupRules(ctx context.Context, client openstackclient.Networking, opts Options, infraStatus *openstackapi.InfrastructureStatus, secGroupID string) error {
	if len(infraStatus.SecurityGroups) == 0 {
		return errors.New("shoot security groups not found")
	}

	allowSSHRule := IngressAllowSSH(opts, rules.EtherType4, infraStatus.SecurityGroups[0].ID, "", secGroupID)
	if err := createSecurityGroupRuleIfNotExist(ctx, opts.Logr, client, allowSSHRule); err != nil {
		return fmt.Errorf("failed to add shoot worker security group rule for %s: %w", allowSSHRule.Description, err)
	}
	return nil
}

func ensureEndpoints(instance servers.Server, opts Options) (bastionEndpoints, error) {
	// check if the instance already exists and has an IP
	endpoints, err := getInstanceEndpoints(instance, opts)
	if err != nil {
		return bastionEndpoints{}, err
	}

	if !endpoints.ready() {
		return bastionEndpoints{}, &ctrlerror.RequeueAfterError{
			// requeue rather soon, so that the user (most likely gardenctl eventually)
			// doesn't have to wait too long for the public endpoint to become available
			RequeueAfter: 5 * time.Second,
			Cause:        errors.New("bastion instance has no public/private endpoints yet"),
		}
	}

	opts.Logr.Info("bastion endpoints ready")

	return endpoints, nil
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
