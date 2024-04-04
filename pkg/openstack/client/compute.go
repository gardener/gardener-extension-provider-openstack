// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/floatingips"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/keypairs"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/servergroups"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/images"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	flavorutils "github.com/gophercloud/utils/openstack/compute/v2/flavors"
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
func (c *ComputeClient) CreateServerGroup(name, policy string) (*servergroups.ServerGroup, error) {
	if policy != ServerGroupPolicyAffinity && policy != ServerGroupPolicyAntiAffinity {
		c.client.Microversion = softPolicyMicroversion
	}

	createOpts := servergroups.CreateOpts{
		Name:     name,
		Policies: []string{policy},
	}

	return servergroups.Create(c.client, createOpts).Extract()
}

// GetServerGroup retrieves the server group with the specified id.
func (c *ComputeClient) GetServerGroup(id string) (*servergroups.ServerGroup, error) {
	return servergroups.Get(c.client, id).Extract()
}

// DeleteServerGroup deletes the server group with the specified id. It returns nil if the server group could not be found.
func (c *ComputeClient) DeleteServerGroup(id string) error {
	err := servergroups.Delete(c.client, id).ExtractErr()
	if err != nil && !IsNotFoundError(err) {
		return err
	}

	return nil
}

// ListServerGroups retrieves the list of server groups.
func (c *ComputeClient) ListServerGroups() ([]servergroups.ServerGroup, error) {
	pages, err := servergroups.List(c.client, nil).AllPages()
	if err != nil {
		return nil, err
	}

	return servergroups.ExtractServerGroups(pages)
}

// CreateServer retrieves the Create of Compute service.
func (c *ComputeClient) CreateServer(createOpts servers.CreateOpts) (*servers.Server, error) {
	return servers.Create(c.client, createOpts).Extract()
}

// DeleteServer delete the Compute service.
func (c *ComputeClient) DeleteServer(id string) error {
	return servers.Delete(c.client, id).ExtractErr()
}

// FindServersByName retrieves the Compute Server by Name
func (c *ComputeClient) FindServersByName(name string) ([]servers.Server, error) {
	listOpts := servers.ListOpts{
		Name: name,
	}
	allPages, err := servers.List(c.client, listOpts).AllPages()
	if err != nil {
		return nil, err
	}

	allServers, err := servers.ExtractServers(allPages)
	if err != nil {
		return nil, err
	}
	return allServers, nil
}

// AssociateFIPWithInstance associate floating ip with instance
func (c *ComputeClient) AssociateFIPWithInstance(serverID string, associateOpts floatingips.AssociateOpts) error {
	return floatingips.AssociateInstance(c.client, serverID, associateOpts).ExtractErr()
}

// FindFloatingIDByInstanceID find floating id by instance id
func (c *ComputeClient) FindFloatingIDByInstanceID(id string) (string, error) {
	allPages, err := floatingips.List(c.client).AllPages()
	if err != nil {
		return "", err
	}

	allFloatingIPs, err := floatingips.ExtractFloatingIPs(allPages)
	if err != nil {
		return "", err
	}

	for _, fip := range allFloatingIPs {
		if fip.InstanceID == id {
			return fip.ID, nil
		}
	}
	return "", nil
}

// FindFlavorID find flavor ID by flavor name
func (c *ComputeClient) FindFlavorID(name string) (string, error) {
	return flavorutils.IDFromName(c.client, name)
}

// FindImages find image ID by images name
func (c *ComputeClient) FindImages(name string) ([]images.Image, error) {
	listOpts := images.ListOpts{
		Name: name,
	}
	return c.ListImages(listOpts)
}

// ListImages list all images
func (c *ComputeClient) ListImages(listOpts images.ListOpts) ([]images.Image, error) {
	allPages, err := images.ListDetail(c.client, listOpts).AllPages()
	if err != nil {
		return nil, err
	}
	return images.ExtractImages(allPages)
}

// FindImageByID returns the image with the given ID. It returns nil if the image is not found.
func (c *ComputeClient) FindImageByID(id string) (*images.Image, error) {
	image, err := images.Get(c.client, id).Extract()
	return image, IgnoreNotFoundError(err)
}

// CreateKeyPair creates an SSH key pair
func (c *ComputeClient) CreateKeyPair(name, publicKey string) (*keypairs.KeyPair, error) {
	opts := keypairs.CreateOpts{
		Name:      name,
		PublicKey: publicKey,
	}
	return keypairs.Create(c.client, opts).Extract()
}

// GetKeyPair gets an SSH key pair by name
func (c *ComputeClient) GetKeyPair(name string) (*keypairs.KeyPair, error) {
	keypair, err := keypairs.Get(c.client, name, nil).Extract()
	return keypair, IgnoreNotFoundError(err)
}

// DeleteKeyPair deletes an SSH key pair by name
func (c *ComputeClient) DeleteKeyPair(name string) error {
	return keypairs.Delete(c.client, name, nil).ExtractErr()
}
