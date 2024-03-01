// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
