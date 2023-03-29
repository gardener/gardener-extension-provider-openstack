// Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"github.com/gophercloud/gophercloud/openstack/sharedfilesystems/v2/sharenetworks"
	"github.com/gophercloud/gophercloud/openstack/sharedfilesystems/v2/shares"
)

// CreateShareNetwork creates a share network.
func (c *SharedFileSystemClient) CreateShareNetwork(opts sharenetworks.CreateOpts) (*sharenetworks.ShareNetwork, error) {
	return sharenetworks.Create(c.client, opts).Extract()
}

// ListShareNetworks lists share networks.
func (c *SharedFileSystemClient) ListShareNetworks(listOpts sharenetworks.ListOpts) ([]sharenetworks.ShareNetwork, error) {
	page, err := sharenetworks.ListDetail(c.client, listOpts).AllPages()
	if err != nil {
		return nil, err
	}
	return sharenetworks.ExtractShareNetworks(page)
}

// UpdateShareNetwork updates a share network.
func (c *SharedFileSystemClient) UpdateShareNetwork(networkID string, updateOpts sharenetworks.UpdateOpts) (*sharenetworks.ShareNetwork, error) {
	return sharenetworks.Update(c.client, networkID, updateOpts).Extract()
}

// GetShareNetwork gets share network by ID.
func (c *SharedFileSystemClient) GetShareNetwork(networkID string) (*sharenetworks.ShareNetwork, error) {
	return sharenetworks.Get(c.client, networkID).Extract()
}

// GetShareNetworksByName gets share network by name.
func (c *SharedFileSystemClient) GetShareNetworksByName(name string) ([]sharenetworks.ShareNetwork, error) {
	listOpts := sharenetworks.ListOpts{
		Name: name,
	}
	return c.ListShareNetworks(listOpts)
}

// DeleteShareNetwork deletess share network by ID.
func (c *SharedFileSystemClient) DeleteShareNetwork(networkID string) error {
	return sharenetworks.Delete(c.client, networkID).ExtractErr()
}

// ListShares lists shares.
func (c *SharedFileSystemClient) ListShares(listOpts shares.ListOpts) ([]shares.Share, error) {
	page, err := shares.ListDetail(c.client, listOpts).AllPages()
	if err != nil {
		return nil, err
	}
	return shares.ExtractShares(page)
}

// DeleteShare deletes share.
func (c *SharedFileSystemClient) DeleteShare(shareID string) error {
	return shares.Delete(c.client, shareID).ExtractErr()
}
