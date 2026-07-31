// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package selfhostedshootexposure

import (
	"context"
	"fmt"

	extensionsselfhostedshootexposure "github.com/gardener/gardener/extensions/pkg/controller/selfhostedshootexposure"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	ctrlerror "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	"github.com/gophercloud/gophercloud/v2/openstack/loadbalancer/v2/loadbalancers"
	"github.com/gophercloud/gophercloud/v2/openstack/loadbalancer/v2/pools"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/layer3/floatingips"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/security/rules"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
)

type actuator struct {
	client                 client.Client
	openstackClientFactory openstackclient.FactoryFactory
}

// NewActuator creates a new Actuator for SelfHostedShootExposure reconciliation.
func NewActuator(mgr manager.Manager, openstackClientFactory openstackclient.FactoryFactory) extensionsselfhostedshootexposure.Actuator {
	return &actuator{
		client:                 mgr.GetClient(),
		openstackClientFactory: openstackClientFactory,
	}
}

// findLoadBalancer discovers the load balancer of the exposure by its tag, disambiguating
// client-side by the exact tag and deterministic name (Octavia tag semantics vary and names are
// not unique). It returns nil when no matching load balancer exists.
func findLoadBalancer(ctx context.Context, lbClient openstackclient.Loadbalancing, exposure *extensionsv1alpha1.SelfHostedShootExposure) (*loadbalancers.LoadBalancer, error) {
	tag := exposureTag(exposure)
	name := resourceName(exposure)

	lbs, err := lbClient.ListLoadbalancers(ctx, loadbalancers.ListOpts{Tags: []string{tag}})
	if err != nil {
		return nil, fmt.Errorf("could not list load balancers: %w", err)
	}

	var matches []loadbalancers.LoadBalancer
	for _, lb := range lbs {
		if lb.Name != name {
			continue
		}
		if !hasTag(lb.Tags, tag) {
			continue
		}
		matches = append(matches, lb)
	}

	switch len(matches) {
	case 0:
		return nil, nil
	case 1:
		return &matches[0], nil
	default:
		return nil, fmt.Errorf("found %d load balancers matching tag %q and name %q, expected at most one", len(matches), tag, name)
	}
}

func hasTag(tags []string, want string) bool {
	for _, tag := range tags {
		if tag == want {
			return true
		}
	}
	return false
}

// ensureLoadBalancer returns the existing load balancer for the exposure or creates it (with the
// full listener/pool/monitor/member tree) when absent. A freshly created load balancer is
// returned with its PENDING_CREATE status so the caller requeues until it becomes ACTIVE.
func ensureLoadBalancer(ctx context.Context, lbClient openstackclient.Loadbalancing, exposure *extensionsv1alpha1.SelfHostedShootExposure, vipSubnetID, nodesSubnetID string, memberAddresses []string) (*loadbalancers.LoadBalancer, error) {
	lb, err := findLoadBalancer(ctx, lbClient, exposure)
	if err != nil {
		return nil, err
	}

	if lb != nil {
		// Refresh to obtain the current provisioning status and VIP port. Guard against a
		// List/Get race where the load balancer was deleted concurrently.
		refreshed, err := lbClient.GetLoadbalancer(ctx, lb.ID)
		if err != nil {
			return nil, fmt.Errorf("could not get load balancer %s: %w", lb.ID, err)
		}
		if refreshed == nil {
			return nil, &ctrlerror.RequeueAfterError{
				RequeueAfter: requeueAfterProvisioning,
				Cause:        fmt.Errorf("load balancer disappeared during reconciliation, will retry"),
			}
		}
		return refreshed, nil
	}

	created, err := lbClient.CreateLoadbalancer(ctx, loadBalancerCreateOpts(exposure, vipSubnetID, nodesSubnetID, memberAddresses))
	if err != nil {
		return nil, fmt.Errorf("could not create load balancer: %w", err)
	}
	return created, nil
}

// findPool discovers the pool of the exposure's load balancer by load balancer ID and name.
func findPool(ctx context.Context, lbClient openstackclient.Loadbalancing, exposure *extensionsv1alpha1.SelfHostedShootExposure, lbID string) (*pools.Pool, error) {
	name := resourceName(exposure)
	ps, err := lbClient.ListPools(ctx, pools.ListOpts{LoadbalancerID: lbID, Name: name})
	if err != nil {
		return nil, fmt.Errorf("could not list pools: %w", err)
	}
	if len(ps) == 0 {
		return nil, nil
	}
	return &ps[0], nil
}

