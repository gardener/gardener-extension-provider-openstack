// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	"encoding/json"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	apiv1alpha1 "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"
	. "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/validation"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
	"github.com/gardener/gardener/pkg/apis/core"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
)

var _ = Describe("Shoot validation", func() {
	Describe("#ValidateNetworking", func() {
		networkingPath := field.NewPath("spec", "networking")

		It("should return no error because nodes CIDR was provided", func() {
			networking := &core.Networking{
				Nodes: ptr.To("1.2.3.4/5"),
			}

			errorList := ValidateNetworking(networking, networkingPath)

			Expect(errorList).To(BeEmpty())
		})

		It("should return an error because no nodes CIDR was provided", func() {
			networking := &core.Networking{}

			errorList := ValidateNetworking(networking, networkingPath)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.networking.nodes"),
				})),
			))
		})
	})
	Describe("#validateWorkerConfig", func() {
		var (
			nilPath *field.Path
			workers []core.Worker
		)

		BeforeEach(func() {
			workers = []core.Worker{
				{
					Name: "worker1",
					Volume: &core.Volume{
						Type:       ptr.To("Volume"),
						VolumeSize: "30G",
					},
					Minimum: 1,
					Maximum: 2,
					Zones:   []string{"1", "2"},
				},
				{
					Name: "worker2",
					Volume: &core.Volume{
						Type:       ptr.To("Volume"),
						VolumeSize: "20G",
					},
					Minimum: 1,
					Maximum: 2,
					Zones:   []string{"1", "2"},
				},
			}
		})

		Describe("#ValidateWorkers", func() {
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

			It("should forbid specifying volume type without size", func() {
				workers[0].Volume = &core.Volume{
					Type: ptr.To("standard"),
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

			Context("#ValidateMachineLabels", func() {
				It("should pass if some machine labels are defined", func() {
					workers[0].ProviderConfig = &runtime.RawExtension{
						Object: &apiv1alpha1.WorkerConfig{
							TypeMeta: metav1.TypeMeta{
								Kind:       "WorkerConfig",
								APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
							},
							MachineLabels: []apiv1alpha1.MachineLabel{
								{Name: "m1", Value: "v1"},
								{Name: "m2", Value: "v2"},
							},
						},
					}

					errorList := ValidateWorkers(workers, nil, nilPath)

					Expect(errorList).To(BeEmpty())
				})

				It("should fail on duplicate labels", func() {
					workers[0].Labels = map[string]string{"l1": "x", "l2": "x"}
					workers[0].ProviderConfig = &runtime.RawExtension{
						Object: &apiv1alpha1.WorkerConfig{
							TypeMeta: metav1.TypeMeta{
								Kind:       "WorkerConfig",
								APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
							},
							MachineLabels: []apiv1alpha1.MachineLabel{
								{Name: "m1", Value: "v1"},
								{Name: "m1", Value: "v2"},
								{Name: "l1", Value: "v2"},
							},
						},
					}
					errorList := ValidateWorkers(workers, nil, nilPath)
					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":     Equal(field.ErrorTypeDuplicate),
							"BadValue": Equal("m1"),
							"Field":    Equal("[0].providerConfig.machineLabels[1].name"),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":     Equal(field.ErrorTypeInvalid),
							"Field":    Equal("[0].providerConfig.machineLabels[2].name"),
							"BadValue": Equal("l1"),
							"Detail":   Equal("label name already defined as pool label"),
						})),
					))
				})
			})

			Describe("#validateWorkerConfig", func() {
				It("should return no errors for a valid nodetemplate configuration", func() {
					workers[0].ProviderConfig = &runtime.RawExtension{
						Object: &apiv1alpha1.WorkerConfig{
							TypeMeta: metav1.TypeMeta{
								Kind:       "WorkerConfig",
								APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
							},
							NodeTemplate: &extensionsv1alpha1.NodeTemplate{
								Capacity: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1"),
									corev1.ResourceMemory: resource.MustParse("50Gi"),
									"gpu":                 resource.MustParse("0"),
								},
							},
						}}
					Expect(ValidateWorkers(workers, nil, nilPath)).To(BeEmpty())
				})

				It("should return error when all resources not specified", func() {
					workers[0].ProviderConfig = &runtime.RawExtension{
						Object: &apiv1alpha1.WorkerConfig{
							TypeMeta: metav1.TypeMeta{
								Kind:       "WorkerConfig",
								APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
							},
							NodeTemplate: &extensionsv1alpha1.NodeTemplate{
								Capacity: corev1.ResourceList{
									"gpu": resource.MustParse("0"),
								},
							},
						},
					}

					Expect(ValidateWorkers(workers, nil, nilPath)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeRequired),
							"Field":  Equal("[0].providerConfig.nodeTemplate.capacity"),
							"Detail": Equal("cpu is a mandatory field"),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeRequired),
							"Field":  Equal("[0].providerConfig.nodeTemplate.capacity"),
							"Detail": Equal("memory is a mandatory field"),
						})),
					))
				})

				It("should return error when resource value is negative", func() {
					workers[0].ProviderConfig = &runtime.RawExtension{
						Object: &apiv1alpha1.WorkerConfig{
							TypeMeta: metav1.TypeMeta{
								Kind:       "WorkerConfig",
								APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
							},
							NodeTemplate: &extensionsv1alpha1.NodeTemplate{
								Capacity: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1"),
									corev1.ResourceMemory: resource.MustParse("-50Gi"),
									"gpu":                 resource.MustParse("0"),
								},
							},
						}}

					Expect(ValidateWorkers(workers, nil, nilPath)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":     Equal(field.ErrorTypeInvalid),
							"Field":    Equal("[0].providerConfig.nodeTemplate.capacity.memory"),
							"BadValue": Equal("-50Gi"),
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
	cp := append(workers[:0:0], workers...)
	for i := range cp {
		cp[i].Zones = append(workers[i].Zones[:0:0], workers[i].Zones...)
	}
	return cp
}
