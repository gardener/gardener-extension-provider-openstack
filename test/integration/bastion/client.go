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
func NewOpenstackClient(opts gophercloud.AuthOptions, region string) (*OpenstackClient, error) {
	openstackClient := &OpenstackClient{
		Regionopts: gophercloud.EndpointOpts{Region: region},
	}

	providerClient, err := openstack.AuthenticatedClient(opts)
	if err != nil {
		return nil, err
	}

	openstackClient.ProviderClient = providerClient

	err = openstackClient.setComputeClient()
	if err != nil {
		return nil, err
	}

	err = openstackClient.setNetworkingClient()
	if err != nil {
		return nil, err
	}

	return openstackClient, nil
}

func (o *OpenstackClient) setComputeClient() error {
	computeClient, err := openstack.NewComputeV2(o.ProviderClient, o.Regionopts)
	o.ComputeClient = computeClient
	return err
}

func (o *OpenstackClient) setNetworkingClient() error {
	networkingClient, err := openstack.NewNetworkV2(o.ProviderClient, o.Regionopts)
	o.NetworkingClient = networkingClient
	return err
}
