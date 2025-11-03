package helper

import (
	"slices"

	gardencoreapi "github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"
)

// SimulateTransformToParentFormat simulates the transformation of the given NamespacedCloudProfile and its providerConfig
// to the parent CloudProfile format. This includes the transformation of both the providerConfig and the spec.
func SimulateTransformToParentFormat(cloudProfileConfig *api.CloudProfileConfig, cloudProfile *core.NamespacedCloudProfile, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition) error {
	cloudProfileConfigV1alpha1 := &v1alpha1.CloudProfileConfig{}
	if err := Scheme.Convert(cloudProfileConfig, cloudProfileConfigV1alpha1, nil); err != nil {
		return field.InternalError(field.NewPath("spec.providerConfig"), err)
	}
	namespacedCloudProfileSpecV1beta1 := gardencorev1beta1.NamespacedCloudProfileSpec{}
	if err := gardencoreapi.Scheme.Convert(&cloudProfile.Spec, &namespacedCloudProfileSpecV1beta1, nil); err != nil {
		return field.InternalError(field.NewPath("spec"), err)
	}

	// simulate transformation to parent spec format
	// - performed in mutating extension webhook
	transformedSpecConfig := TransformProviderConfigToParentFormat(cloudProfileConfigV1alpha1, capabilityDefinitions)
	// - performed in namespaced cloud profile controller
	transformedSpec := gutil.TransformSpecToParentFormat(namespacedCloudProfileSpecV1beta1, capabilityDefinitions)

	if err := Scheme.Convert(transformedSpecConfig, cloudProfileConfig, nil); err != nil {
		return field.InternalError(field.NewPath("spec.providerConfig"), err)
	}
	if err := gardencoreapi.Scheme.Convert(&transformedSpec, &cloudProfile.Spec, nil); err != nil {
		return field.InternalError(field.NewPath("spec"), err)
	}
	return nil
}

// TransformProviderConfigToParentFormat supports the migration from the deprecated architecture fields to architecture capabilities.
// Depending on whether the parent CloudProfile is in capability format or not, it transforms the given config to
// the capability format or the deprecated architecture fields format respectively.
// It assumes that the given config is either completely in the capability format or in the deprecated architecture fields format.
func TransformProviderConfigToParentFormat(config *v1alpha1.CloudProfileConfig, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition) *v1alpha1.CloudProfileConfig {
	if config == nil {
		return &v1alpha1.CloudProfileConfig{}
	}

	transformedConfig := v1alpha1.CloudProfileConfig{
		TypeMeta:      config.TypeMeta,
		MachineImages: transformMachineImages(config.MachineImages, capabilityDefinitions),
	}

	return &transformedConfig
}

func transformMachineImages(images []v1alpha1.MachineImages, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition) []v1alpha1.MachineImages {
	result := make([]v1alpha1.MachineImages, 0, len(images))

	for _, img := range images {
		transformedVersions := transformImageVersions(img.Versions, capabilityDefinitions)
		result = append(result, v1alpha1.MachineImages{
			Name:     img.Name,
			Versions: transformedVersions,
		})
	}

	return result
}

func transformImageVersions(versions []v1alpha1.MachineImageVersion, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition) []v1alpha1.MachineImageVersion {
	result := make([]v1alpha1.MachineImageVersion, 0, len(versions))

	for _, version := range versions {
		transformed := v1alpha1.MachineImageVersion{Version: version.Version}
		if len(capabilityDefinitions) != 0 {
			transformed.CapabilityFlavors = transformToCapabilityFormat(version, capabilityDefinitions)
		} else {
			transformed.Regions = transformToLegacyFormat(version)
		}
		result = append(result, transformed)
	}

	return result
}

// sortRegions sorts a slice of RegionIDMapping by name
func sortRegions(regions []v1alpha1.RegionIDMapping) {
	slices.SortFunc(regions, func(a, b v1alpha1.RegionIDMapping) int {
		if a.Name < b.Name {
			return -1
		}
		if a.Name > b.Name {
			return 1
		}
		return 0
	})
}

// transformToCapabilityFormat converts legacy format (regions with architecture) to capability format
func transformToCapabilityFormat(version v1alpha1.MachineImageVersion, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition) []v1alpha1.MachineImageFlavor {
	if len(version.CapabilityFlavors) > 0 {
		// Already in capability format, return as-is
		return version.CapabilityFlavors
	}

	if len(version.Regions) == 0 {
		return nil
	}

	// Group regions by architecture
	architectureGroups := make(map[string][]v1alpha1.RegionIDMapping)

	for _, region := range version.Regions {
		// Default to "amd64" if architecture is not specified (backward compatibility)
		arch := ptr.Deref(region.Architecture, v1beta1constants.ArchitectureAMD64)
		// Create a clean region mapping without architecture field for capability format
		cleanRegion := v1alpha1.RegionIDMapping{
			Name: region.Name,
			ID:   region.ID,
			// Architecture field is omitted in capability format
		}

		architectureGroups[arch] = append(architectureGroups[arch], cleanRegion)
	}

	// Convert groups to capability flavors
	var imageFlavors []v1alpha1.MachineImageFlavor
	for arch, regions := range architectureGroups {
		sortRegions(regions)
		flavor := v1alpha1.MachineImageFlavor{
			Regions: regions,
			Capabilities: gardencorev1beta1.Capabilities{
				v1beta1constants.ArchitectureName: []string{arch},
			},
		}
		imageFlavors = append(imageFlavors, flavor)
	}

	// Sort flavors for consistent output (alphabetically by architecture)\
	slices.SortFunc(imageFlavors, func(a, b v1alpha1.MachineImageFlavor) int {
		archA := getFirstArchitecture(a.Capabilities, capabilityDefinitions)
		archB := getFirstArchitecture(b.Capabilities, capabilityDefinitions)

		if archA < archB {
			return -1
		}
		if archA > archB {
			return 1
		}
		return 0
	})

	return imageFlavors
}

// transformToLegacyFormat converts capability format to legacy format (regions with architecture)
func transformToLegacyFormat(version v1alpha1.MachineImageVersion) []v1alpha1.RegionIDMapping {
	if len(version.Regions) > 0 {
		// Already in legacy format, return as-is
		return version.Regions
	}

	if len(version.CapabilityFlavors) == 0 {
		return nil
	}

	var allRegions []v1alpha1.RegionIDMapping

	for _, flavor := range version.CapabilityFlavors {
		// Extract architecture from capabilities
		arch := getFirstArchitecture(flavor.Capabilities, nil)

		// Add architecture field to each region
		for _, region := range flavor.Regions {
			legacyRegion := v1alpha1.RegionIDMapping{
				Name:         region.Name,
				ID:           region.ID,
				Architecture: &arch,
			}
			allRegions = append(allRegions, legacyRegion)
		}
	}

	// Sort regions by name for consistent output
	sortRegions(allRegions)

	return allRegions
}

// getFirstArchitecture extracts the first architecture from capabilities, defaults to "amd64"
func getFirstArchitecture(capabilities gardencorev1beta1.Capabilities, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition) string {
	defaultedCapabilities := capabilities
	if len(capabilityDefinitions) > 0 {
		defaultedCapabilities = gardencorev1beta1.GetCapabilitiesWithAppliedDefaults(capabilities, capabilityDefinitions)
	}

	if defaultedCapabilities == nil {
		return v1beta1constants.ArchitectureAMD64
	}

	archList, exists := defaultedCapabilities["architecture"]
	if !exists || len(archList) == 0 {
		return v1beta1constants.ArchitectureAMD64
	}

	return archList[0]
}
