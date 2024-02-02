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
	"github.com/gophercloud/gophercloud/openstack/sharedfilesystems/v2/sharenetworks"
	"k8s.io/utils/pointer"

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
	g := flow.NewGraph("Openstack infrastructure reconciliation")

	ensureExternalNetwork := c.AddTask(g, "ensure external network",
		c.ensureExternalNetwork,
		Timeout(defaultTimeout))

	ensureRouter := c.AddTask(g, "ensure router",
		c.ensureRouter,
		Timeout(defaultTimeout), Dependencies(ensureExternalNetwork))

	ensureNetwork := c.AddTask(g, "ensure network",
		c.ensureNetwork,
		Timeout(defaultTimeout))

	ensureSubnet := c.AddTask(g, "ensure subnet",
		c.ensureSubnet,
		Timeout(defaultTimeout), Dependencies(ensureNetwork))

	_ = c.AddTask(g, "ensure router interface",
		c.ensureRouterInterface,
		Timeout(defaultTimeout), Dependencies(ensureRouter, ensureSubnet))

	ensureSecGroup := c.AddTask(g, "ensure security group",
		c.ensureSecGroup,
		Timeout(defaultTimeout), Dependencies(ensureRouter))

	_ = c.AddTask(g, "ensure security group rules",
		c.ensureSecGroupRules,
		Timeout(defaultTimeout), Dependencies(ensureSecGroup))

	_ = c.AddTask(g, "ensure ssh key pair",
		c.ensureSSHKeyPair,
		Timeout(defaultTimeout), Dependencies(ensureRouter))

	_ = c.AddTask(g, "ensure share network",
		c.ensureShareNetwork,
		Timeout(defaultTimeout), Dependencies(ensureSubnet),
	)

	return g
}

func (c *FlowContext) ensureExternalNetwork(_ context.Context) error {
	externalNetwork, err := c.networking.GetExternalNetworkByName(c.config.FloatingPoolName)
	if err != nil {
		return err
	}
	if externalNetwork == nil {
		return fmt.Errorf("external network for floating pool name %s not found", c.config.FloatingPoolName)
	}
	c.state.Set(IdentifierFloatingNetwork, externalNetwork.ID)
	c.state.Set(NameFloatingNetwork, externalNetwork.Name)
	return nil
}

func (c *FlowContext) ensureRouter(ctx context.Context) error {
	externalNetworkID := c.state.Get(IdentifierFloatingNetwork)
	if externalNetworkID == nil {
		return fmt.Errorf("missing external network ID")
	}

	if c.config.Networks.Router != nil {
		return c.ensureConfiguredRouter(ctx)
	}
	return c.ensureNewRouter(ctx, *externalNetworkID)
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
		_, err := c.access.UpdateRouter(desired, current)
		return err
	}

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

	return nil
}

