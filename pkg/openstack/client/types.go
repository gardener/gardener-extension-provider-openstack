// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate mockgen -destination=mocks/client_mocks.go -package=mocks . Factory,FactoryFactory,Compute,DNS,Networking,Loadbalancing,SharedFilesystem
package client

import (
	"context"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/keypairs"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servergroups"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/v2/openstack/image/v2/images"
	"github.com/gophercloud/gophercloud/v2/openstack/loadbalancer/v2/loadbalancers"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/layer3/floatingips"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/layer3/routers"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/security/groups"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/security/rules"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/ports"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/subnets"
	"github.com/gophercloud/gophercloud/v2/openstack/sharedfilesystems/v2/sharenetworks"

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

// ImageClient is a client for images
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
	CreateServerGroup(ctx context.Context, name, policy string) (*servergroups.ServerGroup, error)
	GetServerGroup(ctx context.Context, id string) (*servergroups.ServerGroup, error)
	DeleteServerGroup(ctx context.Context, id string) error
	// Server
	ListServerGroups(ctx context.Context) ([]servergroups.ServerGroup, error)
	CreateServer(ctx context.Context, createOpts servers.CreateOpts) (*servers.Server, error)
	DeleteServer(ctx context.Context, id string) error
	FindServersByName(ctx context.Context, name string) ([]servers.Server, error)

	// Flavor
	FindFlavorID(ctx context.Context, name string) (string, error)

	// KeyPairs
	CreateKeyPair(ctx context.Context, name, publicKey string) (*keypairs.KeyPair, error)
	GetKeyPair(ctx context.Context, name string) (*keypairs.KeyPair, error)
	DeleteKeyPair(ctx context.Context, name string) error
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
	GetExternalNetworkByName(ctx context.Context, name string) (*networks.Network, error)
	// Network
	CreateNetwork(ctx context.Context, opts networks.CreateOpts) (*networks.Network, error)
	ListNetwork(ctx context.Context, listOpts networks.ListOpts) ([]networks.Network, error)
	UpdateNetwork(ctx context.Context, networkID string, opts networks.UpdateOpts) (*networks.Network, error)
	GetNetworkByName(ctx context.Context, name string) ([]networks.Network, error)
	GetNetworkByID(ctx context.Context, id string) (*networks.Network, error)
	DeleteNetwork(ctx context.Context, networkID string) error
	// FloatingIP
	CreateFloatingIP(ctx context.Context, createOpts floatingips.CreateOpts) (*floatingips.FloatingIP, error)
	DeleteFloatingIP(ctx context.Context, id string) error
	ListFip(ctx context.Context, listOpts floatingips.ListOpts) ([]floatingips.FloatingIP, error)
	GetFipByName(ctx context.Context, name string) ([]floatingips.FloatingIP, error)
	GetFloatingIP(ctx context.Context, listOpts floatingips.ListOpts) (floatingips.FloatingIP, error)
	// Security Group
	CreateSecurityGroup(ctx context.Context, listOpts groups.CreateOpts) (*groups.SecGroup, error)
	DeleteSecurityGroup(ctx context.Context, groupID string) error
	ListSecurityGroup(ctx context.Context, listOpts groups.ListOpts) ([]groups.SecGroup, error)
	GetSecurityGroup(ctx context.Context, groupID string) (*groups.SecGroup, error)
	GetSecurityGroupByName(ctx context.Context, name string) ([]groups.SecGroup, error)
	// Security Group rules
	CreateRule(ctx context.Context, createOpts rules.CreateOpts) (*rules.SecGroupRule, error)
	ListRules(ctx context.Context, listOpts rules.ListOpts) ([]rules.SecGroupRule, error)
	DeleteRule(ctx context.Context, ruleID string) error
	// Routers
	GetRouterByID(ctx context.Context, id string) (*routers.Router, error)
	ListRouters(ctx context.Context, listOpts routers.ListOpts) ([]routers.Router, error)
	UpdateRoutesForRouter(ctx context.Context, routes []routers.Route, routerID string) (*routers.Router, error)
	UpdateRouter(ctx context.Context, routerID string, updateOpts routers.UpdateOpts) (*routers.Router, error)
	CreateRouter(ctx context.Context, createOpts routers.CreateOpts) (*routers.Router, error)
	DeleteRouter(ctx context.Context, routerID string) error
	AddRouterInterface(ctx context.Context, routerID string, addOpts routers.AddInterfaceOpts) (*routers.InterfaceInfo, error)
	RemoveRouterInterface(ctx context.Context, routerID string, removeOpts routers.RemoveInterfaceOpts) (*routers.InterfaceInfo, error)
	// Subnets
	CreateSubnet(ctx context.Context, createOpts subnets.CreateOpts) (*subnets.Subnet, error)
	GetSubnetByID(ctx context.Context, id string) (*subnets.Subnet, error)
	ListSubnets(ctx context.Context, listOpts subnets.ListOpts) ([]subnets.Subnet, error)
	UpdateSubnet(ctx context.Context, id string, updateOpts subnets.UpdateOpts) (*subnets.Subnet, error)
	DeleteSubnet(ctx context.Context, subnetID string) error
	// Ports
	GetPort(ctx context.Context, portID string) (*ports.Port, error)
	GetRouterInterfacePort(ctx context.Context, routerID, subnetID string) (*ports.Port, error)
	GetInstancePorts(ctx context.Context, instanceID string) ([]ports.Port, error)
	UpdateFIPWithPort(ctx context.Context, fipID, portID string) error
}

// Loadbalancing describes the operations of a client interacting with OpenStack's Octavia service.
type Loadbalancing interface {
	ListLoadbalancers(ctx context.Context, listOpts loadbalancers.ListOpts) ([]loadbalancers.LoadBalancer, error)
	DeleteLoadbalancer(ctx context.Context, id string, opts loadbalancers.DeleteOpts) error
	GetLoadbalancer(ctx context.Context, id string) (*loadbalancers.LoadBalancer, error)
}

// SharedFilesystem describes operations for OpenStack's Manila service.
type SharedFilesystem interface {
	// Share Networks
	GetShareNetwork(ctx context.Context, id string) (*sharenetworks.ShareNetwork, error)
	CreateShareNetwork(ctx context.Context, createOpts sharenetworks.CreateOpts) (*sharenetworks.ShareNetwork, error)
	ListShareNetworks(ctx context.Context, listOpts sharenetworks.ListOpts) ([]sharenetworks.ShareNetwork, error)
	DeleteShareNetwork(ctx context.Context, id string) error
}

// FactoryFactory creates instances of Factory.
type FactoryFactory interface {
	// NewFactory creates a new instance of Factory for the given Openstack credentials.
	NewFactory(credentials *openstack.Credentials) (Factory, error)
}

// Images describes the operations of a client interacting with images
type Images interface {
	ListImages(ctx context.Context, opts images.ListOpts) ([]images.Image, error)
}

// FactoryFactoryFunc is a function that implements FactoryFactory.
type FactoryFactoryFunc func(credentials *openstack.Credentials) (Factory, error)

// NewFactory creates a new instance of Factory for the given Openstack credentials.
func (f FactoryFactoryFunc) NewFactory(credentials *openstack.Credentials) (Factory, error) {
	return f(credentials)
}
