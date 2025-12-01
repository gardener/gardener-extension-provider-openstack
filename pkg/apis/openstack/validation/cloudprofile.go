// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"maps"
	"net"
	"slices"

	gardencoreapi "github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/gardener"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
)

// ValidateCloudProfileConfig validates a CloudProfileConfig object.
func ValidateCloudProfileConfig(cloudProfile *api.CloudProfileConfig, machineImages []core.MachineImage, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	floatingPoolPath := fldPath.Child("constraints", "floatingPools")
	if len(cloudProfile.Constraints.FloatingPools) == 0 {
		allErrs = append(allErrs, field.Required(floatingPoolPath, "must provide at least one floating pool"))
	}

	combinationFound := sets.NewString()
	for i, pool := range cloudProfile.Constraints.FloatingPools {
		idxPath := floatingPoolPath.Index(i)
		if len(pool.Name) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("name"), "must provide a name"))
		}

		if pool.Region != nil || pool.Domain != nil {
			region := "*"
			domain := "*"
			if pool.Region != nil {
				if len(*pool.Region) == 0 {
					allErrs = append(allErrs, field.Required(idxPath.Child("region"), "must provide a region if key is present"))
				}
				region = *pool.Region
			}
			if pool.Domain != nil {
				if len(*pool.Domain) == 0 {
					allErrs = append(allErrs, field.Required(idxPath.Child("domain"), "must provide a domain if key is present"))
				}
				domain = *pool.Domain
			}
			key := fmt.Sprintf("%s,%s,%s", pool.Name, domain, region)
			if combinationFound.Has(key) {
				// duplicate for given name/domain/region combination
				allErrs = append(allErrs, field.Duplicate(idxPath.Child("name"), pool.Name))
			}
			combinationFound.Insert(key)
		}
	}

	for i, pool := range cloudProfile.Constraints.FloatingPools {
		allErrs = append(allErrs, ValidateLoadBalancerClasses(pool.LoadBalancerClasses, floatingPoolPath.Index(i).Child("loadBalancerClasses"))...)
	}

	loadBalancerProviderPath := fldPath.Child("constraints", "loadBalancerProviders")
	if len(cloudProfile.Constraints.LoadBalancerProviders) == 0 {
		allErrs = append(allErrs, field.Required(loadBalancerProviderPath, "must provide at least one load balancer provider"))
	}

	regionsFound := sets.NewString()
	for i, provider := range cloudProfile.Constraints.LoadBalancerProviders {
		idxPath := loadBalancerProviderPath.Index(i)

		if len(provider.Name) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("name"), "must provide a name"))
		}

		if provider.Region != nil {
			if len(*provider.Region) == 0 {
				allErrs = append(allErrs, field.Required(idxPath.Child("region"), "must provide a region if key is present"))
			}
			providerID := fmt.Sprintf("%s,%s", provider.Name, *provider.Region)
			if regionsFound.Has(providerID) {
				allErrs = append(allErrs, field.Duplicate(idxPath, fmt.Sprintf("duplicate provider %q for region %q", provider.Name, *provider.Region)))
			}
			regionsFound.Insert(providerID)
		}
	}

	machineImagesPath := fldPath.Child("machineImages")
	if len(cloudProfile.MachineImages) == 0 {
		allErrs = append(allErrs, field.Required(machineImagesPath, "must provide at least one machine image"))
	}
	for i, machineImage := range cloudProfile.MachineImages {
		idxPath := machineImagesPath.Index(i)
		allErrs = append(allErrs, ValidateProviderMachineImage(machineImage, capabilityDefinitions, idxPath)...)
	}
	allErrs = append(allErrs, validateMachineImageMapping(machineImages, cloudProfile, capabilityDefinitions, field.NewPath("spec").Child("machineImages"))...)

	if len(cloudProfile.KeyStoneURL) == 0 && len(cloudProfile.KeyStoneURLs) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("keyStoneURL"), "must provide the URL to KeyStone"))
	}
	if ca := cloudProfile.KeyStoneCACert; ca != nil && len(*ca) > 0 {
		_, err := utils.DecodeCertificate([]byte(*ca))
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("caCert"), *ca, "caCert is not a valid PEM-encoded certificate"))
		}
	}

	regionsFound = sets.NewString()
	for i, val := range cloudProfile.KeyStoneURLs {
		idxPath := fldPath.Child("keyStoneURLs").Index(i)

		if len(val.Region) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("region"), "must provide a region"))
		}

		if len(val.URL) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("url"), "must provide an url"))
		}

		if ca := val.CACert; ca != nil && len(*ca) > 0 {
			_, err := utils.DecodeCertificate([]byte(*ca))
			if err != nil {
				allErrs = append(allErrs, field.Invalid(idxPath.Child("caCert"), *ca, "caCert is not a valid PEM-encoded certificate"))
			}
		}

		if regionsFound.Has(val.Region) {
			allErrs = append(allErrs, field.Duplicate(idxPath.Child("region"), val.Region))
		}
		regionsFound.Insert(val.Region)
	}

	for i, ip := range cloudProfile.DNSServers {
		if net.ParseIP(ip) == nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("dnsServers").Index(i), ip, "must provide a valid IP"))
		}
	}

	if cloudProfile.DHCPDomain != nil && len(*cloudProfile.DHCPDomain) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("dhcpDomain"), "must provide a dhcp domain when the key is specified"))
	}

	serverGroupPath := fldPath.Child("serverGroupPolicies")
	for i, policy := range cloudProfile.ServerGroupPolicies {
		idxPath := serverGroupPath.Index(i)

		if len(policy) == 0 {
			allErrs = append(allErrs, field.Required(idxPath, "policy cannot be empty"))
		}
	}

	return allErrs
}

