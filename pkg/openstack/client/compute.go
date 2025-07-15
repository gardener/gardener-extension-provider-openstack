// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"fmt"

	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/keypairs"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servergroups"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/v2/openstack/image/v2/images"
)

const (
	// ServerGroupPolicyAntiAffinity is a constant for the anti-affinity server group policy.
	ServerGroupPolicyAntiAffinity = "anti-affinity"
	// ServerGroupPolicyAffinity is a constant for the affinity server group policy.
	ServerGroupPolicyAffinity = "affinity"

	// softPolicyMicroversion defines the minimum API microversion for Nova that can support soft-* policy variants for server groups.
	// We set the minimum supported microversion, since later versions (>=2.64) have non-backwards-compatible changes forcing the use of
	// a new field to set the policy.
	//
	// See:
	// https://docs.openstack.org/api-guide/compute/microversions.html
	// https://docs.openstack.org/api-ref/compute/?expanded=create-server-group-detail#create-server-group
	softPolicyMicroversion = "2.15"
)

// CreateServerGroup creates a server group with the specified policy.
func (c *ComputeClient) CreateServerGroup(ctx context.Context, name, policy string) (*servergroups.ServerGroup, error) {
	if policy != ServerGroupPolicyAffinity && policy != ServerGroupPolicyAntiAffinity {
		c.client.Microversion = softPolicyMicroversion
	}

	createOpts := servergroups.CreateOpts{
		Name:     name,
		Policies: []string{policy},
	}

	return servergroups.Create(ctx, c.client, createOpts).Extract()
}

// GetServerGroup retrieves the server group with the specified id.
func (c *ComputeClient) GetServerGroup(ctx context.Context, id string) (*servergroups.ServerGroup, error) {
	return servergroups.Get(ctx, c.client, id).Extract()
}

// DeleteServerGroup deletes the server group with the specified id. It returns nil if the server group could not be found.
func (c *ComputeClient) DeleteServerGroup(ctx context.Context, id string) error {
	err := servergroups.Delete(ctx, c.client, id).ExtractErr()
	if err != nil && !IsNotFoundError(err) {
		return err
	}

	return nil
}

// ListServerGroups retrieves the list of server groups.
func (c *ComputeClient) ListServerGroups(ctx context.Context) ([]servergroups.ServerGroup, error) {
	pages, err := servergroups.List(c.client, nil).AllPages(ctx)
	if err != nil {
		return nil, err
	}

	return servergroups.ExtractServerGroups(pages)
}

// CreateServer retrieves the Create of Compute service.
func (c *ComputeClient) CreateServer(ctx context.Context, createOpts servers.CreateOpts) (*servers.Server, error) {
	return servers.Create(ctx, c.client, createOpts, nil).Extract()
}

// DeleteServer delete the Compute service.
func (c *ComputeClient) DeleteServer(ctx context.Context, id string) error {
	return servers.Delete(ctx, c.client, id).ExtractErr()
}

// FindServersByName retrieves the Compute Server by Name
func (c *ComputeClient) FindServersByName(ctx context.Context, name string) ([]servers.Server, error) {
	listOpts := servers.ListOpts{
		Name: name,
	}
	allPages, err := servers.List(c.client, listOpts).AllPages(ctx)
	if err != nil {
		return nil, err
	}

	allServers, err := servers.ExtractServers(allPages)
	if err != nil {
		return nil, err
	}
	return allServers, nil
}

// FindFlavorID find flavor ID by flavor name.
func (c *ComputeClient) FindFlavorID(ctx context.Context, name string) (string, error) {
	// unfortunately, there is no way to filter by name
	allPages, err := flavors.ListDetail(c.client, nil).AllPages(ctx)
	if err != nil {
		return "", fmt.Errorf("unable to list flavors: %w", err)
	}

	// Extract and filter
	allFlavors, err := flavors.ExtractFlavors(allPages)
	if err != nil {
		return "", fmt.Errorf("unable to extract flavors: %w", err)
	}

	for _, flavor := range allFlavors {
		if flavor.Name == name {
			return flavor.ID, nil
		}
	}

	return "", fmt.Errorf("flavor with name %q not found", name)
}

// FindImages find image ID by images name.
func (c *ComputeClient) FindImages(ctx context.Context, name string) ([]images.Image, error) {
	listOpts := images.ListOpts{
		Name: name,
	}
	return c.ListImages(ctx, listOpts)
}

// ListImages list all images.
func (c *ComputeClient) ListImages(ctx context.Context, listOpts images.ListOpts) ([]images.Image, error) {
	allPages, err := images.List(c.client, listOpts).AllPages(ctx)
	if err != nil {
		return nil, err
	}
	return images.ExtractImages(allPages)
}

// FindImageByID returns the image with the given ID. It returns nil if the image is not found.
func (c *ComputeClient) FindImageByID(ctx context.Context, id string) (*images.Image, error) {
	image, err := images.Get(ctx, c.client, id).Extract()
	return image, IgnoreNotFoundError(err)
}

// CreateKeyPair creates an SSH key pair
func (c *ComputeClient) CreateKeyPair(ctx context.Context, name, publicKey string) (*keypairs.KeyPair, error) {
	opts := keypairs.CreateOpts{
		Name:      name,
		PublicKey: publicKey,
	}
	return keypairs.Create(ctx, c.client, opts).Extract()
}

// GetKeyPair gets an SSH key pair by name
func (c *ComputeClient) GetKeyPair(ctx context.Context, name string) (*keypairs.KeyPair, error) {
	keypair, err := keypairs.Get(ctx, c.client, name, nil).Extract()
	return keypair, IgnoreNotFoundError(err)
}

// DeleteKeyPair deletes an SSH key pair by name
func (c *ComputeClient) DeleteKeyPair(ctx context.Context, name string) error {
	return keypairs.Delete(ctx, c.client, name, nil).ExtractErr()
}
