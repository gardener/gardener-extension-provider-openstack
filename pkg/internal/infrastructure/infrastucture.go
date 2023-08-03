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
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/gophercloud/gophercloud/openstack/loadbalancer/v2/loadbalancers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/layer3/routers"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
)

const (
	servicePrefix = "kube_service_"
)

func CleanupKubernetesLoadbalancers(ctx context.Context, log logr.Logger, client openstackclient.Loadbalancing, subnetID, clusterName string) error {
	lbList, err := client.ListLoadbalancers(loadbalancers.ListOpts{
		VipSubnetID: subnetID,
	})

	// do we need that if we anyway want to delete the gardener managed subnet ?
	k8sSvcPrefix := servicePrefix + clusterName
	res := make(chan error, len(lbList))
	acceptableStates := map[string]struct{}{
		"ACTIVE": {},
		"ERROR":  {},
	}
	w := sync.WaitGroup{}
	b := wait.Backoff{
		Duration: 1 * time.Second,
		Jitter:   1.2,
		Steps:    10,
	}
	for _, lb := range lbList {
		lb := lb
		if !strings.HasPrefix(lb.Name, k8sSvcPrefix) {
			continue
		}

		if _, ok := acceptableStates[lb.ProvisioningStatus]; !ok {
			return fmt.Errorf("load balancer %s can't be updated currently due to provisioning state: %s", lb.ID, lb.ProvisioningStatus)
		}

		log.Info("deleting orphan loadbalancer", "ID", lb.ID, "name", lb.Name)
		w.Add(1)
		go func() {
			defer w.Done()
			if err := client.DeleteLoadbalancer(lb.ID, loadbalancers.DeleteOpts{Cascade: true}); err != nil {
				res <- err
				return
			}

			err := wait.ExponentialBackoffWithContext(ctx, b, func() (done bool, err error) {
				lb, err := client.GetLoadbalancer(lb.ID)
				if err != nil {
					return false, err
				}
				if lb == nil {
					return true, nil
				}
				return false, nil
			})
			if err != nil {
				res <- fmt.Errorf("failed to ensure loadbalancers are deleted: %v", err)
			}
		}()
	}
	w.Wait()
	close(res)
	for errIn := range res {
		err = errors.Join(err, errIn)
	}
	return err
}

// CleanupKubernetesRoutes deletes all routes from the router which have a nextHop in the subnet.
func CleanupKubernetesRoutes(_ context.Context, client openstackclient.Networking, routerID, workers string) error {
	router, err := client.GetRouterByID(routerID)
	if err != nil {
		return err
	}

	if router == nil {
		return nil
	}

	routes := []routers.Route{}
	_, workersNet, err := net.ParseCIDR(workers)
	if err != nil {
		return err
	}

	for _, route := range router.Routes {
		ipNode, _, err := net.ParseCIDR(route.NextHop + "/32")
		if err != nil {
			return err
		}
		if !workersNet.Contains(ipNode) {
			routes = append(routes, route)
		}
	}

	// return early if no changes were made
	if len(router.Routes) == len(routes) {
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