// ValidateProviderMachineImage validates a CloudProfileConfig MachineImages entry.
func ValidateProviderMachineImage(providerImage api.MachineImages, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, validationPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(providerImage.Name) == 0 {
		allErrs = append(allErrs, field.Required(validationPath.Child("name"), "must provide a name"))
	}

	if len(providerImage.Versions) == 0 {
		allErrs = append(allErrs, field.Required(validationPath.Child("versions"), fmt.Sprintf("must provide at least one version for machine image %q", providerImage.Name)))
	}

	// Validate each version
	for j, version := range providerImage.Versions {
		jdxPath := validationPath.Child("versions").Index(j)
		allErrs = append(allErrs, validateMachineImageVersion(providerImage, capabilityDefinitions, version, jdxPath)...)
	}

	return allErrs
}

// validateMachineImageVersion validates a specific machine image version
func validateMachineImageVersion(providerImage api.MachineImages, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, version api.MachineImageVersion, jdxPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(version.Version) == 0 {
		allErrs = append(allErrs, field.Required(jdxPath.Child("version"), "must provide a version"))
	}

	if len(capabilityDefinitions) > 0 {
		allErrs = append(allErrs, validateCapabilityFlavors(providerImage, version, capabilityDefinitions, jdxPath)...)
	} else {
		allErrs = append(allErrs, validateRegions(version.Regions, providerImage.Name, version.Version, capabilityDefinitions, jdxPath)...)
		if len(version.CapabilityFlavors) > 0 {
			allErrs = append(allErrs, field.Forbidden(jdxPath.Child("capabilityFlavors"), "must not be set as CloudProfile does not define capabilities. Use regions instead."))
		}
	}
	return allErrs
}

// validateCapabilityFlavors validates the capability flavors of a machine image version.
func validateCapabilityFlavors(providerImage api.MachineImages, version api.MachineImageVersion, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, jdxPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// When using capabilities, regions must not be set
	if len(version.Regions) > 0 {
		allErrs = append(allErrs, field.Forbidden(jdxPath.Child("regions"), "must not be set as CloudProfile defines capabilities. Use capabilityFlavors.regions instead."))
	}

	// Validate each flavor's capabilities and regions
	for k, capabilitySet := range version.CapabilityFlavors {
		kdxPath := jdxPath.Child("capabilityFlavors").Index(k)
		allErrs = append(allErrs, gutil.ValidateCapabilities(capabilitySet.Capabilities, capabilityDefinitions, kdxPath.Child("capabilities"))...)
		allErrs = append(allErrs, validateRegions(capabilitySet.Regions, providerImage.Name, version.Version, capabilityDefinitions, kdxPath)...)
	}
	return allErrs
}

