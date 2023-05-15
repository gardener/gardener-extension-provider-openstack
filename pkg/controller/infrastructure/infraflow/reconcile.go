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
	"fmt"
	"time"

	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/security/groups"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/security/rules"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/subnets"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/controller/infrastructure/infraflow/access"
	. "github.com/gardener/gardener-extension-provider-openstack/pkg/controller/infrastructure/infraflow/shared"
)

const (
	defaultTimeout     = 90 * time.Second
	defaultLongTimeout = 3 * time.Minute
)

// Reconcile creates and runs the flow to reconcile the AWS infrastructure.
func (c *FlowContext) Reconcile(ctx context.Context) error {
	g := c.buildReconcileGraph()
	f := g.Compile()
	if err := f.Run(ctx, flow.Opts{Log: c.Log}); err != nil {
		return flow.Causes(err)
	}
	return nil
}

func (c *FlowContext) buildReconcileGraph() *flow.Graph {
	//createNetwork := c.config.Networks.ID == nil
	g := flow.NewGraph("Openstack infrastructure reconcilation")

	ensureRouter := c.AddTask(g, "ensure router",
		c.ensureRouter,
		Timeout(defaultTimeout))

	ensureNetwork := c.AddTask(g, "ensure network",
		c.ensureNetwork,
		Timeout(defaultTimeout))

	ensureSubnet := c.AddTask(g, "ensure subnet",
		c.ensureSubnet,
		Timeout(defaultTimeout), Dependencies(ensureNetwork))

	_ = c.AddTask(g, "ensure router interface",
		c.ensureRouterInterface,
		Timeout(defaultTimeout), Dependencies(ensureRouter, ensureSubnet))

	_ = c.AddTask(g, "ensure security group",
		c.ensureSecGroup,
		Timeout(defaultTimeout), Dependencies(ensureRouter))

	_ = c.AddTask(g, "ensure ssh key pair",
		c.ensureSSHKeyPair,
		Timeout(defaultTimeout), Dependencies(ensureRouter))

	return g
}

func (c *FlowContext) ensureRouter(ctx context.Context) error {
	externalNetwork, err := c.networking.GetExternalNetworkByName(c.config.FloatingPoolName)
	if err != nil {
		return err
	}
	if externalNetwork == nil {
		return fmt.Errorf("external network for floating pool name %s not found", c.config.FloatingPoolName)
	}
	c.state.Set(IdentifierFloatingNetwork, externalNetwork.ID)
	c.state.Set(NameFloatingNetwork, externalNetwork.Name)

	if c.config.Networks.Router != nil {
		return c.ensureConfiguredRouter(ctx)
	}
	return c.ensureNewRouter(ctx, externalNetwork.ID)
}

func (c *FlowContext) ensureConfiguredRouter(_ context.Context) error {
	router, err := c.access.GetRouterByID(c.config.Networks.Router.ID)
	if err != nil {
		c.state.Set(IdentifierRouter, "")
		return err
	}
	c.state.Set(IdentifierRouter, c.config.Networks.Router.ID)
	c.state.Set(RouterIP, router.ExternalFixedIPs[0].IPAddress)
	return nil
}

func (c *FlowContext) ensureNewRouter(ctx context.Context, externalNetworkID string) error {
	log := c.LogFromContext(ctx)

	desired := &access.Router{
		Name:              c.namespace,
		ExternalNetworkID: externalNetworkID,
		EnableSNAT:        c.cloudProfileConfig.UseSNAT,
	}
	current, err := c.findExistingRouter()
	if err != nil {
		return err
	}
	if current != nil {
		c.state.Set(IdentifierRouter, current.ID)
		c.state.Set(RouterIP, current.ExternalFixedIPs[0].IPAddress)
		if _, err := c.access.UpdateRouter(desired, current); err != nil {
			return err
		}
	} else {
		floatingPoolSubnetName := c.findFloatingPoolSubnetName()
		c.state.SetPtr(NameFloatingPoolSubnet, floatingPoolSubnetName)
		if floatingPoolSubnetName != nil {
			log.Info("looking up floating pool subnets...")
			desired.ExternalSubnetIDs, err = c.access.LookupFloatingPoolSubnetIDs(externalNetworkID, *floatingPoolSubnetName)
			if err != nil {
				return err
			}
		}
		log.Info("creating...")
		created, err := c.access.CreateRouter(desired)
		if err != nil {
			return err
		}
		c.state.Set(IdentifierRouter, created.ID)
		c.state.Set(RouterIP, created.ExternalFixedIPs[0].IPAddress)
	}

	return nil
}

func (c *FlowContext) findExistingRouter() (*access.Router, error) {
	return findExisting(c.state.Get(IdentifierRouter), c.namespace, c.access.GetRouterByID, c.access.GetRouterByName)
}

