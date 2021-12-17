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

package etcdcopybackupstask

import (
	"context"
	"fmt"
	"time"

	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// DefaultInterval is the default interval for retry operations.
	DefaultInterval = 5 * time.Second
	// DefaultSevereThreshold is the default threshold until an error reported by another component is treated as 'severe'.
	DefaultSevereThreshold = 3 * time.Minute
	// DefaultTimeout is the default timeout and defines how long Gardener should wait
	// for a successful reconciliation of an EtcdCopyBackupsTasks resource.
	DefaultTimeout = 5 * time.Minute

	// DefaultWaitForFinalSnapshotTimeout is the default timeout for waiting for a final full snapshot.
	DefaultWaitForFinalSnapshotTimeout = 30 * time.Minute
)

// Interface contains functions to manage EtcdCopyBackupsTasks.
type Interface interface {
	component.DeployWaiter
	// SetSourceStore sets the specifications for the object store provider from which backups will be copied.
	SetSourceStore(druidv1alpha1.StoreSpec)
	// SetTargetStore sets the specifications for the object store provider to which backups will be copied.
	SetTargetStore(druidv1alpha1.StoreSpec)
}

// Values contains the values used to create an EtcdCopyBackupsTask resources.
type Values struct {
	// Name is the name of the EtcdCopyBackupsTask.
	Name string
	// Namespace is the namespace of the EtcdCopyBackupsTask.
	Namespace string
	// SourceStore is the specification of the objct store from which etcd backups will be copied.
	SourceStore druidv1alpha1.StoreSpec
	// TargetStore is the specification of the objcet store to which etcd backups will be copied.
	TargetStore druidv1alpha1.StoreSpec
	// MaxBackups is the maximum number of backups that will be copied starting with the most recent ones.
	MaxBackups *uint32
	// MaxBackupAge is the maximum age in days that a backup must have in order to be copied.
	MaxBackupAge *uint32
	// WaitForFinalSnapshot defines the parameters for waiting for a final full snapshot before copying backups.
	WaitForFinalSnapshot *druidv1alpha1.WaitForFinalSnapshotSpec
}

type etcdCopyBackupsTask struct {
	values              *Values
	logger              logrus.FieldLogger
	client              client.Client
	waitInterval        time.Duration
	waitSevereThreshold time.Duration
	waitTimeout         time.Duration

	task *druidv1alpha1.EtcdCopyBackupsTask
}

// New creates a new instance of Interface
func New(
	logger logrus.FieldLogger,
	client client.Client,
	values *Values,
	waitInterval time.Duration,
	waitSevereThreshold time.Duration,
	waitTimeout time.Duration,
) Interface {

	return &etcdCopyBackupsTask{
		values,
		logger,
		client,
		waitInterval,
		waitSevereThreshold,
		waitTimeout,
		&druidv1alpha1.EtcdCopyBackupsTask{
			ObjectMeta: metav1.ObjectMeta{
				Name:      values.Name,
				Namespace: values.Namespace,
			},
		},
	}
}

// Deploy creates the EtcdCopyBackupsTask resource.
func (e *etcdCopyBackupsTask) Deploy(ctx context.Context) error {
	e.task.Spec.MaxBackupAge = e.values.MaxBackupAge
	e.task.Spec.MaxBackups = e.values.MaxBackups
	e.task.Spec.SourceStore = e.values.SourceStore
	e.task.Spec.TargetStore = e.values.TargetStore
	e.task.Spec.WaitForFinalSnapshot = e.values.WaitForFinalSnapshot
	return e.client.Create(ctx, e.task)
}

// Wait waits until the EtcdCopyBackupsTask is ready.
func (e *etcdCopyBackupsTask) Wait(ctx context.Context) error {
	if err := extensions.WaitUntilObjectReadyWithHealthFunction(
		ctx,
		e.client,
		e.logger,
		waitForConditions,
		e.task,
		"EtcdCopyBackupsTask",
		e.waitInterval,
		e.waitSevereThreshold,
		e.waitTimeout,
		e.checkConditions,
	); err != nil {
		e.logger.Error(err)
		return err
	}
	return nil
}

// Destroy deletes the EtcdCopyBackupsTask resource.
func (e *etcdCopyBackupsTask) Destroy(ctx context.Context) error {
	return kutil.DeleteObject(ctx, e.client, e.task)
}

// WaitCleanup waits until the EtcdCopyBackupsTask is deleted.
func (e *etcdCopyBackupsTask) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, e.waitTimeout)
	defer cancel()
	return kutil.WaitUntilResourceDeleted(timeoutCtx, e.client, e.task, e.waitInterval)
}

// SetSourceStore sets the specifications for the object store provider from which backups will be copied.
func (e *etcdCopyBackupsTask) SetSourceStore(store druidv1alpha1.StoreSpec) {
	e.values.SourceStore = store
}

// SetTargetStore sets the specifications for the object store provider to which backups will be copied.
func (e *etcdCopyBackupsTask) SetTargetStore(store druidv1alpha1.StoreSpec) {
	e.values.TargetStore = store
}

// waitForConditions waits until the EtcdCopyBackupsTask conditions have been populated by the etcd-druid.
func waitForConditions(obj client.Object) error {
	task, ok := obj.(*druidv1alpha1.EtcdCopyBackupsTask)
	if !ok {
		return fmt.Errorf("expected *druidv1alpha1.EtcdCopyBackupsTask but got %T", obj)
	}
	if task.DeletionTimestamp != nil {
		return fmt.Errorf("task %s has a deletion timestamp", kutil.ObjectName(task))
	}

	generation := task.Generation
	observedGeneration := task.Status.ObservedGeneration
	if observedGeneration == nil {
		return fmt.Errorf("observed generation not recorded")
	}
	if generation != *observedGeneration {
		return fmt.Errorf("observed generation outdated (%d/%d)", *observedGeneration, generation)
	}

	if task.Status.LastError != nil {
		return retry.RetriableError(fmt.Errorf("error during reconciliation: %s", *task.Status.LastError))
	}

	for _, condition := range task.Status.Conditions {
		if (condition.Type == druidv1alpha1.EtcdCopyBackupsTaskSucceeded || condition.Type == druidv1alpha1.EtcdCopyBackupsTaskFailed) &&
			condition.Status == druidv1alpha1.ConditionTrue {
			return nil
		}
	}
	return fmt.Errorf("expected condition %s or %s, has not been reported yet", druidv1alpha1.EtcdCopyBackupsTaskSucceeded, druidv1alpha1.EtcdCopyBackupsTaskFailed)
}

// checkConditions checks the EtcdCopyBackupsTask conditions to determine if the copy operation has completed successfully or not.
func (e *etcdCopyBackupsTask) checkConditions() error {
	for _, condition := range e.task.Status.Conditions {
		if condition.Type == druidv1alpha1.EtcdCopyBackupsTaskFailed && condition.Status == druidv1alpha1.ConditionTrue {
			return fmt.Errorf("condition %s has status %s: %s", condition.Type, condition.Status, condition.Message)
		}
	}
	return nil
}
