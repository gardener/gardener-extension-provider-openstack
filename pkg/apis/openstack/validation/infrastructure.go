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
	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"

	cidrvalidation "github.com/gardener/gardener/pkg/utils/validation/cidr"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateInfrastructureConfig validates a InfrastructureConfig object.
func ValidateInfrastructureConfig(infra *api.InfrastructureConfig, nodesCIDR *string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(infra.FloatingPoolName) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("floatingPoolName"), "must provide the name of a floating pool"))
	}

	var nodes cidrvalidation.CIDR
	if nodesCIDR != nil {
		nodes = cidrvalidation.NewCIDR(*nodesCIDR, nil)
	}

	networksPath := fldPath.Child("networks")
	if len(infra.Networks.Worker) == 0 && len(infra.Networks.Workers) == 0 {
		allErrs = append(allErrs, field.Required(networksPath.Child("workers"), "must specify the network range for the worker network"))
	}

	var workerCIDR cidrvalidation.CIDR
	if infra.Networks.Worker != "" {
		workerCIDR = cidrvalidation.NewCIDR(infra.Networks.Worker, networksPath.Child("worker"))
		allErrs = append(allErrs, cidrvalidation.ValidateCIDRParse(workerCIDR)...)
		allErrs = append(allErrs, cidrvalidation.ValidateCIDRIsCanonical(networksPath.Child("worker"), infra.Networks.Worker)...)
	}
	if infra.Networks.Workers != "" {
		workerCIDR = cidrvalidation.NewCIDR(infra.Networks.Workers, networksPath.Child("workers"))
		allErrs = append(allErrs, cidrvalidation.ValidateCIDRParse(workerCIDR)...)
		allErrs = append(allErrs, cidrvalidation.ValidateCIDRIsCanonical(networksPath.Child("workers"), infra.Networks.Workers)...)
	}

	if nodes != nil {
		allErrs = append(allErrs, nodes.ValidateSubset(workerCIDR)...)
	}

	if infra.Networks.Router != nil && len(infra.Networks.Router.ID) == 0 {
		allErrs = append(allErrs, field.Invalid(networksPath.Child("router", "id"), infra.Networks.Router.ID, "router id must not be empty when router key is provided"))
	}

	return allErrs
}

// ValidateInfrastructureConfigUpdate validates a InfrastructureConfig object.
func ValidateInfrastructureConfigUpdate(oldConfig, newConfig *api.InfrastructureConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newConfig.Networks, oldConfig.Networks, fldPath.Child("networks"))...)

	return allErrs
}

// ValidateInfrastructureConfigAgainstCloudProfile validates the given InfrastructureConfig against constraints in the given CloudProfile.
func ValidateInfrastructureConfigAgainstCloudProfile(infra *api.InfrastructureConfig, shootRegion string, cloudProfileConfig *api.CloudProfileConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if ok, validFloatingPoolNames := validateFloatingPoolNameConstraints(cloudProfileConfig.Constraints.FloatingPools, shootRegion, infra.FloatingPoolName); !ok {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("floatingPoolName"), infra.FloatingPoolName, validFloatingPoolNames))
	}

	return allErrs
}

func validateFloatingPoolNameConstraints(names []api.FloatingPool, region string, name string) (bool, []string) {
	var (
		validValues []string
		fallback    *api.FloatingPool
	)

	for _, n := range names {
		// store the first non-regional image for fallback value if no floating pool for the given
		// region was found
		if n.Region == nil && fallback == nil {
			v := n
			fallback = &v
			continue
		}

		// floating pool for the given region found, validate it
		if n.Region != nil && *n.Region == region {
			validValues = append(validValues, n.Name)
			if n.Name == name {
				return true, nil
			}
			return false, validValues
		}
	}

	// no floating pool for the given region found yet, check if the non-regional fallback is used
	if fallback != nil {
		validValues = append(validValues, fallback.Name)
		if fallback.Name == name {
			return true, nil
		}
	}

	return false, validValues
}
