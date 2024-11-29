// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"context"
	"fmt"
	"slices"

	"github.com/gardener/gardener/extensions/pkg/util"
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/validation"
)

// NewNamespacedCloudProfileValidator returns a new instance of a namespaced cloud profile validator.
func NewNamespacedCloudProfileValidator(mgr manager.Manager) extensionswebhook.Validator {
	return &namespacedCloudProfile{
		client:  mgr.GetClient(),
		decoder: serializer.NewCodecFactory(mgr.GetScheme(), serializer.EnableStrict).UniversalDecoder(),
	}
}

type namespacedCloudProfile struct {
	client  client.Client
	decoder runtime.Decoder
}

// Validate validates the given NamespacedCloudProfile objects.
func (p *namespacedCloudProfile) Validate(ctx context.Context, newObj, _ client.Object) error {
	profile, ok := newObj.(*core.NamespacedCloudProfile)
	if !ok {
		return fmt.Errorf("wrong object type %T", newObj)
	}

	if profile.DeletionTimestamp != nil {
		return nil
	}

	cpConfig := &api.CloudProfileConfig{}
	if profile.Spec.ProviderConfig != nil {
		var err error
		cpConfig, err = decodeCloudProfileConfig(p.decoder, profile.Spec.ProviderConfig)
		if err != nil {
			return err
		}
	}

	parentCloudProfile := profile.Spec.Parent
	if parentCloudProfile.Kind != constants.CloudProfileReferenceKindCloudProfile {
		return fmt.Errorf("parent reference must be of kind CloudProfile (unsupported kind: %s)", parentCloudProfile.Kind)
	}
	parentProfile := &gardencorev1beta1.CloudProfile{}
	if err := p.client.Get(ctx, client.ObjectKey{Name: parentCloudProfile.Name}, parentProfile); err != nil {
		return err
	}

	return p.validateNamespacedCloudProfileProviderConfig(cpConfig, profile.Spec, parentProfile.Spec).ToAggregate()
}

// validateNamespacedCloudProfileProviderConfig validates the CloudProfileConfig passed with a NamespacedCloudProfile.
func (p *namespacedCloudProfile) validateNamespacedCloudProfileProviderConfig(providerConfig *api.CloudProfileConfig, profileSpec core.NamespacedCloudProfileSpec, parentSpec gardencorev1beta1.CloudProfileSpec) field.ErrorList {
	allErrs := field.ErrorList{}

	validationProviderConfig := &api.CloudProfileConfig{
		MachineImages: providerConfig.MachineImages,
	}
	if !equality.Semantic.DeepEqual(validationProviderConfig, providerConfig) {
		allErrs = append(allErrs, field.Forbidden(
			field.NewPath("spec.providerConfig"),
			"must only set machineImages",
		))
	}

	allErrs = append(allErrs, p.validateMachineImages(providerConfig, profileSpec.MachineImages, parentSpec)...)

	return allErrs
}

