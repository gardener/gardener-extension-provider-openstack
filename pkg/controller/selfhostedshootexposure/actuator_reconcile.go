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
	corev1 "k8s.io/api/core/v1"

	openstackapi "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
)

func (a *actuator) Reconcile(ctx context.Context, log logr.Logger, exposure *extensionsv1alpha1.SelfHostedShootExposure, cluster *extensionscontroller.Cluster) ([]corev1.LoadBalancerIngress, error) {
	infraStatus, err := getInfrastructureStatus(ctx, a.client, exposure.Namespace, cluster.Shoot.Name)
	if err != nil {
		return nil, err
	}

	nodesSubnet, err := helper.FindSubnetByPurpose(infraStatus.Networks.Subnets, openstackapi.PurposeNodes)
	if err != nil {
		return nil, &ctrlerror.RequeueAfterError{
			RequeueAfter: requeueAfterDependency,
			Cause:        fmt.Errorf("waiting for nodes subnet in infrastructure status: %w", err),
		}
	}
	floatingPoolID := infraStatus.Networks.FloatingPool.ID
	if floatingPoolID == "" || nodesSubnet.ID == "" {
		return nil, &ctrlerror.RequeueAfterError{
			RequeueAfter: requeueAfterDependency,
			Cause:        fmt.Errorf("waiting for floating pool and nodes subnet to be populated in infrastructure status"),
		}
	}

	if len(exposure.Spec.Endpoints) == 0 {
		return nil, &ctrlerror.RequeueAfterError{
			RequeueAfter: requeueAfterDependency,
			Cause:        fmt.Errorf("waiting for endpoints to be populated in spec"),
		}
	}

	family := primaryIPFamily(cluster)
	memberAddresses, err := desiredMemberAddresses(exposure, family)
	if err != nil {
		return nil, err
	}
	if len(memberAddresses) == 0 {
		return nil, &ctrlerror.RequeueAfterError{
			RequeueAfter: requeueAfterDependency,
			Cause:        fmt.Errorf("waiting for %s control-plane node addresses in endpoints", family),
		}
	}

	credentials, err := openstack.GetCredentials(ctx, a.client, secretReference(exposure), false)
	if err != nil {
		return nil, fmt.Errorf("could not get OpenStack credentials: %w", err)
	}
	factory, err := a.openstackClientFactory.NewFactory(ctx, credentials)
	if err != nil {
		return nil, util.DetermineError(fmt.Errorf("could not create OpenStack client factory: %w", err), helper.KnownCodes)
	}
	lbClient, err := factory.Loadbalancing()
	if err != nil {
		return nil, util.DetermineError(err, helper.KnownCodes)
	}
	networkingClient, err := factory.Networking()
	if err != nil {
		return nil, util.DetermineError(err, helper.KnownCodes)
	}

	// Ensure the load balancer (with its listener/pool/monitor/members) exists.
	lb, err := ensureLoadBalancer(ctx, lbClient, exposure, nodesSubnet.ID, nodesSubnet.ID, memberAddresses)
	if err != nil {
		return nil, requeueOnConflict(util.DetermineError(err, helper.KnownCodes))
	}

	// Gate on the provisioning status: Octavia serializes operations per load balancer, so we
	// must only mutate children once the load balancer is ACTIVE.
	switch lb.ProvisioningStatus {
	case provisioningStatusActive:
		// proceed
	case provisioningStatusError:
		log.Info("Load balancer is in ERROR state, deleting it so it is recreated on the next reconcile", "id", lb.ID)
		if err := lbClient.DeleteLoadbalancer(ctx, lb.ID, deleteOptsCascade()); err != nil {
			return nil, util.DetermineError(fmt.Errorf("could not delete load balancer in ERROR state: %w", err), helper.KnownCodes)
		}
		return nil, &ctrlerror.RequeueAfterError{
			RequeueAfter: requeueAfterProvisioning,
			Cause:        fmt.Errorf("load balancer was in ERROR state, recreating"),
		}
	default:
		return nil, &ctrlerror.RequeueAfterError{
			RequeueAfter: requeueAfterProvisioning,
			Cause:        fmt.Errorf("waiting for load balancer to become ACTIVE, current status: %s", lb.ProvisioningStatus),
		}
	}

	// Reconcile the pool members declaratively.
	pool, err := findPool(ctx, lbClient, exposure, lb.ID)
	if err != nil {
		return nil, util.DetermineError(err, helper.KnownCodes)
	}
	if pool == nil {
		return nil, &ctrlerror.RequeueAfterError{
			RequeueAfter: requeueAfterProvisioning,
			Cause:        fmt.Errorf("waiting for load balancer pool to be created"),
		}
	}

	log.Info("Reconciling load balancer pool members", "pool", pool.ID, "members", len(memberAddresses))
	if err := lbClient.BatchUpdatePoolMembers(ctx, pool.ID, batchMemberOpts(exposure, nodesSubnet.ID, memberAddresses)); err != nil {
		return nil, requeueOnConflict(util.DetermineError(fmt.Errorf("could not update pool members: %w", err), helper.KnownCodes))
	}

	// Allow the load balancer (and its health-monitor probes) to reach the control-plane machines
	// on the exposure port. The amphora backend port address is not discoverable via the load
	// balancer API, so the rule cannot be scoped to a single source; it allows the exposure port
	// (the TLS-protected kube-apiserver) from anywhere.
	nodesSecurityGroup, err := helper.FindSecurityGroupByPurpose(infraStatus.SecurityGroups, openstackapi.PurposeNodes)
	if err != nil {
		return nil, &ctrlerror.RequeueAfterError{
			RequeueAfter: requeueAfterDependency,
			Cause:        fmt.Errorf("waiting for nodes security group in infrastructure status: %w", err),
		}
	}
	if err := ensureSecurityGroupRule(ctx, networkingClient, exposure, nodesSecurityGroup.ID, family); err != nil {
		return nil, util.DetermineError(err, helper.KnownCodes)
	}

	// Allocate and associate the floating IP.
	fip, err := ensureFloatingIP(ctx, networkingClient, exposure, floatingPoolID, lb.VipPortID)
	if err != nil {
		return nil, util.DetermineError(err, helper.KnownCodes)
	}
	if fip.FloatingIP == "" {
		return nil, &ctrlerror.RequeueAfterError{
			RequeueAfter: requeueAfterProvisioning,
			Cause:        fmt.Errorf("waiting for floating IP address to be assigned"),
		}
	}

	log.Info("Self-hosted shoot exposure ready", "ip", fip.FloatingIP)
	return []corev1.LoadBalancerIngress{{IP: fip.FloatingIP}}, nil
}

// requeueOnConflict translates an Octavia 409 (load balancer immutable while PENDING) into a
// short requeue instead of a hard error, so a concurrent status transition is retried.
func requeueOnConflict(err error) error {
	if err != nil && openstackclient.IsConflictError(err) {
		return &ctrlerror.RequeueAfterError{
			RequeueAfter: requeueAfterProvisioning,
			Cause:        fmt.Errorf("load balancer is not in a mutable state yet: %w", err),
		}
	}
	return err
}
