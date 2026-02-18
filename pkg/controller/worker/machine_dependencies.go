// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"context"
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servergroups"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	osclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
)

// PreReconcileHook implements genericactuator.WorkerDelegate.
func (w *WorkerDelegate) PreReconcileHook(ctx context.Context) error {
	computeClient, err := w.openstackClient.Compute(osclient.WithRegion(w.worker.Spec.Region))
	if err != nil {
		return err
	}

	workerStatus, err := w.decodeWorkerProviderStatus()
	if err != nil {
		return err
	}

	serverGroupDepSet, err := w.reconcileServerGroups(ctx, computeClient, workerStatus.DeepCopy())
	return w.updateMachineDependenciesStatus(ctx, workerStatus, serverGroupDepSet.extract(), err)
}

func (w *WorkerDelegate) reconcileServerGroups(ctx context.Context, computeClient osclient.Compute, workerStatus *api.WorkerStatus) (serverGroupDependencySet, error) {
	serverGroupDepSet := newServerGroupDependencySet(workerStatus.ServerGroupDependencies)
	for _, pool := range w.worker.Spec.Pools {
		serverGroupDependencyStatus, err := w.reconcilePoolServerGroup(ctx, computeClient, pool, serverGroupDepSet)
		if err != nil {
			return serverGroupDepSet, fmt.Errorf("reconciling server groups failed for pool %q: %w", pool.Name, err)
		}
		serverGroupDepSet.upsert(serverGroupDependencyStatus)
	}
	return serverGroupDepSet, nil
}

func (w *WorkerDelegate) reconcilePoolServerGroup(ctx context.Context, computeClient osclient.Compute, pool extensionsv1alpha1.WorkerPool, set serverGroupDependencySet) (*api.ServerGroupDependency, error) {
	poolProviderConfig, err := helper.WorkerConfigFromRawExtension(pool.ProviderConfig)
	if err != nil {
		return nil, err
	}

	if !isServerGroupRequired(poolProviderConfig) {
		return nil, nil
	}

	// Determine Kubernetes version and naming strategy
	k8sVersion, err := semver.NewVersion(ptr.Deref(pool.KubernetesVersion, w.cluster.Shoot.Spec.Kubernetes.Version))
	if err != nil {
		return nil, fmt.Errorf("failed to parse Kubernetes version for worker pool %q: %w", pool.Name, err)
	}
	forceNewNameFormat := versionutils.ConstraintK8sGreaterEqual135.Check(k8sVersion)

	// Generate expected server group names
	name, err := generateServerGroupNameV2(string(w.cluster.Shoot.GetUID()), pool.Name, poolProviderConfig.ServerGroup.Policy)
	if err != nil {
		return nil, fmt.Errorf("failed to generate server group name for worker pool %q: %w", pool.Name, err)
	}

	policyMatch := func(sg *servergroups.ServerGroup) bool {
		return sg != nil && len(sg.Policies) > 0 && sg.Policies[0] == poolProviderConfig.ServerGroup.Policy
	}

	// Check if we have a current dependency in the status
	currentPoolDependency := set.getByPoolName(pool.Name)
	if currentPoolDependency != nil {
		serverGroup, err := computeClient.GetServerGroup(ctx, currentPoolDependency.ID)
		if err != nil && !osclient.IsNotFoundError(err) {
			return nil, err
		} else if err == nil {
			// Server group exists, check if it's valid
			if forceNewNameFormat {
				// For K8s >= 1.35: must use new name format
				if serverGroup.Name == name && serverGroup.Name == currentPoolDependency.Name && policyMatch(serverGroup) {
					// Name and policy match, keep it
					return nil, nil
				}
			} else {
				// For K8s < 1.35: accept any name format, but policy must match
				if serverGroup.Name == currentPoolDependency.Name && policyMatch(serverGroup) {
					// Current server group is valid, keep it
					return nil, nil
				}
				// Policy changed, will recreate below
			}
		}
	}

	// List all server groups to find existing ones we can adopt
	serverGroupList, err := computeClient.ListServerGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list server groups for worker pool %q: %w", pool.Name, err)
	}

	for _, serverGroup := range serverGroupList {
		if forceNewNameFormat {
			if serverGroup.Name == name && policyMatch(&serverGroup) {
				return &api.ServerGroupDependency{
					PoolName: pool.Name,
					ID:       serverGroup.ID,
					Name:     serverGroup.Name,
				}, nil
			}
		} else {
			oldPrefix := generateServerGroupNamePrefixV1(w.ClusterTechnicalName(), pool.Name)
			// For K8s < 1.35: accept both name formats, but policy must match. This allows us to adopt existing server groups created with the old name format and also allows us to adopt server groups created with the new name format (e.g. if the adoption with the old name format failed due to a policy change and the server group was recreated by the user with the new name format).
			if (serverGroup.Name == name || strings.HasPrefix(serverGroup.Name, oldPrefix)) && policyMatch(&serverGroup) {
				return &api.ServerGroupDependency{
					PoolName: pool.Name,
					ID:       serverGroup.ID,
					Name:     serverGroup.Name,
				}, nil
			}
		}
	}

	// if we did not have a valid matching candidate for adoption, create a new server group.
	result, err := computeClient.CreateServerGroup(ctx, name, poolProviderConfig.ServerGroup.Policy)
	if err != nil {
		return nil, err
	}

	return &api.ServerGroupDependency{
		PoolName: pool.Name,
		ID:       result.ID,
		Name:     result.Name,
	}, nil
}

