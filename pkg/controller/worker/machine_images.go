// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"k8s.io/utils/ptr"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
)

// UpdateMachineImagesStatus updates the machine images inside the worker status. It generates the machine images if not yet generated.
func (w *WorkerDelegate) UpdateMachineImagesStatus(ctx context.Context) error {
	if w.machineImages == nil {
		if err := w.generateMachineConfig(ctx); err != nil {
			return fmt.Errorf("unable to generate the machine config: %w", err)
		}
	}

	// Decode the current worker provider status.
	workerStatus, err := w.decodeWorkerProviderStatus()
	if err != nil {
		return fmt.Errorf("unable to decode the worker provider status: %w", err)
	}

	workerStatus.MachineImages = w.machineImages
	if err := w.updateWorkerProviderStatus(ctx, workerStatus); err != nil {
		return fmt.Errorf("unable to update worker provider status: %w", err)
	}

	return nil
}

func (w *WorkerDelegate) findMachineImage(name, version, architecture string) (*api.MachineImage, error) {
	image, err := helper.FindImageFromCloudProfile(w.cloudProfileConfig, name, version, w.cluster.Shoot.Spec.Region, architecture)
	if err == nil {
		return image, nil
	}

	// Try to look up machine image in worker provider status as it was not found in componentconfig.
	if providerStatus := w.worker.Status.ProviderStatus; providerStatus != nil {
		workerStatus := &api.WorkerStatus{}
		if _, _, err := w.decoder.Decode(providerStatus.Raw, nil, workerStatus); err != nil {
			return nil, fmt.Errorf("could not decode worker status of worker '%s': %w", k8sclient.ObjectKeyFromObject(w.worker), err)
		}

		machineImage, err := helper.FindMachineImage(workerStatus.MachineImages, name, version, architecture)
		if err != nil {
			return nil, worker.ErrorMachineImageNotFound(name, version)
		}

		// The architecture field might not be present in the WorkerStatus if the Shoot has been created before introduction
		// of the field. Hence, initialize it if it's empty.
		machineImage = machineImage.DeepCopy()
		if machineImage.Architecture == nil {
			machineImage.Architecture = &architecture
		}

		return machineImage, nil
	}

	return nil, worker.ErrorMachineImageNotFound(name, version)
}

func appendMachineImage(machineImages []api.MachineImage, machineImage api.MachineImage) []api.MachineImage {
	if _, err := helper.FindMachineImage(machineImages, machineImage.Name, machineImage.Version, ptr.Deref(machineImage.Architecture, v1beta1constants.ArchitectureAMD64)); err != nil {
		return append(machineImages, machineImage)
	}
	return machineImages
}