// validateRegions validates the regions of a machine image version or capability flavor.
func validateRegions(regions []api.RegionIDMapping, name, version string, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, jdxPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for k, region := range regions {
		kdxPath := jdxPath.Child("regions").Index(k)
		arch := ptr.Deref(region.Architecture, v1beta1constants.ArchitectureAMD64)

		if len(region.Name) == 0 {
			allErrs = append(allErrs, field.Required(kdxPath.Child("name"), "must provide a name"))
		}
		if len(region.ID) == 0 {
			allErrs = append(allErrs, field.Required(kdxPath.Child("id"), "must provide an image ID"))
		}
		if len(capabilityDefinitions) == 0 {
			if !slices.Contains(v1beta1constants.ValidArchitectures, arch) {
				allErrs = append(allErrs, field.NotSupported(kdxPath.Child("architecture"), arch, v1beta1constants.ValidArchitectures))
			}
		}
		// This should be commented in once the defaulting of the architecture field is implemented via mutating webhook
		// currently there is no way to distinguish between a user set architecture and the default one
		if len(capabilityDefinitions) > 0 {
			if region.Architecture != nil {
				allErrs = append(allErrs, field.Forbidden(kdxPath.Child("architecture"), "must be defined in .capabilities.architecture"))
			}
		}
	}
	return allErrs
}

// ValidateProviderMachineImageLegacy validates a CloudProfileConfig MachineImages entry.
func ValidateProviderMachineImageLegacy(validationPath *field.Path, machineImage api.MachineImages) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(machineImage.Name) == 0 {
		allErrs = append(allErrs, field.Required(validationPath.Child("name"), "must provide a name"))
	}

	if len(machineImage.Versions) == 0 {
		allErrs = append(allErrs, field.Required(validationPath.Child("versions"), fmt.Sprintf("must provide at least one version for machine image %q", machineImage.Name)))
	}
	for j, version := range machineImage.Versions {
		jdxPath := validationPath.Child("versions").Index(j)

		if len(version.Version) == 0 {
			allErrs = append(allErrs, field.Required(jdxPath.Child("version"), "must provide a version"))
		}

		for k, region := range version.Regions {
			kdxPath := jdxPath.Child("regions").Index(k)

			if len(region.Name) == 0 {
				allErrs = append(allErrs, field.Required(kdxPath.Child("name"), "must provide a name"))
			}
			if len(region.ID) == 0 {
				allErrs = append(allErrs, field.Required(kdxPath.Child("id"), "must provide an image ID"))
			}
			if !slices.Contains(v1beta1constants.ValidArchitectures, ptr.Deref(region.Architecture, v1beta1constants.ArchitectureAMD64)) {
				allErrs = append(allErrs, field.NotSupported(kdxPath.Child("architecture"), *region.Architecture, v1beta1constants.ValidArchitectures))
			}
		}
	}

	return allErrs
}

// NewProviderImagesContext creates a new ImagesContext for provider images.
func NewProviderImagesContext(providerImages []api.MachineImages) *gardener.ImagesContext[api.MachineImages, api.MachineImageVersion] {
	return gardener.NewImagesContext(
		utils.CreateMapFromSlice(providerImages, func(mi api.MachineImages) string { return mi.Name }),
		func(mi api.MachineImages) map[string]api.MachineImageVersion {
			return utils.CreateMapFromSlice(mi.Versions, func(v api.MachineImageVersion) string { return v.Version })
		},
	)
}

