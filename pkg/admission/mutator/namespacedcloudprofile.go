// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package mutator

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/utils"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"
)

// NewNamespacedCloudProfileMutator returns a new instance of a NamespacedCloudProfile mutator.
// It handles both spec mutations (populating capabilityFlavors) and status mutations (merging provider config).
func NewNamespacedCloudProfileMutator(mgr manager.Manager) extensionswebhook.Mutator {
	return &namespacedCloudProfile{
		client:  mgr.GetClient(),
		decoder: serializer.NewCodecFactory(mgr.GetScheme(), serializer.EnableStrict).UniversalDecoder(),
	}
}

type namespacedCloudProfile struct {
	client  client.Client
	decoder runtime.Decoder
}

// Mutate mutates the given NamespacedCloudProfile object.
// It performs two independent mutations:
// 1. Populates capabilityFlavors on spec.machineImages from the providerConfig (for spec create/update)
// 2. Merges spec providerConfig into status providerConfig (for status subresource updates)
func (p *namespacedCloudProfile) Mutate(ctx context.Context, newObj, _ client.Object) error {
	profile, ok := newObj.(*gardencorev1beta1.NamespacedCloudProfile)
	if !ok {
		return fmt.Errorf("wrong object type %T", newObj)
	}

	if profile.DeletionTimestamp != nil || profile.Spec.ProviderConfig == nil {
		return nil
	}

	specConfig := &v1alpha1.CloudProfileConfig{}
	if _, _, err := p.decoder.Decode(profile.Spec.ProviderConfig.Raw, nil, specConfig); err != nil {
		return fmt.Errorf("could not decode providerConfig of namespacedCloudProfile spec for '%s': %w", profile.Name, err)
	}

	// Mutation 1: Populate capabilityFlavors on spec.machineImages if parent has machineCapabilities
	if err := p.mutateSpecCapabilityFlavors(ctx, profile, specConfig); err != nil {
		return err
	}

	// Mutation 2: Merge spec providerConfig into status (only when status is available)
	if shouldMergeStatus(profile) {
		if err := p.mergeStatusProviderConfig(profile, specConfig); err != nil {
			return err
		}
	}

	return nil
}

// mutateSpecCapabilityFlavors populates capabilityFlavors on spec.machineImages versions from the providerConfig.
func (p *namespacedCloudProfile) mutateSpecCapabilityFlavors(ctx context.Context, profile *gardencorev1beta1.NamespacedCloudProfile, specConfig *v1alpha1.CloudProfileConfig) error {
	// Fetch parent CloudProfile to check for machineCapabilities
	parentProfile := &gardencorev1beta1.CloudProfile{}
	if err := p.client.Get(ctx, client.ObjectKey{Name: profile.Spec.Parent.Name}, parentProfile); err != nil {
		return fmt.Errorf("could not get parent CloudProfile %q: %w", profile.Spec.Parent.Name, err)
	}

	// Skip if parent has no machineCapabilities
	if len(parentProfile.Spec.MachineCapabilities) == 0 {
		return nil
	}

	mutateMachineImageCapabilityFlavors(profile.Spec.MachineImages, specConfig)
	return nil
}

// mergeStatusProviderConfig merges the spec providerConfig into the status providerConfig.
func (p *namespacedCloudProfile) mergeStatusProviderConfig(profile *gardencorev1beta1.NamespacedCloudProfile, specConfig *v1alpha1.CloudProfileConfig) error {
	statusConfig := &v1alpha1.CloudProfileConfig{}
	if _, _, err := p.decoder.Decode(profile.Status.CloudProfileSpec.ProviderConfig.Raw, nil, statusConfig); err != nil {
		return fmt.Errorf("could not decode providerConfig of namespacedCloudProfile status for '%s': %w", profile.Name, err)
	}

	statusConfig.MachineImages = mergeMachineImages(specConfig.MachineImages, statusConfig.MachineImages)

	modifiedStatusConfig, err := json.Marshal(statusConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal status config: %w", err)
	}
	profile.Status.CloudProfileSpec.ProviderConfig.Raw = modifiedStatusConfig
	return nil
}

// shouldMergeStatus checks if the status merge should be performed.
// Status merge is only applicable when the status has been populated (status subresource update).
func shouldMergeStatus(profile *gardencorev1beta1.NamespacedCloudProfile) bool {
	return profile.Generation == profile.Status.ObservedGeneration &&
		profile.Status.CloudProfileSpec.ProviderConfig != nil
}

func mergeMachineImages(specMachineImages, statusMachineImages []v1alpha1.MachineImages) []v1alpha1.MachineImages {
	specImages := utils.CreateMapFromSlice(specMachineImages, func(mi v1alpha1.MachineImages) string { return mi.Name })
	statusImages := utils.CreateMapFromSlice(statusMachineImages, func(mi v1alpha1.MachineImages) string { return mi.Name })
	for _, specMachineImage := range specImages {
		if _, exists := statusImages[specMachineImage.Name]; !exists {
			statusImages[specMachineImage.Name] = specMachineImage
		} else {
			statusImageVersions := utils.CreateMapFromSlice(statusImages[specMachineImage.Name].Versions, func(v v1alpha1.MachineImageVersion) string { return v.Version })
			specImageVersions := utils.CreateMapFromSlice(specImages[specMachineImage.Name].Versions, func(v v1alpha1.MachineImageVersion) string { return v.Version })
			for _, version := range specImageVersions {
				statusImageVersions[version.Version] = version
			}

			statusImages[specMachineImage.Name] = v1alpha1.MachineImages{
				Name:     specMachineImage.Name,
				Versions: slices.Collect(maps.Values(statusImageVersions)),
			}
		}
	}
	return slices.Collect(maps.Values(statusImages))
}