// ensureFloatingIP allocates a floating IP from the floating pool and associates it with the
// load balancer's VIP port, or re-associates an existing floating IP if it drifted.
func ensureFloatingIP(ctx context.Context, networkingClient openstackclient.Networking, exposure *extensionsv1alpha1.SelfHostedShootExposure, floatingPoolID, vipPortID string) (*floatingips.FloatingIP, error) {
	tag := exposureTag(exposure)

	fips, err := networkingClient.GetFipByName(ctx, tag)
	if err != nil {
		return nil, fmt.Errorf("could not list floating IPs: %w", err)
	}

	switch len(fips) {
	case 0:
		created, err := networkingClient.CreateFloatingIP(ctx, floatingips.CreateOpts{
			FloatingNetworkID: floatingPoolID,
			Description:       tag,
			PortID:            vipPortID,
		})
		if err != nil {
			return nil, fmt.Errorf("could not create floating IP: %w", err)
		}
		return created, nil
	case 1:
		fip := fips[0]
		if fip.PortID != vipPortID {
			if err := networkingClient.UpdateFIPWithPort(ctx, fip.ID, vipPortID); err != nil {
				return nil, fmt.Errorf("could not associate floating IP %s with VIP port %s: %w", fip.ID, vipPortID, err)
			}
			fip.PortID = vipPortID
		}
		return &fip, nil
	default:
		return nil, fmt.Errorf("found %d floating IPs for exposure, expected at most one", len(fips))
	}
}

// ensureSecurityGroupRule ensures the control-plane machines' security group allows ingress on
// the exposure port so the load balancer (and its health-monitor probes) can reach the members.
// It is idempotent: it creates the rule if absent and replaces it if the desired shape changed.
func ensureSecurityGroupRule(ctx context.Context, networkingClient openstackclient.Networking, exposure *extensionsv1alpha1.SelfHostedShootExposure, securityGroupID string, family gardencorev1beta1.IPFamily) error {
	desired := securityGroupRuleOpts(exposure, securityGroupID, family)

	existing, err := networkingClient.ListRules(ctx, rules.ListOpts{
		SecGroupID:  securityGroupID,
		Description: desired.Description,
	})
	if err != nil {
		return fmt.Errorf("could not list security group rules: %w", err)
	}

	for _, rule := range existing {
		if securityGroupRuleMatches(rule, desired) {
			return nil
		}
		// The rule drifted from what we want (e.g. port or ether type changed): replace it.
		if err := networkingClient.DeleteRule(ctx, rule.ID); openstackclient.IgnoreNotFoundError(err) != nil {
			return fmt.Errorf("could not delete stale security group rule %s: %w", rule.ID, err)
		}
	}

	if _, err := networkingClient.CreateRule(ctx, desired); err != nil {
		return fmt.Errorf("could not create security group rule: %w", err)
	}
	return nil
}

// deleteSecurityGroupRule removes the ingress rule created for the exposure, ignoring NotFound.
func deleteSecurityGroupRule(ctx context.Context, networkingClient openstackclient.Networking, exposure *extensionsv1alpha1.SelfHostedShootExposure, securityGroupID string) error {
	existing, err := networkingClient.ListRules(ctx, rules.ListOpts{
		SecGroupID:  securityGroupID,
		Description: resourceName(exposure),
	})
	if err != nil {
		return fmt.Errorf("could not list security group rules: %w", err)
	}
	for _, rule := range existing {
		if err := networkingClient.DeleteRule(ctx, rule.ID); openstackclient.IgnoreNotFoundError(err) != nil {
			return fmt.Errorf("could not delete security group rule %s: %w", rule.ID, err)
		}
	}
	return nil
}

func securityGroupRuleMatches(rule rules.SecGroupRule, desired rules.CreateOpts) bool {
	return rule.Direction == string(desired.Direction) &&
		rule.EtherType == string(desired.EtherType) &&
		rule.Protocol == string(desired.Protocol) &&
		rule.PortRangeMin == desired.PortRangeMin &&
		rule.PortRangeMax == desired.PortRangeMax &&
		rule.RemoteIPPrefix == desired.RemoteIPPrefix
}
