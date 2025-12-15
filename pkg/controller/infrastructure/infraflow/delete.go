// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	"context"

	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gophercloud/gophercloud/v2/openstack/sharedfilesystems/v2/sharenetworks"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/controller/infrastructure/infraflow/shared"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
)

// Delete creates and runs the flow to delete the AWS infrastructure.
func (fctx *FlowContext) Delete(ctx context.Context) error {
	if fctx.state.IsEmpty() {
		// nothing to do, e.g. if cluster was created with wrong credentials
		return nil
	}

	fctx.BasicFlowContext = shared.NewBasicFlowContext().WithSpan().WithLogger(fctx.log).WithPersist(fctx.persistState)
	g := fctx.buildDeleteGraph()
	f := g.Compile()
	if err := f.Run(ctx, flow.Opts{Log: fctx.log}); err != nil {
		return flow.Causes(err)
	}
	return nil
}

func (fctx *FlowContext) buildDeleteGraph() *flow.Graph {
	g := flow.NewGraph("Openstack infrastructure destruction")

	needToDeleteNetwork := fctx.config.Networks.ID == nil
	needToDeleteRouter := fctx.config.Networks.Router == nil

	_ = fctx.AddTask(g, "delete ssh key pair",
		fctx.deleteSSHKeyPair,
		shared.Timeout(defaultTimeout))
	_ = fctx.AddTask(g, "delete security group",
		fctx.deleteSecGroup,
		shared.Timeout(defaultTimeout))
	recoverRouterID := fctx.AddTask(g, "recover router ID",
		fctx.recoverRouterID,
		shared.Timeout(defaultTimeout))
	recoverNetworkID := fctx.AddTask(g, "recover network ID",
		fctx.recoverNetworkID,
		shared.Timeout(defaultTimeout))
	recoverSubnetID := fctx.AddTask(g, "recover subnet ID",
		fctx.recoverSubnetID,
		shared.Timeout(defaultTimeout), shared.Dependencies(recoverNetworkID))
	recoverSubnetIPv6ID := fctx.AddTask(g, "recover IPv6 subnet ID",
		fctx.recoverSubnetIPv6ID,
		shared.Timeout(defaultTimeout), shared.Dependencies(recoverNetworkID))

	recoverIDs := flow.NewTaskIDs(recoverNetworkID, recoverRouterID, recoverSubnetID, recoverSubnetIPv6ID)
	k8sRoutes := fctx.AddTask(g, "delete kubernetes routes",
		func(ctx context.Context) error {
			routerID := fctx.state.Get(IdentifierRouter)
			if routerID == nil {
				return nil
			}
			return fctx.cleanupKubernetesRoutes(ctx, *routerID)
		},
		shared.Timeout(defaultTimeout),
		shared.Dependencies(recoverIDs),
	)
	k8sLoadBalancers := fctx.AddTask(g, "delete kubernetes loadbalancers",
		func(ctx context.Context) error {
			subnetID := fctx.state.Get(IdentifierSubnet)
			if subnetID == nil {
				return nil
			}
			return fctx.cleanupKubernetesLoadbalancers(ctx, shared.LogFromContext(ctx), *subnetID)
		},
		shared.Timeout(defaultTimeout),
		shared.Dependencies(recoverIDs),
	)

	k8sLoadBalancersIPv6 := fctx.AddTask(g, "delete kubernetes IPv6 loadbalancers",
		func(ctx context.Context) error {
			subnetIPv6ID := fctx.state.Get(IdentifierSubnetIPv6)
			if subnetIPv6ID == nil {
				return nil
			}
			return fctx.cleanupKubernetesLoadbalancers(ctx, shared.LogFromContext(ctx), *subnetIPv6ID)
		},
		shared.Timeout(defaultTimeout),
		shared.Dependencies(recoverIDs),
	)

	_ = fctx.AddTask(g, "delete share network",
		fctx.deleteShareNetwork,
		shared.Timeout(defaultTimeout), shared.Dependencies(recoverIDs))
	deleteRouterInterface := fctx.AddTask(g, "delete router interface",
		fctx.deleteRouterInterface,
		shared.Timeout(defaultTimeout), shared.Dependencies(recoverIDs, k8sRoutes))
	deleteRouterInterfaceIPv6 := fctx.AddTask(g, "delete IPv6 router interface",
		fctx.deleteRouterInterfaceIPv6,
		shared.Timeout(defaultTimeout), shared.Dependencies(recoverIDs, k8sRoutes))

	// subnet deletion only needed if network is given by spec
	_ = fctx.AddTask(g, "delete subnet",
		fctx.deleteSubnet,
		shared.DoIf(!needToDeleteNetwork), shared.Timeout(defaultTimeout), shared.Dependencies(deleteRouterInterface, k8sLoadBalancers))
	_ = fctx.AddTask(g, "delete IPv6 subnet",
		fctx.deleteSubnetIPv6,
		shared.DoIf(!needToDeleteNetwork), shared.Timeout(defaultTimeout), shared.Dependencies(deleteRouterInterfaceIPv6, k8sLoadBalancersIPv6))
	_ = fctx.AddTask(g, "delete network",
		fctx.deleteNetwork,
		shared.DoIf(needToDeleteNetwork), shared.Timeout(defaultTimeout), shared.Dependencies(deleteRouterInterface, deleteRouterInterfaceIPv6))
	_ = fctx.AddTask(g, "delete router",
		fctx.deleteRouter,
		shared.DoIf(needToDeleteRouter), shared.Timeout(defaultTimeout), shared.Dependencies(deleteRouterInterface, deleteRouterInterfaceIPv6))
	_ = fctx.AddTask(g, "cleanup marker",
		func(_ context.Context) error {
			fctx.state.Set(CreatedResourcesExistKey, "")
			return nil
		})

	return g
}

