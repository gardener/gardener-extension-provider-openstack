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

package access

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"time"

	"github.com/go-logr/logr"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/layer3/routers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/security/groups"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/security/rules"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/subnets"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
)

// NetworkingAccess provides methods for managing routers and networks
type NetworkingAccess interface {
	// Routers
	CreateRouter(desired *Router) (*Router, error)
	GetRouterByID(id string) (*Router, error)
	GetRouterByName(name string) ([]*Router, error)
	UpdateRouter(desired, current *Router) (modified bool, err error)
	LookupFloatingPoolSubnetIDs(networkID, floatingPoolSubnetNameRegex string) ([]string, error)
	AddRouterInterfaceAndWait(ctx context.Context, routerID, subnetID string) error
	GetRouterInterfacePortID(routerID, subnetID string) (portID *string, err error)
	RemoveRouterInterfaceAndWait(ctx context.Context, routerID, subnetID, portID string) error

	// Networks
	CreateNetwork(desired *Network) (*Network, error)
	GetNetworkByID(id string) (*Network, error)
	GetNetworkByName(name string) ([]*Network, error)
	UpdateNetwork(desired, current *Network) (modified bool, err error)

	// Subnets
	CreateSubnet(desired *subnets.Subnet) (*subnets.Subnet, error)
	GetSubnetByID(id string) (*subnets.Subnet, error)
	GetSubnetByName(networkID, name string) ([]*subnets.Subnet, error)
	UpdateSubnet(desired, current *subnets.Subnet) (modified bool, err error)

	// SecurityGroups
	CreateSecurityGroup(desired *groups.SecGroup) (*groups.SecGroup, error)
	GetSecurityGroupByID(id string) (*groups.SecGroup, error)
	GetSecurityGroupByName(name string) ([]*groups.SecGroup, error)
	UpdateSecurityGroupRules(group *groups.SecGroup, desiredRules []rules.SecGroupRule, allowDelete func(rule *rules.SecGroupRule) bool) (modified bool, err error)
}

// Router is a simplified router resource
type Router struct {
	ID                string
	Name              string
	ExternalNetworkID string
	EnableSNAT        *bool
	ExternalSubnetIDs []string

	Status           string                    // only output
	ExternalFixedIPs []routers.ExternalFixedIP // only output
}

// Network is a simplified network resource
type Network struct {
	ID           string
	Name         string
	AdminStateUp bool

	Status string
}

const (
	// SecurityGroupIDSelf special placeholder for self secgroup ID
	SecurityGroupIDSelf = "self"
)

type networkingAccess struct {
	networking client.Networking
	log        logr.Logger
}

var _ NetworkingAccess = &networkingAccess{}

// NewNetworkingAccess creates a new access object
func NewNetworkingAccess(networking client.Networking, log logr.Logger) (NetworkingAccess, error) {
	return &networkingAccess{
		networking: networking,
		log:        log,
	}, nil
}

// CreateRouter creates a router.
// If the input router object specifies external subnet ids, the router is created in the
// first available subnet.
func (a *networkingAccess) CreateRouter(desired *Router) (router *Router, err error) {
	if len(desired.ExternalSubnetIDs) == 0 {
		return a.tryCreateRouter(desired, nil)
	}
	// create router in first available subnet
	for _, subnetID := range desired.ExternalSubnetIDs {
		router, err = a.tryCreateRouter(desired, &subnetID)
		if err != nil && !retryOnError(a.log, err) {
			return
		}
		if err == nil {
			break
		}
	}
	return
}

func (a *networkingAccess) tryCreateRouter(desired *Router, subnetID *string) (*Router, error) {
	options := routers.CreateOpts{
		Name: desired.Name,
		GatewayInfo: &routers.GatewayInfo{
			NetworkID:  desired.ExternalNetworkID,
			EnableSNAT: desired.EnableSNAT,
		},
	}
	if subnetID != nil {
		options.GatewayInfo.ExternalFixedIPs = []routers.ExternalFixedIP{{SubnetID: *subnetID}}
	}
	raw, err := a.networking.CreateRouter(options)
	if err != nil {
		return nil, err
	}
	return a.toRouter(raw), nil
}

