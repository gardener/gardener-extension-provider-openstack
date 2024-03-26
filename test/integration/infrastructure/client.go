// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"crypto/tls"
	"net/http"
	"strings"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/utils/openstack/clientconfig"
)

type OpenstackClientOpts struct {
	AuthURL          string
	DomainName       string
	FloatingPoolName string
	Password         string
	Region           string
	TenantName       string
	UserName         string
	AppID            string
	AppName          string
	AppSecret        string
}

// OpenstackClient used to perform openstack operations
type OpenstackClient struct {
	OpenstackClientOpts
	ProviderClient   *gophercloud.ProviderClient
	ComputeClient    *gophercloud.ServiceClient
	NetworkingClient *gophercloud.ServiceClient
	IdentityClient   *gophercloud.ServiceClient
}

// NewOpenstackClient creates an openstack struct
func NewOpenstackClient(opts OpenstackClientOpts) (*OpenstackClient, error) {
	openstackClient := &OpenstackClient{OpenstackClientOpts: opts}

	err := openstackClient.setProviderClient()
	if err != nil {
		return nil, err
	}

	err = openstackClient.setComputeClient()
	if err != nil {
		return nil, err
	}

	err = openstackClient.setNetworkingClient()
	if err != nil {
		return nil, err
	}

	err = openstackClient.setIdentityClient()
	if err != nil {
		return nil, err
	}

	return openstackClient, nil
}

// createOpenStackClient creates and authenticates a base OpenStack client
func (o *OpenstackClient) setProviderClient() error {
	config := &tls.Config{}
	config.InsecureSkipVerify = false

	opts := &clientconfig.ClientOpts{
		AuthInfo: &clientconfig.AuthInfo{
			AuthURL:                     strings.TrimSpace(string(o.AuthURL)),
			DomainName:                  strings.TrimSpace(string(o.DomainName)),
			ProjectName:                 strings.TrimSpace(string(o.TenantName)),
			Username:                    strings.TrimSpace(string(o.UserName)),
			Password:                    strings.TrimSpace(string(o.Password)),
			ApplicationCredentialID:     strings.TrimSpace(string(o.AppID)),
			ApplicationCredentialName:   strings.TrimSpace(string(o.AppName)),
			ApplicationCredentialSecret: strings.TrimSpace(string(o.AppSecret)),
		},
		RegionName: strings.TrimSpace(string(o.Region)),
	}
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
	provider.UserAgent.Prepend("Infrastructure Test Controller")

	transport := &http.Transport{Proxy: http.ProxyFromEnvironment, TLSClientConfig: config}
	provider.HTTPClient = http.Client{
		Transport: transport,
	}

	o.ProviderClient = provider

	return nil
}

// createComputeClient is used to create a compute client
func (o *OpenstackClient) setComputeClient() error {
	computeClient, err := openstack.NewComputeV2(o.ProviderClient, gophercloud.EndpointOpts{
		Region:       strings.TrimSpace(o.Region),
		Availability: gophercloud.AvailabilityPublic,
	})

	o.ComputeClient = computeClient

	return err
}

// createNetworkingClient is used to create a networking client
func (o *OpenstackClient) setNetworkingClient() error {
	networkingClient, err := openstack.NewNetworkV2(o.ProviderClient, gophercloud.EndpointOpts{
		Region:       o.Region,
		Availability: gophercloud.AvailabilityPublic,
	})

	o.NetworkingClient = networkingClient

	return err
}

// createIdentityClient is used to create a networking client
func (o *OpenstackClient) setIdentityClient() error {
	identityClient, err := openstack.NewIdentityV2(o.ProviderClient, gophercloud.EndpointOpts{
		Region:       o.Region,
		Availability: gophercloud.AvailabilityPublic,
	})

	o.IdentityClient = identityClient

	return err
}
