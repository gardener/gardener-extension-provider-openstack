// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack/objectstorage/v1/containers"
	"github.com/gophercloud/gophercloud/v2/openstack/objectstorage/v1/objects"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Start below Swift's default limit of 10000 objects to keep requests small.
// If Swift rejects a batch as too large, reduce it for the rest of this call.
const swiftBulkDeleteBatchSize = 1000

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
		return IgnoreNotFoundError(err)
	}

	objectList, err := objects.ExtractNames(allPages)
	if err != nil {
		return fmt.Errorf("unable to extract object names: %w", err)
	}

	// Bulk deletion returns HTTP 200 even when individual deletions fail.
	// Empty listings need no request; Swift rejects empty bulk deletions.
	batchSize := swiftBulkDeleteBatchSize
	for len(objectList) > 0 {
		batch := objectList[:min(batchSize, len(objectList))]
		response, err := objects.BulkDelete(ctx, s.client, container, batch).Extract()
		if len(batch) > 1 && (gophercloud.ResponseCodeIs(err, http.StatusRequestEntityTooLarge) ||
			(err == nil && strings.HasPrefix(response.ResponseStatus, "413 "))) {
			// Retry the same objects in smaller batches, stopping at one object.
			batchSize = len(batch) / 2
			continue
		}
		if err != nil {
			return err
		}
		if response.ResponseStatus != "200 OK" || len(response.Errors) != 0 {
			return swiftBulkDeleteError(response)
		}
		// Already absent objects count as successfully deleted. The deleted and
		// not-found counts must add up to the number of objects in this batch.
		if response.NumberDeleted < 0 || response.NumberDeleted > len(batch) || response.NumberNotFound != len(batch)-response.NumberDeleted {
			return fmt.Errorf("swift bulk delete returned inconsistent counts: %d deleted, %d not found, %d requested", response.NumberDeleted, response.NumberNotFound, len(batch))
		}
		objectList = objectList[len(batch):]
	}
	return nil
}

func swiftBulkDeleteError(response *objects.BulkDeleteResponse) error {
	// Show one example object per distinct status, up to five statuses.
	const maxErrorStatuses = 5
	seenStatuses := make(map[string]struct{})
	var details []string
	var malformedEntries int
	for _, objectError := range response.Errors {
		if len(objectError) != 2 {
			malformedEntries++
			continue
		}
		status := objectError[1]
		if _, seen := seenStatuses[status]; seen {
			continue
		}
		seenStatuses[status] = struct{}{}
		if len(details) < maxErrorStatuses {
			details = append(details, fmt.Sprintf("%q (example object %q)", status, objectError[0]))
		}
	}
	if omitted := len(seenStatuses) - len(details); omitted > 0 {
		details = append(details, fmt.Sprintf("%d additional distinct error statuses omitted", omitted))
	}
	if malformedEntries > 0 {
		details = append(details, fmt.Sprintf("%d malformed object error entries", malformedEntries))
	}
	message := fmt.Sprintf("swift bulk delete returned status %q with %d object errors", response.ResponseStatus, len(response.Errors))
	if len(details) > 0 {
		message += "; " + strings.Join(details, "; ")
	}
	return errors.New(message)
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
	if IsConflictError(err) {
		if err := s.DeleteObjectsWithPrefix(ctx, container, ""); err != nil {
			return err
		}
		// Retry once after cleanup; persistent conflicts are retried by the controller.
		_, err = containers.Delete(ctx, s.client, container).Extract()
	}
	return IgnoreNotFoundError(err)
}
