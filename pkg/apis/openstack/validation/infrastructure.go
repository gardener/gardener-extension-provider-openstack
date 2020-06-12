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
	"reflect"
	"sort"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/utils"

	cidrvalidation "github.com/gardener/gardener/pkg/utils/validation/cidr"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/sets"
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

	if infra.Networks.Router != nil && infra.Networks.Router.ID != nil && len(*infra.Networks.Router.ID) == 0 {
		allErrs = append(allErrs, field.Invalid(networksPath.Child("router", "id"), *infra.Networks.Router.ID, "router id must not be empty when router key is provided"))
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
func ValidateInfrastructureConfigAgainstCloudProfile(infra *api.InfrastructureConfig, domain, shootRegion string, cloudProfileConfig *api.CloudProfileConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateFloatingPoolNameConstraints(cloudProfileConfig.Constraints.FloatingPools, domain, shootRegion, infra.FloatingPoolName, fldPath)...)

	return allErrs
}

func validateFloatingPoolNameConstraints(fps []api.FloatingPool, domain, region string, name string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	_, errs := FindFloatingPool(fps, domain, region, name, fldPath.Child("floatingPoolName"))
	allErrs = append(allErrs, errs...)
	return allErrs
}

// FindFloatingPool finds best match for given domain, region, and name
func FindFloatingPool(floatingPools []api.FloatingPool, domain, shootRegion, floatingPoolName string, fldPath *field.Path) (*api.FloatingPool, field.ErrorList) {
	var (
		allErrs               = field.ErrorList{}
		validValues           = sets.NewString()
		additionalValidValues = sets.NewString()
	)

	found := findFloatingPoolCandidate(floatingPools, &domain, &shootRegion, floatingPoolName, false, validValues, additionalValidValues)
	if found != nil {
		return found, allErrs
	}

	// region and domain constraint must be checked together, as constraints of both must be fulfilled
	nonConstrainingOnly := len(validValues) > 0
	validValuesRegion := sets.NewString()
	validValuesDomain := sets.NewString()
	foundRegion := findFloatingPoolCandidate(floatingPools, nil, &shootRegion, floatingPoolName, nonConstrainingOnly, validValuesRegion, additionalValidValues)
	foundDomain := findFloatingPoolCandidate(floatingPools, &domain, nil, floatingPoolName, nonConstrainingOnly, validValuesDomain, additionalValidValues)
	if foundRegion != nil && foundDomain != nil {
		return foundRegion, allErrs
	}
	if foundRegion != nil && len(validValuesDomain) == 0 {
		return foundRegion, allErrs
	}
	if foundDomain != nil && len(validValuesRegion) == 0 {
		return foundDomain, allErrs
	}
	if len(validValuesRegion) != 0 && len(validValuesDomain) != 0 {
		// if region and domain are constrainted separately, only values are valid which are valid for both
		validValues = validValuesRegion.Intersection(validValuesDomain)
	} else if len(validValuesRegion) != 0 {
		validValues = validValuesRegion
	} else if len(validValuesDomain) != 0 {
		validValues = validValuesDomain
	}

	nonConstrainingOnly = len(validValues) > 0
	found = findFloatingPoolCandidate(floatingPools, nil, nil, floatingPoolName, nonConstrainingOnly, validValues, additionalValidValues)
	if found != nil {
		return found, allErrs
	}

	validValuesList := []string{}
	for key := range validValues {
		validValuesList = append(validValuesList, key)
	}
	for key := range additionalValidValues {
		validValuesList = append(validValuesList, key)
	}
	sort.Strings(validValuesList)
	allErrs = append(allErrs, field.NotSupported(fldPath, floatingPoolName, validValuesList))
	return nil, allErrs
}

// findFloatingPoolCandidate finds floating pool candidate with optional domain and/or region constraints
func findFloatingPoolCandidate(floatingPools []api.FloatingPool, domain, shootRegion *string, floatingPoolName string,
	nonConstrainingOnly bool, validValues, additionalValidValues sets.String) *api.FloatingPool {
	var (
		candidate      *api.FloatingPool
		candidateScore int
	)

	for _, fp := range floatingPools {
		if reflect.DeepEqual(domain, fp.Domain) && reflect.DeepEqual(shootRegion, fp.Region) {
			if fp.NonConstraining != nil && *fp.NonConstraining {
				additionalValidValues.Insert(fp.Name)
			} else {
				if nonConstrainingOnly {
					continue
				}
				validValues.Insert(fp.Name)
			}
			if match, score := utils.SimpleMatch(fp.Name, floatingPoolName); match {
				if candidate == nil || candidateScore < score {
					f := fp
					candidate = &f
					candidateScore = score
				}
			}
		}
	}

	return candidate
}
