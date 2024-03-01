// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"fmt"

	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/external"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/layer3/floatingips"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/layer3/routers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/security/groups"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/security/rules"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/ports"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/subnets"
	"k8s.io/utils/pointer"
)

type networkWithExternalExt struct {
	networks.Network
	external.NetworkExternalExt
}

// GetExternalNetworkNames returns a list of all external network names.
func (c *NetworkingClient) GetExternalNetworkNames(_ context.Context) ([]string, error) {
	externalNetworks, err := c.listExternalNetworks(networks.ListOpts{})
	if err != nil {
		return nil, err
	}
	var externalNetworkNames []string
	for _, externalNetwork := range externalNetworks {
		externalNetworkNames = append(externalNetworkNames, externalNetwork.Name)
	}
	return externalNetworkNames, nil
}

// GetExternalNetworkNames returns a list of all external network names.
func (c *NetworkingClient) listExternalNetworks(listOpts networks.ListOptsBuilder) ([]networkWithExternalExt, error) {
	allPages, err := networks.List(c.client, external.ListOptsExt{
		ListOptsBuilder: listOpts,
		External:        pointer.Bool(true),
	}).AllPages()
	if err != nil {
		return nil, err
	}

	var externalNetworks []networkWithExternalExt
	err = networks.ExtractNetworksInto(allPages, &externalNetworks)
	if err != nil {
		return nil, err
	}
	return externalNetworks, nil
}

// GetExternalNetworkByName returns an external network by name
func (c *NetworkingClient) GetExternalNetworkByName(name string) (*networks.Network, error) {
	externalNetworks, err := c.listExternalNetworks(networks.ListOpts{Name: name})
	if err != nil {
		return nil, err
	}
	if len(externalNetworks) == 0 {
		return nil, nil
	}
	if len(externalNetworks) > 1 {
		return nil, fmt.Errorf("duplicate external network name: %s (%d)", name, len(externalNetworks))
	}
	return &externalNetworks[0].Network, nil
}

// ListNetwork returns a list of all network info by listOpts
func (c *NetworkingClient) ListNetwork(listOpts networks.ListOpts) ([]networks.Network, error) {
	pages, err := networks.List(c.client, listOpts).AllPages()
	if err != nil {
		return nil, err
	}
	return networks.ExtractNetworks(pages)
}

// UpdateNetwork updates settings of a network resource
func (c *NetworkingClient) UpdateNetwork(networkID string, opts networks.UpdateOpts) (*networks.Network, error) {
	return networks.Update(c.client, networkID, opts).Extract()
}

// GetNetworkByName return a network info by name
func (c *NetworkingClient) GetNetworkByName(name string) ([]networks.Network, error) {
	listOpts := networks.ListOpts{
		Name: name,
	}
	return c.ListNetwork(listOpts)
}

// CreateNetwork creates a network
func (c *NetworkingClient) CreateNetwork(opts networks.CreateOpts) (*networks.Network, error) {
	return networks.Create(c.client, opts).Extract()
}

// DeleteNetwork deletes a network
func (c *NetworkingClient) DeleteNetwork(networkID string) error {
	return networks.Delete(c.client, networkID).ExtractErr()
}

// CreateFloatingIP create floating ip
func (c *NetworkingClient) CreateFloatingIP(createOpts floatingips.CreateOpts) (*floatingips.FloatingIP, error) {
	return floatingips.Create(c.client, createOpts).Extract()
}

// ListFip returns a list of all network info
func (c *NetworkingClient) ListFip(listOpts floatingips.ListOpts) ([]floatingips.FloatingIP, error) {
	allPages, err := floatingips.List(c.client, listOpts).AllPages()
	if err != nil {
		return nil, err
	}
	return floatingips.ExtractFloatingIPs(allPages)
}

// GetFipByName returns floating IP info by floatingip name
func (c *NetworkingClient) GetFipByName(name string) ([]floatingips.FloatingIP, error) {
	listOpts := floatingips.ListOpts{
		Description: name,
	}
	return c.ListFip(listOpts)
}

// DeleteFloatingIP delete floatingip by floatingip id
func (c *NetworkingClient) DeleteFloatingIP(id string) error {
	return floatingips.Delete(c.client, id).ExtractErr()
}

// ListRules returns a list of security group rules
func (c *NetworkingClient) ListRules(listOpts rules.ListOpts) ([]rules.SecGroupRule, error) {
	allPages, err := rules.List(c.client, listOpts).AllPages()
	if err != nil {
		return nil, err
	}
	return rules.ExtractRules(allPages)
}

// CreateRule create security group rule
func (c *NetworkingClient) CreateRule(createOpts rules.CreateOpts) (*rules.SecGroupRule, error) {
	return rules.Create(c.client, createOpts).Extract()
}

// DeleteRule delete security group rule
func (c *NetworkingClient) DeleteRule(ruleID string) error {
	return rules.Delete(c.client, ruleID).ExtractErr()
}

