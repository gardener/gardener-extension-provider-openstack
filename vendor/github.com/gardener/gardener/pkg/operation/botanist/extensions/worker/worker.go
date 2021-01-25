// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package worker

import (
	"context"
	"fmt"
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils"

	"github.com/Masterminds/semver"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// DefaultInterval is the default interval for retry operations.
	DefaultInterval = 5 * time.Second
	// DefaultSevereThreshold is the default threshold until an error reported by another component is treated as
	// 'severe'.
	DefaultSevereThreshold = 30 * time.Second
	// DefaultTimeout is the default timeout and defines how long Gardener should wait for a successful reconciliation
	// of a Worker resource.
	DefaultTimeout = 10 * time.Minute
)

// TimeNow returns the current time. Exposed for testing.
var TimeNow = time.Now

// Values contains the values used to create a Worker resources.
type Values struct {
	// Namespace is the Shoot namespace in the seed.
	Namespace string
	// Name is the name of the Worker resource.
	Name string
	// Type is the type of the Worker provider.
	Type string
	// Region is the region of the shoot.
	Region string
	// Workers is the list of worker pools.
	Workers []gardencorev1beta1.Worker
	// KubernetesVersion is the Kubernetes version of the cluster for which the worker nodes shall be created.
	KubernetesVersion *semver.Version
	// SSHPublicKey is the public SSH key that shall be installed on the worker nodes.
	SSHPublicKey []byte
	// InfrastructureProviderStatus is the provider status of the Infrastructure resource which might be relevant for
	// the Worker reconciliation.
	InfrastructureProviderStatus *runtime.RawExtension
	// OperatingSystemConfigsMap contains the operating system configurations for the worker pools.
	OperatingSystemConfigsMap map[string]shoot.OperatingSystemConfigs
}

// New creates a new instance of a Worker deployer.
func New(
	logger logrus.FieldLogger,
	client client.Client,
	values *Values,
	waitInterval time.Duration,
	waitSevereThreshold time.Duration,
	waitTimeout time.Duration,
) shoot.ExtensionWorker {
	return &worker{
		client:              client,
		logger:              logger,
		values:              values,
		waitInterval:        waitInterval,
		waitSevereThreshold: waitSevereThreshold,
		waitTimeout:         waitTimeout,
	}
}

type worker struct {
	values              *Values
	logger              logrus.FieldLogger
	client              client.Client
	waitInterval        time.Duration
	waitSevereThreshold time.Duration
	waitTimeout         time.Duration

	machineDeployments []extensionsv1alpha1.MachineDeployment
}

// Deploy uses the seed client to create or update the Worker resource.
func (w *worker) Deploy(ctx context.Context) error {
	_, err := w.deploy(ctx, v1beta1constants.GardenerOperationReconcile)
	return err
}

func (w *worker) deploy(ctx context.Context, operation string) (extensionsv1alpha1.Object, error) {
	var (
		worker = &extensionsv1alpha1.Worker{
			ObjectMeta: metav1.ObjectMeta{
				Name:      w.values.Name,
				Namespace: w.values.Namespace,
			},
		}
		pools []extensionsv1alpha1.WorkerPool
	)

	for _, workerPool := range w.values.Workers {
		var volume *extensionsv1alpha1.Volume
		if workerPool.Volume != nil {
			volume = &extensionsv1alpha1.Volume{
				Name:      workerPool.Volume.Name,
				Type:      workerPool.Volume.Type,
				Size:      workerPool.Volume.VolumeSize,
				Encrypted: workerPool.Volume.Encrypted,
			}
		}

		var dataVolumes []extensionsv1alpha1.DataVolume
		if len(workerPool.DataVolumes) > 0 {
			for _, dataVolume := range workerPool.DataVolumes {
				dataVolumes = append(dataVolumes, extensionsv1alpha1.DataVolume{
					Name:      dataVolume.Name,
					Type:      dataVolume.Type,
					Size:      dataVolume.VolumeSize,
					Encrypted: dataVolume.Encrypted,
				})
			}
		}

		// copy labels map
		labels := utils.MergeStringMaps(workerPool.Labels)
		if labels == nil {
			labels = map[string]string{}
		}

		// k8s node role labels
		if versionConstraintK8sSmaller115.Check(w.values.KubernetesVersion) {
			labels["kubernetes.io/role"] = "node"
			labels["node-role.kubernetes.io/node"] = ""
		} else {
			labels["node.kubernetes.io/role"] = "node"
		}

		if gardencorev1beta1helper.SystemComponentsAllowed(&workerPool) {
			labels[v1beta1constants.LabelWorkerPoolSystemComponents] = "true"
		}

		// worker pool name labels
		labels[v1beta1constants.LabelWorkerPool] = workerPool.Name
		labels[v1beta1constants.LabelWorkerPoolDeprecated] = workerPool.Name

		// add CRI labels selected by the RuntimeClass
		if workerPool.CRI != nil {
			labels[extensionsv1alpha1.CRINameWorkerLabel] = string(workerPool.CRI.Name)
			if len(workerPool.CRI.ContainerRuntimes) > 0 {
				for _, cr := range workerPool.CRI.ContainerRuntimes {
					key := fmt.Sprintf(extensionsv1alpha1.ContainerRuntimeNameWorkerLabel, cr.Type)
					labels[key] = "true"
				}
			}
		}

		var pConfig *runtime.RawExtension
		if workerPool.ProviderConfig != nil {
			pConfig = &runtime.RawExtension{
				Raw: workerPool.ProviderConfig.Raw,
			}
		}

		var userData []byte
		if val, ok := w.values.OperatingSystemConfigsMap[workerPool.Name]; ok {
			userData = []byte(val.Downloader.Data.Content)
		}

		pools = append(pools, extensionsv1alpha1.WorkerPool{
			Name:           workerPool.Name,
			Minimum:        workerPool.Minimum,
			Maximum:        workerPool.Maximum,
			MaxSurge:       *workerPool.MaxSurge,
			MaxUnavailable: *workerPool.MaxUnavailable,
			Annotations:    workerPool.Annotations,
			Labels:         labels,
			Taints:         workerPool.Taints,
			MachineType:    workerPool.Machine.Type,
			MachineImage: extensionsv1alpha1.MachineImage{
				Name:    workerPool.Machine.Image.Name,
				Version: *workerPool.Machine.Image.Version,
			},
			ProviderConfig:                   pConfig,
			UserData:                         userData,
			Volume:                           volume,
			DataVolumes:                      dataVolumes,
			KubeletDataVolumeName:            workerPool.KubeletDataVolumeName,
			Zones:                            workerPool.Zones,
			MachineControllerManagerSettings: workerPool.MachineControllerManagerSettings,
		})
	}

	_, err := controllerutil.CreateOrUpdate(ctx, w.client, worker, func() error {
		metav1.SetMetaDataAnnotation(&worker.ObjectMeta, v1beta1constants.GardenerOperation, operation)
		metav1.SetMetaDataAnnotation(&worker.ObjectMeta, v1beta1constants.GardenerTimestamp, TimeNow().UTC().String())

		worker.Spec = extensionsv1alpha1.WorkerSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type: w.values.Type,
			},
			Region: w.values.Region,
			SecretRef: corev1.SecretReference{
				Name:      v1beta1constants.SecretNameCloudProvider,
				Namespace: worker.Namespace,
			},
			SSHPublicKey:                 w.values.SSHPublicKey,
			InfrastructureProviderStatus: w.values.InfrastructureProviderStatus,
			Pools:                        pools,
		}

		return nil
	})

	return worker, err
}

