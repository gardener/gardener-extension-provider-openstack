// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/gophercloud/gophercloud/openstack/loadbalancer/v2/loadbalancers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/layer3/routers"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	openstackv1alpha1 "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/utils"
)

const (
	servicePrefix = "kube_service_"
)

// CleanupKubernetesLoadbalancers cleans loadbalancers that could prevent shoot deletion from proceeding. Particularly it tries to prevent orphan ports from blocking subnet deletion.
// It filters for LBs that bear the "kube_service" prefix along with the cluster name.
// Note that this deletion may still leave some leftover resources like the floating IPs. This is intentional because the users may want to preserve them but without the k8s
// service object we cannot decide that - therefore the floating IPs will be untouched.
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

			err := wait.ExponentialBackoffWithContext(ctx, b, func(_ context.Context) (done bool, err error) {
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

// PatchProviderStatusAndState patches the infrastructure status with the given provider specific status and state.
func PatchProviderStatusAndState(
	ctx context.Context,
	runtimeClient client.Client,
	infra *extensionsv1alpha1.Infrastructure,
	status *openstackv1alpha1.InfrastructureStatus,
	state *runtime.RawExtension,
) error {
	patch := client.MergeFrom(infra.DeepCopy())
	if status != nil {
		infra.Status.ProviderStatus = &runtime.RawExtension{Object: status}
		infra.Status.EgressCIDRs = utils.ComputeEgressCIDRs(status.Networks.Router.ExternalFixedIPs)
	}

	if state != nil {
		infra.Status.State = state
	}

	// do not make a patch request if nothing has changed.
	if data, err := patch.Data(infra); err != nil {
		return fmt.Errorf("failed getting patch data for infra %s: %w", infra.Name, err)
	} else if string(data) == `{}` {
		return nil
	}

	return runtimeClient.Status().Patch(ctx, infra, patch)
}
