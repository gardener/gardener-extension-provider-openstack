// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"

	"github.com/gophercloud/gophercloud/v2/openstack/sharedfilesystems/v2/sharenetworks"
)

// CreateShareNetwork creates the share network.
func (c *SharedFilesystemClient) CreateShareNetwork(ctx context.Context, createOpts sharenetworks.CreateOpts) (*sharenetworks.ShareNetwork, error) {
	return sharenetworks.Create(ctx, c.client, createOpts).Extract()
}

// ListShareNetworks returns a list of share networks
func (c *SharedFilesystemClient) ListShareNetworks(ctx context.Context, listOpts sharenetworks.ListOpts) ([]sharenetworks.ShareNetwork, error) {
	page, err := sharenetworks.ListDetail(c.client, listOpts).AllPages(ctx)
	if err != nil {
		return nil, err
	}
	return sharenetworks.ExtractShareNetworks(page)
}

// DeleteShareNetwork deletes a share network by identifier
func (c *SharedFilesystemClient) DeleteShareNetwork(ctx context.Context, id string) error {
	return sharenetworks.Delete(ctx, c.client, id).ExtractErr()
}

// GetShareNetwork returns a share network by identifier
func (c *SharedFilesystemClient) GetShareNetwork(ctx context.Context, id string) (*sharenetworks.ShareNetwork, error) {
	sn, err := sharenetworks.Get(ctx, c.client, id).Extract()
	if err != nil && !IsNotFoundError(err) {
		return nil, err
	}
	if IsNotFoundError(err) {
		return nil, nil
	}
	return sn, nil
}