// GetRouterByID retrieves router by identifier
func (a *networkingAccess) GetRouterByID(id string) (*Router, error) {
	routers, err := a.networking.ListRouters(routers.ListOpts{ID: id})
	if err != nil {
		return nil, err
	}
	if len(routers) == 0 {
		return nil, nil
	}
	return a.toRouter(&routers[0]), nil
}

// GetRouterByName retrieves routers by name
func (a *networkingAccess) GetRouterByName(name string) ([]*Router, error) {
	routers, err := a.networking.ListRouters(routers.ListOpts{Name: name})
	if err != nil {
		return nil, err
	}
	var result []*Router
	for _, raw := range routers {
		result = append(result, a.toRouter(&raw))
	}
	return result, nil
}

func (a *networkingAccess) toRouter(raw *routers.Router) *Router {
	router := &Router{
		ID:                raw.ID,
		Name:              raw.Name,
		ExternalNetworkID: raw.GatewayInfo.NetworkID,
		EnableSNAT:        raw.GatewayInfo.EnableSNAT,
		Status:            raw.Status,
		ExternalFixedIPs:  raw.GatewayInfo.ExternalFixedIPs,
	}
	return router
}

// UpdateRouter updates the router if important fields have changed
func (a *networkingAccess) UpdateRouter(desired, current *Router) (modified bool, err error) {
	updateOpts := routers.UpdateOpts{}
	if desired.Name != current.Name {
		modified = true
		updateOpts.Name = desired.Name
	}
	if desired.ExternalNetworkID != current.ExternalNetworkID ||
		(desired.EnableSNAT != nil && !reflect.DeepEqual(desired.EnableSNAT, current.EnableSNAT)) {
		modified = true
		updateOpts.GatewayInfo = &routers.GatewayInfo{
			NetworkID:        desired.ExternalNetworkID,
			EnableSNAT:       desired.EnableSNAT,
			ExternalFixedIPs: current.ExternalFixedIPs, // unchanged
		}
	}
	if modified {
		_, err = a.networking.UpdateRouter(current.ID, updateOpts)
	}
	return
}

// AddRouterInterfaceAndWait adds router interface and waits up to
func (a *networkingAccess) AddRouterInterfaceAndWait(ctx context.Context, routerID, subnetID string) error {
	info, err := a.networking.AddRouterInterface(routerID, routers.AddInterfaceOpts{SubnetID: subnetID})
	if err != nil {
		return err
	}
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		time.Sleep(1 * time.Second)
		port, err := a.networking.GetPort(info.PortID)
		if err != nil {
			return err
		}
		switch port.Status {
		case "BUILD", "PENDING_CREATE", "PENDING_UPDATE", "DOWN":
			time.Sleep(3 * time.Second)
			continue
		case "ACTIVE":
			return nil
		default:
			return fmt.Errorf("router interface has unexpected status: %s", port.Status)
		}
	}
}

func (a *networkingAccess) GetRouterInterfacePortID(routerID, subnetID string) (portID *string, err error) {
	port, err := a.networking.GetRouterInterfacePort(routerID, subnetID)
	if err != nil {
		return
	}
	if port == nil {
		return
	}
	portID = &port.ID
	return
}

// RemoveRouterInterfaceAndWait removes the router interface. Either subnetID or portID must be specified
func (a *networkingAccess) RemoveRouterInterfaceAndWait(ctx context.Context, routerID, subnetID, portID string) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		_, err := a.networking.RemoveRouterInterface(routerID, routers.RemoveInterfaceOpts{SubnetID: subnetID, PortID: portID})
		if err != nil {
			if _, ok := err.(gophercloud.ErrDefault404); ok {
				return nil
			}
			if _, ok := err.(gophercloud.ErrDefault409); !ok {
				return err
			}
		}
		if err == nil {
			return nil
		}
		time.Sleep(3 * time.Second)
	}
}