// validateMachineImageMapping validates that for each machine image there is a corresponding cpConfig image.
func validateMachineImageMapping(machineImages []core.MachineImage, cpConfig *api.CloudProfileConfig, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	providerImages := NewProviderImagesContext(cpConfig.MachineImages)

	// validate machine images
	for idxImage, machineImage := range machineImages {
		if len(machineImage.Versions) == 0 {
			continue
		}
		machineImagePath := fldPath.Index(idxImage)
		// validate that for each machine image there is a corresponding cpConfig image
		if _, existsInConfig := providerImages.GetImage(machineImage.Name); !existsInConfig {
			allErrs = append(allErrs, field.Required(machineImagePath,
				fmt.Sprintf("must provide an image mapping for image %q in providerConfig", machineImage.Name)))
			continue
		}
		// validate that for each machine image version entry a mapped entry in cpConfig exists
		for idxVersion, version := range machineImage.Versions {
			machineImageVersionPath := machineImagePath.Child("versions").Index(idxVersion)
			if len(capabilityDefinitions) > 0 {
				// check that each MachineImageFlavor in version.CapabilityFlavors has a corresponding imageVersion.CapabilityFlavors
				imageVersion, exists := providerImages.GetImageVersion(machineImage.Name, version.Version)
				if !exists {
					allErrs = append(allErrs, field.Required(machineImageVersionPath,
						fmt.Sprintf("machine image version %s@%s is not defined in the providerConfig",
							machineImage.Name, version.Version),
					))
					continue
				}
				allErrs = append(allErrs, validateImageFlavorMapping(machineImage, version, machineImageVersionPath, capabilityDefinitions, imageVersion)...)
			} else {
				for _, expectedArchitecture := range version.Architectures {
					// validate machine image version architectures
					if !slices.Contains(v1beta1constants.ValidArchitectures, expectedArchitecture) {
						allErrs = append(allErrs, field.NotSupported(
							machineImageVersionPath.Child("architectures"),
							expectedArchitecture, v1beta1constants.ValidArchitectures))
					}
					// validate that machine image version exists in cpConfig
					imageVersion, exists := providerImages.GetImageVersion(machineImage.Name, version.Version)
					if !exists {
						allErrs = append(allErrs, field.Required(machineImageVersionPath,
							fmt.Sprintf("machine image version %s@%s is not defined in the providerConfig",
								machineImage.Name, version.Version),
						))
						continue
					}

					// Regions is an optional field
					if len(imageVersion.Regions) > 0 {
						// validate that machine image version with architecture x exists in cpConfig
						architecturesMap := utils.CreateMapFromSlice(imageVersion.Regions, func(re api.RegionIDMapping) string {
							return ptr.Deref(re.Architecture, v1beta1constants.ArchitectureAMD64)
						})
						architectures := slices.Collect(maps.Keys(architecturesMap))
						if !slices.Contains(architectures, expectedArchitecture) {
							allErrs = append(allErrs, field.Required(machineImageVersionPath,
								fmt.Sprintf("missing providerConfig mapping for machine image version %s@%s and architecture: %s",
									machineImage.Name, version.Version, expectedArchitecture),
							))
							continue
						}
					}
				}
			}
		}
	}

	return allErrs
}

// validateLoadBalancerClass validates LoadBalancerClass object.
func validateLoadBalancerClass(lbClass api.LoadBalancerClass, fldPath *field.Path) field.ErrorList {
	var allErrs = field.ErrorList{}

	if lbClass.Purpose != nil && *lbClass.Purpose != api.DefaultLoadBalancerClass && *lbClass.Purpose != api.PrivateLoadBalancerClass && *lbClass.Purpose != api.VPNLoadBalancerClass {
		allErrs = append(allErrs, field.Invalid(fldPath, *lbClass.Purpose, fmt.Sprintf("invalid LoadBalancerClass purpose. Valid values are %q or %q", api.DefaultLoadBalancerClass, api.PrivateLoadBalancerClass)))
	}
	if lbClass.FloatingNetworkID != nil {
		allErrs = append(allErrs, uuid(*lbClass.FloatingNetworkID, fldPath.Child("floatingNetworkID"))...)
	}
	if lbClass.FloatingSubnetID != nil {
		allErrs = append(allErrs, uuid(*lbClass.FloatingSubnetID, fldPath.Child("floatingSubnetID"))...)
	}

	allErrs = append(allErrs, validateResourceName(lbClass.Name, fldPath.Child("name"))...)
	if lbClass.FloatingSubnetName != nil {
		allErrs = append(allErrs, validateResourceName(*lbClass.FloatingSubnetName, fldPath.Child("floatingSubnetName"))...)
	}
	if lbClass.FloatingSubnetTags != nil {
		allErrs = append(allErrs, validateResourceName(*lbClass.FloatingSubnetTags, fldPath.Child("floatingSubnetTags"))...)
	}

	if lbClass.FloatingSubnetID != nil && lbClass.FloatingSubnetName != nil && lbClass.FloatingSubnetTags != nil {
		return append(allErrs, field.Forbidden(fldPath, "cannot select floating subnet by id, name and tags in parallel"))
	}
	if lbClass.FloatingSubnetID != nil && (lbClass.FloatingSubnetName != nil || lbClass.FloatingSubnetTags != nil) {
		return append(allErrs, field.Forbidden(fldPath, "specify floating subnet id and name or tags is not possible"))
	}
	if lbClass.FloatingSubnetName != nil && (lbClass.FloatingSubnetID != nil || lbClass.FloatingSubnetTags != nil) {
		return append(allErrs, field.Forbidden(fldPath, "specify floating subnet name and id or tags is not possible"))
	}
	if lbClass.FloatingSubnetTags != nil && (lbClass.FloatingSubnetID != nil || lbClass.FloatingSubnetName != nil) {
		return append(allErrs, field.Forbidden(fldPath, "specify floating subnet tags and id or name is not possible"))
	}

	return allErrs
}

