// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package infrastructure

import (
	"crypto/tls"
	"net/http"
	"strings"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/utils/openstack/clientconfig"
)

// OpenstackClient used to perform openstack operations
type OpenstackClient struct {
	AuthURL          string
	DomainName       string
	FloatingPoolName string
	Password         string
	Region           string
	TenantName       string
	UserName         string
	ProviderClient   *gophercloud.ProviderClient
	ComputeClient    *gophercloud.ServiceClient
	NetworkingClient *gophercloud.ServiceClient
	IdentityClient   *gophercloud.ServiceClient
}

// NewOpenstackClient creates an openstack struct
func NewOpenstackClient(authURL, domainName, floatingPoolName, password, region, tenantName, username string) (*OpenstackClient, error) {

	openstackClient := &OpenstackClient{
		AuthURL:          authURL,
		DomainName:       domainName,
		FloatingPoolName: floatingPoolName,
		Password:         password,
		Region:           region,
		TenantName:       tenantName,
		UserName:         username,
	}

	providerClient, err := openstackClient.createProviderClient()
	if err != nil {
		return nil, err
	}

	openstackClient.ProviderClient = providerClient

	computeClient, err := openstackClient.createComputeClient()
	if err != nil {
		return nil, err
	}

	networkingClient, err := openstackClient.createNetworkingClient()
	if err != nil {
		return nil, err
	}

	identityClient, err := openstackClient.createIdentityClient()
	if err != nil {
		return nil, err
	}

	openstackClient.ComputeClient = computeClient
	openstackClient.NetworkingClient = networkingClient
	openstackClient.IdentityClient = identityClient

	return openstackClient, nil
}

// createOpenStackClient creates and authenticates a base OpenStack client
func (o *OpenstackClient) createProviderClient() (*gophercloud.ProviderClient, error) {
	config := &tls.Config{}
	config.InsecureSkipVerify = false

	opts := &clientconfig.ClientOpts{
		AuthInfo: &clientconfig.AuthInfo{
			AuthURL:     strings.TrimSpace(string(o.AuthURL)),
			Username:    strings.TrimSpace(string(o.UserName)),
			Password:    strings.TrimSpace(string(o.Password)),
			DomainName:  strings.TrimSpace(string(o.DomainName)),
			ProjectName: strings.TrimSpace(string(o.TenantName)),
		},
		RegionName: strings.TrimSpace(string(o.Region)),
	}
	authOpts, err := clientconfig.AuthOptions(opts)
	if err != nil {
		return nil, err
	}

	// AllowReauth should be set to true if you grant permission for Gophercloud to
	// cache your credentials in memory, and to allow Gophercloud to attempt to
	// re-authenticate automatically if/when your token expires.
	authOpts.AllowReauth = true

	provider, err := openstack.AuthenticatedClient(*authOpts)
	if err != nil {
		return nil, err

	}

	// Set UserAgent
	provider.UserAgent.Prepend("Infrastructure Test Controller")

	transport := &http.Transport{Proxy: http.ProxyFromEnvironment, TLSClientConfig: config}
	provider.HTTPClient = http.Client{
		Transport: transport,
	}

	return provider, nil
}

// createComputeClient is used to create a compute client
func (o *OpenstackClient) createComputeClient() (*gophercloud.ServiceClient, error) {
	return openstack.NewComputeV2(o.ProviderClient, gophercloud.EndpointOpts{
		Region:       strings.TrimSpace(o.Region),
		Availability: gophercloud.AvailabilityPublic,
	})
}

// createNetworkingClient is used to create a networking client
func (o *OpenstackClient) createNetworkingClient() (*gophercloud.ServiceClient, error) {
	return openstack.NewNetworkV2(o.ProviderClient, gophercloud.EndpointOpts{
		Region:       o.Region,
		Availability: gophercloud.AvailabilityPublic,
	})
}

// createIdentityClient is used to create a networking client
func (o *OpenstackClient) createIdentityClient() (*gophercloud.ServiceClient, error) {
	return openstack.NewIdentityV2(o.ProviderClient, gophercloud.EndpointOpts{
		Region:       o.Region,
		Availability: gophercloud.AvailabilityPublic,
	})
}
