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

	"github.com/gardener/gardener/pkg/apis/core"
	validationutils "github.com/gardener/gardener/pkg/utils/validation"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
)

// ValidateNetworking validates the network settings of a Shoot.
func ValidateNetworking(networking core.Networking, fldPath *field.Path) field.ErrorList {
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

		if worker.Volume != nil && worker.Volume.Type != nil && worker.Volume.VolumeSize == "" {
			allErrs = append(allErrs, field.Forbidden(workerFldPath.Child("volume", "type"), "specifying volume type without a custom volume size is not allowed"))
		}

		if worker.ProviderConfig != nil {
			workerConfig, err := helper.WorkerConfigFromRawExtension(worker.ProviderConfig)
			if err != nil {
				allErrs = append(allErrs, field.Invalid(workerFldPath.Child("providerConfig"), string(worker.ProviderConfig.Raw), fmt.Sprint("providerConfig could not be decoded", err)))
				continue
			}

			allErrs = append(allErrs, validateWorkerConfig(workerFldPath.Child("providerConfig"), &worker, workerConfig, cloudProfileCfg)...)
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

func validateWorkerConfig(parent *field.Path, worker *core.Worker, workerConfig *api.WorkerConfig, cloudProfileConfig *api.CloudProfileConfig) field.ErrorList {
	allErrs := field.ErrorList{}

	if workerConfig.ServerGroup == nil {
		return allErrs
	}

	if workerConfig.ServerGroup.Policy == "" {
		allErrs = append(allErrs, field.Invalid(parent.Child("serverGroup", "policy"), workerConfig.ServerGroup.Policy, "policy field cannot be empty"))
		return allErrs
	}

	isPolicyMatching := func() bool {
		if cloudProfileConfig == nil {
			return false
		}

		for _, policy := range cloudProfileConfig.ServerGroupPolicies {
			if policy == workerConfig.ServerGroup.Policy {
				return true
			}
		}
		return false
	}()

	if !isPolicyMatching {
		allErrs = append(allErrs, field.Invalid(parent.Child("serverGroup", "policy"), workerConfig.ServerGroup.Policy, "no matching server group policy found in cloudprofile"))
		return allErrs
	}

	if len(worker.Zones) > 1 && workerConfig.ServerGroup.Policy == openstackclient.ServerGroupPolicyAffinity {
		allErrs = append(allErrs, field.Forbidden(parent.Child("serverGroup", "policy"), fmt.Sprintf("using %q policy with multiple availability zones is not allowed", openstackclient.ServerGroupPolicyAffinity)))
	}

	return allErrs
}
