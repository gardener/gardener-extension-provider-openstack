// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack/objectstorage/v1/containers"
	"github.com/gophercloud/gophercloud/v2/openstack/objectstorage/v1/objects"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewStorageClientFromSecretRef retrieves the openstack client from specified by the secret reference.
func NewStorageClientFromSecretRef(ctx context.Context, c client.Client, secretRef corev1.SecretReference, region string) (Storage, error) {
	base, err := NewOpenStackClientFromSecretRef(ctx, c, secretRef, nil)
	if err != nil {
		return nil, err
	}

	return base.Storage(WithRegion(region))
}

// DeleteObjectsWithPrefix deletes the blob objects with the specific <prefix> from <container>. If it does not exist,
// no error is returned.
func (s *StorageClient) DeleteObjectsWithPrefix(ctx context.Context, container, prefix string) error {
	// The objectstorage/v1/containers.ListOpts#Full and objectstorage/v1/objects.ListOpts#Full
	// properties are removed from the Gophercloud API.
	// Plaintext listing is unfixably wrong and won't handle special characters reliably (i.e. \n).
	// Object listing and container listing now always behave like “Full” did.
	opts := &objects.ListOpts{
		Prefix: prefix,
	}

	allPages, err := objects.List(s.client, container, opts).AllPages(ctx)
	if err != nil {
		return err
	}

	objectList, err := objects.ExtractNames(allPages)
	if err != nil {
		return fmt.Errorf("unable to extract object names: %w", err)
	}

	// NOTE: Though there is options of bulk-delete with openstack API,
	// Gophercloud doesn't yet support the bulk delete and we are not sure whether the openstack setup has enabled
	// bulk delete support. So, here we will fetch the list of object and delete it one by one.
	// In  future if support is added to upstream, we could switch to it.
	_, err = objects.BulkDelete(ctx, s.client, container, objectList).Extract()
	return err
}

// CreateContainerIfNotExists creates the openstack blob container with name <container>. If it already exist,
// no error is returned.
func (s *StorageClient) CreateContainerIfNotExists(ctx context.Context, container string) error {
	result := containers.Create(ctx, s.client, container, nil)
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
	_, err := containers.Delete(ctx, s.client, container).Extract()
	var unexpectedErr gophercloud.ErrUnexpectedResponseCode
	if errors.As(err, &unexpectedErr) {
		switch unexpectedErr.Actual {
		case http.StatusNotFound:
			return nil
		case http.StatusConflict:
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
