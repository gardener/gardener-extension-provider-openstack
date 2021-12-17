// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package client

import (
	"context"

	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/external"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/layer3/floatingips"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/security/groups"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/security/rules"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"k8s.io/utils/pointer"
)

type networkWithExternalExt struct {
	networks.Network
	external.NetworkExternalExt
}

// GetExternalNetworkNames returns a list of all external network names.
func (c *NetworkingClient) GetExternalNetworkNames(_ context.Context) ([]string, error) {
	allPages, err := networks.List(c.client, external.ListOptsExt{
		ListOptsBuilder: networks.ListOpts{},
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

	var externalNetworkNames []string
	for _, externalNetwork := range externalNetworks {
		externalNetworkNames = append(externalNetworkNames, externalNetwork.Name)
	}
	return externalNetworkNames, nil
}

// ListNetwork returns a list of all network info by listOpts
func (c *NetworkingClient) ListNetwork(listOpts networks.ListOpts) ([]networks.Network, error) {
	pages, err := networks.List(c.client, listOpts).AllPages()
	if err != nil {
		return nil, err
	}
	return networks.ExtractNetworks(pages)
}

// GetNetworkByName return a network info by name
func (c *NetworkingClient) GetNetworkByName(name string) ([]networks.Network, error) {
	listOpts := networks.ListOpts{
		Name: name,
	}
	return c.ListNetwork(listOpts)
}

// GetExternalNetworkInfoByName return external network info by name
func (c *NetworkingClient) GetExternalNetworkInfoByName(name string) ([]networks.Network, error) {
	allPages, err := networks.List(c.client, external.ListOptsExt{
		ListOptsBuilder: networks.ListOpts{Name: name},
		External:        pointer.Bool(true),
	}).AllPages()
	if err != nil {
		return nil, err
	}

	return networks.ExtractNetworks(allPages)
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

// GetSecurityGroupbyName returns a security group info by security group name
func (c *NetworkingClient) GetSecurityGroupbyName(name string) ([]groups.SecGroup, error) {
	listOpts := groups.ListOpts{
		Name: name,
	}
	return c.ListSecurityGroup(listOpts)
}
