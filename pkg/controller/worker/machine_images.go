// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"k8s.io/utils/ptr"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
)

// UpdateMachineImagesStatus updates the worker provider status with the machine images used in the worker spec.
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

func (w *WorkerDelegate) selectMachineImageForWorkerPool(name, version string, region string, machineCapabilities gardencorev1beta1.Capabilities, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition) (*api.MachineImage, error) {
	selectedMachineImage := &api.MachineImage{
		Name:    name,
		Version: version,
	}

	if capabilitySet, err := helper.FindImageInCloudProfile(w.cloudProfileConfig, name, version, region, machineCapabilities, capabilityDefinitions); err == nil {
		selectedMachineImage.Capabilities = capabilitySet.Capabilities
		// ID takes precedence over Image attribute. Reminder: Image is global name while ID is region specific.
		if len(capabilitySet.Regions) > 0 && capabilitySet.Regions[0].ID != "" {
			selectedMachineImage.ID = capabilitySet.Regions[0].ID
		} else {
			selectedMachineImage.Image = capabilitySet.Image
		}

		return selectedMachineImage, nil
	}

	// Try to look up machine image in worker provider status as it was not found in componentconfig.
	if providerStatus := w.worker.Status.ProviderStatus; providerStatus != nil {
		workerStatus := &api.WorkerStatus{}
		if _, _, err := w.decoder.Decode(providerStatus.Raw, nil, workerStatus); err != nil {
			return nil, fmt.Errorf("could not decode worker status of worker '%s': %w", k8sclient.ObjectKeyFromObject(w.worker), err)
		}

		// Pass the original (non-normalized) MachineCapabilities so FindImageInWorkerStatus
		// can distinguish legacy format (Architecture field) from capability format (Capabilities field).
		return helper.FindImageInWorkerStatus(workerStatus.MachineImages, name, version, nil, machineCapabilities, w.cluster.CloudProfile.Spec.MachineCapabilities)
	}

	archValues := machineCapabilities[v1beta1constants.ArchitectureName]
	arch := v1beta1constants.ArchitectureAMD64
	if len(archValues) > 0 {
		arch = archValues[0]
	}
	return nil, worker.ErrorMachineImageNotFound(name, version, arch, region)
}

func appendMachineImage(machineImages []api.MachineImage, machineImage api.MachineImage, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition) []api.MachineImage {
	// support for cloudprofile machine images without capabilities
	if len(capabilityDefinitions) == 0 {
		// Extract architecture from Capabilities if Architecture is not set
		// (happens when image came from capability-based lookup with normalized definitions)
		architecture := machineImage.Architecture
		if architecture == nil {
			if archValues, ok := machineImage.Capabilities[v1beta1constants.ArchitectureName]; ok && len(archValues) > 0 {
				architecture = &archValues[0]
			}
		}
		for _, image := range machineImages {
			if image.Name == machineImage.Name && image.Version == machineImage.Version && ptr.Deref(architecture, "") == ptr.Deref(image.Architecture, "") {
				// If the image already exists without capabilities, we can just return the existing list.
				return machineImages
			}
		}
		return append(machineImages, api.MachineImage{
			Name:         machineImage.Name,
			Version:      machineImage.Version,
			Image:        machineImage.Image,
			ID:           machineImage.ID,
			Architecture: architecture,
		})
	}

	defaultedCapabilities := gardencorev1beta1.GetCapabilitiesWithAppliedDefaults(machineImage.Capabilities, capabilityDefinitions)

	for _, existingMachineImage := range machineImages {
		existingDefaultedCapabilities := gardencorev1beta1.GetCapabilitiesWithAppliedDefaults(existingMachineImage.Capabilities, capabilityDefinitions)
		if existingMachineImage.Name == machineImage.Name && existingMachineImage.Version == machineImage.Version && gardencorev1beta1helper.AreCapabilitiesEqual(defaultedCapabilities, existingDefaultedCapabilities) {
			// If the image already exists with the same capabilities return the existing list.
			return machineImages
		}
	}

	// If the image does not exist, we create a new machine image entry with the capabilities.
	machineImages = append(machineImages, api.MachineImage{
		Name:         machineImage.Name,
		Version:      machineImage.Version,
		ID:           machineImage.ID,
		Capabilities: machineImage.Capabilities,
	})

	return machineImages
}
