// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package selfhostedshootexposure

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	ctrlerror "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	"github.com/gophercloud/gophercloud/v2/openstack/loadbalancer/v2/listeners"
	"github.com/gophercloud/gophercloud/v2/openstack/loadbalancer/v2/loadbalancers"
	"github.com/gophercloud/gophercloud/v2/openstack/loadbalancer/v2/pools"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/security/rules"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	openstackapi "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
)

const (
	// requeueAfterProvisioning is how long to wait while the Octavia load balancer is still
	// provisioning (PENDING_*) or has just been (re)created.
	requeueAfterProvisioning = 5 * time.Second
	// requeueAfterDependency is how long to wait when a prerequisite resource is not yet ready.
	requeueAfterDependency = 30 * time.Second

	// resourcePrefix is the deterministic name prefix for all OpenStack resources created for an exposure.
	resourcePrefix = "shoot-exposure"
	// tagKey is the key of the tag used to discover the load balancer belonging to an exposure.
	tagKey = "gardener.cloud/shoot-exposure"

	// provisioningStatusActive is the Octavia provisioning status of a fully provisioned resource.
	provisioningStatusActive = "ACTIVE"
	// provisioningStatusError is the Octavia provisioning status of a resource that failed to provision.
	provisioningStatusError = "ERROR"
	// provisioningStatusPendingDelete is the Octavia provisioning status of a resource being deleted.
	provisioningStatusPendingDelete = "PENDING_DELETE"
)

// resourceName returns the deterministic, namespace-qualified name used for the load balancer,
// listener and pool of an exposure. The control-plane namespace is unique per shoot, so this is
// stable across reconciles and unique across shoots sharing an OpenStack project.
func resourceName(exposure *extensionsv1alpha1.SelfHostedShootExposure) string {
	return resourcePrefix + "-" + exposure.Namespace
}

// exposureTag returns the tag applied to the load balancer (and used as the floating IP
// description) so the resources can be discovered deterministically. Octavia names are not
// unique, so discovery is by tag.
func exposureTag(exposure *extensionsv1alpha1.SelfHostedShootExposure) string {
	return fmt.Sprintf("%s=%s", tagKey, exposure.Namespace)
}

func validateNodesSubnetCIDR(subnet *openstackapi.Subnet, family gardencorev1beta1.IPFamily) error {
	if subnet.CIDR == "" {
		return &ctrlerror.RequeueAfterError{
			RequeueAfter: requeueAfterDependency,
			Cause:        fmt.Errorf("waiting for %s nodes subnet %q CIDR in infrastructure status; reconcile Infrastructure to populate it", family, subnet.ID),
		}
	}
	prefix, err := netip.ParsePrefix(subnet.CIDR)
	if err != nil {
		return fmt.Errorf("invalid CIDR %q for nodes subnet %q in infrastructure status: %w", subnet.CIDR, subnet.ID, err)
	}
	matchesFamily := (family == gardencorev1beta1.IPFamilyIPv4 && prefix.Addr().Is4()) ||
		(family == gardencorev1beta1.IPFamilyIPv6 && prefix.Addr().Is6() && !prefix.Addr().Is4In6())
	if !matchesFamily {
		return fmt.Errorf("CIDR %q for nodes subnet %q in infrastructure status does not match IP family %s", subnet.CIDR, subnet.ID, family)
	}
	return nil
}

// securityGroupRuleOpts builds the ingress rule that lets the load balancer forward traffic from
// the nodes subnet to the control-plane machines on the exposure port.
func securityGroupRuleOpts(exposure *extensionsv1alpha1.SelfHostedShootExposure, securityGroupID string, family gardencorev1beta1.IPFamily, nodes string) rules.CreateOpts {
	etherType := rules.EtherType4
	prefix := nodes
	if family == gardencorev1beta1.IPFamilyIPv6 {
		etherType = rules.EtherType6
	}
	port := int(exposure.Spec.Port)
	return rules.CreateOpts{
		Direction:      rules.DirIngress,
		Description:    resourceName(exposure),
		EtherType:      etherType,
		Protocol:       rules.ProtocolTCP,
		PortRangeMin:   port,
		PortRangeMax:   port,
		SecGroupID:     securityGroupID,
		RemoteIPPrefix: prefix,
	}
}

// primaryIPFamily returns the primary IP family of the shoot. A single Octavia VIP and Neutron
// floating IP are single-family, so the exposure is created for the primary family only.
func primaryIPFamily(cluster *extensionscontroller.Cluster) gardencorev1beta1.IPFamily {
	if cluster.Shoot.Spec.Networking != nil && len(cluster.Shoot.Spec.Networking.IPFamilies) > 0 {
		return cluster.Shoot.Spec.Networking.IPFamilies[0]
	}
	return gardencorev1beta1.IPFamilyIPv4
}