func (p *namespacedCloudProfile) validateMachineImages(providerConfig *api.CloudProfileConfig, machineImages []core.MachineImage, parentSpec gardencorev1beta1.CloudProfileSpec) field.ErrorList {
	allErrs := field.ErrorList{}

	machineImagesPath := field.NewPath("spec.providerConfig.machineImages")
	for i, machineImage := range providerConfig.MachineImages {
		idxPath := machineImagesPath.Index(i)
		allErrs = append(allErrs, validation.ValidateMachineImage(idxPath, machineImage)...)
	}

	profileImages := util.NewCoreImagesContext(machineImages)
	parentImages := util.NewV1beta1ImagesContext(parentSpec.MachineImages)
	providerImages := newProviderImagesContext(providerConfig.MachineImages)

	for _, machineImage := range profileImages.Images {
		// Check that for each new image version defined in the NamespacedCloudProfile, the image is also defined in the providerConfig.
		_, existsInParent := parentImages.GetImage(machineImage.Name)
		if _, existsInProvider := providerImages.GetImage(machineImage.Name); !existsInParent && !existsInProvider {
			allErrs = append(allErrs, field.Required(
				field.NewPath("spec.providerConfig.machineImages"),
				fmt.Sprintf("machine image %s is not defined in the NamespacedCloudProfile providerConfig", machineImage.Name),
			))
			continue
		}
		for _, version := range machineImage.Versions {
			_, existsInParent := parentImages.GetImageVersion(machineImage.Name, version.Version)
			providerImageVersion, exists := providerImages.GetImageVersion(machineImage.Name, version.Version)
			if !existsInParent && !exists {
				allErrs = append(allErrs, field.Required(
					field.NewPath("spec.providerConfig.machineImages"),
					fmt.Sprintf("machine image version %s@%s is not defined in the NamespacedCloudProfile providerConfig", machineImage.Name, version.Version),
				))
			}

			allErrs = append(allErrs, validateMachineImageArchitectures(machineImage, version, providerImageVersion)...)
		}
	}
	for imageIdx, machineImage := range providerConfig.MachineImages {
		// Check that the machine image version is not already defined in the parent CloudProfile.
		if _, exists := parentImages.GetImage(machineImage.Name); exists {
			for versionIdx, version := range machineImage.Versions {
				if _, exists := parentImages.GetImageVersion(machineImage.Name, version.Version); exists {
					allErrs = append(allErrs, field.Forbidden(
						field.NewPath("spec.providerConfig.machineImages").Index(imageIdx).Child("versions").Index(versionIdx),
						fmt.Sprintf("machine image version %s@%s is already defined in the parent CloudProfile", machineImage.Name, version.Version),
					))
				}
			}
		}
		// Check that the machine image version is defined in the NamespacedCloudProfile.
		if _, exists := profileImages.GetImage(machineImage.Name); !exists {
			allErrs = append(allErrs, field.Required(
				field.NewPath("spec.providerConfig.machineImages").Index(imageIdx),
				fmt.Sprintf("machine image %s is not defined in the NamespacedCloudProfile .spec.machineImages", machineImage.Name),
			))
			continue
		}
		for versionIdx, version := range machineImage.Versions {
			if _, exists := profileImages.GetImageVersion(machineImage.Name, version.Version); !exists {
				allErrs = append(allErrs, field.Invalid(
					field.NewPath("spec.providerConfig.machineImages").Index(imageIdx).Child("versions").Index(versionIdx),
					fmt.Sprintf("%s@%s", machineImage.Name, version.Version),
					"machine image version is not defined in the NamespacedCloudProfile",
				))
			}
		}
	}

	return allErrs
}

func validateMachineImageArchitectures(machineImage core.MachineImage, version core.MachineImageVersion, providerImageVersion api.MachineImageVersion) field.ErrorList {
	allErrs := field.ErrorList{}
	regionsArchitectureMap := map[string][]string{}

	for _, regionMapping := range providerImageVersion.Regions {
		providerConfigArchitecture := ptr.Deref(regionMapping.Architecture, constants.ArchitectureAMD64)
		if !slices.Contains(version.Architectures, providerConfigArchitecture) {
			allErrs = append(allErrs, field.Forbidden(
				field.NewPath("spec.providerConfig.machineImages"),
				fmt.Sprintf("machine image version %s@%s in region %q has an excess entry for architecture %q, which is not defined in the machineImages spec",
					machineImage.Name, version.Version, regionMapping.Name, providerConfigArchitecture),
			))
		}
		regionsArchitectureMap[regionMapping.Name] = append(regionsArchitectureMap[regionMapping.Name], providerConfigArchitecture)
	}

	for _, expectedArchitecture := range version.Architectures {
		if len(regionsArchitectureMap) == 0 {
			allErrs = append(allErrs, field.Required(
				field.NewPath("spec.providerConfig.machineImages"),
				fmt.Sprintf("machine image version %s@%s with architecture %q is not defined in the NamespacedCloudProfile providerConfig",
					machineImage.Name, version.Version, expectedArchitecture),
			))
		}
		for region, architectures := range regionsArchitectureMap {
			if !slices.Contains(architectures, expectedArchitecture) {
				allErrs = append(allErrs, field.Required(
					field.NewPath("spec.providerConfig.machineImages"),
					fmt.Sprintf("machine image version %s@%s for region %q with architecture %q is not defined in the NamespacedCloudProfile providerConfig",
						machineImage.Name, version.Version, region, expectedArchitecture),
				))
			}
		}
	}

	return allErrs
}

func newProviderImagesContext(providerImages []api.MachineImages) *util.ImagesContext[api.MachineImages, api.MachineImageVersion] {
	return util.NewImagesContext(
		utils.CreateMapFromSlice(providerImages, func(mi api.MachineImages) string { return mi.Name }),
		func(mi api.MachineImages) map[string]api.MachineImageVersion {
			return utils.CreateMapFromSlice(mi.Versions, func(v api.MachineImageVersion) string { return v.Version })
		},
	)
}