// ValidateLoadBalancerClasses validates a given list of LoadBalancerClass objects.
func ValidateLoadBalancerClasses(loadBalancerClasses []api.LoadBalancerClass, fldPath *field.Path) field.ErrorList {
	var (
		defaultClassExists bool
		privateClassExists bool

		allErrs      = field.ErrorList{}
		lbClassNames = sets.NewString()
	)

	for i, class := range loadBalancerClasses {
		lbClassPath := fldPath.Index(i)

		// Validate first the load balancer class itself.
		allErrs = append(allErrs, validateLoadBalancerClass(class, lbClassPath)...)

		// All load balancer classes need to have an unique name. Check for duplicates.
		if lbClassNames.Has(class.Name) {
			allErrs = append(allErrs, field.Duplicate(lbClassPath.Child("name"), class.Name))
		} else {
			lbClassNames.Insert(class.Name)
		}

		// There can only be one default load balancer class. Check for multiple default classes.
		if (class.Purpose != nil && *class.Purpose == api.DefaultLoadBalancerClass) || class.Name == api.DefaultLoadBalancerClass {
			if defaultClassExists {
				allErrs = append(allErrs, field.Invalid(fldPath, loadBalancerClasses, "not allowed to configure multiple default load balancer classes"))
			} else {
				defaultClassExists = true
			}
		}

		// There can only be one private load balancer class. Check for multiple private classes.
		if (class.Purpose != nil && *class.Purpose == api.PrivateLoadBalancerClass) || class.Name == api.PrivateLoadBalancerClass {
			if privateClassExists {
				allErrs = append(allErrs, field.Invalid(fldPath, loadBalancerClasses, "not allowed to configure multiple private load balancer classes"))
			} else {
				privateClassExists = true
			}
		}
	}

	return allErrs
}

// validateImageFlavorMapping validates that each flavor in a version has a corresponding mapping
func validateImageFlavorMapping(machineImage core.MachineImage, version core.MachineImageVersion, machineImageVersionPath *field.Path, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, imageVersion api.MachineImageVersion) field.ErrorList {
	allErrs := field.ErrorList{}

	var v1beta1Version gardencorev1beta1.MachineImageVersion
	if err := gardencoreapi.Scheme.Convert(&version, &v1beta1Version, nil); err != nil {
		return append(allErrs, field.InternalError(machineImageVersionPath, err))
	}

	defaultedCapabilityFlavors := gardencorev1beta1helper.GetImageFlavorsWithAppliedDefaults(v1beta1Version.CapabilityFlavors, capabilityDefinitions)
	for idxCapability, defaultedCapabilitySet := range defaultedCapabilityFlavors {
		isFound := false
		// search for the corresponding imageVersion.MachineImageFlavor
		for _, providerCapabilitySet := range imageVersion.CapabilityFlavors {
			providerDefaultedCapabilities := gardencorev1beta1.GetCapabilitiesWithAppliedDefaults(providerCapabilitySet.Capabilities, capabilityDefinitions)
			if gardencorev1beta1helper.AreCapabilitiesEqual(defaultedCapabilitySet.Capabilities, providerDefaultedCapabilities) {
				isFound = true
				break
			}
		}
		if !isFound {
			allErrs = append(allErrs, field.Required(machineImageVersionPath.Child("capabilityFlavors").Index(idxCapability),
				fmt.Sprintf("missing providerConfig mapping for machine image version %s@%s and capabilitySet %v", machineImage.Name, version.Version, defaultedCapabilitySet.Capabilities)))
		}
	}
	return allErrs
}
