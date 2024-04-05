// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	"context"

	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gophercloud/gophercloud/openstack/sharedfilesystems/v2/sharenetworks"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener-extension-provider-openstack/pkg/controller/infrastructure/infraflow/shared"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/internal/infrastructure"
)

// Delete creates and runs the flow to delete the AWS infrastructure.
func (c *FlowContext) Delete(ctx context.Context) error {
	if c.state.IsEmpty() {
		// nothing to do, e.g. if cluster was created with wrong credentials
		return nil
	}
	g := c.buildDeleteGraph()
	f := g.Compile()
	if err := f.Run(ctx, flow.Opts{Log: c.Log}); err != nil {
		return flow.Causes(err)
	}
	return nil
}

func (c *FlowContext) buildDeleteGraph() *flow.Graph {
	g := flow.NewGraph("Openstack infrastructure destruction")

	needToDeleteNetwork := c.config.Networks.ID == nil
	needToDeleteRouter := c.config.Networks.Router == nil

	_ = c.AddTask(g, "delete ssh key pair",
		c.deleteSSHKeyPair,
		Timeout(defaultTimeout))
	_ = c.AddTask(g, "delete security group",
		c.deleteSecGroup,
		Timeout(defaultTimeout))
	recoverRouterID := c.AddTask(g, "recover router ID",
		c.recoverRouterID,
		Timeout(defaultTimeout))
	recoverSubnetID := c.AddTask(g, "recover subnet ID",
		c.recoverSubnetID,
		Timeout(defaultTimeout))
	k8sRoutes := c.AddTask(g, "delete kubernetes routes",
		func(ctx context.Context) error {
			routerID := c.state.Get(IdentifierRouter)
			if routerID == nil {
				return nil
			}
			return infrastructure.CleanupKubernetesRoutes(ctx, c.networking, *routerID, infrastructure.WorkersCIDR(c.config))
		},
		Timeout(defaultTimeout),
	)
	k8sLoadBalancers := c.AddTask(g, "delete kubernetes loadbalancers",
		func(ctx context.Context) error {
			subnetID := c.state.Get(IdentifierSubnet)
			if subnetID == nil {
				return nil
			}
			return infrastructure.CleanupKubernetesLoadbalancers(ctx, c.LogFromContext(ctx), c.loadbalancing, *subnetID, c.namespace)
		},
		Timeout(defaultTimeout),
	)

	_ = c.AddTask(g, "delete share network",
		c.deleteShareNetwork,
		Timeout(defaultTimeout), Dependencies(recoverSubnetID))
	deleteRouterInterface := c.AddTask(g, "delete router interface",
		c.deleteRouterInterface,
		Timeout(defaultTimeout), Dependencies(recoverRouterID, recoverSubnetID, k8sRoutes))
	// subnet deletion only needed if network is given by spec
	_ = c.AddTask(g, "delete subnet",
		c.deleteSubnet,
		DoIf(!needToDeleteNetwork), Timeout(defaultTimeout), Dependencies(deleteRouterInterface, k8sLoadBalancers))
	_ = c.AddTask(g, "delete network",
		c.deleteNetwork,
		DoIf(needToDeleteNetwork), Timeout(defaultTimeout), Dependencies(deleteRouterInterface))
	_ = c.AddTask(g, "delete router",
		c.deleteRouter,
		DoIf(needToDeleteRouter), Timeout(defaultTimeout), Dependencies(deleteRouterInterface))

	return g
}

func (c *FlowContext) deleteRouter(ctx context.Context) error {
	log := c.LogFromContext(ctx)
	current, err := c.findExistingRouter()
	if err != nil {
		return err
	}
	if current != nil {
		log.Info("deleting...", "router", current.ID)
		if err := c.networking.DeleteRouter(current.ID); err != nil {
			return err
		}
	}
	return nil
}

func (c *FlowContext) deleteNetwork(ctx context.Context) error {
	log := c.LogFromContext(ctx)
	current, err := c.findExistingNetwork()
	if err != nil {
		return err
	}
	if current != nil {
		log.Info("deleting...", "network", current.ID)
		if err := c.networking.DeleteNetwork(current.ID); err != nil {
			return err
		}
	}
	c.state.Set(NameNetwork, "")
	return nil
}