// LookupFloatingPoolSubnetIDs returns a list of subnet ids matching the given regex of the subnet name
func (a *networkingAccess) LookupFloatingPoolSubnetIDs(networkID, floatingPoolSubnetNameRegex string) ([]string, error) {
	allSubnets, err := a.networking.ListSubnets(subnets.ListOpts{
		NetworkID: networkID,
	})
	if err != nil {
		return nil, err
	}
	var subnetIDs []string
	if floatingPoolSubnetNameRegex == "" {
		for _, subnet := range allSubnets {
			subnetIDs = append(subnetIDs, subnet.ID)
		}
		return subnetIDs, nil
	}

	r, err := regexp.Compile(floatingPoolSubnetNameRegex)
	if err != nil {
		return nil, err
	}
	for _, subnet := range allSubnets {
		// Check for a very rare case where the response would include no
		// subnet name. No name means nothing to attempt a match against,
		// therefore we are skipping such subnet.
		if subnet.Name == "" {
			a.log.V(2).Info("[WARN] Unable to find subnet name to match against for subnet ID, nothing to do.",
				"subnetID", subnet.ID)
			continue
		}
		if r.MatchString(subnet.Name) {
			subnetIDs = append(subnetIDs, subnet.ID)
		}
	}
	return subnetIDs, nil
}

// CreateNetwork creates a private network
func (a *networkingAccess) CreateNetwork(desired *Network) (*Network, error) {
	raw, err := a.networking.CreateNetwork(networks.CreateOpts{
		AdminStateUp: &desired.AdminStateUp,
		Name:         desired.Name,
	})
	if err != nil {
		return nil, err
	}
	return a.toNetwork(raw), nil
}

// GetNetworkByID retrieves a network by identifer
func (a *networkingAccess) GetNetworkByID(id string) (*Network, error) {
	networks, err := a.networking.ListNetwork(networks.ListOpts{ID: id})
	if err != nil {
		return nil, err
	}
	if len(networks) == 0 {
		return nil, nil
	}
	return a.toNetwork(&networks[0]), nil
}

// GetNetworkByName retrieves networks by name
func (a *networkingAccess) GetNetworkByName(name string) ([]*Network, error) {
	networks, err := a.networking.GetNetworkByName(name)
	if err != nil {
		return nil, err
	}
	var result []*Network
	for _, raw := range networks {
		result = append(result, a.toNetwork(&raw))
	}
	return result, nil
}

// UpdateNetwork updates a network
func (a *networkingAccess) UpdateNetwork(desired, current *Network) (modified bool, err error) {
	updateOpts := networks.UpdateOpts{}
	if desired.Name != current.Name {
		modified = true
		updateOpts.Name = &desired.Name
	}
	if desired.AdminStateUp != current.AdminStateUp {
		modified = true
		updateOpts.AdminStateUp = &desired.AdminStateUp
	}
	if modified {
		_, err = a.networking.UpdateNetwork(current.ID, updateOpts)
	}
	return
}

func (a *networkingAccess) toNetwork(raw *networks.Network) *Network {
	return &Network{
		ID:           raw.ID,
		Name:         raw.Name,
		AdminStateUp: raw.AdminStateUp,
		Status:       raw.Status,
	}
}