func (c *FlowContext) getRouterID() (*string, error) {
	if c.config.Networks.Router != nil {
		return &c.config.Networks.Router.ID, nil
	}
	routerID := c.state.Get(IdentifierRouter)
	if routerID != nil {
		return routerID, nil
	}
	router, err := c.findExistingRouter()
	if err != nil {
		return nil, err
	}
	if router != nil {
		c.state.Set(IdentifierRouter, router.ID)
		c.state.Set(RouterIP, router.ExternalFixedIPs[0].IPAddress)
		return &router.ID, nil
	}
	return nil, nil
}

func (c *FlowContext) findFloatingPoolSubnetName() *string {
	if c.config.FloatingPoolSubnetName != nil {
		return c.config.FloatingPoolSubnetName
	}

	// Second: Check if the CloudProfile contains a default floating subnet and use it.
	if floatingPool, err := helper.FindFloatingPool(c.cloudProfileConfig.Constraints.FloatingPools, c.config.FloatingPoolName, c.infraSpec.Region, nil); err == nil && floatingPool.DefaultFloatingSubnet != nil {
		return floatingPool.DefaultFloatingSubnet
	}

	return nil
}

func (c *FlowContext) ensureNetwork(ctx context.Context) error {
	if c.config.Networks.ID != nil {
		return c.ensureConfiguredNetwork(ctx)
	}
	return c.ensureNewNetwork(ctx)
}

func (c *FlowContext) ensureConfiguredNetwork(_ context.Context) error {
	network, err := c.access.GetNetworkByID(*c.config.Networks.ID)
	if err != nil {
		c.state.Set(IdentifierNetwork, "")
		c.state.Set(NameNetwork, "")
		return err
	}
	c.state.Set(IdentifierNetwork, *c.config.Networks.ID)
	c.state.Set(NameNetwork, network.Name)
	return nil
}

func (c *FlowContext) ensureNewNetwork(ctx context.Context) error {
	log := c.LogFromContext(ctx)

	desired := &access.Network{
		Name:         c.namespace,
		AdminStateUp: true,
	}
	current, err := c.findExistingNetwork()
	if err != nil {
		return err
	}
	if current != nil {
		c.state.Set(IdentifierNetwork, current.ID)
		c.state.Set(NameNetwork, current.Name)
		if _, err := c.access.UpdateNetwork(desired, current); err != nil {
			return err
		}
	} else {
		log.Info("creating...")
		created, err := c.access.CreateNetwork(desired)
		if err != nil {
			return err
		}
		c.state.Set(IdentifierNetwork, created.ID)
		c.state.Set(NameNetwork, created.Name)
	}

	return nil
}

func (c *FlowContext) findExistingNetwork() (*access.Network, error) {
	return findExisting(c.state.Get(IdentifierNetwork), c.namespace, c.access.GetNetworkByID, c.access.GetNetworkByName)
}

func (c *FlowContext) getNetworkID() (*string, error) {
	if c.config.Networks.ID != nil {
		return c.config.Networks.ID, nil
	}
	networkID := c.state.Get(IdentifierNetwork)
	if networkID != nil {
		return networkID, nil
	}
	network, err := c.findExistingNetwork()
	if err != nil {
		return nil, err
	}
	if network != nil {
		c.state.Set(IdentifierNetwork, network.ID)
		return &network.ID, nil
	}
	return nil, nil
}

func (c *FlowContext) ensureSubnet(ctx context.Context) error {
	log := c.LogFromContext(ctx)

	networkID := *c.state.Get(IdentifierNetwork)
	workersCIDR := c.config.Networks.Workers
	// Backwards compatibility - remove this code in a future version.
	if workersCIDR == "" {
		workersCIDR = c.config.Networks.Worker
	}
	desired := &subnets.Subnet{
		Name:           c.namespace,
		NetworkID:      networkID,
		CIDR:           workersCIDR,
		IPVersion:      4,
		DNSNameservers: c.cloudProfileConfig.DNSServers,
	}
	current, err := c.findExistingSubnet()
	if err != nil {
		return err
	}
	if current != nil {
		c.state.Set(IdentifierSubnet, current.ID)
		if _, err := c.access.UpdateSubnet(desired, current); err != nil {
			return err
		}
	} else {
		log.Info("creating...")
		created, err := c.access.CreateSubnet(desired)
		if err != nil {
			return err
		}
		c.state.Set(IdentifierSubnet, created.ID)
	}
	return nil
}

func (c *FlowContext) findExistingSubnet() (*subnets.Subnet, error) {
	networkID, err := c.getNetworkID()
	if err != nil {
		return nil, err
	}
	if networkID == nil {
		return nil, fmt.Errorf("network not found")
	}
	getByName := func(name string) ([]*subnets.Subnet, error) {
		return c.access.GetSubnetByName(*networkID, name)
	}
	return findExisting(c.state.Get(IdentifierSubnet), c.namespace, c.access.GetSubnetByID, getByName)
}

