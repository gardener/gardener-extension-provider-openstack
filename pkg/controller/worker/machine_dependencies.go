/*
 * Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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

func (w *workerDelegate) DeployMachineDependencies(ctx context.Context) error {
	computeClient, err := w.openstackClient.Compute()
	if err != nil {
		return err
	}

	workerStatus, err := w.decodeWorkerProviderStatus()
	if err != nil {
		return err
	}

	serverGroupDepSet, err := w.reconcileServerGroups(computeClient, workerStatus.DeepCopy())
	return w.updateMachineDependenciesStatus(ctx, workerStatus, serverGroupDepSet.extract(), err)
}

func (w *workerDelegate) reconcileServerGroups(computeClient osclient.Compute, workerStatus *api.WorkerStatus) (serverGroupDependencySet, error) {
	serverGroupDepSet := newServerGroupDependencySet(workerStatus.ServerGroupDependencies)
	for _, pool := range w.worker.Spec.Pools {
		serverGroupDependencyStatus, err := w.reconcilePoolServerGroup(computeClient, pool, serverGroupDepSet)
		if err != nil {
			return serverGroupDepSet, fmt.Errorf("reconciling server groups failed for pool %q: %w", pool.Name, err)
		}
		serverGroupDepSet.upsert(serverGroupDependencyStatus)
	}
	return serverGroupDepSet, nil
}

func (w *workerDelegate) reconcilePoolServerGroup(computeClient osclient.Compute, pool extensionsv1alpha1.WorkerPool, set serverGroupDependencySet) (*api.ServerGroupDependency, error) {
	poolProviderConfig, err := helper.WorkerConfigFromRawExtension(pool.ProviderConfig)
	if err != nil {
		return nil, err
	}

	if !isServerGroupRequired(poolProviderConfig) {
		return nil, nil
	}

	poolDep := set.getByPoolName(pool.Name)
	if poolDep != nil {
		serverGroup, err := computeClient.GetServerGroup(poolDep.ID)
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

	result, err := computeClient.CreateServerGroup(name, poolProviderConfig.ServerGroup.Policy)
	if err != nil {
		return nil, err
	}

	return &api.ServerGroupDependency{
		PoolName: pool.Name,
		ID:       result.ID,
		Name:     result.Name,
	}, nil
}

func (w *workerDelegate) CleanupMachineDependencies(ctx context.Context) error {
	computeClient, err := w.openstackClient.Compute()
	if err != nil {
		return err
	}

	workerStatus, err := w.decodeWorkerProviderStatus()
	if err != nil {
		return err
	}

	serverGroupDepSet := newServerGroupDependencySet(workerStatus.DeepCopy().ServerGroupDependencies)
	err = w.cleanupServerGroupDependencies(computeClient, serverGroupDepSet)

	return w.updateMachineDependenciesStatus(ctx, workerStatus, serverGroupDepSet.extract(), err)
}

// cleanupServerGroupDependencies handles deletion of excess server groups managed by the worker.
// Cases handled:
// a) worker is terminating and all server groups have to be deleted
// b) worker pool is deleted
// c) worker pool's server group configuration (e.g. policy) changed
// d) worker pool no longer requires use of server groups
func (w *workerDelegate) cleanupServerGroupDependencies(computeClient osclient.Compute, set serverGroupDependencySet) error {
	groups, err := computeClient.ListServerGroups()
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

		err = computeClient.DeleteServerGroup(group.ID)
		if err != nil {
			return err
		}
	}

	// handles case [a]
	if w.worker.DeletionTimestamp != nil {
		return set.forEach(func(d api.ServerGroupDependency) error {
			if err := computeClient.DeleteServerGroup(d.ID); err != nil {
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

		if err := computeClient.DeleteServerGroup(d.ID); err != nil {
			return err
		}

		set.deleteByPoolName(d.PoolName)
		return nil
	})
}

// PreReconcileHook implements genericactuator.WorkerDelegate.
func (w *workerDelegate) PreReconcileHook(_ context.Context) error {
	return nil
}

// PostReconcileHook implements genericactuator.WorkerDelegate.
func (w *workerDelegate) PostReconcileHook(_ context.Context) error {
	return nil
}

// PreDeleteHook implements genericactuator.WorkerDelegate.
func (w *workerDelegate) PreDeleteHook(_ context.Context) error {
	return nil
}

// PostDeleteHook implements genericactuator.WorkerDelegate.
func (w *workerDelegate) PostDeleteHook(_ context.Context) error {
	return nil
}