// Restore uses the seed client and the ShootState to create the Worker resources and restore their state.
func (w *worker) Restore(ctx context.Context, shootState *gardencorev1alpha1.ShootState) error {
	return common.RestoreExtensionWithDeployFunction(
		ctx,
		shootState,
		w.client,
		extensionsv1alpha1.WorkerResource,
		w.values.Namespace,
		w.deploy,
	)
}

// Migrate migrates the Worker resource.
func (w *worker) Migrate(ctx context.Context) error {
	return common.MigrateExtensionCR(
		ctx,
		w.client,
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.Worker{} },
		w.values.Namespace,
		w.values.Name,
	)
}

// Destroy deletes the Worker resource.
func (w *worker) Destroy(ctx context.Context) error {
	return common.DeleteExtensionCR(
		ctx,
		w.client,
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.Worker{} },
		w.values.Namespace,
		w.values.Name,
	)
}

// Wait waits until the Worker resource is ready.
func (w *worker) Wait(ctx context.Context) error {
	return common.WaitUntilExtensionCRReady(
		ctx,
		w.client,
		w.logger,
		func() client.Object { return &extensionsv1alpha1.Worker{} },
		extensionsv1alpha1.WorkerResource,
		w.values.Namespace,
		w.values.Name,
		w.waitInterval,
		w.waitSevereThreshold,
		w.waitTimeout,
		func(obj runtime.Object) error {
			worker, ok := obj.(*extensionsv1alpha1.Worker)
			if !ok {
				return fmt.Errorf("expected extensionsv1alpha1.Worker but got %T", worker)
			}

			w.machineDeployments = worker.Status.MachineDeployments
			return nil
		},
	)
}

// WaitMigrate waits until the Worker resources are migrated successfully.
func (w *worker) WaitMigrate(ctx context.Context) error {
	return common.WaitUntilExtensionCRMigrated(
		ctx,
		w.client,
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.Worker{} },
		w.values.Namespace,
		w.values.Name,
		w.waitInterval,
		w.waitTimeout,
	)
}

// WaitCleanup waits until the Worker resource is deleted.
func (w *worker) WaitCleanup(ctx context.Context) error {
	return common.WaitUntilExtensionCRDeleted(
		ctx,
		w.client,
		w.logger,
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.Worker{} },
		extensionsv1alpha1.WorkerResource,
		w.values.Namespace,
		w.values.Name,
		w.waitInterval,
		w.waitTimeout,
	)
}

// SetPublicSSHKey sets the public SSH key in the values.
func (w *worker) SetSSHPublicKey(key []byte) {
	w.values.SSHPublicKey = key
}

// SetInfrastructureProviderStatus sets the infrastructure provider status in the values.
func (w *worker) SetInfrastructureProviderStatus(status *runtime.RawExtension) {
	w.values.InfrastructureProviderStatus = status
}

// SetOperatingSystemConfigMaps sets the operating system config maps in the values.
func (w *worker) SetOperatingSystemConfigMaps(maps map[string]shoot.OperatingSystemConfigs) {
	w.values.OperatingSystemConfigsMap = maps
}

// MachineDeployments returns the generated machine deployments of the Worker.
func (w *worker) MachineDeployments() []extensionsv1alpha1.MachineDeployment {
	return w.machineDeployments
}

var versionConstraintK8sSmaller115 *semver.Constraints

func init() {
	var err error

	versionConstraintK8sSmaller115, err = semver.NewConstraint("< 1.15")
	utilruntime.Must(err)
}
