// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/gophercloud/gophercloud/v2/openstack/loadbalancer/v2/loadbalancers"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/layer3/routers"
	"k8s.io/apimachinery/pkg/util/wait"
	netutils "k8s.io/utils/net"
)

const (
	servicePrefix = "kube_service_"
)

// ErrorMultipleMatches is returned when the findExisting finds multiple resources matching a name.
var ErrorMultipleMatches = fmt.Errorf("error multiple matches")

func findExisting[T any](ctx context.Context, id *string, name string,
	getter func(ctx context.Context, id string) (*T, error),
	finder func(ctx context.Context, name string) ([]*T, error)) (*T, error) {
	if id != nil {
		found, err := getter(ctx, *id)
		if err != nil {
			return nil, err
		}
		if found != nil {
			return found, nil
		}
	}

	found, err := finder(ctx, name)
	if err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, nil
	}
	if len(found) > 1 {
		return nil, fmt.Errorf("%w: found %d matches for name %q", ErrorMultipleMatches, len(found), name)
	}
	return found[0], nil
}

func sliceToPtr[T any](slice []T) []*T {
	res := make([]*T, 0)
	for _, t := range slice {
		res = append(res, &t)
	}
	return res
}

// ComputeEgressCIDRs converts an IP to a CIDR depending on the IP family.
func ComputeEgressCIDRs(ips []string) []string {
	var result []string
	for _, ip := range ips {
		switch {
		case netutils.IsIPv4String(ip):
			result = append(result, fmt.Sprintf("%s/32", ip))
		case netutils.IsIPv6String(ip):
			result = append(result, fmt.Sprintf("%s/128", ip))
		}
	}
	return result
}

// CleanupKubernetesLoadbalancers cleans loadbalancers that could prevent shoot deletion from proceeding. Particularly it tries to prevent orphan ports from blocking subnet deletion.
// It filters for LBs that bear the "kube_service" prefix along with the cluster name.
// Note that this deletion may still leave some leftover resources like the floating IPs. This is intentional because the users may want to preserve them but without the k8s
// service object we cannot decide that - therefore the floating IPs will be untouched.
func (fctx *FlowContext) cleanupKubernetesLoadbalancers(ctx context.Context, log logr.Logger, subnetID string) error {
	lbList, err := fctx.loadbalancing.ListLoadbalancers(ctx, loadbalancers.ListOpts{
		VipSubnetID: subnetID,
	})

	var (
		clusterName = fctx.infra.Namespace
		// do we need that if we anyway want to delete the gardener managed subnet ?
		k8sSvcPrefix     = servicePrefix + clusterName
		res              = make(chan error, len(lbList))
		acceptableStates = map[string]struct{}{
			"ACTIVE": {},
			"ERROR":  {},
		}
		w = sync.WaitGroup{}
		b = wait.Backoff{
			Duration: 1 * time.Second,
			Jitter:   1.2,
			Steps:    10,
		}
	)

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
			if err := fctx.loadbalancing.DeleteLoadbalancer(ctx, lb.ID, loadbalancers.DeleteOpts{Cascade: true}); err != nil {
				res <- err
				return
			}

			err := wait.ExponentialBackoffWithContext(ctx, b, func(_ context.Context) (done bool, err error) {
				lb, err := fctx.loadbalancing.GetLoadbalancer(ctx, lb.ID)
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

// cleanupKubernetesRoutes deletes all routes from the router which have a nextHop in the subnet.
func (fctx *FlowContext) cleanupKubernetesRoutes(ctx context.Context, routerID string) error {
	router, err := fctx.networking.GetRouterByID(ctx, routerID)
	if err != nil {
		return err
	}

	if router == nil {
		return nil
	}

	var routes []routers.Route
	_, workersNet, err := net.ParseCIDR(fctx.workersCIDR())
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

	if _, err := fctx.networking.UpdateRoutesForRouter(ctx, routes, routerID); err != nil {
		return err
	}
	return nil
}
func (fctx *FlowContext) defaultRouterName() string {
	return fctx.infra.Namespace
}

func (fctx *FlowContext) defaultSSHKeypairName() string {
	return fctx.infra.Namespace
}

func (fctx *FlowContext) defaultNetworkName() string {
	return fctx.infra.Namespace
}

func (fctx *FlowContext) defaultSubnetName() string {
	return fctx.infra.Namespace
}

func (fctx *FlowContext) defaultSecurityGroupName() string {
	return fctx.infra.Namespace
}

func (fctx *FlowContext) defaultSharedNetworkName() string {
	return fctx.infra.Namespace
}

func (fctx *FlowContext) workersCIDR() string {
	return fctx.config.WorkersCIDR()
}
