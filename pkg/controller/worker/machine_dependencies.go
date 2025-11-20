// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"context"
	"fmt"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"k8s.io/apimachinery/pkg/util/sets"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	osclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
)

// DeployMachineDependencies implements genericactuator.WorkerDelegate.
// Deprecated: Do not use this func. It is deprecated in genericactuator.WorkerDelegate.
func (w *WorkerDelegate) DeployMachineDependencies(_ context.Context) error {
	return nil
}

// CleanupMachineDependencies implements genericactuator.WorkerDelegate.
// Deprecated: Do not use this func. It is deprecated in genericactuator.WorkerDelegate.
func (w *WorkerDelegate) CleanupMachineDependencies(_ context.Context) error {
	return nil
}

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

	poolDep := set.getByPoolName(pool.Name)
	if poolDep != nil {
		serverGroup, err := computeClient.GetServerGroup(ctx, poolDep.ID)
		if err != nil && !osclient.IsNotFoundError(err) {
			return nil, err
		} else if err == nil {
			if serverGroup.Name == poolDep.Name && (len(serverGroup.Policies) > 0 && serverGroup.Policies[0] == poolProviderConfig.ServerGroup.Policy) {
				// if the current dependency's spec matches the provider resource, do nothing.
				return nil, nil
			}
		}
	}

	name, err := generateServerGroupName(w.ClusterTechnicalName(), pool.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to generate server group name for worker pool %q: %w", pool.Name, err)
	}

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

	workerManagedServerGroups := filterServerGroupsByPrefix(groups, fmt.Sprintf("%s-", w.ClusterTechnicalName()))

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