func (c *FlowContext) findExistingRouter() (*access.Router, error) {
	return findExisting(c.state.Get(IdentifierRouter), c.namespace, c.access.GetRouterByID, c.access.GetRouterByName)
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

func (c *FlowContext) ensureRouterInterface(ctx context.Context) error {
	log := c.LogFromContext(ctx)

	routerID := c.state.Get(IdentifierRouter)
	if routerID == nil {
		return fmt.Errorf("internal error: missing routerID")
	}
	subnetID := c.state.Get(IdentifierSubnet)
	if subnetID == nil {
		return fmt.Errorf("internal error: missing subnetID")
	}
	portID, err := c.access.GetRouterInterfacePortID(*routerID, *subnetID)
	if err != nil {
		return err
	}
	if portID != nil {
		return nil
	}
	log.Info("creating...")
	return c.access.AddRouterInterfaceAndWait(ctx, *routerID, *subnetID)
}

func (c *FlowContext) ensureSecGroup(ctx context.Context) error {
	log := c.LogFromContext(ctx)

	desired := &groups.SecGroup{
		Name:        c.namespace,
		Description: "Cluster Nodes",
	}
	current, err := findExisting(c.state.Get(IdentifierSecGroup), c.namespace, c.access.GetSecurityGroupByID, c.access.GetSecurityGroupByName)
	if err != nil {
		return err
	}
	if current != nil {
		c.state.Set(IdentifierSecGroup, current.ID)
		c.state.Set(NameSecGroup, current.Name)
		c.state.SetObject(ObjectSecGroup, current)
		return nil
	}

	log.Info("creating...")
	created, err := c.access.CreateSecurityGroup(desired)
	if err != nil {
		return err
	}
	c.state.Set(IdentifierSecGroup, created.ID)
	c.state.Set(NameSecGroup, created.Name)
	c.state.SetObject(ObjectSecGroup, created)
	return nil
}

func (c *FlowContext) ensureSecGroupRules(ctx context.Context) error {
	log := c.LogFromContext(ctx)

	obj := c.state.GetObject(ObjectSecGroup)
	if obj == nil {
		return fmt.Errorf("internal error: security group object not found")
	}
	group, ok := obj.(*groups.SecGroup)
	if !ok {
		return fmt.Errorf("internal error: casting to SecGroup failed")
	}

	desiredRules := []rules.SecGroupRule{
		{
			Direction:     string(rules.DirIngress),
			EtherType:     string(rules.EtherType4),
			RemoteGroupID: access.SecurityGroupIDSelf,
			Description:   "IPv4: allow all incoming traffic within the same security group",
		},
		{
			Direction:   string(rules.DirEgress),
			EtherType:   string(rules.EtherType4),
			Description: "IPv4: allow all outgoing traffic",
		},
		{
			Direction:   string(rules.DirEgress),
			EtherType:   string(rules.EtherType6),
			Description: "IPv6: allow all outgoing traffic",
		},
		{
			Direction:      string(rules.DirIngress),
			EtherType:      string(rules.EtherType4),
			Protocol:       string(rules.ProtocolTCP),
			PortRangeMin:   30000,
			PortRangeMax:   32767,
			RemoteIPPrefix: "0.0.0.0/0",
			Description:    "IPv4: allow all incoming tcp traffic with port range 30000-32767",
		},
		{
			Direction:      string(rules.DirIngress),
			EtherType:      string(rules.EtherType4),
			Protocol:       string(rules.ProtocolUDP),
			PortRangeMin:   30000,
			PortRangeMax:   32767,
			RemoteIPPrefix: "0.0.0.0/0",
			Description:    "IPv4: allow all incoming udp traffic with port range 30000-32767",
		},
	}

	if modified, err := c.access.UpdateSecurityGroupRules(group, desiredRules, func(rule *rules.SecGroupRule) bool {
		// Do NOT delete unknown rules to keep permissive behaviour as with terraform.
		// As we don't store the role ids in the state, this function needs to be adjusted
		// if values in existing rules are changed to identify them for update by replacement.
		return false
	}); err != nil {
		return err
	} else if modified {
		log.Info("updated rules")
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

func (c *FlowContext) ensureShareNetwork(ctx context.Context) error {
	log := c.LogFromContext(ctx)
	networkID := pointer.StringDeref(c.state.Get(IdentifierNetwork), "")
	subnetID := pointer.StringDeref(c.state.Get(IdentifierSubnet), "")
	current, err := findExisting(c.state.Get(IdentifierShareNetwork),
		c.namespace,
		noopFinder[sharenetworks.ShareNetwork],
		func(name string) ([]*sharenetworks.ShareNetwork, error) {
			list, err := c.sharedFilesystem.ListShareNetworks(sharenetworks.ListOpts{
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
		c.state.Set(IdentifierShareNetwork, current.ID)
		return nil
	}

	log.Info("creating...")
	created, err := c.sharedFilesystem.CreateShareNetwork(sharenetworks.CreateOpts{
		NeutronNetID:    networkID,
		NeutronSubnetID: subnetID,
		Name:            c.namespace,
	})
	if err != nil {
		return err
	}
	c.state.Set(IdentifierShareNetwork, created.ID)
	c.state.Set(NameShareNetwork, created.Name)
	return nil
}
