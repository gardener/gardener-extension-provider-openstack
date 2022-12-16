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

	"github.com/Masterminds/semver"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	validationutils "github.com/gardener/gardener/pkg/utils/validation"
	"github.com/gardener/gardener/pkg/utils/version"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/gengo/examples/set-gen/sets"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
)

// ValidateShootCredentialsForK8sVersion validates that the authentication method used by the user's credentials is supported for the requested
// Kubernetes version. All Kubernetes (CCM, kubelet, CSI) and Gardener (MCM) components that communicate with OpenStack's API must be able to use
// the provided credentials
// For K8s version <1.19, the kubelet is configured to use the in-tree cloud-provider, which doesn't support authentication with application credentials.
func ValidateShootCredentialsForK8sVersion(k8sVersion string, credentials openstack.Credentials, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// The Kubernetes version is the version of the CSI migration, where we stopped using the in-tree providers.
	// see: pkg/webhook/controlplane/ensurer.go
	k8sVersionLessThan19, err := version.CompareVersions(k8sVersion, "<", openstack.CSIMigrationKubernetesVersion)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, k8sVersion, "not a valid version"))
	}

	// if credentials.ApplicationCredentialSecret is defined then authentication is done with OpenStack's application credentials.
	if k8sVersionLessThan19 && credentials.ApplicationCredentialSecret != "" {
		allErrs = append(allErrs, field.Invalid(fldPath, k8sVersion, "application credentials are not supported for Kubernetes versions < v1.19"))
	}

	return allErrs
}

// ValidateNetworking validates the network settings of a Shoot.
func ValidateNetworking(networking core.Networking, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if networking.Nodes == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("nodes"), "a nodes CIDR must be provided for Openstack shoots"))
	}

	return allErrs
}

// ValidateWorkers validates the workers of a Shoot.
func ValidateWorkers(workers []core.Worker, region string, regions []gardencorev1beta1.Region, cloudProfileCfg *api.CloudProfileConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	csiMigrationVersion, err := semver.NewVersion(openstack.CSIMigrationKubernetesVersion)
	if err != nil {
		allErrs = append(allErrs, field.InternalError(fldPath, err))
		return allErrs
	}

	for i, worker := range workers {
		workerFldPath := fldPath.Index(i)

		// Ensure the kubelet version is not lower than the version in which the extension performs CSI migration.
		if worker.Kubernetes != nil && worker.Kubernetes.Version != nil {
			versionPath := workerFldPath.Child("kubernetes", "version")

			v, err := semver.NewVersion(*worker.Kubernetes.Version)
			if err != nil {
				allErrs = append(allErrs, field.Invalid(versionPath, *worker.Kubernetes.Version, err.Error()))
				return allErrs
			}

			if v.LessThan(csiMigrationVersion) {
				allErrs = append(allErrs, field.Forbidden(versionPath, fmt.Sprintf("cannot use kubelet version (%s) lower than CSI migration version (%s)", v.String(), csiMigrationVersion.String())))
			}
		}

		if len(worker.Zones) == 0 {
			allErrs = append(allErrs, field.Required(workerFldPath.Child("zones"), "at least one zone must be configured"))
			continue
		}
		allErrs = append(allErrs, validateWorkerZones(worker.Zones, region, regions, workerFldPath.Child("zones"))...)

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

// validateWorkerZones validates worker zones for duplicates and existence in specified region
func validateWorkerZones(zones []string, regionName string, regions []gardencorev1beta1.Region, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	var region *gardencorev1beta1.Region
	for i := range regions {
		if regionName == regions[i].Name {
			region = &regions[i]
			break
		}
	}
	if region != nil {
		usedZones := sets.NewString()
	outer:
		for i, zone := range zones {
			if usedZones.Has(zone) {
				allErrs = append(allErrs, field.Duplicate(fldPath.Index(i), zone))
			}
			usedZones.Insert(zone)
			for _, z := range region.Zones {
				if z.Name == zone {
					continue outer
				}
			}
			allErrs = append(allErrs, field.Invalid(fldPath.Index(i), zone, fmt.Sprintf("zone %s not existing in region %s", zone, region.Name)))
		}
	}

	return allErrs
}

// ValidateWorkersUpdate validates updates on Workers.
func ValidateWorkersUpdate(oldWorkers, newWorkers []core.Worker, region string, regions []gardencorev1beta1.Region, fldPath *field.Path) field.ErrorList {
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
		allErrs = append(allErrs, validateWorkerZones(newWorker.Zones, region, regions, fldPath.Index(i).Child("zones"))...)
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
