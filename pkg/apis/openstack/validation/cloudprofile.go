// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"net"
	"slices"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
)

// ValidateCloudProfileConfig validates a CloudProfileConfig object.
func ValidateCloudProfileConfig(cloudProfile *api.CloudProfileConfig, fldPath *field.Path) field.ErrorList {
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
	for i, pool := range cloudProfile.Constraints.LoadBalancerProviders {
		idxPath := loadBalancerProviderPath.Index(i)

		if len(pool.Name) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("name"), "must provide a name"))
		}

		if pool.Region != nil {
			if len(*pool.Region) == 0 {
				allErrs = append(allErrs, field.Required(idxPath.Child("region"), "must provide a region if key is present"))
			}
			if regionsFound.Has(*pool.Region) {
				allErrs = append(allErrs, field.Duplicate(idxPath.Child("region"), *pool.Region))
			}
			regionsFound.Insert(*pool.Region)
		}
	}

	machineImagesPath := fldPath.Child("machineImages")
	if len(cloudProfile.MachineImages) == 0 {
		allErrs = append(allErrs, field.Required(machineImagesPath, "must provide at least one machine image"))
	}
	for i, machineImage := range cloudProfile.MachineImages {
		idxPath := machineImagesPath.Index(i)

		if len(machineImage.Name) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("name"), "must provide a name"))
		}

		if len(machineImage.Versions) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("versions"), fmt.Sprintf("must provide at least one version for machine image %q", machineImage.Name)))
		}
		for j, version := range machineImage.Versions {
			jdxPath := idxPath.Child("versions").Index(j)

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
	}

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

// validateLoadBalancerClass validates LoadBalancerClass object.
func validateLoadBalancerClass(lbClass api.LoadBalancerClass, fldPath *field.Path) field.ErrorList {
	var allErrs = field.ErrorList{}

	if lbClass.Purpose != nil && *lbClass.Purpose != api.DefaultLoadBalancerClass && *lbClass.Purpose != api.PrivateLoadBalancerClass && *lbClass.Purpose != api.VPNLoadBalancerClass {
		allErrs = append(allErrs, field.Invalid(fldPath, *lbClass.Purpose, fmt.Sprintf("invalid LoadBalancerClass purpose. Valid values are %q or %q", api.DefaultLoadBalancerClass, api.PrivateLoadBalancerClass)))
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
