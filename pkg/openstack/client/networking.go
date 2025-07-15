// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"fmt"
	"slices"

	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/external"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/layer3/floatingips"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/layer3/routers"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/security/groups"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/security/rules"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/ports"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/subnets"
	"k8s.io/utils/ptr"
)

type networkWithExternalExt struct {
	networks.Network
	external.NetworkExternalExt
}

// GetExternalNetworkNames returns a list of all external network names.
func (c *NetworkingClient) GetExternalNetworkNames(ctx context.Context) ([]string, error) {
	externalNetworks, err := c.listExternalNetworks(ctx, networks.ListOpts{})
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
func (c *NetworkingClient) listExternalNetworks(ctx context.Context, listOpts networks.ListOptsBuilder) ([]networkWithExternalExt, error) {
	allPages, err := networks.List(c.client, external.ListOptsExt{
		ListOptsBuilder: listOpts,
		External:        ptr.To(true),
	}).AllPages(ctx)
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
func (c *NetworkingClient) GetExternalNetworkByName(ctx context.Context, name string) (*networks.Network, error) {
	externalNetworks, err := c.listExternalNetworks(ctx, networks.ListOpts{Name: name})
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
func (c *NetworkingClient) ListNetwork(ctx context.Context, listOpts networks.ListOpts) ([]networks.Network, error) {
	pages, err := networks.List(c.client, listOpts).AllPages(ctx)
	if err != nil {
		return nil, err
	}
	return networks.ExtractNetworks(pages)
}

// UpdateNetwork updates settings of a network resource
func (c *NetworkingClient) UpdateNetwork(ctx context.Context, networkID string, opts networks.UpdateOpts) (*networks.Network, error) {
	return networks.Update(ctx, c.client, networkID, opts).Extract()
}

// GetNetworkByName return a network info by name
func (c *NetworkingClient) GetNetworkByName(ctx context.Context, name string) ([]networks.Network, error) {
	listOpts := networks.ListOpts{
		Name: name,
	}
	return c.ListNetwork(ctx, listOpts)
}

// GetNetworkByID return a network info by id
func (c *NetworkingClient) GetNetworkByID(ctx context.Context, id string) (*networks.Network, error) {
	network, err := networks.Get(ctx, c.client, id).Extract()
	return network, IgnoreNotFoundError(err)
}

// CreateNetwork creates a network
func (c *NetworkingClient) CreateNetwork(ctx context.Context, opts networks.CreateOpts) (*networks.Network, error) {
	return networks.Create(ctx, c.client, opts).Extract()
}

// DeleteNetwork deletes a network
func (c *NetworkingClient) DeleteNetwork(ctx context.Context, networkID string) error {
	return networks.Delete(ctx, c.client, networkID).ExtractErr()
}

// CreateFloatingIP create floating ip
func (c *NetworkingClient) CreateFloatingIP(ctx context.Context, createOpts floatingips.CreateOpts) (*floatingips.FloatingIP, error) {
	return floatingips.Create(ctx, c.client, createOpts).Extract()
}

// ListFip returns a list of all network info
func (c *NetworkingClient) ListFip(ctx context.Context, listOpts floatingips.ListOpts) ([]floatingips.FloatingIP, error) {
	allPages, err := floatingips.List(c.client, listOpts).AllPages(ctx)
	if err != nil {
		return nil, err
	}
	return floatingips.ExtractFloatingIPs(allPages)
}

// GetFipByName returns floating IP info by floatingip name
func (c *NetworkingClient) GetFipByName(ctx context.Context, name string) ([]floatingips.FloatingIP, error) {
	listOpts := floatingips.ListOpts{
		Description: name,
	}
	return c.ListFip(ctx, listOpts)
}

// DeleteFloatingIP delete floatingip by floatingip id
func (c *NetworkingClient) DeleteFloatingIP(ctx context.Context, id string) error {
	return floatingips.Delete(ctx, c.client, id).ExtractErr()
}

// GetFloatingIP gets the Floating IP ID by listOpts.
func (c *NetworkingClient) GetFloatingIP(ctx context.Context, listOpts floatingips.ListOpts) (floatingips.FloatingIP, error) {
	allPages, err := floatingips.List(c.client, listOpts).AllPages(ctx)
	if err != nil {
		return floatingips.FloatingIP{}, err
	}

	allFloatingIPs, err := floatingips.ExtractFloatingIPs(allPages)
	if err != nil {
		return floatingips.FloatingIP{}, err
	}

	if len(allFloatingIPs) == 1 {
		return allFloatingIPs[0], nil
	}
	// we don't want to throw an error if the floating IP is not found
	return floatingips.FloatingIP{}, nil
}

// ListRules returns a list of security group rules
func (c *NetworkingClient) ListRules(ctx context.Context, listOpts rules.ListOpts) ([]rules.SecGroupRule, error) {
	allPages, err := rules.List(c.client, listOpts).AllPages(ctx)
	if err != nil {
		return nil, err
	}
	return rules.ExtractRules(allPages)
}

// CreateRule create security group rule
func (c *NetworkingClient) CreateRule(ctx context.Context, createOpts rules.CreateOpts) (*rules.SecGroupRule, error) {
	return rules.Create(ctx, c.client, createOpts).Extract()
}

// DeleteRule delete security group rule
func (c *NetworkingClient) DeleteRule(ctx context.Context, ruleID string) error {
	return groups.Delete(ctx, c.client, ruleID).ExtractErr()
}

// CreateSecurityGroup create a security group
func (c *NetworkingClient) CreateSecurityGroup(ctx context.Context, listOpts groups.CreateOpts) (*groups.SecGroup, error) {
	return groups.Create(ctx, c.client, listOpts).Extract()
}

// DeleteSecurityGroup delete a security group
func (c *NetworkingClient) DeleteSecurityGroup(ctx context.Context, groupID string) error {
	return groups.Delete(ctx, c.client, groupID).ExtractErr()
}

// ListSecurityGroup returns a list of security group
func (c *NetworkingClient) ListSecurityGroup(ctx context.Context, listOpts groups.ListOpts) ([]groups.SecGroup, error) {
	allPages, err := groups.List(c.client, listOpts).AllPages(ctx)
	if err != nil {
		return nil, err
	}
	return groups.ExtractGroups(allPages)
}

// GetSecurityGroupByName returns a security group info by security group name
func (c *NetworkingClient) GetSecurityGroupByName(ctx context.Context, name string) ([]groups.SecGroup, error) {
	listOpts := groups.ListOpts{
		Name: name,
	}
	return c.ListSecurityGroup(ctx, listOpts)
}

// GetRouterByID return a router info by name
func (c *NetworkingClient) GetRouterByID(ctx context.Context, id string) (*routers.Router, error) {
	router, err := routers.Get(ctx, c.client, id).Extract()
	return router, IgnoreNotFoundError(err)
}

// GetSecurityGroup returns a security group info by id
func (c *NetworkingClient) GetSecurityGroup(ctx context.Context, groupID string) (*groups.SecGroup, error) {
	return groups.Get(ctx, c.client, groupID).Extract()
}

// CreateRouter creates a router
func (c *NetworkingClient) CreateRouter(ctx context.Context, createOpts routers.CreateOpts) (*routers.Router, error) {
	return routers.Create(ctx, c.client, createOpts).Extract()
}

// ListRouters returns a list of routers
func (c *NetworkingClient) ListRouters(ctx context.Context, listOpts routers.ListOpts) ([]routers.Router, error) {
	allPages, err := routers.List(c.client, listOpts).AllPages(ctx)
	if err != nil {
		return nil, err
	}
	return routers.ExtractRouters(allPages)
}

// UpdateRoutesForRouter updates the route list for a router
func (c *NetworkingClient) UpdateRoutesForRouter(ctx context.Context, routes []routers.Route, routerID string) (*routers.Router, error) {
	updateOpts := routers.UpdateOpts{
		Routes: &routes,
	}
	return routers.Update(ctx, c.client, routerID, updateOpts).Extract()
}

// UpdateRouter updates router settings
func (c *NetworkingClient) UpdateRouter(ctx context.Context, routerID string, updateOpts routers.UpdateOpts) (*routers.Router, error) {
	return routers.Update(ctx, c.client, routerID, updateOpts).Extract()
}

// DeleteRouter deletes a router by identifier
func (c *NetworkingClient) DeleteRouter(ctx context.Context, routerID string) error {
	return routers.Delete(ctx, c.client, routerID).ExtractErr()
}

// AddRouterInterface adds a router interface
func (c *NetworkingClient) AddRouterInterface(ctx context.Context, routerID string, addOpts routers.AddInterfaceOpts) (*routers.InterfaceInfo, error) {
	return routers.AddInterface(ctx, c.client, routerID, addOpts).Extract()
}

// RemoveRouterInterface removes a router interface
func (c *NetworkingClient) RemoveRouterInterface(ctx context.Context, routerID string, removeOpts routers.RemoveInterfaceOpts) (*routers.InterfaceInfo, error) {
	return routers.RemoveInterface(ctx, c.client, routerID, removeOpts).Extract()
}

// CreateSubnet creates a subnet
func (c *NetworkingClient) CreateSubnet(ctx context.Context, createOpts subnets.CreateOpts) (*subnets.Subnet, error) {
	return subnets.Create(ctx, c.client, createOpts).Extract()
}

// GetSubnetByID return a subnet info by id
func (c *NetworkingClient) GetSubnetByID(ctx context.Context, id string) (*subnets.Subnet, error) {
	subnet, err := subnets.Get(ctx, c.client, id).Extract()
	return subnet, IgnoreNotFoundError(err)
}

// ListSubnets returns a list of subnets
func (c *NetworkingClient) ListSubnets(ctx context.Context, listOpts subnets.ListOpts) ([]subnets.Subnet, error) {
	page, err := subnets.List(c.client, listOpts).AllPages(ctx)
	if err != nil {
		return nil, err
	}
	return subnets.ExtractSubnets(page)
}

// UpdateSubnet updates a subnet
func (c *NetworkingClient) UpdateSubnet(ctx context.Context, id string, updateOpts subnets.UpdateOpts) (*subnets.Subnet, error) {
	return subnets.Update(ctx, c.client, id, updateOpts).Extract()
}

// DeleteSubnet deletes a subnet by identifier
func (c *NetworkingClient) DeleteSubnet(ctx context.Context, subnetID string) error {
	return subnets.Delete(ctx, c.client, subnetID).ExtractErr()
}

// GetPort gets a port by identifier
func (c *NetworkingClient) GetPort(ctx context.Context, portID string) (*ports.Port, error) {
	return ports.Get(ctx, c.client, portID).Extract()
}

// GetRouterInterfacePort gets a port for a router interface
func (c *NetworkingClient) GetRouterInterfacePort(ctx context.Context, routerID, subnetID string) (*ports.Port, error) {
	page, err := ports.List(c.client, ports.ListOpts{
		DeviceID: routerID,
		FixedIPs: []ports.FixedIPOpts{
			{SubnetID: subnetID},
		},
	}).AllPages(ctx)
	if err != nil {
		return nil, err
	}
	list, err := ports.ExtractPorts(page)
	if err != nil {
		return nil, err
	}

	validDeviceOwners := []string{
		"network:router_interface",
		"network:router_interface_distributed",
		"network:ha_router_replicated_interface",
	}
	filtered := slices.DeleteFunc(list, func(p ports.Port) bool {
		return !slices.Contains(validDeviceOwners, p.DeviceOwner)
	})

	if len(filtered) == 0 {
		return nil, nil
	}
	return &filtered[0], nil
}

// GetInstancePorts retrieves the ports of the instance.
func (c *NetworkingClient) GetInstancePorts(ctx context.Context, instanceID string) ([]ports.Port, error) {
	portListOpts := ports.ListOpts{
		DeviceID: instanceID,
	}
	allPorts, err := ports.List(c.client, portListOpts).AllPages(ctx)
	if err != nil {
		return nil, err
	}

	return ports.ExtractPorts(allPorts)
}

// UpdateFIPWithPort updates a Floating IP by adding a port.
func (c *NetworkingClient) UpdateFIPWithPort(ctx context.Context, fipID, portID string) error {
	updateOpts := floatingips.UpdateOpts{
		PortID: &portID,
	}
	_, err := floatingips.Update(ctx, c.client, fipID, updateOpts).Extract()
	return err
}