// PostReconcileHook implements genericactuator.WorkerDelegate.
func (w *WorkerDelegate) PostReconcileHook(ctx context.Context) error {
	return w.cleanupMachineDependencies(ctx)
}

// PreDeleteHook implements genericactuator.WorkerDelegate.
func (w *WorkerDelegate) PreDeleteHook(_ context.Context) error {
	return nil
}

// PostDeleteHook implements genericactuator.WorkerDelegate.
func (w *WorkerDelegate) PostDeleteHook(ctx context.Context) error {
	return w.cleanupMachineDependencies(ctx)
}

// cleanupMachineDependencies cleans up machine dependencies.
//
// TODO(dkistner, kon-angelo): Currently both PostReconcileHook and PostDeleteHook funcs call cleanupMachineDependencies.
// cleanupMachineDependencies calls cleanupServerGroupDependencies. cleanupServerGroupDependencies handles the cases when the Worker is being
// deleted (logic applicable for PostDeleteHook) and is not being deleted (logic applicable for PostReconcileHook).
// Refactor this so that PostDeleteHook executes only the handling for Worker being deleted and PostReconcileHook executes only
// the handling for Worker reconciled (not being deleted).
func (w *WorkerDelegate) cleanupMachineDependencies(ctx context.Context) error {
	computeClient, err := w.openstackClient.Compute(osclient.WithRegion(w.worker.Spec.Region))
	if err != nil {
		return err
	}

	workerStatus, err := w.decodeWorkerProviderStatus()
	if err != nil {
		return err
	}

	serverGroupDepSet := newServerGroupDependencySet(workerStatus.DeepCopy().ServerGroupDependencies)
	err = w.cleanupServerGroupDependencies(ctx, computeClient, serverGroupDepSet)

	return w.updateMachineDependenciesStatus(ctx, workerStatus, serverGroupDepSet.extract(), err)
}

// cleanupServerGroupDependencies handles deletion of excess server groups managed by the worker.
// Cases handled:
// a) worker is terminating and all server groups have to be deleted
// b) worker pool is deleted
// c) worker pool's server group configuration (e.g. policy) changed
// d) worker pool no longer requires use of server groups
func (w *WorkerDelegate) cleanupServerGroupDependencies(ctx context.Context, computeClient osclient.Compute, set serverGroupDependencySet) error {
	groups, err := computeClient.ListServerGroups(ctx)
	if err != nil {
		return err
	}

	// TODO remove after K8s v1.35.0 is the minimum required version, as the old name format will no longer be generated for new server groups and existing server groups should have been deleted by then.
	// append both name formats for deletion
	workerManagedServerGroups := filterServerGroupsByPrefix(groups, fmt.Sprintf("%s-", w.ClusterTechnicalName()))
	workerManagedServerGroups = append(workerManagedServerGroups, filterServerGroupsByPrefix(groups, generateServerGroupNamePrefixV2(string(w.cluster.Shoot.GetUID())))...)

	// handles case [c]
	for _, group := range workerManagedServerGroups {
		dep := set.getById(group.ID)
		if dep != nil {
			continue
		}

		err = computeClient.DeleteServerGroup(ctx, group.ID)
		if err != nil {
			return err
		}
	}

	// handles case [a]
	if w.worker.DeletionTimestamp != nil {
		return set.forEach(func(d api.ServerGroupDependency) error {
			if err := computeClient.DeleteServerGroup(ctx, d.ID); err != nil {
				return err
			}

			set.deleteByPoolName(d.PoolName)
			return nil
		})
	}

	// Find out which worker pools use server groups. Deps whose worker pool is not present in the map will be deleted.
	configs := sets.NewString()
	for _, pool := range w.worker.Spec.Pools {
		poolConfig, err := helper.WorkerConfigFromRawExtension(pool.ProviderConfig)
		if err != nil {
			return err
		}

		if !isServerGroupRequired(poolConfig) {
			continue
		}

		configs.Insert(pool.Name)
	}

	// handles cases [b,d]
	return set.forEach(func(d api.ServerGroupDependency) error {
		if configs.Has(d.PoolName) {
			return nil
		}

		if err := computeClient.DeleteServerGroup(ctx, d.ID); err != nil {
			return err
		}

		set.deleteByPoolName(d.PoolName)
		return nil
	})
}
