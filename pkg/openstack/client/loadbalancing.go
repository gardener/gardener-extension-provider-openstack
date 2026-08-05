// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"

	"github.com/gophercloud/gophercloud/v2/openstack/loadbalancer/v2/loadbalancers"
	"github.com/gophercloud/gophercloud/v2/openstack/loadbalancer/v2/pools"
)

// ListLoadbalancers returns a list of all loadbalancers info by listOpts
func (c *LoadbalancingClient) ListLoadbalancers(ctx context.Context, listOpts loadbalancers.ListOpts) ([]loadbalancers.LoadBalancer, error) {
	pages, err := loadbalancers.List(c.client, listOpts).AllPages(ctx)
	if err != nil {
		return nil, err
	}
	return loadbalancers.ExtractLoadBalancers(pages)
}

// CreateLoadbalancer creates a new loadbalancer. The passed opts may contain a fully-populated
// tree (listener -> pool -> health monitor -> members) which Octavia provisions atomically.
func (c *LoadbalancingClient) CreateLoadbalancer(ctx context.Context, opts loadbalancers.CreateOpts) (*loadbalancers.LoadBalancer, error) {
	return loadbalancers.Create(ctx, c.client, opts).Extract()
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

// ListPools returns the pools matching the given listOpts.
func (c *LoadbalancingClient) ListPools(ctx context.Context, opts pools.ListOpts) ([]pools.Pool, error) {
	pages, err := pools.List(c.client, opts).AllPages(ctx)
	if err != nil {
		return nil, err
	}
	return pools.ExtractPools(pages)
}

// BatchUpdatePoolMembers reconciles the member set of the given pool to exactly match opts.
// Octavia adds new members, removes stale ones and updates changed ones in a single call.
func (c *LoadbalancingClient) BatchUpdatePoolMembers(ctx context.Context, poolID string, opts []pools.BatchUpdateMemberOpts) error {
	return pools.BatchUpdateMembers(ctx, c.client, poolID, opts).ExtractErr()
}
