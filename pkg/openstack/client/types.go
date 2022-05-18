// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

//go:generate mockgen -destination=mocks/client_mocks.go -package=mocks . Factory,FactoryFactory,Compute,DNS,Networking,Identity
package client

import (
	"context"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	"github.com/gophercloud/gophercloud"
	computefip "github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/floatingips"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/servergroups"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/images"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/openstack/identity/v3/applicationcredentials"
	"github.com/gophercloud/gophercloud/openstack/identity/v3/tokens"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/layer3/floatingips"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/security/groups"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/security/rules"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
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

// IdentityClient is a client for the Identity/Keystone service.
type IdentityClient struct {
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
	Identity(options ...Option) (Identity, error)
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

	FindFlavorID(name string) (string, error)
	FindImages(name string) ([]images.Image, error)
	ListImages(listOpts images.ListOpts) ([]images.Image, error)
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
	ListNetwork(listOpts networks.ListOpts) ([]networks.Network, error)
	GetNetworkByName(name string) ([]networks.Network, error)
	// FloatingIP
	CreateFloatingIP(createOpts floatingips.CreateOpts) (*floatingips.FloatingIP, error)
	DeleteFloatingIP(id string) error
	ListFip(listOpts floatingips.ListOpts) ([]floatingips.FloatingIP, error)
	GetFipByName(name string) ([]floatingips.FloatingIP, error)
	// Security Group
	CreateSecurityGroup(listOpts groups.CreateOpts) (*groups.SecGroup, error)
	DeleteSecurityGroup(groupID string) error
	ListSecurityGroup(listOpts groups.ListOpts) ([]groups.SecGroup, error)
	GetSecurityGroupByName(name string) ([]groups.SecGroup, error)
	// Security Group rules
	CreateRule(createOpts rules.CreateOpts) (*rules.SecGroupRule, error)
	ListRules(listOpts rules.ListOpts) ([]rules.SecGroupRule, error)
	DeleteRule(ruleID string) error
}

type Identity interface {
	GetApplicationCredential(ctx context.Context, parentUserID, applicationCredentialID string) (*applicationcredentials.ApplicationCredential, error)
	ListApplicationCredentials(ctx context.Context, parentUserID string) ([]applicationcredentials.ApplicationCredential, error)
	CreateApplicationCredential(ctx context.Context, parentUserID, name, description, expirationTime string) (*applicationcredentials.ApplicationCredential, error)
	DeleteApplicationCredential(ctx context.Context, parentUserID, applicationCredentialID string) error
	GetClientUser() (*tokens.User, error)
}

// FactoryFactory creates instances of Factory.s
type FactoryFactory interface {
	// NewFactory creates a new instance of Factory for the given Openstack credentials.
	NewFactory(credentials *openstack.Credentials) (Factory, error)
}

// FactoryFactoryFunc is a function that implements FactoryFactory.
type FactoryFactoryFunc func(credentials *openstack.Credentials) (Factory, error)

// NewFactory creates a new instance of Factory for the given Openstack credentials.
func (f FactoryFactoryFunc) NewFactory(credentials *openstack.Credentials) (Factory, error) {
	return f(credentials)
}
