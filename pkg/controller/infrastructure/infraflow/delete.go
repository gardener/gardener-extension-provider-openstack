// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	"context"

	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gophercloud/gophercloud/openstack/sharedfilesystems/v2/sharenetworks"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/controller/infrastructure/infraflow/shared"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/internal/infrastructure"
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
	// skip deletion of the subnet if we need to delete the network (it will be deleted anyway), or if the subnet is user provided.
	needToDeleteSubnet := fctx.config.Networks.SubnetID == nil

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

	recoverIDs := flow.NewTaskIDs(recoverNetworkID, recoverRouterID, recoverSubnetID)
	k8sRoutes := fctx.AddTask(g, "delete kubernetes routes",
		func(ctx context.Context) error {
			routerID := fctx.state.Get(IdentifierRouter)
			if routerID == nil {
				return nil
			}
			subnetID := fctx.state.Get(IdentifierSubnet)
			if subnetID == nil {
				return nil
			}
			return infrastructure.CleanupKubernetesRoutes(ctx, fctx.networking, *routerID, *subnetID)
		},
		shared.DoIf(needToDeleteRouter || needToDeleteSubnet),
		shared.Timeout(defaultTimeout),
		shared.Dependencies(recoverIDs),
	)
	k8sLoadBalancers := fctx.AddTask(g, "delete kubernetes loadbalancers",
		func(ctx context.Context) error {
			subnetID := fctx.state.Get(IdentifierSubnet)
			if subnetID == nil {
				return nil
			}
			return infrastructure.CleanupKubernetesLoadbalancers(ctx, shared.LogFromContext(ctx), fctx.loadbalancing, *subnetID, fctx.infra.Namespace)
		},
		shared.Timeout(defaultTimeout),
		shared.Dependencies(recoverIDs),
	)

	_ = fctx.AddTask(g, "delete share network",
		fctx.deleteShareNetwork,
		shared.Timeout(defaultTimeout), shared.Dependencies(recoverIDs))
	deleteRouterInterface := fctx.AddTask(g, "delete router interface",
		fctx.deleteRouterInterface,
		shared.DoIf(needToDeleteSubnet || needToDeleteRouter),
		shared.Timeout(defaultTimeout), shared.Dependencies(recoverIDs, k8sRoutes))
	// subnet deletion only needed if network is given by spec
	_ = fctx.AddTask(g, "delete subnet",
		fctx.deleteSubnet,
		shared.DoIf(!needToDeleteNetwork && needToDeleteSubnet), shared.Timeout(defaultTimeout), shared.Dependencies(deleteRouterInterface, k8sLoadBalancers))
	_ = fctx.AddTask(g, "delete network",
		fctx.deleteNetwork,
		shared.DoIf(needToDeleteNetwork), shared.Timeout(defaultTimeout), shared.Dependencies(deleteRouterInterface))
	_ = fctx.AddTask(g, "delete router",
		fctx.deleteRouter,
		shared.DoIf(needToDeleteRouter), shared.Timeout(defaultTimeout), shared.Dependencies(deleteRouterInterface))
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
	if err := fctx.networking.DeleteRouter(*routerID); client.IgnoreNotFoundError(err) != nil {
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
	if err := fctx.networking.DeleteNetwork(*networkID); client.IgnoreNotFoundError(err) != nil {
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
	if err := fctx.networking.DeleteSubnet(*subnetID); client.IgnoreNotFoundError(err) != nil {
		return err
	}
	fctx.state.Set(IdentifierSubnet, "")
	return nil
}

func (fctx *FlowContext) recoverRouterID(_ context.Context) error {
	if fctx.config.Networks.Router != nil {
		fctx.state.Set(IdentifierRouter, fctx.config.Networks.Router.ID)
		return nil
	}
	routerID := fctx.state.Get(IdentifierRouter)
	if routerID != nil {
		return nil
	}
	router, err := fctx.findExistingRouter()
	if err != nil {
		return err
	}
	if router != nil {
		fctx.state.Set(IdentifierRouter, router.ID)
	}
	return nil
}

func (fctx *FlowContext) recoverNetworkID(_ context.Context) error {
	_, err := fctx.getNetworkID()
	return err
}

func (fctx *FlowContext) recoverSubnetID(_ context.Context) error {
	if fctx.state.Get(IdentifierSubnet) != nil {
		return nil
	}

	subnet, err := fctx.findExistingSubnet()
	if err != nil {
		return err
	}
	if subnet != nil {
		fctx.state.Set(IdentifierSubnet, subnet.ID)
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

	portID, err := fctx.access.GetRouterInterfacePortID(*routerID, *subnetID)
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

func (fctx *FlowContext) deleteSecGroup(ctx context.Context) error {
	log := shared.LogFromContext(ctx)
	current, err := findExisting(fctx.state.Get(IdentifierSecGroup), fctx.defaultSecurityGroupName(), fctx.access.GetSecurityGroupByID, fctx.access.GetSecurityGroupByName)
	if err != nil {
		return err
	}
	if current != nil {
		log.Info("deleting...", "securityGroup", current.ID)
		if err := fctx.networking.DeleteSecurityGroup(current.ID); client.IgnoreNotFoundError(err) != nil {
			return err
		}
	}
	fctx.state.Set(NameSecGroup, "")
	fctx.state.SetObject(ObjectSecGroup, nil)
	return nil
}

func (fctx *FlowContext) deleteSSHKeyPair(ctx context.Context) error {
	log := shared.LogFromContext(ctx)
	current, err := fctx.compute.GetKeyPair(fctx.defaultSSHKeypairName())
	if err != nil {
		return err
	}
	if current != nil {
		log.Info("deleting...")
		if err := fctx.compute.DeleteKeyPair(current.Name); client.IgnoreNotFoundError(err) != nil {
			return err
		}
	}
	return nil
}

func (fctx *FlowContext) deleteShareNetwork(ctx context.Context) error {
	if sn := fctx.config.Networks.ShareNetwork; sn == nil || !sn.Enabled {
		return nil
	}

	log := shared.LogFromContext(ctx)
	networkID := ptr.Deref(fctx.state.Get(IdentifierNetwork), "")
	subnetID := ptr.Deref(fctx.state.Get(IdentifierSubnet), "")
	current, err := findExisting(fctx.state.Get(IdentifierShareNetwork),
		fctx.defaultSharedNetworkName(),
		fctx.sharedFilesystem.GetShareNetwork,
		func(name string) ([]*sharenetworks.ShareNetwork, error) {
			list, err := fctx.sharedFilesystem.ListShareNetworks(sharenetworks.ListOpts{
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
		if err := fctx.sharedFilesystem.DeleteShareNetwork(current.ID); client.IgnoreNotFoundError(err) != nil {
			return err
		}
	}
	fctx.state.Set(IdentifierShareNetwork, "")
	fctx.state.Set(NameShareNetwork, "")
	return nil
}
