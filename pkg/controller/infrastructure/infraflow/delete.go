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

package infraflow

import (
	"context"

	"github.com/gardener/gardener/pkg/utils/flow"

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
