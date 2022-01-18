// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"context"

	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/external"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"k8s.io/utils/pointer"
)

type networkWithExternalExt struct {
	networks.Network
	external.NetworkExternalExt
}

// GetExternalNetworkNames returns a list of all external network names.
func (c *NetworkingClient) GetExternalNetworkNames(_ context.Context) ([]string, error) {
	allPages, err := networks.List(c.client, external.ListOptsExt{
		ListOptsBuilder: networks.ListOpts{},
		External:        pointer.Bool(true),
	}).AllPages()
	if err != nil {
		return nil, err
	}

	var externalNetworks []networkWithExternalExt
	err = networks.ExtractNetworksInto(allPages, &externalNetworks)
	if err != nil {
		return nil, err
	}

	var externalNetworkNames []string
	for _, externalNetwork := range externalNetworks {
		externalNetworkNames = append(externalNetworkNames, externalNetwork.Name)
	}
	return externalNetworkNames, nil
}
