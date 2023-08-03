//  Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"github.com/gophercloud/gophercloud/openstack/loadbalancer/v2/loadbalancers"
)

// ListLoadbalancers returns a list of all loadbalancers info by listOpts
func (c *LoadbalancingClient) ListLoadbalancers(listOpts loadbalancers.ListOpts) ([]loadbalancers.LoadBalancer, error) {
	pages, err := loadbalancers.List(c.client, listOpts).AllPages()
	if err != nil {
		return nil, err
	}

	return loadbalancers.ExtractLoadBalancers(pages)
}

// DeleteLoadbalancer deletes the loadbalancer with the specified ID.
func (c *LoadbalancingClient) DeleteLoadbalancer(id string, opts loadbalancers.DeleteOpts) error {
	err := loadbalancers.Delete(c.client, id, opts).ExtractErr()
	if err != nil && !IsNotFoundError(err) {
		return err
	}
	return nil
}

// GetLoadbalancer returns the loadbalancer with the specified ID.
func (c *LoadbalancingClient) GetLoadbalancer(id string) (*loadbalancers.LoadBalancer, error) {
	lb, err := loadbalancers.Get(c.client, id).Extract()
	if err != nil && !IsNotFoundError(err) {
		return nil, err
	}
	if IsNotFoundError(err) {
		return nil, nil
	}
	return lb, nil
}
