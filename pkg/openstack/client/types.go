// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate mockgen -destination=mocks/client_mocks.go -package=mocks . Factory,FactoryFactory,Compute,DNS,Networking,Loadbalancing,SharedFilesystem
package client

import (
	"context"

	"github.com/gophercloud/gophercloud"
	computefip "github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/floatingips"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/keypairs"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/servergroups"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/images"
	"github.com/gophercloud/gophercloud/openstack/loadbalancer/v2/loadbalancers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/layer3/floatingips"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/layer3/routers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/security/groups"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/security/rules"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/ports"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/subnets"
	"github.com/gophercloud/gophercloud/openstack/sharedfilesystems/v2/sharenetworks"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
)

// OpenstackClientFactory implements a factory that can construct clients for Openstack services.
type OpenstackClientFactory struct {
	providerClient *gophercloud.ProviderClient
}

// StorageClient is a client for the Swift service.
type StorageClient struct {
	client *gophercloud.ServiceClient
}

// ComputeClient is a client for the Nova service.
type ComputeClient struct {
	client *gophercloud.ServiceClient
}

// DNSClient is a client for the Designate service.
type DNSClient struct {
	client *gophercloud.ServiceClient
}

// NetworkingClient is a client for the Neutron service.
type NetworkingClient struct {
	client *gophercloud.ServiceClient
}

// LoadbalancingClient is a client for Octavia service.
type LoadbalancingClient struct {
	client *gophercloud.ServiceClient
}

// SharedFilesystemClient is a client for Manila service.
type SharedFilesystemClient struct {
	client *gophercloud.ServiceClient
}

type ImageClient struct {
	client *gophercloud.ServiceClient
}

// Option can be passed to Factory implementations to modify the produced clients.
type Option func(opts gophercloud.EndpointOpts) gophercloud.EndpointOpts

// Factory is an interface for constructing OpenStack service clients.
type Factory interface {
	Compute(options ...Option) (Compute, error)
	Storage(options ...Option) (Storage, error)
	DNS(options ...Option) (DNS, error)
	Networking(options ...Option) (Networking, error)
	Loadbalancing(options ...Option) (Loadbalancing, error)
	SharedFilesystem(options ...Option) (SharedFilesystem, error)
	Images(options ...Option) (Images, error)
}

// Storage describes the operations of a client interacting with OpenStack's ObjectStorage service.
type Storage interface {
	DeleteObjectsWithPrefix(ctx context.Context, container, prefix string) error
	CreateContainerIfNotExists(ctx context.Context, container string) error
	DeleteContainerIfExists(ctx context.Context, container string) error
}

// Compute describes the operations of a client interacting with OpenStack's Compute service.
type Compute interface {
	CreateServerGroup(name, policy string) (*servergroups.ServerGroup, error)
	GetServerGroup(id string) (*servergroups.ServerGroup, error)
	DeleteServerGroup(id string) error
	// Server
	CreateServer(createOpts servers.CreateOpts) (*servers.Server, error)
	DeleteServer(id string) error
	ListServerGroups() ([]servergroups.ServerGroup, error)
	FindServersByName(name string) ([]servers.Server, error)
	AssociateFIPWithInstance(serverID string, associateOpts computefip.AssociateOpts) error
	// FloatingID
	FindFloatingIDByInstanceID(id string) (string, error)

	// Flavor
	FindFlavorID(name string) (string, error)

	// KeyPairs
	CreateKeyPair(name, publicKey string) (*keypairs.KeyPair, error)
	GetKeyPair(name string) (*keypairs.KeyPair, error)
	DeleteKeyPair(name string) error
}

// DNS describes the operations of a client interacting with OpenStack's DNS service.
type DNS interface {
	GetZones(ctx context.Context) (map[string]string, error)
	CreateOrUpdateRecordSet(ctx context.Context, zoneID, name, recordType string, records []string, ttl int) error
	DeleteRecordSet(ctx context.Context, zoneID, name, recordType string) error
}

