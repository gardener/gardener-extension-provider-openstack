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

package validation

import (
	"fmt"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"k8s.io/apimachinery/pkg/api/equality"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateControlPlaneConfig validates a ControlPlaneConfig object.
func ValidateControlPlaneConfig(controlPlaneConfig *api.ControlPlaneConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(controlPlaneConfig.LoadBalancerProvider) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("loadBalancerProvider"), "must provide the name of a load balancer provider"))
	}

	if len(controlPlaneConfig.Zone) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("zone"), "must provide the name of a zone in this region"))
	}

	return allErrs
}

// ValidateControlPlaneConfigUpdate validates a ControlPlaneConfig object.
func ValidateControlPlaneConfigUpdate(oldConfig, newConfig *api.ControlPlaneConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newConfig.Zone, oldConfig.Zone, fldPath.Child("zone"))...)

	return allErrs
}

// ValidateControlPlaneConfigAgainstCloudProfile validates the given ControlPlaneConfig against constraints in the given CloudProfile.
func ValidateControlPlaneConfigAgainstCloudProfile(cpConfig *api.ControlPlaneConfig, shootRegion, floatingPoolName string, cloudProfile *gardencorev1beta1.CloudProfile, cloudProfileConfig *api.CloudProfileConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if ok, validLoadBalancerProviders := validateLoadBalancerProviderConstraints(cloudProfileConfig.Constraints.LoadBalancerProviders, shootRegion, cpConfig.LoadBalancerProvider); !ok {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("loadBalancerProvider"), cpConfig.LoadBalancerProvider, validLoadBalancerProviders))
	}

	allErrs = append(allErrs, validateLoadBalancerClassesConstraints(cloudProfileConfig.Constraints.FloatingPools, cpConfig.LoadBalancerClasses, shootRegion, floatingPoolName, fldPath.Child("loadBalancerClasses"))...)

	if ok, validZones := validateZoneConstraints(cloudProfile.Spec.Regions, shootRegion, cpConfig.Zone); !ok {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("zone"), cpConfig.Zone, validZones))
	}

	return allErrs
}

func validateLoadBalancerProviderConstraints(providers []api.LoadBalancerProvider, region, provider string) (bool, []string) {
	var (
		validValues = []string{}
		fallback    *api.LoadBalancerProvider
	)

	for _, p := range providers {
		// store the first non-regional image for fallback value if no load balancer provider for the given
		// region was found
		if p.Region == nil && fallback == nil {
			v := p
			fallback = &v
			continue
		}

		// load balancer provider for the given region found, validate it
		if p.Region != nil && *p.Region == region {
			validValues = append(validValues, p.Name)
			if p.Name == provider {
				return true, nil
			}
			return false, validValues
		}
	}

	// no load balancer provider for the given region found yet, check if the non-regional fallback is used
	if fallback != nil {
		validValues = append(validValues, fallback.Name)
		if fallback.Name == provider {
			return true, nil
		}
	}

	return false, validValues
}

func validateZoneConstraints(regions []gardencorev1beta1.Region, region, zone string) (bool, []string) {
	var validValues []string

	for _, r := range regions {
		if r.Name != region {
			continue
		}

		for _, z := range r.Zones {
			validValues = append(validValues, z.Name)
			if z.Name == zone {
				return true, nil
			}
		}
	}

	return false, validValues
}

func validateLoadBalancerClassesConstraints(floatingPools []api.FloatingPool, shootLBClasses []api.LoadBalancerClass, shootRegion, floatingPoolName string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(shootLBClasses) == 0 {
		return allErrs
	}

	fp, errs := findFloatingPool(floatingPools, shootRegion, floatingPoolName, fldPath)
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
			if equality.Semantic.DeepEqual(shootLBClass, lbClass) {
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
