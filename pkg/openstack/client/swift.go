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

package client

import (
	"context"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/internal"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/objectstorage/v1/containers"
	"github.com/gophercloud/gophercloud/openstack/objectstorage/v1/objects"
	"github.com/gophercloud/gophercloud/pagination"
	"github.com/gophercloud/utils/openstack/clientconfig"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewStorageClientFromSecretRef retrieves the openstack client from specified by the secret reference.
func NewStorageClientFromSecretRef(ctx context.Context, c client.Client, secretRef corev1.SecretReference, region string) (*StorageClient, error) {
	credentials, err := internal.GetCredentials(ctx, c, secretRef)
	if err != nil {
		return nil, err
	}

	return newStorageClientFromCredentials(credentials, region)
}

// newStorageClientFromCredentials create the storage client from credentials.
func newStorageClientFromCredentials(credentials *internal.Credentials, region string) (*StorageClient, error) {
	opts := &clientconfig.ClientOpts{
		AuthInfo: &clientconfig.AuthInfo{
			AuthURL:     credentials.AuthURL,
			Username:    credentials.Username,
			Password:    credentials.Password,
			ProjectName: credentials.TenantName,
			DomainName:  credentials.DomainName,
		},
		RegionName: region,
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
	client, err := openstack.NewObjectStorageV1(provider, gophercloud.EndpointOpts{})
	if err != nil {
		return nil, err

	}

	return &StorageClient{
		client: client,
	}, nil
}

// DeleteObjectsWithPrefix deletes the blob objects with the specific <prefix> from <container>. If it does not exist,
// no error is returned.
func (s *StorageClient) DeleteObjectsWithPrefix(ctx context.Context, container, prefix string) error {
	opts := &objects.ListOpts{
		Full:   false,
		Prefix: prefix,
	}
	// NOTE: Though there is options of bulk-delete with openstack API,
	// Gophercloud doesn't yet support the bulk delete and we are not sure whether the openstack setup has enabled
	// bulk delete support. So, here we will fetch the list of object and delete it one by one.
	// In  future if support is added to upstream, we could switch to it.

	// Retrieve a pager (i.e. a paginated collection)
	pager := objects.List(s.client, container, opts)

	return pager.EachPage(func(page pagination.Page) (bool, error) {
		objectList, err := objects.ExtractNames(page)
		if err != nil {
			return false, err
		}
		for _, object := range objectList {
			if err := s.deleteObjectIfExists(ctx, container, object); err != nil {
				return false, err
			}
		}
		return true, nil
	})
}

// deleteObjectIfExists deletes the openstack object with name <objectName> from <container>. If it does not exist,
// no error is returned.
func (s *StorageClient) deleteObjectIfExists(ctx context.Context, container, objectName string) error {
	result := objects.Delete(s.client, container, objectName, nil)
	if _, err := result.Extract(); err != nil {
		if _, ok := result.Err.(gophercloud.Err404er); ok {
			return result.Err
		}
	}
	return nil
}

// CreateContainerIfNotExists creates the openstack blob container with name <container>. If it already exist,
// no error is returned.
func (s *StorageClient) CreateContainerIfNotExists(ctx context.Context, container string) error {
	result := containers.Create(s.client, container, nil)
	if _, err := result.Extract(); err != nil {
		// Note: Openstack swift doesn't return any error if container already exists.
		// So, no special handling added here.
		return err
	}
	return nil
}

// DeleteContainerIfExists deletes the openstack blob container with name <container>. If it does not exist,
// no error is returned.
func (s *StorageClient) DeleteContainerIfExists(ctx context.Context, container string) error {
	result := containers.Delete(s.client, container)
	if _, err := result.Extract(); err != nil {
		switch result.Err.(type) {
		case gophercloud.ErrDefault404:
			return nil
		case gophercloud.ErrDefault409:
			if err := s.DeleteObjectsWithPrefix(ctx, container, ""); err != nil {
				return err
			}
			return s.DeleteContainerIfExists(ctx, container)
		default:
			return err
		}
	}
	return nil
}