func (a *networkingAccess) CreateSubnet(desired *subnets.Subnet) (*subnets.Subnet, error) {
	raw, err := a.networking.CreateSubnet(subnets.CreateOpts{
		NetworkID:      desired.NetworkID,
		CIDR:           desired.CIDR,
		Name:           desired.Name,
		IPVersion:      gophercloud.IPVersion(desired.IPVersion),
		DNSNameservers: desired.DNSNameservers,
	})
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func (a *networkingAccess) GetSubnetByID(id string) (*subnets.Subnet, error) {
	list, err := a.networking.ListSubnets(subnets.ListOpts{ID: id})
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	return &list[0], nil
}

func (a *networkingAccess) GetSubnetByName(networkID, name string) ([]*subnets.Subnet, error) {
	list, err := a.networking.ListSubnets(subnets.ListOpts{NetworkID: networkID, Name: name})
	if err != nil {
		return nil, err
	}
	var result []*subnets.Subnet
	for _, raw := range list {
		tmp := raw
		result = append(result, &tmp)
	}
	return result, nil
}

func (a *networkingAccess) UpdateSubnet(desired, current *subnets.Subnet) (modified bool, err error) {
	updateOpts := subnets.UpdateOpts{}
	if desired.Name != current.Name {
		modified = true
		updateOpts.Name = &desired.Name
	}
	if !reflect.DeepEqual(desired.DNSNameservers, current.DNSNameservers) {
		modified = true
		updateOpts.DNSNameservers = &desired.DNSNameservers
	}
	if modified {
		_, err = a.networking.UpdateSubnet(current.ID, updateOpts)
	}
	return
}

func (a *networkingAccess) CreateSecurityGroup(desired *groups.SecGroup) (*groups.SecGroup, error) {
	opts := groups.CreateOpts{
		Name:        desired.Name,
		Description: desired.Description,
	}
	return a.networking.CreateSecurityGroup(opts)
}

func (a *networkingAccess) GetSecurityGroupByID(id string) (*groups.SecGroup, error) {
	sg, err := a.networking.GetSecurityGroup(id)
	return sg, client.IgnoreNotFoundError(err)
}

func (a *networkingAccess) GetSecurityGroupByName(name string) ([]*groups.SecGroup, error) {
	list, err := a.networking.ListSecurityGroup(groups.ListOpts{Name: name})
	if err != nil {
		return nil, err
	}
	var result []*groups.SecGroup
	for _, raw := range list {
		tmp := raw
		result = append(result, &tmp)
	}
	return result, nil
}

func (a *networkingAccess) UpdateSecurityGroupRules(
	group *groups.SecGroup,
	desiredRules []rules.SecGroupRule,
	allowDelete func(rule *rules.SecGroupRule) bool,
) (modified bool, err error) {
	for i := range desiredRules {
		rule := &desiredRules[i]
		rule.SecGroupID = group.ID
		rule.ProjectID = group.ProjectID
		rule.TenantID = group.TenantID
		if rule.RemoteGroupID == SecurityGroupIDSelf {
			rule.RemoteGroupID = group.ID
		}
	}

	for i := range group.Rules {
		rule := &group.Rules[i]
		if desiredRule, _ := a.findMatchingRule(rule, desiredRules); desiredRule == nil {
			if allowDelete == nil || allowDelete(rule) {
				if err = a.networking.DeleteRule(rule.ID); err != nil {
					err = fmt.Errorf("Error deleting rule for security group %s: %s", rule.ID, err)
					return
				}
				modified = true
			}
		} else {
			desiredRule.ID = rule.ID // mark as found
		}
	}
	for i := range desiredRules {
		rule := &desiredRules[i]
		if rule.ID != "" {
			// ignore found rules
			continue
		}
		createOpts := rules.CreateOpts{
			Direction:      rules.RuleDirection(rule.Direction),
			Description:    rule.Description,
			EtherType:      rules.RuleEtherType(rule.EtherType),
			SecGroupID:     rule.SecGroupID,
			PortRangeMax:   rule.PortRangeMax,
			PortRangeMin:   rule.PortRangeMin,
			Protocol:       rules.RuleProtocol(rule.Protocol),
			RemoteGroupID:  rule.RemoteGroupID,
			RemoteIPPrefix: rule.RemoteIPPrefix,
			ProjectID:      rule.ProjectID,
		}
		if _, err = a.networking.CreateRule(createOpts); err != nil {
			err = fmt.Errorf("Error creating rule %d for security group: %s", i, err)
			return
		}
		modified = true
	}
	return
}

func (a *networkingAccess) findMatchingRule(rule *rules.SecGroupRule, desiredRules []rules.SecGroupRule) (*rules.SecGroupRule, bool) {
	for i := range desiredRules {
		desired := &desiredRules[i]
		if desired.ID != "" {
			// ignore already found rules
			continue
		}
		if rule.Direction == desired.Direction &&
			rule.EtherType == desired.EtherType &&
			rule.Protocol == desired.Protocol &&
			rule.RemoteIPPrefix == desired.RemoteIPPrefix &&
			rule.RemoteGroupID == desired.RemoteGroupID &&
			rule.PortRangeMin == desired.PortRangeMin &&
			rule.PortRangeMax == desired.PortRangeMax &&
			rule.ProjectID == desired.ProjectID &&
			rule.TenantID == desired.TenantID {
			return desired, rule.Description != desired.Description
		}
	}
	return nil, false
}
