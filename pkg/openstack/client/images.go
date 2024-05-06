// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/images"
)

// ListImages lists all images filtered by listOpts
func (c *ImageClient) ListImages(listOpts images.ListOpts) ([]images.Image, error) {
	pages, err := images.List(c.client, listOpts).AllPages()
	if err != nil {
		return nil, err
	}

	return images.ExtractImages(pages)
}
