// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"github.com/gardener/gardener/extensions/pkg/controller/worker"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"

	"github.com/gardener/gardener/extensions/pkg/util"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// GetMachineImages returns the used machine images for the `Worker` resource.
func (w *workerDelegate) GetMachineImages(ctx context.Context) (runtime.Object, error) {
	if w.machineImages == nil {
		if err := w.generateMachineConfig(ctx); err != nil {
			return nil, err
		}
	}

	var (
		workerStatus = &api.WorkerStatus{
			TypeMeta: metav1.TypeMeta{
				APIVersion: api.SchemeGroupVersion.String(),
				Kind:       "WorkerStatus",
			},
			MachineImages: w.machineImages,
		}

		workerStatusV1alpha1 = &v1alpha1.WorkerStatus{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1alpha1.SchemeGroupVersion.String(),
				Kind:       "WorkerStatus",
			},
		}
	)

	if err := w.Scheme().Convert(workerStatus, workerStatusV1alpha1, nil); err != nil {
		return nil, err
	}

	return workerStatusV1alpha1, nil
}

func (w *workerDelegate) findMachineImage(name, version string) (*api.MachineImage, error) {
	image, err := helper.FindImageFromCloudProfile(w.cloudProfileConfig, name, version, w.cluster.Shoot.Spec.Region)
	if err == nil {
		return image, nil
	}

	// Try to look up machine image in worker provider status as it was not found in componentconfig.
	if providerStatus := w.worker.Status.ProviderStatus; providerStatus != nil {
		workerStatus := &api.WorkerStatus{}
		if _, _, err := w.Decoder().Decode(providerStatus.Raw, nil, workerStatus); err != nil {
			return nil, errors.Wrapf(err, "could not decode worker status of worker '%s'", util.ObjectName(w.worker))
		}

		machineImage, err := helper.FindMachineImage(workerStatus.MachineImages, name, version)
		if err != nil {
			return nil, worker.ErrorMachineImageNotFound(name, version)
		}

		return machineImage, nil
	}

	return nil, worker.ErrorMachineImageNotFound(name, version)
}

func appendMachineImage(machineImages []api.MachineImage, machineImage api.MachineImage) []api.MachineImage {
	if _, err := helper.FindMachineImage(machineImages, machineImage.Name, machineImage.Version); err != nil {
		return append(machineImages, machineImage)
	}
	return machineImages
}
