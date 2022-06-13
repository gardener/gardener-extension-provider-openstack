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

package bastion

import (
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
)

// OpenstackClient used to perform openstack operations
type OpenstackClient struct {
	Regionopts       gophercloud.EndpointOpts
	ProviderClient   *gophercloud.ProviderClient
	ComputeClient    *gophercloud.ServiceClient
	NetworkingClient *gophercloud.ServiceClient
}

// NewOpenstackClient creates an openstack struct
func NewOpenstackClient(authURL, domainName, password, region, tenantName, username string) (*OpenstackClient, error) {
	openstackClient := &OpenstackClient{
		Regionopts: gophercloud.EndpointOpts{Region: region}}

	opts := gophercloud.AuthOptions{
		IdentityEndpoint: authURL,
		Username:         username,
		Password:         password,
		DomainName:       domainName,
		TenantName:       tenantName,
	}

	providerClient, err := openstack.AuthenticatedClient(opts)
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

	openstackClient.ComputeClient = computeClient
	openstackClient.NetworkingClient = networkingClient

	return openstackClient, nil
}

func (o *OpenstackClient) createComputeClient() (*gophercloud.ServiceClient, error) {
	return openstack.NewComputeV2(o.ProviderClient, o.Regionopts)
}

func (o *OpenstackClient) createNetworkingClient() (*gophercloud.ServiceClient, error) {
	return openstack.NewNetworkV2(o.ProviderClient, o.Regionopts)
}
