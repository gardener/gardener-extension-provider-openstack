// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"math"

	"github.com/gardener/gardener/pkg/apis/core"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	validationutils "github.com/gardener/gardener/pkg/utils/validation"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
)

// ValidateNetworking validates the network settings of a Shoot.
func ValidateNetworking(networking *core.Networking, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if networking.Nodes == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("nodes"), "a nodes CIDR must be provided for Openstack shoots"))
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

			allErrs = append(allErrs, validateWorkerConfig(&worker, workerConfig, cloudProfileCfg, workerFldPath.Child("providerConfig"))...)
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

				break
			}
		}
	}
	return allErrs
}

// validateWorkerConfig validates the providerConfig section of a Worker resource.
func validateWorkerConfig(worker *core.Worker, workerConfig *api.WorkerConfig, cloudProfileConfig *api.CloudProfileConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateServerGroup(worker, workerConfig.ServerGroup, cloudProfileConfig, fldPath.Child("serverGroup"))...)
	allErrs = append(allErrs, validateNodeTemplate(workerConfig.NodeTemplate, fldPath.Child("nodeTemplate"))...)
	allErrs = append(allErrs, validateMachineLabels(worker, workerConfig, fldPath.Child("machineLabels"))...)

	return allErrs
}

func validateServerGroup(worker *core.Worker, sg *api.ServerGroup, cloudProfileConfig *api.CloudProfileConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if sg == nil {
		return allErrs
	}

	if sg.Policy == "" {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("policy"), sg.Policy, "policy field cannot be empty"))
		return allErrs
	}

	isPolicyMatching := func() bool {
		if cloudProfileConfig == nil {
			return false
		}

		for _, policy := range cloudProfileConfig.ServerGroupPolicies {
			if policy == sg.Policy {
				return true
			}
		}
		return false
	}()

	if !isPolicyMatching {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("policy"), sg.Policy, "no matching server group policy found in cloudprofile"))
		return allErrs
	}

	if len(worker.Zones) > 1 && sg.Policy == openstackclient.ServerGroupPolicyAffinity {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("policy"), fmt.Sprintf("using %q policy with multiple availability zones is not allowed", openstackclient.ServerGroupPolicyAffinity)))
	}

	return allErrs
}

func validateNodeTemplate(nodeTemplate *extensionsv1alpha1.NodeTemplate, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if nodeTemplate == nil {
		return nil
	}
	for _, capacityAttribute := range []corev1.ResourceName{corev1.ResourceCPU, "gpu", corev1.ResourceMemory} {
		value, ok := nodeTemplate.Capacity[capacityAttribute]
		if !ok {
			allErrs = append(allErrs, field.Required(fldPath.Child("capacity"), fmt.Sprintf("%s is a mandatory field", capacityAttribute)))
			continue
		}
		allErrs = append(allErrs, validateResourceQuantityValue(capacityAttribute, value, fldPath.Child("capacity").Child(string(capacityAttribute)))...)
	}

	return allErrs
}

func validateResourceQuantityValue(key corev1.ResourceName, value resource.Quantity, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if value.Cmp(resource.Quantity{}) < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, value.String(), fmt.Sprintf("%s value must not be negative", key)))
	}

	return allErrs
}

func validateMachineLabels(worker *core.Worker, workerConfig *api.WorkerConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	machineLabelNames := sets.New[string]()
	for i, ml := range workerConfig.MachineLabels {
		idxPath := fldPath.Index(i)

		if machineLabelNames.Has(ml.Name) {
			allErrs = append(allErrs, field.Duplicate(idxPath.Child("name"), ml.Name))
		} else if _, found := worker.Labels[ml.Name]; found {
			allErrs = append(allErrs, field.Invalid(idxPath.Child("name"), ml.Name, "label name already defined as pool label"))
		} else {
			machineLabelNames.Insert(ml.Name)
		}
	}

	return allErrs
}