// Networking describes the operations of a client interacting with OpenStack's Networking service.
type Networking interface {
	// External Network
	GetExternalNetworkNames(ctx context.Context) ([]string, error)
	GetExternalNetworkByName(name string) (*networks.Network, error)
	// Network
	CreateNetwork(opts networks.CreateOpts) (*networks.Network, error)
	ListNetwork(listOpts networks.ListOpts) ([]networks.Network, error)
	UpdateNetwork(networkID string, opts networks.UpdateOpts) (*networks.Network, error)
	GetNetworkByName(name string) ([]networks.Network, error)
	GetNetworkByID(id string) (*networks.Network, error)
	DeleteNetwork(networkID string) error
	// FloatingIP
	CreateFloatingIP(createOpts floatingips.CreateOpts) (*floatingips.FloatingIP, error)
	DeleteFloatingIP(id string) error
	ListFip(listOpts floatingips.ListOpts) ([]floatingips.FloatingIP, error)
	GetFipByName(name string) ([]floatingips.FloatingIP, error)
	// Security Group
	CreateSecurityGroup(listOpts groups.CreateOpts) (*groups.SecGroup, error)
	DeleteSecurityGroup(groupID string) error
	ListSecurityGroup(listOpts groups.ListOpts) ([]groups.SecGroup, error)
	GetSecurityGroup(groupID string) (*groups.SecGroup, error)
	GetSecurityGroupByName(name string) ([]groups.SecGroup, error)
	// Security Group rules
	CreateRule(createOpts rules.CreateOpts) (*rules.SecGroupRule, error)
	ListRules(listOpts rules.ListOpts) ([]rules.SecGroupRule, error)
	DeleteRule(ruleID string) error
	// Routers
	GetRouterByID(id string) (*routers.Router, error)
	ListRouters(listOpts routers.ListOpts) ([]routers.Router, error)
	UpdateRoutesForRouter(routes []routers.Route, routerID string) (*routers.Router, error)
	UpdateRouter(routerID string, updateOpts routers.UpdateOpts) (*routers.Router, error)
	CreateRouter(createOpts routers.CreateOpts) (*routers.Router, error)
	DeleteRouter(routerID string) error
	AddRouterInterface(routerID string, addOpts routers.AddInterfaceOpts) (*routers.InterfaceInfo, error)
	RemoveRouterInterface(routerID string, removeOpts routers.RemoveInterfaceOpts) (*routers.InterfaceInfo, error)
	// Subnets
	CreateSubnet(createOpts subnets.CreateOpts) (*subnets.Subnet, error)
	GetSubnetByID(id string) (*subnets.Subnet, error)
	ListSubnets(listOpts subnets.ListOpts) ([]subnets.Subnet, error)
	UpdateSubnet(subnetID string, updateOpts subnets.UpdateOpts) (*subnets.Subnet, error)
	DeleteSubnet(subnetID string) error
	// Ports
	GetPort(portID string) (*ports.Port, error)
	GetRouterInterfacePort(routerID, subnetID string) (*ports.Port, error)
}

// Loadbalancing describes the operations of a client interacting with OpenStack's Octavia service.
type Loadbalancing interface {
	ListLoadbalancers(opts loadbalancers.ListOpts) ([]loadbalancers.LoadBalancer, error)
	DeleteLoadbalancer(id string, opts loadbalancers.DeleteOpts) error
	GetLoadbalancer(id string) (*loadbalancers.LoadBalancer, error)
}

// SharedFilesystem describes operations for OpenStack's Manila service.
type SharedFilesystem interface {
	// Share Networks
	GetShareNetwork(id string) (*sharenetworks.ShareNetwork, error)
	CreateShareNetwork(createOpts sharenetworks.CreateOpts) (*sharenetworks.ShareNetwork, error)
	ListShareNetworks(listOpts sharenetworks.ListOpts) ([]sharenetworks.ShareNetwork, error)
	DeleteShareNetwork(id string) error
}

// FactoryFactory creates instances of Factory.
type FactoryFactory interface {
	// NewFactory creates a new instance of Factory for the given Openstack credentials.
	NewFactory(credentials *openstack.Credentials) (Factory, error)
}

type Images interface {
	ListImages(opts images.ListOpts) ([]images.Image, error)
}

// FactoryFactoryFunc is a function that implements FactoryFactory.
type FactoryFactoryFunc func(credentials *openstack.Credentials) (Factory, error)

// NewFactory creates a new instance of Factory for the given Openstack credentials.
func (f FactoryFactoryFunc) NewFactory(credentials *openstack.Credentials) (Factory, error) {
	return f(credentials)
}
