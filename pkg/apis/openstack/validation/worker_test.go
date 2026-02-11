// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
	"github.com/gardener/gardener/pkg/apis/core"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/validation"
)

var _ = Describe("ValidateWorkerConfig", func() {
	Describe("#ValidateServerGroup", func() {
		var (
			fldPath            = field.NewPath("config")
			worker             *core.Worker
			sg                 *api.ServerGroup
			cloudProfileConfig *api.CloudProfileConfig
		)

		BeforeEach(func() {
			worker = &core.Worker{
				Name: "worker1",
				Volume: &core.Volume{
					Type:       ptr.To("Volume"),
					VolumeSize: "30G",
				},
				Minimum: 1,
				Maximum: 2,
				Zones:   []string{"1", "2"},
			}

			sg = &api.ServerGroup{
				Policy: "validPolicy",
			}

			cloudProfileConfig = &api.CloudProfileConfig{
				ServerGroupPolicies: []string{"validPolicy"},
			}
		})

		It("should return no errors for a valid server group configuration and single serverGroupPolicy in cloudprofile config", func() {
			Expect(ValidateServerGroup(worker, sg, cloudProfileConfig, fldPath)).To(BeEmpty())
		})

		It("should return no errors for a valid server group configuration and multiple serverGroupPolicy in cloudprofile config", func() {
			cloudProfileConfig.ServerGroupPolicies = append(cloudProfileConfig.ServerGroupPolicies, "anotherValidPolicy")
			Expect(ValidateServerGroup(worker, sg, cloudProfileConfig, fldPath)).To(BeEmpty())
		})

		It("should return as error when serverGroup has no policy specified", func() {
			sg.Policy = ""

			Expect(ValidateServerGroup(worker, sg, cloudProfileConfig, fldPath)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("config.policy"),
					"Detail": Equal("policy field cannot be empty"),
				})),
			))
		})

		It("should return as error when serverGroup policy does not match any of the policies in cloud profile config", func() {
			sg.Policy = "invalidPolicy"

			Expect(ValidateServerGroup(worker, sg, cloudProfileConfig, fldPath)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("config.policy"),
					"Detail": Equal("no matching server group policy found in cloudprofile"),
				})),
			))
		})

		It("should return as error when serverGroup policy is affinity and multiple zones are specified", func() {
			sg.Policy = openstackclient.ServerGroupPolicyAffinity
			cloudProfileConfig.ServerGroupPolicies = append(cloudProfileConfig.ServerGroupPolicies, openstackclient.ServerGroupPolicyAffinity)

			Expect(ValidateServerGroup(worker, sg, cloudProfileConfig, fldPath)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("config.policy"),
					"Detail": Equal("using \"affinity\" policy with multiple availability zones is not allowed"),
				})),
			))
		})
	})

	Describe("#ValidateMachineLabels", func() {
		var (
			fldPath      = field.NewPath("config")
			worker       *core.Worker
			workerConfig *api.WorkerConfig
		)

		BeforeEach(func() {
			worker = &core.Worker{
				Name: "worker1",
				Volume: &core.Volume{
					Type:       ptr.To("Volume"),
					VolumeSize: "30G",
				},
				Minimum: 1,
				Maximum: 2,
				Zones:   []string{"1", "2"},
			}

			workerConfig = &api.WorkerConfig{
				TypeMeta:      metav1.TypeMeta{},
				NodeTemplate:  nil,
				ServerGroup:   nil,
				MachineLabels: nil,
			}
		})

		It("should return no error for an empty workerConfig", func() {
			Expect(ValidateMachineLabels(worker, workerConfig, fldPath.Child("machineLabels"))).To(BeEmpty())
		})

		It("should return no error for valid machine labels", func() {
			workerConfig.MachineLabels = []api.MachineLabel{
				{
					Name:                   "label1",
					Value:                  "value1",
					TriggerRollingOnUpdate: false,
				},
			}

			Expect(ValidateMachineLabels(worker, workerConfig, fldPath.Child("machineLabels"))).To(BeEmpty())
		})

		Describe("machine label name", func() {
			It("should return an error for machine labels with empty name", func() {
				workerConfig.MachineLabels = []api.MachineLabel{
					{
						Name:                   "",
						Value:                  "value1",
						TriggerRollingOnUpdate: false,
					},
				}

				Expect(ValidateMachineLabels(worker, workerConfig, fldPath.Child("machineLabels"))).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeRequired),
						"Field":  Equal("config.machineLabels[0].name"),
						"Detail": Equal("cannot be empty"),
					})),
				))
			})

			It("should return an error for machine labels with an invalid name", func() {
				workerConfig.MachineLabels = []api.MachineLabel{
					{
						Name:                   "invalid[]label",
						Value:                  "value1",
						TriggerRollingOnUpdate: false,
					},
				}

				Expect(ValidateMachineLabels(worker, workerConfig, fldPath.Child("machineLabels"))).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("config.machineLabels[0].name"),
						"Detail": Equal("does not match expected regex \"^[^{}\\\\[\\\\]\\\\n]+$\""),
					})),
				))
			})

			It("should return an error for machine labels with name longer than 255 characters", func() {
				longName := ""
				for i := 0; i < 256; i++ {
					longName += "a"
				}

				workerConfig.MachineLabels = []api.MachineLabel{
					{
						Name:                   longName,
						Value:                  "value1",
						TriggerRollingOnUpdate: false,
					},
				}

				Expect(ValidateMachineLabels(worker, workerConfig, fldPath.Child("machineLabels"))).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("config.machineLabels[0].name"),
						"Detail": Equal("must not be more than 255 characters, got 256"),
					})),
				))
			})
		})

		Describe("machine label value", func() {
			It("should return an error for machine labels with empty name", func() {
				workerConfig.MachineLabels = []api.MachineLabel{
					{
						Name:                   "label1",
						Value:                  "",
						TriggerRollingOnUpdate: false,
					},
				}

				Expect(ValidateMachineLabels(worker, workerConfig, fldPath.Child("machineLabels"))).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeRequired),
						"Field":  Equal("config.machineLabels[0].value"),
						"Detail": Equal("cannot be empty"),
					})),
				))
			})

			It("should return an error for machine labels with an invalid name", func() {
				workerConfig.MachineLabels = []api.MachineLabel{
					{
						Name:                   "label1",
						Value:                  "invalid[]value",
						TriggerRollingOnUpdate: false,
					},
				}

				Expect(ValidateMachineLabels(worker, workerConfig, fldPath.Child("machineLabels"))).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("config.machineLabels[0].value"),
						"Detail": Equal("does not match expected regex \"^[^{}\\\\[\\\\]\\\\n]+$\""),
					})),
				))
			})

			It("should return an error for machine labels with name longer than 255 characters", func() {
				longValue := ""
				for i := 0; i < 256; i++ {
					longValue += "a"
				}

				workerConfig.MachineLabels = []api.MachineLabel{
					{
						Name:                   "label1",
						Value:                  longValue,
						TriggerRollingOnUpdate: false,
					},
				}

				Expect(ValidateMachineLabels(worker, workerConfig, fldPath.Child("machineLabels"))).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("config.machineLabels[0].value"),
						"Detail": Equal("must not be more than 255 characters, got 256"),
					})),
				))
			})
		})

		It("duplicate labels should return an error", func() {
			workerConfig.MachineLabels = []api.MachineLabel{
				{
					Name:                   "label1",
					Value:                  "value1",
					TriggerRollingOnUpdate: false,
				},
				{
					Name:                   "label1",
					Value:                  "value2",
					TriggerRollingOnUpdate: false,
				},
			}

			Expect(ValidateMachineLabels(worker, workerConfig, fldPath.Child("machineLabels"))).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeDuplicate),
					"Field":  Equal("config.machineLabels[1].name"),
					"Detail": Equal(""),
				})),
			))
		})

		It("should return an error when workerConfig labels are already present in worker.Labels", func() {
			workerConfig.MachineLabels = []api.MachineLabel{
				{
					Name:                   "label1",
					Value:                  "value1",
					TriggerRollingOnUpdate: false,
				},
			}

			worker.Labels = map[string]string{
				"label1": "value1",
			}

			Expect(ValidateMachineLabels(worker, workerConfig, fldPath.Child("machineLabels"))).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("config.machineLabels[0].name"),
					"Detail": Equal("label name already defined as pool label"),
				})),
			))
		})
	})

	Describe("#ValidateNodeTemplate", func() {
		var (
			fldPath      = field.NewPath("config")
			nodeTemplate *extensionsv1alpha1.NodeTemplate
		)

		BeforeEach(func() {
			nodeTemplate = &extensionsv1alpha1.NodeTemplate{}
		})

		It("should return no error for a nil workerConfig", func() {
			nodeTemplate = nil

			Expect(ValidateNodeTemplate(nodeTemplate, fldPath)).To(BeEmpty())
		})

		It("should return no error for an empty workerConfig", func() {
			Expect(ValidateNodeTemplate(nodeTemplate, fldPath)).To(BeEmpty())
		})

		Describe("nodeTemplate.capacity", func() {
			It("should not return an error for valid capacities", func() {
				nodeTemplate.Capacity = corev1.ResourceList{
					"cpu":    resource.MustParse("4"),
					"memory": resource.MustParse("5Gi"),
					"gpu":    resource.MustParse("0"),
				}

				Expect(ValidateNodeTemplate(nodeTemplate, fldPath)).To(BeEmpty())
			})

			It("should return errors for negative cpu, memory, and gpu values", func() {
				nodeTemplate.Capacity = corev1.ResourceList{
					"cpu":    resource.MustParse("-1"),
					"memory": resource.MustParse("-5Gi"),
					"gpu":    resource.MustParse("-1"),
				}

				Expect(ValidateNodeTemplate(nodeTemplate, fldPath)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("config.capacity.cpu"),
						"Detail": Equal("cpu value must not be negative"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("config.capacity.memory"),
						"Detail": Equal("memory value must not be negative"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("config.capacity.gpu"),
						"Detail": Equal("gpu value must not be negative"),
					})),
				))
			})

			It("should not consider other custom resources in capacity for negative value validation", func() {
				nodeTemplate.Capacity = corev1.ResourceList{
					"cpu":    resource.MustParse("4"),
					"memory": resource.MustParse("5Gi"),
					"gpu":    resource.MustParse("0"),
					"foo":    resource.MustParse("-1"),
				}

				Expect(ValidateNodeTemplate(nodeTemplate, fldPath)).To(BeEmpty())
			})
		})

		Describe("nodeTemplate.virtualCapacity", func() {
			It("should not return an error for valid virtual capacity", func() {
				nodeTemplate.VirtualCapacity = corev1.ResourceList{
					"subdomain.domain.com/vResource": resource.MustParse("2"),
				}

				Expect(ValidateNodeTemplate(nodeTemplate, fldPath)).To(BeEmpty())
			})

			It("should return an error for non whole number virtual capacity", func() {
				nodeTemplate.VirtualCapacity = corev1.ResourceList{
					"subdomain.domain.com/vResource": resource.MustParse("2.5"),
				}

				Expect(ValidateNodeTemplate(nodeTemplate, fldPath)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("config.virtualCapacity.subdomain.domain.com/vResource"),
						"Detail": Equal("subdomain.domain.com/vResource value must be a whole number"),
					})),
				))
			})

			It("should return errors for negative virtual capacity values", func() {
				nodeTemplate.VirtualCapacity = corev1.ResourceList{
					"subdomain.domain.com/vResource": resource.MustParse("-1"),
				}

				Expect(ValidateNodeTemplate(nodeTemplate, fldPath)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("config.virtualCapacity.subdomain.domain.com/vResource"),
						"Detail": Equal("subdomain.domain.com/vResource value must not be negative"),
					})),
				))
			})
		})
	})
})
