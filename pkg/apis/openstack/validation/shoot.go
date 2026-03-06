// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"math"

	"github.com/gardener/gardener/pkg/apis/core"
	corehelper "github.com/gardener/gardener/pkg/apis/core/helper"
	validationutils "github.com/gardener/gardener/pkg/utils/validation"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
)

// ValidateNetworking validates the network settings of a Shoot.
func ValidateNetworking(networking *core.Networking, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if networking.Nodes == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("nodes"), "a nodes CIDR must be provided for Openstack shoots"))
	}

	if core.IsIPv6SingleStack(networking.IPFamilies) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("ipFamilies"), networking.IPFamilies, "IPv6 single-stack networking is not supported"))
	}

	return allErrs
}

// ValidateWorkers validates the workers of a Shoot.
func ValidateWorkers(workers []core.Worker, cloudProfileCfg *api.CloudProfileConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for i, worker := range workers {
		workerFldPath := fldPath.Index(i)

		if len(worker.Zones) == 0 {
			allErrs = append(allErrs, field.Required(workerFldPath.Child("zones"), "at least one zone must be configured"))
			continue
		}

		if len(worker.Zones) > math.MaxInt32 {
			allErrs = append(allErrs, field.Invalid(workerFldPath.Child("zones"), len(worker.Zones), "too many zones"))
			continue
		}

		if worker.Volume != nil && worker.Volume.Type != nil && worker.Volume.VolumeSize == "" {
			allErrs = append(allErrs, field.Forbidden(workerFldPath.Child("volume", "type"), "specifying volume type without a custom volume size is not allowed"))
		}

		if worker.ProviderConfig != nil {
			workerConfig, err := helper.WorkerConfigFromRawExtension(worker.ProviderConfig)
			if err != nil {
				allErrs = append(allErrs, field.Invalid(workerFldPath.Child("providerConfig"), string(worker.ProviderConfig.Raw), fmt.Sprint("providerConfig could not be decoded", err)))
				continue
			}

			allErrs = append(allErrs, ValidateWorkerConfig(&worker, workerConfig, cloudProfileCfg, workerFldPath.Child("providerConfig"))...)
		}
	}

	return allErrs
}

// ValidateWorkersUpdate validates updates on Workers.
func ValidateWorkersUpdate(oldWorkers, newWorkers []core.Worker, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	for i, newWorker := range newWorkers {
		for _, oldWorker := range oldWorkers {
			if newWorker.Name == oldWorker.Name {
				if validationutils.ShouldEnforceImmutability(newWorker.Zones, oldWorker.Zones) {
					allErrs = append(allErrs, apivalidation.ValidateImmutableField(newWorker.Zones, oldWorker.Zones, fldPath.Index(i).Child("zones"))...)
				}

				if corehelper.IsUpdateStrategyInPlace(newWorker.UpdateStrategy) {
					if !apiequality.Semantic.DeepEqual(newWorker.ProviderConfig, oldWorker.ProviderConfig) {
						allErrs = append(allErrs, field.Invalid(fldPath.Index(i).Child("providerConfig"), newWorker.ProviderConfig, "providerConfig is immutable when update strategy is in-place"))
					}

					if !apiequality.Semantic.DeepEqual(newWorker.DataVolumes, oldWorker.DataVolumes) {
						allErrs = append(allErrs, field.Invalid(fldPath.Index(i).Child("dataVolumes"), newWorker.DataVolumes, "dataVolumes is immutable when update strategy is in-place"))
					}
				}

				break
			}
		}
	}
	return allErrs
}