func (c *FlowContext) deleteSubnet(ctx context.Context) error {
	log := c.LogFromContext(ctx)
	current, err := c.findExistingSubnet()
	if err != nil {
		return err
	}
	if current != nil {
		log.Info("deleting...", "subnet", current.ID)
		if err := c.networking.DeleteSubnet(current.ID); err != nil {
			return err
		}
	}
	return nil
}

func (c *FlowContext) recoverRouterID(_ context.Context) error {
	if c.config.Networks.Router != nil {
		c.state.Set(IdentifierRouter, c.config.Networks.Router.ID)
		return nil
	}
	routerID := c.state.Get(IdentifierRouter)
	if routerID != nil {
		return nil
	}
	router, err := c.findExistingRouter()
	if err != nil {
		return err
	}
	if router != nil {
		c.state.Set(IdentifierRouter, router.ID)
	}
	return nil
}

func (c *FlowContext) recoverSubnetID(_ context.Context) error {
	if c.state.Get(IdentifierSubnet) != nil {
		return nil
	}

	subnet, err := c.findExistingSubnet()
	if err != nil {
		return err
	}
	if subnet != nil {
		c.state.Set(IdentifierSubnet, subnet.ID)
	}
	return nil
}

func (c *FlowContext) deleteRouterInterface(ctx context.Context) error {
	routerID := c.state.Get(IdentifierRouter)
	if routerID == nil {
		return nil
	}
	subnetID := c.state.Get(IdentifierSubnet)
	if subnetID == nil {
		return nil
	}

	portID, err := c.access.GetRouterInterfacePortID(*routerID, *subnetID)
	if err != nil {
		return err
	}
	if portID == nil {
		return nil
	}

	log := c.LogFromContext(ctx)
	log.Info("deleting...")
	err = c.access.RemoveRouterInterfaceAndWait(ctx, *routerID, *subnetID, *portID)
	if err != nil {
		return err
	}
	return nil
}

func (c *FlowContext) deleteSecGroup(ctx context.Context) error {
	log := c.LogFromContext(ctx)
	current, err := findExisting(c.state.Get(IdentifierSecGroup), c.namespace, c.access.GetSecurityGroupByID, c.access.GetSecurityGroupByName)
	if err != nil {
		return err
	}
	if current != nil {
		log.Info("deleting...", "securityGroup", current.ID)
		if err := c.networking.DeleteSecurityGroup(current.ID); err != nil {
			return err
		}
	}
	c.state.Set(NameSecGroup, "")
	c.state.SetObject(ObjectSecGroup, nil)
	return nil
}

func (c *FlowContext) deleteSSHKeyPair(ctx context.Context) error {
	log := c.LogFromContext(ctx)
	current, err := c.compute.GetKeyPair(c.namespace)
	if err != nil {
		return err
	}
	if current != nil {
		log.Info("deleting...")
		if err := c.compute.DeleteKeyPair(current.Name); err != nil {
			return err
		}
	}
	return nil
}

func (c *FlowContext) deleteShareNetwork(ctx context.Context) error {
	log := c.LogFromContext(ctx)
	networkID := ptr.Deref(c.state.Get(IdentifierNetwork), "")
	subnetID := ptr.Deref(c.state.Get(IdentifierSubnet), "")
	current, err := findExisting(c.state.Get(IdentifierShareNetwork),
		c.namespace,
		c.sharedFilesystem.GetShareNetwork,
		func(name string) ([]*sharenetworks.ShareNetwork, error) {
			list, err := c.sharedFilesystem.ListShareNetworks(sharenetworks.ListOpts{
				AllTenants:      false,
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
		if err := c.sharedFilesystem.DeleteShareNetwork(current.ID); err != nil {
			return err
		}
	}
	c.state.Set(IdentifierShareNetwork, "")
	c.state.Set(NameShareNetwork, "")
	return nil
}
