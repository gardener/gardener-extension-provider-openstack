// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package selfhostedshootexposure

import (
	"context"
	"fmt"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	ctrlerror "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	openstackapi "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
)

func (a *actuator) Delete(ctx context.Context, log logr.Logger, exposure *extensionsv1alpha1.SelfHostedShootExposure, cluster *extensionscontroller.Cluster) error {
	lbClient, networkingClient, err := a.clients(ctx, exposure)
	if err != nil {
		// If the cloudprovider secret is already gone (e.g. during shoot deletion), there is
		// nothing left for us to clean up.
		if apierrors.IsNotFound(err) {
			log.Info("Cloudprovider secret not found, nothing to clean up")
			return nil
		}
		return err
	}

	// Delete the floating IP first so it is released even if the load balancer deletion needs
	// several requeues.
	if err := deleteFloatingIP(ctx, networkingClient, exposure); err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	// Remove the ingress rule we added to the control-plane machines' security group.
	if err := a.cleanupSecurityGroupRule(ctx, networkingClient, exposure, cluster); err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	lb, err := findLoadBalancer(ctx, lbClient, exposure)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}
	if lb == nil {
		log.Info("Load balancer and floating IP gone, releasing SelfHostedShootExposure")
		return nil
	}

	if lb.ProvisioningStatus == provisioningStatusPendingDelete {
		return &ctrlerror.RequeueAfterError{
			RequeueAfter: requeueAfterProvisioning,
			Cause:        fmt.Errorf("waiting for load balancer to be deleted"),
		}
	}

	log.Info("Deleting load balancer", "id", lb.ID)
	if err := lbClient.DeleteLoadbalancer(ctx, lb.ID, deleteOptsCascade()); err != nil {
		return util.DetermineError(fmt.Errorf("could not delete load balancer: %w", err), helper.KnownCodes)
	}
	return &ctrlerror.RequeueAfterError{
		RequeueAfter: requeueAfterProvisioning,
		Cause:        fmt.Errorf("waiting for load balancer to be deleted"),
	}
}

func (a *actuator) ForceDelete(ctx context.Context, log logr.Logger, exposure *extensionsv1alpha1.SelfHostedShootExposure, cluster *extensionscontroller.Cluster) error {
	// Best-effort teardown: attempt to release the floating IP and load balancer but never block
	// the shoot's force deletion on OpenStack errors or a load balancer stuck in a PENDING state.
	lbClient, networkingClient, err := a.clients(ctx, exposure)
	if err != nil {
		log.Info("Could not build OpenStack clients for force deletion, ignoring", "error", err.Error())
		return nil
	}

	if err := deleteFloatingIP(ctx, networkingClient, exposure); err != nil {
		log.Info("Could not delete floating IP during force deletion, ignoring", "error", err.Error())
	}

	if err := a.cleanupSecurityGroupRule(ctx, networkingClient, exposure, cluster); err != nil {
		log.Info("Could not delete security group rule during force deletion, ignoring", "error", err.Error())
	}

	lb, err := findLoadBalancer(ctx, lbClient, exposure)
	if err != nil {
		log.Info("Could not find load balancer during force deletion, ignoring", "error", err.Error())
		return nil
	}
	if lb == nil {
		return nil
	}
	if err := lbClient.DeleteLoadbalancer(ctx, lb.ID, deleteOptsCascade()); err != nil {
		log.Info("Could not delete load balancer during force deletion, ignoring", "error", err.Error())
	}
	return nil
}

// clients builds the OpenStack Loadbalancing and Networking clients from the cloudprovider
// credentials in the shoot control-plane namespace.
func (a *actuator) clients(ctx context.Context, exposure *extensionsv1alpha1.SelfHostedShootExposure) (openstackclient.Loadbalancing, openstackclient.Networking, error) {
	credentials, err := openstack.GetCredentials(ctx, a.client, secretReference(exposure), false)
	if err != nil {
		return nil, nil, err
	}
	factory, err := a.openstackClientFactory.NewFactory(ctx, credentials)
	if err != nil {
		return nil, nil, util.DetermineError(fmt.Errorf("could not create OpenStack client factory: %w", err), helper.KnownCodes)
	}
	lbClient, err := factory.Loadbalancing()
	if err != nil {
		return nil, nil, util.DetermineError(err, helper.KnownCodes)
	}
	networkingClient, err := factory.Networking()
	if err != nil {
		return nil, nil, util.DetermineError(err, helper.KnownCodes)
	}
	return lbClient, networkingClient, nil
}

// deleteFloatingIP releases the floating IP allocated for the exposure, ignoring NotFound so the
// operation is idempotent.
func deleteFloatingIP(ctx context.Context, networkingClient openstackclient.Networking, exposure *extensionsv1alpha1.SelfHostedShootExposure) error {
	fips, err := networkingClient.GetFipByName(ctx, exposureTag(exposure))
	if err != nil {
		return fmt.Errorf("could not list floating IPs: %w", err)
	}
	for _, fip := range fips {
		if err := networkingClient.DeleteFloatingIP(ctx, fip.ID); openstackclient.IgnoreNotFoundError(err) != nil {
			return fmt.Errorf("could not delete floating IP %s: %w", fip.ID, err)
		}
	}
	return nil
}

// cleanupSecurityGroupRule removes the ingress rule added to the control-plane machines' security
// group. If the Infrastructure or its nodes security group is already gone (e.g. the infra was
// deleted first), there is nothing to clean up as the security group is removed with it.
func (a *actuator) cleanupSecurityGroupRule(ctx context.Context, networkingClient openstackclient.Networking, exposure *extensionsv1alpha1.SelfHostedShootExposure, cluster *extensionscontroller.Cluster) error {
	infra := &extensionsv1alpha1.Infrastructure{}
	if err := a.client.Get(ctx, client.ObjectKey{Namespace: exposure.Namespace, Name: cluster.Shoot.Name}, infra); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("error getting infrastructure: %w", err)
	}
	if infra.Status.ProviderStatus == nil {
		return nil
	}
	infraStatus, err := helper.InfrastructureStatusFromRaw(infra.Status.ProviderStatus)
	if err != nil {
		return err
	}
	nodesSecurityGroup, err := helper.FindSecurityGroupByPurpose(infraStatus.SecurityGroups, openstackapi.PurposeNodes)
	if err != nil {
		// No nodes security group means there is no rule of ours to remove.
		return nil //nolint:nilerr // absence of the nodes SG is expected during teardown
	}
	return deleteSecurityGroupRule(ctx, networkingClient, exposure, nodesSecurityGroup.ID)
}
