// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package access

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"time"

	"github.com/go-logr/logr"
	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/layer3/routers"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/security/groups"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/security/rules"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/subnets"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
)

// NetworkingAccess provides methods for managing routers and networks
type NetworkingAccess interface {
	// Routers
	CreateRouter(ctx context.Context, desired *Router) (router *Router, err error)
	GetRouterByID(ctx context.Context, id string) (*Router, error)
	GetRouterByName(ctx context.Context, name string) ([]*Router, error)
	UpdateRouter(ctx context.Context, desired, current *Router) (modified bool, err error)
	LookupFloatingPoolSubnetIDs(ctx context.Context, networkID, floatingPoolSubnetNameRegex string) ([]string, error)
	AddRouterInterfaceAndWait(ctx context.Context, routerID, subnetID string) error
	GetRouterInterfacePortID(ctx context.Context, routerID, subnetID string) (portID *string, err error)
	RemoveRouterInterfaceAndWait(ctx context.Context, routerID, subnetID, portID string) error

	// Networks
	CreateNetwork(ctx context.Context, desired *Network) (*Network, error)
	GetNetworkByID(ctx context.Context, id string) (*Network, error)
	GetNetworkByName(ctx context.Context, name string) ([]*Network, error)
	UpdateNetwork(ctx context.Context, desired, current *Network) (modified bool, err error)

	// Subnets
	CreateSubnet(ctx context.Context, desired *subnets.Subnet) (*subnets.Subnet, error)
	GetSubnetByID(ctx context.Context, id string) (*subnets.Subnet, error)
	GetSubnetByName(ctx context.Context, networkID, name string) ([]*subnets.Subnet, error)
	UpdateSubnet(ctx context.Context, desired, current *subnets.Subnet) (modified bool, err error)

	// SecurityGroups
	CreateSecurityGroup(ctx context.Context, desired *groups.SecGroup) (*groups.SecGroup, error)
	GetSecurityGroupByID(ctx context.Context, id string) (*groups.SecGroup, error)
	GetSecurityGroupByName(ctx context.Context, name string) ([]*groups.SecGroup, error)
	UpdateSecurityGroupRules(ctx context.Context, group *groups.SecGroup, desiredRules []rules.SecGroupRule, allowDelete func(rule *rules.SecGroupRule) bool) (modified bool, err error)
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
func (a *networkingAccess) CreateRouter(ctx context.Context, desired *Router) (router *Router, err error) {
	if len(desired.ExternalSubnetIDs) == 0 {
		return a.tryCreateRouter(ctx, desired, nil)
	}
	// create router in first available subnet
	for _, subnetID := range desired.ExternalSubnetIDs {
		router, err = a.tryCreateRouter(ctx, desired, &subnetID)
		// if there is retryable error, then we keep trying along the available list of subnets for the first successful operation.
		if err != nil && !retryOnError(a.log, err) {
			return
		}
		if err == nil {
			break
		}
	}
	return
}

func (a *networkingAccess) tryCreateRouter(ctx context.Context, desired *Router, subnetID *string) (*Router, error) {
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
	raw, err := a.networking.CreateRouter(ctx, options)
	if err != nil {
		return nil, err
	}
	return a.toRouter(raw), nil
}

// GetRouterByID retrieves router by identifier
func (a *networkingAccess) GetRouterByID(ctx context.Context, id string) (*Router, error) {
	routers, err := a.networking.ListRouters(ctx, routers.ListOpts{ID: id})
	if err != nil {
		return nil, err
	}
	if len(routers) == 0 {
		return nil, nil
	}
	return a.toRouter(&routers[0]), nil
}

// GetRouterByName retrieves routers by name
func (a *networkingAccess) GetRouterByName(ctx context.Context, name string) ([]*Router, error) {
	routers, err := a.networking.ListRouters(ctx, routers.ListOpts{Name: name})
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
func (a *networkingAccess) UpdateRouter(ctx context.Context, desired, current *Router) (modified bool, err error) {
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
		_, err = a.networking.UpdateRouter(ctx, current.ID, updateOpts)
	}
	return
}

// AddRouterInterfaceAndWait adds router interface and waits up to
func (a *networkingAccess) AddRouterInterfaceAndWait(ctx context.Context, routerID, subnetID string) error {
	info, err := a.networking.AddRouterInterface(ctx, routerID, routers.AddInterfaceOpts{SubnetID: subnetID})
	if err != nil {
		return err
	}
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		time.Sleep(1 * time.Second)
		port, err := a.networking.GetPort(ctx, info.PortID)
		if err != nil {
			return err
		}

		if port == nil {
			return fmt.Errorf("port was not found ")

		}

		switch port.Status {
		case "BUILD", "PENDING_CREATE", "PENDING_UPDATE", "DOWN":
			a.log.Info("port is not in expected state", "Port", info.PortID, "Status", port.Status)
			if a.log.V(1).Enabled() {
				marshalled, err := json.Marshal(info)
				if err != nil {
					return err
				}
				a.log.V(1).Info("port info", "Port", string(marshalled))
			}
			time.Sleep(3 * time.Second)
			continue
		case "ACTIVE":
			return nil
		default:
			return fmt.Errorf("router interface has unexpected status: %s", port.Status)
		}
	}
}

