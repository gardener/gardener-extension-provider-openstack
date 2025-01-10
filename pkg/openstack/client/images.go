// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"

	"github.com/gophercloud/gophercloud/v2/openstack/image/v2/images"
)

// ListImages lists all images filtered by listOpts
func (c *ImageClient) ListImages(ctx context.Context, opts images.ListOpts) ([]images.Image, error) {
	pages, err := images.List(c.client, opts).AllPages(ctx)
	if err != nil {
		return nil, err
	}

	return images.ExtractImages(pages)
}
