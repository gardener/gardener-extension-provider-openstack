// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateControlPlaneConfig validates a ControlPlaneConfig object.
func ValidateControlPlaneConfig(controlPlaneConfig *api.ControlPlaneConfig, region string, regions []gardencorev1beta1.Region, constraints api.Constraints) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(controlPlaneConfig.LoadBalancerProvider) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("loadBalancerProvider"), "must provide the name of a load balancer provider"))
	} else if ok, validLoadBalancerProviders := validateLoadBalancerProviderConstraints(constraints.LoadBalancerProviders, region, controlPlaneConfig.LoadBalancerProvider, ""); !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("loadBalancerProvider"), controlPlaneConfig.LoadBalancerProvider, validLoadBalancerProviders))
	}

	if len(controlPlaneConfig.Zone) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("zone"), "must provide the name of a zone in this region"))
	} else if ok, validZones := validateZoneConstraints(regions, region, controlPlaneConfig.Zone, ""); !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("zone"), controlPlaneConfig.Zone, validZones))
	}

	return allErrs
}

// ValidateControlPlaneConfigUpdate validates a ControlPlaneConfig object.
func ValidateControlPlaneConfigUpdate(oldConfig, newConfig *api.ControlPlaneConfig, region string, regions []gardencorev1beta1.Region, constraints api.Constraints) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newConfig.Zone, oldConfig.Zone, field.NewPath("zone"))...)

	return allErrs
}

func validateLoadBalancerProviderConstraints(providers []api.LoadBalancerProvider, region, provider, oldProvider string) (bool, []string) {
	if provider == oldProvider {
		return true, nil
	}

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
			} else {
				return false, validValues
			}
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

func validateZoneConstraints(regions []gardencorev1beta1.Region, region, zone, oldZone string) (bool, []string) {
	if zone == oldZone {
		return true, nil
	}

	validValues := []string{}

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
