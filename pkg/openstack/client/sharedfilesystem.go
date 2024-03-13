// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"github.com/gophercloud/gophercloud/openstack/sharedfilesystems/v2/sharenetworks"
)

// CreateShareNetwork creates the share network.
func (c *SharedFilesystemClient) CreateShareNetwork(createOpts sharenetworks.CreateOpts) (*sharenetworks.ShareNetwork, error) {
	return sharenetworks.Create(c.client, createOpts).Extract()
}

// ListShareNetworks returns a list of share networks
func (c *SharedFilesystemClient) ListShareNetworks(listOpts sharenetworks.ListOpts) ([]sharenetworks.ShareNetwork, error) {
	page, err := sharenetworks.ListDetail(c.client, listOpts).AllPages()
	if err != nil {
		return nil, err
	}
	return sharenetworks.ExtractShareNetworks(page)
}

// DeleteShareNetwork deletes a share network by identifier
func (c *SharedFilesystemClient) DeleteShareNetwork(id string) error {
	return sharenetworks.Delete(c.client, id).ExtractErr()
}

// GetShareNetwork returns a share network by identifier
func (c *SharedFilesystemClient) GetShareNetwork(id string) (*sharenetworks.ShareNetwork, error) {
	sn, err := sharenetworks.Get(c.client, id).Extract()
	if err != nil && !IsNotFoundError(err) {
		return nil, err
	}
	if IsNotFoundError(err) {
		return nil, nil
	}
	return sn, nil
}
