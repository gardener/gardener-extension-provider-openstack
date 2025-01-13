// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"strings"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack"
	"github.com/gophercloud/gophercloud/v2/openstack/config"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	os "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
)

// NewOpenstackClientFromCredentials returns a Factory implementation that can be used to create clients for OpenStack services.
// TODO: respect CloudProfile's requestTimeout for the OpenStack client.
// see https://github.com/kubernetes/cloud-provider-openstack/blob/c44d941cdb5c7fe651f5cb9191d0af23e266c7cb/pkg/openstack/openstack.go#L257
func NewOpenstackClientFromCredentials(credentials *os.Credentials) (Factory, error) {
	authOpts := gophercloud.AuthOptions{
		IdentityEndpoint:            credentials.AuthURL,
		Username:                    credentials.Username,
		Password:                    credentials.Password,
		DomainName:                  credentials.DomainName,
		ApplicationCredentialID:     credentials.ApplicationCredentialID,
		ApplicationCredentialName:   credentials.ApplicationCredentialName,
		ApplicationCredentialSecret: credentials.ApplicationCredentialSecret,
		//// AllowReauth should be set to true if you grant permission for Gophercloud to
		//// cache your credentials in memory, and to allow Gophercloud to attempt to
		//// re-authenticate automatically if/when your token expires.
		AllowReauth: true,
		TenantName:  credentials.TenantName,
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: credentials.Insecure, // #nosec: G402 -- Can be parameterized.
	}
	if len(credentials.CACert) > 0 {
		pool := x509.NewCertPool()
		ok := pool.AppendCertsFromPEM([]byte(credentials.CACert))
		if !ok {
			return nil, fmt.Errorf("failed to load CA Bundle for KeyStone")
		}
		tlsConfig.RootCAs = pool
	}

	//if opts.AuthInfo.ApplicationCredentialSecret != "" {
	//	opts.AuthType = clientconfig.AuthV3ApplicationCredential
	//}

	transport := &http.Transport{Proxy: http.ProxyFromEnvironment, TLSClientConfig: tlsConfig}
	httpClient := http.Client{
		Transport: transport,
	}

	provider, err := config.NewProviderClient(
		context.Background(),
		authOpts,
		config.WithTLSConfig(tlsConfig),
		config.WithHTTPClient(httpClient))
	if err != nil {
		panic(err)
	}

	err = openstack.Authenticate(context.Background(), provider, authOpts)
	if err != nil {
		return nil, err
	}

	return &OpenstackClientFactory{
		providerClient: provider,
	}, nil
}

// NewOpenStackClientFromSecretRef returns a Factory implementation that can be used to create clients for OpenStack services.
// The credentials are fetched from the Kubernetes secret referenced by <secretRef>.
func NewOpenStackClientFromSecretRef(ctx context.Context, c client.Client, secretRef corev1.SecretReference, keyStoneUrl *string) (Factory, error) {
	creds, err := os.GetCredentials(ctx, c, secretRef, false)
	if err != nil {
		return nil, err
	}

	if len(strings.TrimSpace(creds.AuthURL)) == 0 && keyStoneUrl != nil {
		creds.AuthURL = *keyStoneUrl
	}
	return NewOpenstackClientFromCredentials(creds)
}

// WithRegion returns an Option that can modify the region a client targets.
func WithRegion(region string) Option {
	return func(opts gophercloud.EndpointOpts) gophercloud.EndpointOpts {
		opts.Region = region
		return opts
	}
}

// Storage returns a Storage client. The client uses Swift v1 API for issuing calls.
func (oc *OpenstackClientFactory) Storage(options ...Option) (Storage, error) {
	eo := gophercloud.EndpointOpts{}
	for _, opt := range options {
		eo = opt(eo)
	}
	storageClient, err := openstack.NewObjectStorageV1(oc.providerClient, eo)
	if err != nil {
		return nil, err
	}

	return &StorageClient{
		client: storageClient,
	}, nil
}

// Compute returns a Compute client. The client uses Nova v2 API for issuing calls.
func (oc *OpenstackClientFactory) Compute(options ...Option) (Compute, error) {
	eo := gophercloud.EndpointOpts{}
	for _, opt := range options {
		eo = opt(eo)
	}

	client, err := openstack.NewComputeV2(oc.providerClient, eo)
	if err != nil {
		return nil, err
	}

	return &ComputeClient{
		client: client,
	}, nil
}

// DNS returns a DNS client. The client uses Designate v2 API for issuing calls.
func (oc *OpenstackClientFactory) DNS(options ...Option) (DNS, error) {
	eo := gophercloud.EndpointOpts{}
	for _, opt := range options {
		eo = opt(eo)
	}

	client, err := openstack.NewDNSV2(oc.providerClient, eo)
	if err != nil {
		return nil, err
	}

	return &DNSClient{
		client: client,
	}, nil
}

// Networking returns a Networking client. The client uses Neutron v2 API for issuing calls.
func (oc *OpenstackClientFactory) Networking(options ...Option) (Networking, error) {
	eo := gophercloud.EndpointOpts{}
	for _, opt := range options {
		eo = opt(eo)
	}

	client, err := openstack.NewNetworkV2(oc.providerClient, eo)
	if err != nil {
		return nil, err
	}

	return &NetworkingClient{
		client: client,
	}, nil
}

// Loadbalancing creates a Loadbalancing client.
func (oc *OpenstackClientFactory) Loadbalancing(options ...Option) (Loadbalancing, error) {
	eo := gophercloud.EndpointOpts{}
	for _, opt := range options {
		eo = opt(eo)
	}

	client, err := openstack.NewLoadBalancerV2(oc.providerClient, eo)
	if err != nil {
		return nil, err
	}

	return &LoadbalancingClient{
		client: client,
	}, nil
}

// SharedFilesystem creates a new Manila client.
func (oc *OpenstackClientFactory) SharedFilesystem(options ...Option) (SharedFilesystem, error) {
	eo := gophercloud.EndpointOpts{}
	for _, opt := range options {
		eo = opt(eo)
	}

	client, err := openstack.NewSharedFileSystemV2(oc.providerClient, eo)
	if err != nil {
		return nil, err
	}

	return &SharedFilesystemClient{
		client: client,
	}, nil
}

// Images creates a Images client
func (oc *OpenstackClientFactory) Images(options ...Option) (Images, error) {
	eo := gophercloud.EndpointOpts{}
	for _, opt := range options {
		eo = opt(eo)
	}

	client, err := openstack.NewImageV2(oc.providerClient, eo)
	if err != nil {
		return nil, err
	}

	return &ImageClient{
		client: client,
	}, nil
}

// IsNotFoundError checks if an error returned by OpenStack is caused by HTTP 404 status code.
func IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	if gophercloud.ResponseCodeIs(err, http.StatusNotFound) {
		return true
	}

	return false
}

// IgnoreNotFoundError ignore not found error
func IgnoreNotFoundError(err error) error {
	if IsNotFoundError(err) {
		return nil
	}
	return err
}
