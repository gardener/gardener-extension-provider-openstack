// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"

	featurevalidation "github.com/gardener/gardener/pkg/utils/validation/features"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/validation/field"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
)

// ValidateControlPlaneConfig validates a ControlPlaneConfig object.
func ValidateControlPlaneConfig(controlPlaneConfig *api.ControlPlaneConfig, infraConfig *api.InfrastructureConfig, version string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(controlPlaneConfig.LoadBalancerProvider) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("loadBalancerProvider"), "must provide the name of a load balancer provider"))
	}

	loadBalancerClassPath := fldPath.Child("loadBalancerClasses")
	allErrs = append(allErrs, ValidateLoadBalancerClasses(controlPlaneConfig.LoadBalancerClasses, loadBalancerClassPath)...)
	for i, class := range controlPlaneConfig.LoadBalancerClasses {
		// Do not allow that the user specifies a vpn LoadBalancerClass in the controlplane config.
		// It needs to come from the CloudProfile.
		if class.Purpose != nil && *class.Purpose == api.VPNLoadBalancerClass {
			allErrs = append(allErrs, field.Invalid(loadBalancerClassPath.Index(i), class.Purpose, fmt.Sprintf("not allowed to specify a LoadBalancerClass with purpose %q", api.VPNLoadBalancerClass)))
		}
	}

	if controlPlaneConfig.CloudControllerManager != nil {
		allErrs = append(allErrs, featurevalidation.ValidateFeatureGates(controlPlaneConfig.CloudControllerManager.FeatureGates, version, fldPath.Child("cloudControllerManager", "featureGates"))...)
	}

	allErrs = append(allErrs, validateStorage(controlPlaneConfig.Storage, infraConfig.Networks, fldPath.Child("storage"))...)

	return allErrs
}

// ValidateControlPlaneConfigUpdate validates a ControlPlaneConfig object.
func ValidateControlPlaneConfigUpdate(_, _ *api.ControlPlaneConfig, _ *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	return allErrs
}

// ValidateControlPlaneConfigAgainstCloudProfile validates the given ControlPlaneConfig against constraints in the given CloudProfile.
func ValidateControlPlaneConfigAgainstCloudProfile(oldCpConfig, cpConfig *api.ControlPlaneConfig, domain, shootRegion, floatingPoolName string, cloudProfileConfig *api.CloudProfileConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if oldCpConfig == nil || oldCpConfig.LoadBalancerProvider != cpConfig.LoadBalancerProvider {
		if ok, validLoadBalancerProviders := validateLoadBalancerProviderConstraints(cloudProfileConfig.Constraints.LoadBalancerProviders, shootRegion, cpConfig.LoadBalancerProvider); !ok {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("loadBalancerProvider"), cpConfig.LoadBalancerProvider, validLoadBalancerProviders))
		}
	}

	if oldCpConfig == nil || !equality.Semantic.DeepEqual(oldCpConfig.LoadBalancerClasses, cpConfig.LoadBalancerClasses) {
		allErrs = append(allErrs, validateLoadBalancerClassesConstraints(cloudProfileConfig.Constraints.FloatingPools, cpConfig.LoadBalancerClasses, domain, shootRegion, floatingPoolName, fldPath.Child("loadBalancerClasses"))...)
	}

	return allErrs
}

func validateLoadBalancerProviderConstraints(providers []api.LoadBalancerProvider, region, provider string) (bool, []string) {
	var (
		validValues      = []string{}
		regionalFound    = false
		defaultProviders = []string{}
	)

	for _, p := range providers {
		if p.Region == nil {
			defaultProviders = append(defaultProviders, p.Name)
			continue
		}

		if *p.Region == region {
			regionalFound = true
			validValues = append(validValues, p.Name)
			if p.Name == provider {
				return true, nil
			}
		}
	}

	// If regional providers are found, don't allow default providers
	if !regionalFound {
		validValues = defaultProviders
		for _, name := range defaultProviders {
			if name == provider {
				return true, nil
			}
		}
	}

	return false, validValues
}

func validateLoadBalancerClassesConstraints(floatingPools []api.FloatingPool, shootLBClasses []api.LoadBalancerClass, domain, shootRegion, floatingPoolName string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(shootLBClasses) == 0 {
		return allErrs
	}

	fp, errs := FindFloatingPool(floatingPools, domain, shootRegion, floatingPoolName, fldPath)
	if len(errs) > 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("floatingPoolName"), floatingPoolName, errs.ToAggregate().Error()))
		return allErrs
	}

	if len(fp.LoadBalancerClasses) == 0 {
		return allErrs
	}

	for i, shootLBClass := range shootLBClasses {
		var (
			valid       bool
			validValues []string
		)

		for _, lbClass := range fp.LoadBalancerClasses {
			if shootLBClass.IsSemanticallyEqual(lbClass) {
				valid = true
				break
			}
			validValues = append(validValues, fmt.Sprint(lbClass))
		}

		if !valid {
			allErrs = append(allErrs, field.NotSupported(fldPath.Index(i), shootLBClass, validValues))
		}
	}

	return allErrs
}

func validateStorage(storage *api.Storage, networks api.Networks, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	if storage == nil || storage.CSIManila == nil || !storage.CSIManila.Enabled {
		return allErrs
	}
	// for an existing subnet we do not need to validate the storage since we can only do that in runtime.
	if networks.SubnetID != nil {
		return allErrs
	}
	if networks.ShareNetwork != nil && networks.ShareNetwork.Enabled {
		return allErrs

	}
	return append(allErrs, field.Invalid(fldPath.Child("csiManila", "enabled"), storage.CSIManila.Enabled, "share network must be created if CSI manila driver is enabled"))
}
