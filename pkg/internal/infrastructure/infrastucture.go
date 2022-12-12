// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package infrastructure

import (
	"context"
	"net"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"

	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/layer3/routers"
)

// CleanupKubernetesRoutes deletes all routes from the router which have a nextHop in the subnet.
func CleanupKubernetesRoutes(ctx context.Context, client openstackclient.Networking, routerID, workers string) error {

	router, err := client.GetRouterByID(routerID)
	if err != nil {
		return err
	}

	if len(router) == 0 {
		return nil
	}

	routes := []routers.Route{}
	_, workersNet, err := net.ParseCIDR(workers)
	if err != nil {
		return err
	}

	for _, route := range router[0].Routes {
		ipNode, _, err := net.ParseCIDR(route.NextHop + "/32")
		if err != nil {
			return err
		}
		if !workersNet.Contains(ipNode) {
			routes = append(routes, route)
		}
	}

	// return early if no changes were made
	if len(router[0].Routes) == len(routes) {
		return nil
	}

	if _, err := client.UpdateRoutesForRouter(routes, routerID); err != nil {
		return err
	}
	return nil
}

// WorkersCIDR determines the Workers CIDR from the given InfrastructureConfig.
func WorkersCIDR(config *openstack.InfrastructureConfig) string {
	if config == nil {
		return ""
	}

	workersCIDR := config.Networks.Workers
	// Backwards compatibility - remove this code in a future version.
	if workersCIDR == "" {
		workersCIDR = config.Networks.Worker
	}

	return workersCIDR
}
