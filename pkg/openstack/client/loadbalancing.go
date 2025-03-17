// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"

	"github.com/gophercloud/gophercloud/v2/openstack/loadbalancer/v2/loadbalancers"
)

// ListLoadbalancers returns a list of all loadbalancers info by listOpts
func (c *LoadbalancingClient) ListLoadbalancers(ctx context.Context, listOpts loadbalancers.ListOpts) ([]loadbalancers.LoadBalancer, error) {
	pages, err := loadbalancers.List(c.client, listOpts).AllPages(ctx)
	if err != nil {
		return nil, err
	}
	return loadbalancers.ExtractLoadBalancers(pages)
}

// DeleteLoadbalancer deletes the loadbalancer with the specified ID.
func (c *LoadbalancingClient) DeleteLoadbalancer(ctx context.Context, id string, opts loadbalancers.DeleteOpts) error {
	err := loadbalancers.Delete(ctx, c.client, id, opts).ExtractErr()
	if err != nil && !IsNotFoundError(err) {
		return err
	}
	return nil
}

// GetLoadbalancer returns the loadbalancer with the specified ID.
func (c *LoadbalancingClient) GetLoadbalancer(ctx context.Context, id string) (*loadbalancers.LoadBalancer, error) {
	lb, err := loadbalancers.Get(ctx, c.client, id).Extract()
	if err != nil && !IsNotFoundError(err) {
		return nil, err
	}
	if IsNotFoundError(err) {
		return nil, nil
	}
	return lb, nil
}