func (fctx *FlowContext) deleteRouter(ctx context.Context) error {
	routerID := fctx.state.Get(IdentifierRouter)
	if routerID == nil {
		return nil
	}

	shared.LogFromContext(ctx).Info("deleting...", "router", *routerID)
	if err := fctx.networking.DeleteRouter(ctx, *routerID); client.IgnoreNotFoundError(err) != nil {
		return err
	}

	fctx.state.Set(IdentifierRouter, "")
	return nil
}

func (fctx *FlowContext) deleteNetwork(ctx context.Context) error {
	networkID := fctx.state.Get(IdentifierNetwork)
	if networkID == nil {
		return nil
	}

	shared.LogFromContext(ctx).Info("deleting...", "network", *networkID)
	if err := fctx.networking.DeleteNetwork(ctx, *networkID); client.IgnoreNotFoundError(err) != nil {
		return err
	}

	fctx.state.Set(NameNetwork, "")
	fctx.state.Set(IdentifierNetwork, "")
	return nil
}

func (fctx *FlowContext) deleteSubnet(ctx context.Context) error {
	subnetID := fctx.state.Get(IdentifierSubnet)
	if subnetID == nil {
		return nil
	}

	shared.LogFromContext(ctx).Info("deleting...", "subnet", *subnetID)
	if err := fctx.networking.DeleteSubnet(ctx, *subnetID); client.IgnoreNotFoundError(err) != nil {
		return err
	}
	fctx.state.Set(IdentifierSubnet, "")
	return nil
}

func (fctx *FlowContext) deleteSubnetIPv6(ctx context.Context) error {
	subnetIPv6ID := fctx.state.Get(IdentifierSubnetIPv6)
	if subnetIPv6ID == nil {
		return nil
	}

	shared.LogFromContext(ctx).Info("deleting...", "ipv6-subnet", *subnetIPv6ID)
	if err := fctx.networking.DeleteSubnet(ctx, *subnetIPv6ID); client.IgnoreNotFoundError(err) != nil {
		return err
	}
	fctx.state.Set(IdentifierSubnetIPv6, "")
	return nil
}

func (fctx *FlowContext) recoverRouterID(ctx context.Context) error {
	if fctx.config.Networks.Router != nil {
		fctx.state.Set(IdentifierRouter, fctx.config.Networks.Router.ID)
		return nil
	}
	routerID := fctx.state.Get(IdentifierRouter)
	if routerID != nil {
		return nil
	}
	router, err := fctx.findExistingRouter(ctx)
	if err != nil {
		return err
	}
	if router != nil {
		fctx.state.Set(IdentifierRouter, router.ID)
	}
	return nil
}

func (fctx *FlowContext) recoverNetworkID(ctx context.Context) error {
	_, err := fctx.getNetworkID(ctx)
	return err
}

func (fctx *FlowContext) recoverSubnetID(ctx context.Context) error {
	if fctx.state.Get(IdentifierSubnet) != nil {
		return nil
	}

	subnet, err := fctx.findExistingSubnet(ctx)
	if err != nil {
		return err
	}
	if subnet != nil {
		fctx.state.Set(IdentifierSubnet, subnet.ID)
	}
	return nil
}

func (fctx *FlowContext) recoverSubnetIPv6ID(ctx context.Context) error {

	for _, suffix := range []string{"-ipv6-pod", "-ipv6-svc", "-ipv6"} {
		identifier := getSubnetIdentifierBySuffix(suffix)
		if fctx.state.Get(identifier) != nil {
			return nil
		}

		subnet, err := fctx.findExistingSubnetIPv6(ctx, fctx.defaultSubnetIPv6Name()+suffix)
		if err != nil {
			return err
		}
		if subnet != nil {
			fctx.state.Set(identifier, subnet.ID)
		}
	}
	return nil
}