// CreateSecurityGroup create a security group
func (c *NetworkingClient) CreateSecurityGroup(listOpts groups.CreateOpts) (*groups.SecGroup, error) {
	return groups.Create(c.client, listOpts).Extract()
}

// DeleteSecurityGroup delete a security group
func (c *NetworkingClient) DeleteSecurityGroup(groupID string) error {
	return groups.Delete(c.client, groupID).ExtractErr()
}

// ListSecurityGroup returns a list of security group
func (c *NetworkingClient) ListSecurityGroup(listOpts groups.ListOpts) ([]groups.SecGroup, error) {
	allPages, err := groups.List(c.client, listOpts).AllPages()
	if err != nil {
		return nil, err
	}
	return groups.ExtractGroups(allPages)
}

// GetSecurityGroupByName returns a security group info by security group name
func (c *NetworkingClient) GetSecurityGroupByName(name string) ([]groups.SecGroup, error) {
	listOpts := groups.ListOpts{
		Name: name,
	}
	return c.ListSecurityGroup(listOpts)
}

// GetRouterByID return a router info by name
func (c *NetworkingClient) GetRouterByID(id string) (*routers.Router, error) {
	router, err := routers.Get(c.client, id).Extract()
	return router, IgnoreNotFoundError(err)
}

// GetSecurityGroup returns a security group info by id
func (c *NetworkingClient) GetSecurityGroup(groupID string) (*groups.SecGroup, error) {
	return groups.Get(c.client, groupID).Extract()
}

// CreateRouter creates a router
func (c *NetworkingClient) CreateRouter(createOpts routers.CreateOpts) (*routers.Router, error) {
	return routers.Create(c.client, createOpts).Extract()
}

// ListRouters returns a list of routers
func (c *NetworkingClient) ListRouters(listOpts routers.ListOpts) ([]routers.Router, error) {
	allPages, err := routers.List(c.client, listOpts).AllPages()
	if err != nil {
		return nil, err
	}
	return routers.ExtractRouters(allPages)
}

// UpdateRoutesForRouter updates the route list for a router
func (c *NetworkingClient) UpdateRoutesForRouter(routes []routers.Route, routerID string) (*routers.Router, error) {

	updateOpts := routers.UpdateOpts{
		Routes: &routes,
	}
	return routers.Update(c.client, routerID, updateOpts).Extract()
}

// UpdateRouter updates router settings
func (c *NetworkingClient) UpdateRouter(routerID string, updateOpts routers.UpdateOpts) (*routers.Router, error) {
	return routers.Update(c.client, routerID, updateOpts).Extract()
}

// DeleteRouter deletes a router by identifier
func (c *NetworkingClient) DeleteRouter(routerID string) error {
	return routers.Delete(c.client, routerID).ExtractErr()
}

// AddRouterInterface adds a router interface
func (c *NetworkingClient) AddRouterInterface(routerID string, addOpts routers.AddInterfaceOpts) (*routers.InterfaceInfo, error) {
	return routers.AddInterface(c.client, routerID, addOpts).Extract()
}

// RemoveRouterInterface removes a router interface
func (c *NetworkingClient) RemoveRouterInterface(routerID string, removeOpts routers.RemoveInterfaceOpts) (*routers.InterfaceInfo, error) {
	return routers.RemoveInterface(c.client, routerID, removeOpts).Extract()
}

// CreateSubnet creates a subnet
func (c *NetworkingClient) CreateSubnet(createOpts subnets.CreateOpts) (*subnets.Subnet, error) {
	return subnets.Create(c.client, createOpts).Extract()
}

// ListSubnets returns a list of subnets
func (c *NetworkingClient) ListSubnets(listOpts subnets.ListOpts) ([]subnets.Subnet, error) {
	page, err := subnets.List(c.client, listOpts).AllPages()
	if err != nil {
		return nil, err
	}
	return subnets.ExtractSubnets(page)
}

// UpdateSubnet updates a subnet
func (c *NetworkingClient) UpdateSubnet(id string, updateOpts subnets.UpdateOpts) (*subnets.Subnet, error) {
	return subnets.Update(c.client, id, updateOpts).Extract()
}

// DeleteSubnet deletes a subnet by identifier
func (c *NetworkingClient) DeleteSubnet(subnetID string) error {
	return subnets.Delete(c.client, subnetID).ExtractErr()
}

// GetPort gets a port by identifier
func (c *NetworkingClient) GetPort(portID string) (*ports.Port, error) {
	return ports.Get(c.client, portID).Extract()
}

// GetRouterInterfacePort gets a port for a router interface
func (c *NetworkingClient) GetRouterInterfacePort(routerID, subnetID string) (*ports.Port, error) {
	page, err := ports.List(c.client, ports.ListOpts{
		DeviceOwner: "network:router_interface",
		DeviceID:    routerID,
		FixedIPs: []ports.FixedIPOpts{
			{SubnetID: subnetID},
		},
	}).AllPages()
	if err != nil {
		return nil, err
	}
	list, err := ports.ExtractPorts(page)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	return &list[0], nil
}
