// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