func (c *FlowContext) getSubnetID() (*string, error) {
	subnetID := c.state.Get(IdentifierSubnet)
	if subnetID != nil {
		return subnetID, nil
	}
	subnet, err := c.findExistingSubnet()
	if err != nil {
		return nil, err
	}
	if subnet != nil {
		c.state.Set(IdentifierSubnet, subnet.ID)
		return &subnet.ID, nil
	}
	return nil, nil
}

type notFoundError struct {
	msg string
}

var _ error = &notFoundError{}

func (e *notFoundError) Error() string {
	return e.msg
}

func ignoreNotFound(err error) error {
	if _, ok := err.(*notFoundError); ok {
		return nil
	}
	return err
}

func (c *FlowContext) getExistingRouterAndSubnetIDs() (routerID, subnetID string, err error) {
	prouter, err := c.getRouterID()
	if err != nil {
		return
	}
	if prouter == nil {
		err = &notFoundError{msg: "router not found"}
		return
	}
	routerID = *prouter
	psubnet, err := c.getSubnetID()
	if err != nil {
		return
	}
	if psubnet == nil {
		err = &notFoundError{msg: "subnet not found"}
		return
	}
	subnetID = *psubnet
	return
}

func (c *FlowContext) ensureRouterInterface(ctx context.Context) error {
	log := c.LogFromContext(ctx)

	routerID, subnetID, err := c.getExistingRouterAndSubnetIDs()
	if err != nil {
		return err
	}
	portID, actualSubnetID, err := c.access.GetRouterInterfacePortID(routerID)
	if err != nil {
		return err
	}
	if portID != nil {
		if actualSubnetID != nil && *actualSubnetID == subnetID {
			return nil
		}
		log.Info("router interface mismatch, deleting...", "port", *portID)
		err = c.access.RemoveRouterInterfaceAndWait(ctx, routerID, "", *portID)
		if err != nil {
			return err
		}
	}
	log.Info("creating...")
	return c.access.AddRouterInterfaceAndWait(ctx, routerID, subnetID)
}

func (c *FlowContext) ensureSecGroup(ctx context.Context) error {
	log := c.LogFromContext(ctx)

	desired := &groups.SecGroup{
		Name:        c.namespace,
		Description: "Cluster Nodes",
		Rules: []rules.SecGroupRule{
			{
				Direction:     string(rules.DirIngress),
				EtherType:     string(rules.EtherType4),
				RemoteGroupID: access.SecurityGroupIDSelf,
			},
			{
				Direction: string(rules.DirEgress),
				EtherType: string(rules.EtherType4),
			},
			{
				Direction:      string(rules.DirIngress),
				EtherType:      string(rules.EtherType4),
				Protocol:       string(rules.ProtocolTCP),
				RemoteIPPrefix: "0.0.0.0/0",
			},
			{
				Direction:      string(rules.DirIngress),
				EtherType:      string(rules.EtherType4),
				Protocol:       string(rules.ProtocolUDP),
				RemoteIPPrefix: "0.0.0.0/0",
			},
		},
	}
	current, err := findExisting(c.state.Get(IdentifierSecGroup), c.namespace, c.access.GetSecurityGroupByID, c.access.GetSecurityGroupByName)
	if err != nil {
		return err
	}
	if current != nil {
		c.state.Set(IdentifierSecGroup, current.ID)
		c.state.Set(NameSecGroup, current.Name)
		if modified, err := c.access.UpdateSecurityGroup(desired, current); err != nil {
			return err
		} else if modified {
			log.Info("updated rules")
		}
	} else {
		log.Info("creating...")
		created, err := c.access.CreateSecurityGroup(desired)
		if err != nil {
			return err
		}
		c.state.Set(IdentifierSecGroup, created.ID)
		c.state.Set(NameSecGroup, created.Name)
	}
	return nil
}

func (c *FlowContext) ensureSSHKeyPair(ctx context.Context) error {
	log := c.LogFromContext(ctx)

	keyPair, err := c.compute.GetKeyPair(c.namespace)
	if err != nil {
		return err
	}
	if keyPair != nil {
		if keyPair.PublicKey == string(c.infraSpec.SSHPublicKey) {
			c.state.Set(NameKeyPair, keyPair.Name)
			return nil
		}
		log.Info("replacing SSH key pair")
		if err := c.compute.DeleteKeyPair(c.namespace); err != nil {
			return err
		}
		keyPair = nil
	}

	if keyPair == nil {
		c.state.Set(NameKeyPair, "")
		log.Info("creating SSH key pair")
		if keyPair, err = c.compute.CreateKeyPair(c.namespace, string(c.infraSpec.SSHPublicKey)); err != nil {
			return err
		}
	}
	c.state.Set(NameKeyPair, keyPair.Name)
	return nil
}