func (fctx *FlowContext) deleteRouterInterface(ctx context.Context) error {
	routerID := fctx.state.Get(IdentifierRouter)
	if routerID == nil {
		return nil
	}
	subnetID := fctx.state.Get(IdentifierSubnet)
	if subnetID == nil {
		return nil
	}

	portID, err := fctx.access.GetRouterInterfacePortID(ctx, *routerID, *subnetID)
	if err != nil {
		return err
	}
	if portID == nil {
		return nil
	}

	log := shared.LogFromContext(ctx)
	log.Info("deleting...")
	err = fctx.access.RemoveRouterInterfaceAndWait(ctx, *routerID, *subnetID, *portID)
	if err != nil {
		return err
	}
	return nil
}

func (fctx *FlowContext) deleteRouterInterfaceIPv6(ctx context.Context) error {
	routerID := fctx.state.Get(IdentifierRouter)
	if routerID == nil {
		return nil
	}
	subnetIPv6ID := fctx.state.Get(IdentifierSubnetIPv6)
	if subnetIPv6ID == nil {
		return nil
	}

	portID, err := fctx.access.GetRouterInterfacePortID(ctx, *routerID, *subnetIPv6ID)
	if err != nil {
		return err
	}
	if portID == nil {
		return nil
	}

	log := shared.LogFromContext(ctx)
	log.Info("deleting IPv6 router interface...")
	err = fctx.access.RemoveRouterInterfaceAndWait(ctx, *routerID, *subnetIPv6ID, *portID)
	if err != nil {
		return err
	}
	return nil
}

func (fctx *FlowContext) deleteSecGroup(ctx context.Context) error {
	log := shared.LogFromContext(ctx)
	current, err := findExisting(ctx, fctx.state.Get(IdentifierSecGroup), fctx.defaultSecurityGroupName(), fctx.access.GetSecurityGroupByID, fctx.access.GetSecurityGroupByName)
	if err != nil {
		return err
	}
	if current != nil {
		log.Info("deleting...", "securityGroup", current.ID)
		if err := fctx.networking.DeleteSecurityGroup(ctx, current.ID); client.IgnoreNotFoundError(err) != nil {
			return err
		}
	}
	fctx.state.Set(NameSecGroup, "")
	fctx.state.SetObject(ObjectSecGroup, nil)
	return nil
}

func (fctx *FlowContext) deleteSSHKeyPair(ctx context.Context) error {
	log := shared.LogFromContext(ctx)
	current, err := fctx.compute.GetKeyPair(ctx, fctx.defaultSSHKeypairName())
	if err != nil {
		return err
	}
	if current != nil {
		log.Info("deleting...")
		if err := fctx.compute.DeleteKeyPair(ctx, current.Name); client.IgnoreNotFoundError(err) != nil {
			return err
		}
	}
	return nil
}

func (fctx *FlowContext) deleteShareNetwork(ctx context.Context) error {
	if sn := fctx.config.Networks.ShareNetwork; sn == nil || !sn.Enabled {
		return nil
	}

	sharedFilesystemClient, err := fctx.openstackClientFactory.SharedFilesystem(client.WithRegion(fctx.infra.Spec.Region))
	if err != nil {
		return err
	}

	log := shared.LogFromContext(ctx)
	networkID := ptr.Deref(fctx.state.Get(IdentifierNetwork), "")
	subnetID := ptr.Deref(fctx.state.Get(IdentifierSubnet), "")
	current, err := findExisting(ctx, fctx.state.Get(IdentifierShareNetwork),
		fctx.defaultSharedNetworkName(),
		sharedFilesystemClient.GetShareNetwork,
		func(ctx context.Context, name string) ([]*sharenetworks.ShareNetwork, error) {
			list, err := sharedFilesystemClient.ListShareNetworks(ctx, sharenetworks.ListOpts{
				Name:            name,
				NeutronNetID:    networkID,
				NeutronSubnetID: subnetID,
			})
			if err != nil {
				return nil, err
			}
			return sliceToPtr(list), nil
		})
	if err != nil {
		return err
	}
	if current != nil {
		log.Info("deleting...", "shareNetwork", current.ID)
		if err := sharedFilesystemClient.DeleteShareNetwork(ctx, current.ID); client.IgnoreNotFoundError(err) != nil {
			return err
		}
	}
	fctx.state.Set(IdentifierShareNetwork, "")
	fctx.state.Set(NameShareNetwork, "")
	return nil
}
