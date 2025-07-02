// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"reflect"
	"sort"

	cidrvalidation "github.com/gardener/gardener/pkg/utils/validation/cidr"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/utils"
)

// ValidateInfrastructureConfig validates a InfrastructureConfig object.
func ValidateInfrastructureConfig(infra *api.InfrastructureConfig, nodesCIDR *string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(infra.FloatingPoolName) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("floatingPoolName"), "must provide the name of a floating pool"))
	}
	allErrs = append(allErrs, validateResourceName(infra.FloatingPoolName, fldPath.Child("floatingPoolName"))...)

	networkingPath := field.NewPath("networking")
	var nodes cidrvalidation.CIDR
	if nodesCIDR != nil {
		nodes = cidrvalidation.NewCIDR(*nodesCIDR, networkingPath.Child("nodes"))
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

	if infra.Networks.ID != nil {
		allErrs = append(allErrs, uuid(*infra.Networks.ID, networksPath.Child("id"))...)
	}
	if infra.Networks.Router != nil {
		if infra.Networks.Router.ID == "" {
			allErrs = append(allErrs, field.Invalid(networksPath.Child("router", "id"), infra.Networks.Router.ID, "router id must not be empty when router key is provided"))
		} else {
			allErrs = append(allErrs, uuid(infra.Networks.Router.ID, networksPath.Child("router").Child("id"))...)
		}
	}

	if infra.FloatingPoolSubnetName != nil {
		allErrs = append(allErrs, validateResourceName(*infra.FloatingPoolSubnetName, fldPath.Child("floatingPoolSubnetName"))...)

		if infra.Networks.Router != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("floatingPoolSubnetName"), *infra.FloatingPoolSubnetName, "router id must be empty when a floating subnet name is provided"))
		}
	}

	return allErrs
}

// ValidateInfrastructureConfigUpdate validates a InfrastructureConfig object.
func ValidateInfrastructureConfigUpdate(oldConfig, newConfig *api.InfrastructureConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	newNetworks := newConfig.DeepCopy().Networks
	oldNetworks := oldConfig.DeepCopy().Networks

	// only enablement of share network enablement is allowed as update operation. Therefore we ignore it, when checking for other updates.
	// TODO: allow both enabling and disabling of share networks.
	if oldNetworks.ShareNetwork == nil || !oldNetworks.ShareNetwork.Enabled {
		newNetworks.ShareNetwork = nil
		oldNetworks.ShareNetwork = nil
	}
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newNetworks, oldNetworks, fldPath.Child("networks"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newConfig.FloatingPoolName, oldConfig.FloatingPoolName, fldPath.Child("floatingPoolName"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newConfig.FloatingPoolSubnetName, oldConfig.FloatingPoolSubnetName, fldPath.Child("floatingPoolSubnetName"))...)

	return allErrs
}

// ValidateInfrastructureConfigAgainstCloudProfile validates the given InfrastructureConfig against constraints in the given CloudProfile.
func ValidateInfrastructureConfigAgainstCloudProfile(oldInfra, infra *api.InfrastructureConfig, domain, shootRegion string, cloudProfileConfig *api.CloudProfileConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if oldInfra == nil || oldInfra.FloatingPoolName != infra.FloatingPoolName {
		allErrs = append(allErrs, validateFloatingPoolNameConstraints(cloudProfileConfig.Constraints.FloatingPools, domain, shootRegion, infra.FloatingPoolName, fldPath)...)
	}

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
		validValues           = sets.New[string]()
		additionalValidValues = sets.New[string]()
	)

	found := findFloatingPoolCandidate(floatingPools, &domain, &shootRegion, floatingPoolName, false, validValues, additionalValidValues)
	if found != nil {
		return found, allErrs
	}

	// region and domain constraint must be checked together, as constraints of both must be fulfilled
	nonConstrainingOnly := len(validValues) > 0
	validValuesRegion := sets.New[string]()
	validValuesDomain := sets.New[string]()
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
	nonConstrainingOnly bool, validValues, additionalValidValues sets.Set[string]) *api.FloatingPool {
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
