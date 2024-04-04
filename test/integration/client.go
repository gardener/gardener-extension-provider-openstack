// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"crypto/tls"
	"net/http"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/utils/openstack/clientconfig"
)

// OpenstackClient used to perform openstack operations
type OpenstackClient struct {
	Regionopts       gophercloud.EndpointOpts
	ProviderClient   *gophercloud.ProviderClient
	ComputeClient    *gophercloud.ServiceClient
	NetworkingClient *gophercloud.ServiceClient
	IdentityClient   *gophercloud.ServiceClient
}

// NewOSClient creates an openstack struct
func NewOSClient(opts *clientconfig.ClientOpts) (*OpenstackClient, error) {
	osClient := &OpenstackClient{
		Regionopts: gophercloud.EndpointOpts{
			Region:       opts.RegionName,
			Availability: gophercloud.AvailabilityPublic,
		},
	}

	err := osClient.setProviderClient(opts)
	if err != nil {
		return nil, err
	}

	err = osClient.setComputeClient()
	if err != nil {
		return nil, err
	}

	err = osClient.setNetworkingClient()
	if err != nil {
		return nil, err
	}

	return osClient, nil
}

// NewOSClientWithIdentity creates an openstack struct
func NewOSClientWithIdentity(opts *clientconfig.ClientOpts) (*OpenstackClient, error) {
	osClient, err := NewOSClient(opts)
	if err != nil {
		return nil, err
	}

	err = osClient.setIdentityClient()
	if err != nil {
		return nil, err
	}

	return osClient, nil
}

// createOpenStackClient creates and authenticates a base OpenStack client
func (o *OpenstackClient) setProviderClient(opts *clientconfig.ClientOpts) error {
	authOpts, err := clientconfig.AuthOptions(opts)
	if err != nil {
		return err
	}

	// AllowReauth should be set to true if you grant permission for Gophercloud to
	// cache your credentials in memory, and to allow Gophercloud to attempt to
	// re-authenticate automatically if/when your token expires.
	authOpts.AllowReauth = true

	provider, err := openstack.AuthenticatedClient(*authOpts)
	if err != nil {
		return err
	}

	// Set UserAgent
	provider.UserAgent.Prepend("Bastion Test Controller")

	config := &tls.Config{}
	config.InsecureSkipVerify = false
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment, TLSClientConfig: config}
	provider.HTTPClient = http.Client{
		Transport: transport,
	}

	o.ProviderClient = provider

	return nil
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

// createIdentityClient is used to create a networking client
func (o *OpenstackClient) setIdentityClient() error {
	identityClient, err := openstack.NewIdentityV2(o.ProviderClient, o.Regionopts)
	o.IdentityClient = identityClient
	return err
}
