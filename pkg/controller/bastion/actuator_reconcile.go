// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bastion

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/config"
	openstackapi "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	ctrlerror "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	"github.com/go-logr/logr"
	"github.com/gophercloud/gophercloud"
	computefip "github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/floatingips"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/layer3/floatingips"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/security/groups"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/security/rules"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	err := bastionConfigCheck(a.bastionConfig)
	if err != nil {
		return err
	}

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

	securityGroup, err := ensureSecurityGroup(log, networkingClient, opt)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	err = ensureSecurityGroupRules(log, networkingClient, bastion, opt, infraStatus, securityGroup.ID)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	instance, err := ensureComputeInstance(log, computeClient, a.bastionConfig, infraStatus, opt)
	if err != nil || instance == nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	infrastructureConfig := &openstackapi.InfrastructureConfig{}

	if cluster.Shoot.Spec.Provider.InfrastructureConfig.Raw == nil {
		return errors.New("infrastructureConfig raw must not be empty")
	}

	if _, _, err := a.Decoder().Decode(cluster.Shoot.Spec.Provider.InfrastructureConfig.Raw, nil, infrastructureConfig); err != nil {
		return fmt.Errorf("could not decode InfrastructureConfig of cluster Profile': %w", err)
	}

	fipid, err := ensurePublicIPAddress(opt, log, networkingClient, infraStatus)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	err = ensureAssociateFIPWithInstance(computeClient, instance, fipid)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	// refresh instance after public ip attached/created
	instances, err := getBastionInstance(computeClient, opt.BastionInstanceName)
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

func ensurePublicIPAddress(opt *Options, log logr.Logger, client openstackclient.Networking, infraStatus *openstackapi.InfrastructureStatus) (*floatingips.FloatingIP, error) {
	fips, err := getFipByName(client, opt.BastionInstanceName)
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

	createOpts := floatingips.CreateOpts{
		FloatingNetworkID: infraStatus.Networks.FloatingPool.ID,
		Description:       opt.BastionInstanceName,
	}

	fip, err := createFloatingIP(client, createOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to get (create) public ip address: %w", err)
	}

	return fip, nil
}

func ensureComputeInstance(log logr.Logger, client openstackclient.Compute, bastionConfig *config.BastionConfig, infraStatus *openstackapi.InfrastructureStatus, opt *Options) (*servers.Server, error) {
	instances, err := getBastionInstance(client, opt.BastionInstanceName)
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

	flavorID, err := client.FindFlavorID(bastionConfig.FlavorRef)
	if err != nil {
		return nil, err
	}

	if flavorID == "" {
		return nil, errors.New("flavorID not found")
	}

	images, err := client.FindImages(bastionConfig.ImageRef)
	if err != nil {
		return nil, err
	}

	if len(images) == 0 {
		return nil, errors.New("imageID not found")
	}

	createOpts := servers.CreateOpts{
		Name:           opt.BastionInstanceName,
		FlavorRef:      flavorID,
		ImageRef:       images[0].ID,
		SecurityGroups: []string{opt.SecurityGroup},
		Networks:       []servers.Network{{UUID: infraStatus.Networks.ID}},
		UserData:       opt.UserData,
	}

	instance, err := createBastionInstance(client, createOpts)
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

func ensureAssociateFIPWithInstance(client openstackclient.Compute, instance *servers.Server, floatingIP *floatingips.FloatingIP) error {
	fipid, err := findFloatingIDByInstanceID(client, instance.ID)
	if err != nil {
		return err
	}

	if fipid != "" {
		return nil
	}

	if floatingIP.Status != "ACTIVE" || instance.Status != "ACTIVE" {
		return fmt.Errorf("instance or floating ip address not ready yet")
	}

	associateOpts := computefip.AssociateOpts{
		FloatingIP: floatingIP.FloatingIP,
	}

	if err := associateFIPWithInstance(client, instance.ID, associateOpts); err != nil {
		return fmt.Errorf("failed to associate public ip address %s to instance %s: %w", floatingIP.FloatingIP, instance.Name, err)
	}
	return nil
}

func ensureSecurityGroupRules(log logr.Logger, client openstackclient.Networking, bastion *extensionsv1alpha1.Bastion, opt *Options, infraStatus *openstackapi.InfrastructureStatus, secGroupID string) error {
	ingressPermissions, err := ingressPermissions(bastion)
	if err != nil {
		return err
	}

	// The assumption is that the shoot only has one security group
	if len(infraStatus.SecurityGroups) == 0 {
		return errors.New("shoot security groups not found")
	}

	var wantedRules []rules.CreateOpts
	for _, ingressPermission := range ingressPermissions {
		wantedRules = append(wantedRules,
			IngressAllowSSH(opt, ingressPermission.EtherType, secGroupID, ingressPermission.CIDR),
			EgressAllowSSHToWorker(opt, secGroupID, infraStatus.SecurityGroups[0].ID),
		)
	}

	currentRules, err := listRules(client, secGroupID)
	if err != nil {
		return fmt.Errorf("failed to list rules: %w", err)
	}

	rulesToAdd, rulesToDelete := rulesSymmetricDifference(wantedRules, currentRules)

	for _, rule := range rulesToAdd {
		if err := createSecurityGroupRuleIfNotExist(log, client, rule); err != nil {
			return fmt.Errorf("failed to add security group rule %s: %w", rule.Description, err)
		}
	}

	for _, rule := range rulesToDelete {
		if err := deleteRule(client, rule.ID); err != nil {
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

func createSecurityGroupRuleIfNotExist(log logr.Logger, client openstackclient.Networking, createOpts rules.CreateOpts) error {
	if _, err := createRules(client, createOpts); err != nil {
		if _, ok := err.(gophercloud.ErrDefault409); ok {
			return nil
		}
		return fmt.Errorf("failed to create Security Group rule %s: %w", createOpts.Description, err)
	}
	log.Info("Security Group Rule created", "rule", createOpts.Description)
	return nil
}

func ensureSecurityGroup(log logr.Logger, client openstackclient.Networking, opt *Options) (groups.SecGroup, error) {
	securityGroups, err := getSecurityGroups(client, opt.SecurityGroup)
	if err != nil {
		return groups.SecGroup{}, err
	}

	if len(securityGroups) != 0 {
		return securityGroups[0], nil
	}

	result, err := createSecurityGroup(client, groups.CreateOpts{
		Name:        opt.SecurityGroup,
		Description: opt.SecurityGroup,
	})
	if err != nil {
		return groups.SecGroup{}, err
	}

	log.Info("Security Group created", "security group", result.Name)
	return *result, nil
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