func (a *networkingAccess) GetRouterInterfacePortID(ctx context.Context, routerID, subnetID string) (portID *string, err error) {
	port, err := a.networking.GetRouterInterfacePort(ctx, routerID, subnetID)
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
		_, err := a.networking.RemoveRouterInterface(ctx, routerID, routers.RemoveInterfaceOpts{SubnetID: subnetID, PortID: portID})
		if err != nil {
			if gophercloud.ResponseCodeIs(err, http.StatusNotFound) {
				return nil
			}
			if gophercloud.ResponseCodeIs(err, http.StatusConflict) {
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
func (a *networkingAccess) LookupFloatingPoolSubnetIDs(ctx context.Context, networkID, floatingPoolSubnetNameRegex string) ([]string, error) {
	allSubnets, err := a.networking.ListSubnets(ctx, subnets.ListOpts{
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
			a.log.V(1).Info("[WARN] Unable to find subnet name to match against for subnet ID, nothing to do.",
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
func (a *networkingAccess) CreateNetwork(ctx context.Context, desired *Network) (*Network, error) {
	raw, err := a.networking.CreateNetwork(ctx, networks.CreateOpts{
		AdminStateUp: &desired.AdminStateUp,
		Name:         desired.Name,
	})
	if err != nil {
		return nil, err
	}
	return a.toNetwork(raw), nil
}

// GetNetworkByID retrieves a network by identifer
func (a *networkingAccess) GetNetworkByID(ctx context.Context, id string) (*Network, error) {
	networks, err := a.networking.ListNetwork(ctx, networks.ListOpts{ID: id})
	if err != nil {
		return nil, err
	}
	if len(networks) == 0 {
		return nil, nil
	}
	return a.toNetwork(&networks[0]), nil
}

// GetNetworkByName retrieves networks by name
func (a *networkingAccess) GetNetworkByName(ctx context.Context, name string) ([]*Network, error) {
	networks, err := a.networking.GetNetworkByName(ctx, name)
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
func (a *networkingAccess) UpdateNetwork(ctx context.Context, desired, current *Network) (modified bool, err error) {
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
		_, err = a.networking.UpdateNetwork(ctx, current.ID, updateOpts)
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

func (a *networkingAccess) CreateSubnet(ctx context.Context, desired *subnets.Subnet) (*subnets.Subnet, error) {
	raw, err := a.networking.CreateSubnet(ctx, subnets.CreateOpts{
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

func (a *networkingAccess) GetSubnetByID(ctx context.Context, id string) (*subnets.Subnet, error) {
	list, err := a.networking.ListSubnets(ctx, subnets.ListOpts{ID: id})
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	return &list[0], nil
}

func (a *networkingAccess) GetSubnetByName(ctx context.Context, networkID, name string) ([]*subnets.Subnet, error) {
	list, err := a.networking.ListSubnets(ctx, subnets.ListOpts{NetworkID: networkID, Name: name})
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

func (a *networkingAccess) UpdateSubnet(ctx context.Context, desired, current *subnets.Subnet) (modified bool, err error) {
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
		_, err = a.networking.UpdateSubnet(ctx, current.ID, updateOpts)
	}
	return
}

func (a *networkingAccess) CreateSecurityGroup(ctx context.Context, desired *groups.SecGroup) (*groups.SecGroup, error) {
	opts := groups.CreateOpts{
		Name:        desired.Name,
		Description: desired.Description,
	}
	return a.networking.CreateSecurityGroup(ctx, opts)
}

func (a *networkingAccess) GetSecurityGroupByID(ctx context.Context, id string) (*groups.SecGroup, error) {
	sg, err := a.networking.GetSecurityGroup(ctx, id)
	return sg, client.IgnoreNotFoundError(err)
}

func (a *networkingAccess) GetSecurityGroupByName(ctx context.Context, name string) ([]*groups.SecGroup, error) {
	list, err := a.networking.ListSecurityGroup(ctx, groups.ListOpts{Name: name})
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
	ctx context.Context,
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
				if err = a.networking.DeleteRule(ctx, rule.ID); err != nil {
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
		if _, err = a.networking.CreateRule(ctx, createOpts); err != nil {
			err = fmt.Errorf("error creating rule %d for security group: %s", i, err)
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
