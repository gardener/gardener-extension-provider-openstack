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

package validation_test

import (
	"encoding/json"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	. "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/validation"
	credentials "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"
)

var _ = Describe("Shoot validation", func() {
	Describe("#ValidateShootCredentialsForK8sVersion", func() {
		versionPath := field.NewPath("spec", "kubernetes", "version")

		It("should allow using application credentials for k8s version >=1.19", func() {
			errorList := ValidateShootCredentialsForK8sVersion("1.19.0", credentials.Credentials{ApplicationCredentialSecret: "secret"}, versionPath)
			Expect(errorList).To(BeEmpty())
		})

		It("should forbid using application credentials for k8s version <1.19", func() {
			lowerThan119 := "1.18.20"

			errorList := ValidateShootCredentialsForK8sVersion(lowerThan119, credentials.Credentials{ApplicationCredentialSecret: "secret"}, versionPath)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal("spec.kubernetes.version"),
					"BadValue": Equal(lowerThan119),
				})),
			))
		})

		It("should using user credentials for k8s version >=1.19", func() {
			errorList := ValidateShootCredentialsForK8sVersion("1.19.0", credentials.Credentials{Username: "user", Password: "pwd"}, versionPath)
			Expect(errorList).To(BeEmpty())
		})

		It("should allow using user credentials for k8s version < 1.19", func() {
			errorList := ValidateShootCredentialsForK8sVersion("1.18.20", credentials.Credentials{Username: "user", Password: "pwd"}, versionPath)
			Expect(errorList).To(BeEmpty())
		})
	})

	Describe("#ValidateNetworking", func() {
		networkingPath := field.NewPath("spec", "networking")

		It("should return no error because nodes CIDR was provided", func() {
			networking := core.Networking{
				Nodes: pointer.StringPtr("1.2.3.4/5"),
			}

			errorList := ValidateNetworking(networking, networkingPath)

			Expect(errorList).To(BeEmpty())
		})

		It("should return an error because no nodes CIDR was provided", func() {
			networking := core.Networking{}

			errorList := ValidateNetworking(networking, networkingPath)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.networking.nodes"),
				})),
			))
		})
	})
	Describe("#ValidateWorkerConfig", func() {
		var (
			nilPath *field.Path
			workers []core.Worker
		)

		BeforeEach(func() {
			workers = []core.Worker{
				{
					Name: "worker1",
					Volume: &core.Volume{
						Type:       pointer.StringPtr("Volume"),
						VolumeSize: "30G",
					},
					Minimum: 1,
					Maximum: 2,
					Zones:   []string{"1", "2"},
				},
				{
					Name: "worker2",
					Volume: &core.Volume{
						Type:       pointer.StringPtr("Volume"),
						VolumeSize: "20G",
					},
					Minimum: 1,
					Maximum: 2,
					Zones:   []string{"1", "2"},
				},
			}
		})

		Describe("#ValidateWorkers", func() {
			It("should pass when the kubernetes version is equal to the CSI migration version", func() {
				workers[0].Kubernetes = &core.WorkerKubernetes{Version: pointer.String("1.19.0")}

				errorList := ValidateWorkers(workers, nil, field.NewPath(""))

				Expect(errorList).To(BeEmpty())
			})

			It("should pass when the kubernetes version is higher to the CSI migration version", func() {
				workers[0].Kubernetes = &core.WorkerKubernetes{Version: pointer.String("1.20.0")}

				errorList := ValidateWorkers(workers, nil, field.NewPath(""))

				Expect(errorList).To(BeEmpty())
			})

			It("should not allow when the kubernetes version is lower than the CSI migration version", func() {
				workers[0].Kubernetes = &core.WorkerKubernetes{Version: pointer.String("1.18.0")}

				errorList := ValidateWorkers(workers, nil, field.NewPath("workers"))

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeForbidden),
						"Field": Equal("workers[0].kubernetes.version"),
					})),
				))
			})

			It("should pass because workers are configured correctly", func() {
				errorList := ValidateWorkers(workers, nil, nilPath)

				Expect(errorList).To(BeEmpty())
			})

			It("should forbid because worker does not specify a zone", func() {
				workers[0].Zones = nil

				errorList := ValidateWorkers(workers, nil, nilPath)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("[0].zones"),
					})),
				))
			})

			It("should enforce workers min > 0 if max > 0", func() {
				workers[0].Minimum = 0

				errorList := ValidateWorkers(workers, nil, nilPath)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeForbidden),
						"Field": Equal("[0].minimum"),
					})),
				))
			})

			It("should forbid specifying volume type without size", func() {
				workers[0].Volume = &core.Volume{
					Type: pointer.String("standard"),
				}

				errorList := ValidateWorkers(workers, nil, nilPath)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeForbidden),
						"Field": Equal("[0].volume.type"),
					})),
				))
			})

			Context("#ValidateServerGroups", func() {
				var cloudProfileConfig *openstack.CloudProfileConfig

				BeforeEach(func() {
					cloudProfileConfig = &openstack.CloudProfileConfig{
						ServerGroupPolicies: []string{"foo", "bar", openstackclient.ServerGroupPolicyAffinity},
					}
				})

				It("should disallow policies not in cloud profile", func() {
					const invalidPolicyValue = "baz"
					providerConfig := &openstack.WorkerConfig{
						ServerGroup: &openstack.ServerGroup{
							Policy: invalidPolicyValue,
						},
					}

					arr, err := json.Marshal(providerConfig)
					Expect(err).To(BeNil())

					workers[0].ProviderConfig = &runtime.RawExtension{
						Raw: arr,
					}

					errorList := ValidateWorkers(workers, cloudProfileConfig, nilPath)
					Expect(errorList).To(Not(BeEmpty()))
					Expect(errorList).To(HaveLen(1))
					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":     Equal(field.ErrorTypeInvalid),
							"Field":    Equal("[0].providerConfig.serverGroup.policy"),
							"BadValue": Equal(invalidPolicyValue),
						})),
					))
				})

				It("should disallow empty values in policy field", func() {
					providerConfig := &openstack.WorkerConfig{
						ServerGroup: &openstack.ServerGroup{
							Policy: "",
						},
					}

					arr, err := json.Marshal(providerConfig)
					Expect(err).To(BeNil())

					workers[0].ProviderConfig = &runtime.RawExtension{
						Raw: arr,
					}

					errorList := ValidateWorkers(workers, cloudProfileConfig, nilPath)
					Expect(errorList).To(Not(BeEmpty()))
					Expect(errorList).To(HaveLen(1))
					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":     Equal(field.ErrorTypeInvalid),
							"Field":    Equal("[0].providerConfig.serverGroup.policy"),
							"BadValue": Equal(""),
						})),
					))
				})

				It("should allow policies in found in cloud profile", func() {
					providerConfig := &openstack.WorkerConfig{
						ServerGroup: &openstack.ServerGroup{
							Policy: "foo",
						},
					}

					arr, err := json.Marshal(providerConfig)
					Expect(err).To(BeNil())

					workers[0].ProviderConfig = &runtime.RawExtension{
						Raw: arr,
					}

					errorList := ValidateWorkers(workers, cloudProfileConfig, nilPath)
					Expect(errorList).To(BeEmpty())
				})

				It("should not allow hard affinity policy with multiple availability zones", func() {
					providerConfig := &openstack.WorkerConfig{
						ServerGroup: &openstack.ServerGroup{
							Policy: openstackclient.ServerGroupPolicyAffinity,
						},
					}

					arr, err := json.Marshal(providerConfig)
					Expect(err).To(BeNil())

					workers[0].ProviderConfig = &runtime.RawExtension{
						Raw: arr,
					}

					errorList := ValidateWorkers(workers, cloudProfileConfig, nilPath)
					Expect(errorList).NotTo(BeEmpty())
					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeForbidden),
							"Field": Equal("[0].providerConfig.serverGroup.policy"),
						})),
					))
				})
			})
		})

		Describe("#ValidateWorkersUpdate", func() {
			It("should pass because workers are unchanged", func() {
				newWorkers := copyWorkers(workers)
				errorList := ValidateWorkersUpdate(workers, newWorkers, nilPath)

				Expect(errorList).To(BeEmpty())
			})

			It("should allow adding workers", func() {
				newWorkers := append(workers[:0:0], workers...)
				workers = workers[:1]
				errorList := ValidateWorkersUpdate(workers, newWorkers, nilPath)

				Expect(errorList).To(BeEmpty())
			})

			It("should allow adding a zone to a worker", func() {
				newWorkers := copyWorkers(workers)
				newWorkers[0].Zones = append(newWorkers[0].Zones, "another-zone")
				errorList := ValidateWorkersUpdate(workers, newWorkers, nilPath)

				Expect(errorList).To(BeEmpty())
			})

			It("should forbid removing a zone from a worker", func() {
				newWorkers := copyWorkers(workers)
				newWorkers[1].Zones = newWorkers[1].Zones[1:]
				errorList := ValidateWorkersUpdate(workers, newWorkers, nilPath)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("[1].zones"),
					})),
				))
			})

			It("should forbid changing the zone order", func() {
				newWorkers := copyWorkers(workers)
				newWorkers[0].Zones[0] = workers[0].Zones[1]
				newWorkers[0].Zones[1] = workers[0].Zones[0]
				newWorkers[1].Zones[0] = workers[1].Zones[1]
				newWorkers[1].Zones[1] = workers[1].Zones[0]
				errorList := ValidateWorkersUpdate(workers, newWorkers, nilPath)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("[0].zones"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("[1].zones"),
					})),
				))
			})

			It("should forbid adding a zone while changing an existing one", func() {
				newWorkers := copyWorkers(workers)
				newWorkers = append(newWorkers, core.Worker{Name: "worker3", Zones: []string{"zone1"}})
				newWorkers[1].Zones[0] = workers[1].Zones[1]
				errorList := ValidateWorkersUpdate(workers, newWorkers, nilPath)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("[1].zones"),
					})),
				))
			})
		})
	})
})

func copyWorkers(workers []core.Worker) []core.Worker {
	copy := append(workers[:0:0], workers...)
	for i := range copy {
		copy[i].Zones = append(workers[i].Zones[:0:0], workers[i].Zones...)
	}
	return copy
}