// desiredMemberAddresses returns the control-plane node IPs that match the given IP family.
func desiredMemberAddresses(exposure *extensionsv1alpha1.SelfHostedShootExposure, family gardencorev1beta1.IPFamily) ([]string, error) {
	var addresses []string
	seen := map[string]struct{}{}
	for _, endpoint := range exposure.Spec.Endpoints {
		for _, address := range endpoint.Addresses {
			if address.Type != corev1.NodeInternalIP {
				continue
			}
			ip, err := netip.ParseAddr(address.Address)
			if err != nil {
				return nil, fmt.Errorf("could not parse address %q for endpoint %q: %w", address.Address, endpoint.NodeName, err)
			}
			if ip.Is4() != (family == gardencorev1beta1.IPFamilyIPv4) {
				continue
			}
			if _, ok := seen[address.Address]; ok {
				continue
			}
			seen[address.Address] = struct{}{}
			addresses = append(addresses, address.Address)
		}
	}
	return addresses, nil
}

// loadBalancerCreateOpts builds a create request that provisions the load balancer, listener, pool
// and initial members in a single Octavia call. No health monitor is configured: the gardenlet
// already filters .spec.endpoints to healthy control-plane nodes (see GEP-0036).
func loadBalancerCreateOpts(exposure *extensionsv1alpha1.SelfHostedShootExposure, vipSubnetID, nodesSubnetID string, memberAddresses []string) loadbalancers.CreateOpts {
	name := resourceName(exposure)
	port := int(exposure.Spec.Port)

	members := make([]pools.CreateMemberOpts, 0, len(memberAddresses))
	for _, address := range memberAddresses {
		members = append(members, pools.CreateMemberOpts{
			Address:      address,
			ProtocolPort: port,
			SubnetID:     nodesSubnetID,
		})
	}

	return loadbalancers.CreateOpts{
		Name:         name,
		Description:  "Gardener self-hosted shoot control-plane exposure",
		VipSubnetID:  vipSubnetID,
		Tags:         []string{exposureTag(exposure)},
		AdminStateUp: ptr.To(true),
		Listeners: []listeners.CreateOpts{{
			Name:         name,
			Protocol:     listeners.ProtocolTCP,
			ProtocolPort: port,
			DefaultPool: &pools.CreateOpts{
				Name:     name,
				Protocol: pools.ProtocolTCP,
				LBMethod: pools.LBMethodRoundRobin,
				Members:  members,
			},
		}},
	}
}

// deleteOptsCascade tears down the whole load balancer tree (listener, pool, members) in one call.
func deleteOptsCascade() loadbalancers.DeleteOpts {
	return loadbalancers.DeleteOpts{Cascade: true}
}

// batchMemberOpts builds the declarative member set submitted to BatchUpdatePoolMembers.
func batchMemberOpts(exposure *extensionsv1alpha1.SelfHostedShootExposure, nodesSubnetID string, memberAddresses []string) []pools.BatchUpdateMemberOpts {
	port := int(exposure.Spec.Port)
	opts := make([]pools.BatchUpdateMemberOpts, 0, len(memberAddresses))
	for _, address := range memberAddresses {
		opts = append(opts, pools.BatchUpdateMemberOpts{
			Address:      address,
			ProtocolPort: port,
			SubnetID:     ptr.To(nodesSubnetID),
		})
	}
	return opts
}

func getInfrastructureStatus(ctx context.Context, c client.Client, namespace, shootName string) (*openstackapi.InfrastructureStatus, error) {
	infra := &extensionsv1alpha1.Infrastructure{}
	if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: shootName}, infra); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, &ctrlerror.RequeueAfterError{
				RequeueAfter: requeueAfterDependency,
				Cause:        fmt.Errorf("waiting for Infrastructure resource to be created"),
			}
		}
		return nil, fmt.Errorf("error getting infrastructure: %w", err)
	}
	if infra.Status.ProviderStatus == nil {
		return nil, &ctrlerror.RequeueAfterError{
			RequeueAfter: requeueAfterDependency,
			Cause:        fmt.Errorf("waiting for Infrastructure status to be populated"),
		}
	}
	return helper.InfrastructureStatusFromRaw(infra.Status.ProviderStatus)
}

// secretReference returns the reference to the cloudprovider secret in the shoot control-plane
// namespace, from which the OpenStack credentials are read.
func secretReference(exposure *extensionsv1alpha1.SelfHostedShootExposure) corev1.SecretReference {
	return corev1.SecretReference{
		Namespace: exposure.Namespace,
		Name:      v1beta1constants.SecretNameCloudProvider,
	}
}
