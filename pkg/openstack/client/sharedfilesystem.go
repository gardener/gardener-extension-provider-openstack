//  Copyright (c) 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

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
