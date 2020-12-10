/*
 * Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package client

import (
	"context"

	os "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/utils/openstack/clientconfig"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewOpenstackClientFromCredentials returns a Factory implementation that can be used to create clients for OpenStack services.
func NewOpenstackClientFromCredentials(credentials *os.Credentials) (Factory, error) {
	opts := &clientconfig.ClientOpts{
		AuthInfo: &clientconfig.AuthInfo{
			AuthURL:     credentials.AuthURL,
			Username:    credentials.Username,
			Password:    credentials.Password,
			ProjectName: credentials.TenantName,
			DomainName:  credentials.DomainName,
		},
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

	return &OpenstackClientFactory{
		providerClient: provider,
	}, nil
}

// NewOpenStackClientFromSecretRef returns a Factory implementation that can be used to create clients for OpenStack services.
// The credentials are fetched from the Kubernetes secret referenced by <secretRef>.
func NewOpenStackClientFromSecretRef(ctx context.Context, c client.Client, secretRef corev1.SecretReference) (Factory, error) {
	creds, err := os.GetCredentials(ctx, c, secretRef)
	if err != nil {
		return nil, err
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

type serviceVersion struct {
	Version struct {
		Version    string `json:"version"`
		Status     string `json:"status"`
		MinVersion string `json:"min_version"`
	} `json:"version"`
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

	// discover the latest microversion of the API and set it on the client.
	// this is necessary for the creating servergroups with "soft-anti-affinity" policy. Soft variants of affinity policies,
	// are only supported in >=2.1 version of Compute API.
	// https://docs.openstack.org/api-guide/compute/microversions.html
	version := serviceVersion{}
	_, err = client.Get(client.ResourceBaseURL(), &version, nil)
	if err != nil {
		return nil, err
	}
	client.Microversion = version.Version.Version

	return &ComputeClient{
		client: client,
	}, nil
}

// IsNotFoundError checks if an error returned by OpenStack is caused by HTTP 404 status code.
func IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	if _, ok := err.(gophercloud.ErrDefault404); ok {
		return true
	}

	if _, ok := err.(gophercloud.Err404er); ok {
		return true
	}

	return false
}
