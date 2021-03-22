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
	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	. "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/validation"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"
)

var _ = Describe("CloudProfileConfig validation", func() {
	Describe("#ValidateCloudProfileConfig", func() {
		var (
			cloudProfileConfig *api.CloudProfileConfig
			fldPath            *field.Path
		)

		BeforeEach(func() {
			cloudProfileConfig = &api.CloudProfileConfig{
				Constraints: api.Constraints{
					FloatingPools: []api.FloatingPool{
						{Name: "MY-POOL"},
					},
					LoadBalancerProviders: []api.LoadBalancerProvider{
						{Name: "haproxy"},
					},
				},
				DNSServers: []string{
					"1.2.3.4",
					"5.6.7.8",
				},
				KeyStoneURL: "http://url-to-keystone/v3",
				MachineImages: []api.MachineImages{
					{
						Name: "ubuntu",
						Versions: []api.MachineImageVersion{
							{
								Version: "1.2.3",
								Image:   "ubuntu-1.2.3",
							},
						},
					},
				},
			}
			fldPath = field.NewPath("root")
		})

		Context("floating pools constraints", func() {
			It("should enforce that at least one pool has been defined", func() {
				cloudProfileConfig.Constraints.FloatingPools = []api.FloatingPool{}

				errorList := ValidateCloudProfileConfig(cloudProfileConfig, fldPath)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.constraints.floatingPools"),
				}))))
			})

			It("should forbid unsupported pools", func() {
				cloudProfileConfig.Constraints.FloatingPools = []api.FloatingPool{
					{
						Name:   "",
						Region: pointer.StringPtr(""),
						Domain: pointer.StringPtr(""),
					},
				}

				errorList := ValidateCloudProfileConfig(cloudProfileConfig, fldPath)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.constraints.floatingPools[0].name"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.constraints.floatingPools[0].region"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.constraints.floatingPools[0].domain"),
				}))))
			})

			It("should forbid duplicates regions and domains in pools", func() {
				cloudProfileConfig.Constraints.FloatingPools = []api.FloatingPool{
					{
						Name:   "foo",
						Region: pointer.StringPtr("rfoo"),
					},
					{
						Name:   "foo",
						Region: pointer.StringPtr("rfoo"),
					},
					{
						Name:   "foo",
						Domain: pointer.StringPtr("dfoo"),
					},
					{
						Name:   "foo",
						Domain: pointer.StringPtr("dfoo"),
					},
					{
						Name:   "foo",
						Domain: pointer.StringPtr("dfoo"),
						Region: pointer.StringPtr("rfoo"),
					},
					{
						Name:   "foo",
						Domain: pointer.StringPtr("dfoo"),
						Region: pointer.StringPtr("rfoo"),
					},
				}

				errorList := ValidateCloudProfileConfig(cloudProfileConfig, fldPath)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":     Equal(field.ErrorTypeDuplicate),
						"Field":    Equal("root.constraints.floatingPools[1].name"),
						"BadValue": Equal("foo"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":     Equal(field.ErrorTypeDuplicate),
						"Field":    Equal("root.constraints.floatingPools[3].name"),
						"BadValue": Equal("foo"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":     Equal(field.ErrorTypeDuplicate),
						"Field":    Equal("root.constraints.floatingPools[5].name"),
						"BadValue": Equal("foo"),
					}))))
			})
		})

		Context("load balancer provider constraints", func() {
			It("should enforce that at least one provider has been defined", func() {
				cloudProfileConfig.Constraints.LoadBalancerProviders = []api.LoadBalancerProvider{}

				errorList := ValidateCloudProfileConfig(cloudProfileConfig, fldPath)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.constraints.loadBalancerProviders"),
				}))))
			})

			It("should forbid unsupported providers", func() {
				cloudProfileConfig.Constraints.LoadBalancerProviders = []api.LoadBalancerProvider{
					{
						Name:   "",
						Region: pointer.StringPtr(""),
					},
				}

				errorList := ValidateCloudProfileConfig(cloudProfileConfig, fldPath)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.constraints.loadBalancerProviders[0].name"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.constraints.loadBalancerProviders[0].region"),
				}))))
			})

			It("should forbid duplicates regions in providers", func() {
				cloudProfileConfig.Constraints.LoadBalancerProviders = []api.LoadBalancerProvider{
					{
						Name:   "foo",
						Region: pointer.StringPtr("foo"),
					},
					{
						Name:   "foo",
						Region: pointer.StringPtr("foo"),
					},
				}

				errorList := ValidateCloudProfileConfig(cloudProfileConfig, fldPath)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("root.constraints.loadBalancerProviders[1].region"),
				}))))
			})
		})

		Context("keystone url validation", func() {
			It("should forbid keystone urls with unsupported format", func() {
				cloudProfileConfig.KeyStoneURL = ""
				cloudProfileConfig.KeyStoneURLs = nil

				errorList := ValidateCloudProfileConfig(cloudProfileConfig, fldPath)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.keyStoneURL"),
				}))))
			})

			It("should forbid keystone urls with missing keys", func() {
				cloudProfileConfig.KeyStoneURL = ""
				cloudProfileConfig.KeyStoneURLs = []api.KeyStoneURL{{}}

				errorList := ValidateCloudProfileConfig(cloudProfileConfig, fldPath)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.keyStoneURLs[0].region"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.keyStoneURLs[0].url"),
				}))))
			})

			It("should forbid duplicate regions for keystone urls", func() {
				cloudProfileConfig.KeyStoneURL = ""
				cloudProfileConfig.KeyStoneURLs = []api.KeyStoneURL{
					{
						Region: "foo",
						URL:    "bar",
					},
					{
						Region: "foo",
						URL:    "bar",
					},
				}

				errorList := ValidateCloudProfileConfig(cloudProfileConfig, fldPath)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("root.keyStoneURLs[1].region"),
				}))))
			})
		})

		Context("dns server validation", func() {
			It("should forbid not invalid dns server ips", func() {
				cloudProfileConfig.DNSServers = []string{"not-a-valid-ip"}

				errorList := ValidateCloudProfileConfig(cloudProfileConfig, fldPath)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("root.dnsServers[0]"),
				}))))
			})
		})

		Context("dhcp domain validation", func() {
			It("should forbid not specifying a value when the key is present", func() {
				cloudProfileConfig.DHCPDomain = pointer.StringPtr("")

				errorList := ValidateCloudProfileConfig(cloudProfileConfig, fldPath)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.dhcpDomain"),
				}))))
			})
		})

		Context("requestTimeout validation", func() {
			It("should reject invalid durations", func() {
				cloudProfileConfig.RequestTimeout = pointer.StringPtr("1GiB")

				errorList := ValidateCloudProfileConfig(cloudProfileConfig, fldPath)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("root.requestTimeout"),
				}))))
			})
		})

		Context("machine image validation", func() {
			It("should enforce that at least one machine image has been defined", func() {
				cloudProfileConfig.MachineImages = []api.MachineImages{}

				errorList := ValidateCloudProfileConfig(cloudProfileConfig, fldPath)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.machineImages"),
				}))))
			})

			It("should forbid unsupported machine image configuration", func() {
				cloudProfileConfig.MachineImages = []api.MachineImages{{}}

				errorList := ValidateCloudProfileConfig(cloudProfileConfig, fldPath)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.machineImages[0].name"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.machineImages[0].versions"),
				}))))
			})

			It("should forbid unsupported machine image version configuration", func() {
				cloudProfileConfig.MachineImages = []api.MachineImages{
					{
						Name:     "abc",
						Versions: []api.MachineImageVersion{{}},
					},
				}

				errorList := ValidateCloudProfileConfig(cloudProfileConfig, fldPath)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.machineImages[0].versions[0].version"),
				}))))
			})
		})

		Context("server group policy validation", func() {
			It("should forbid empty server group policy", func() {
				cloudProfileConfig.ServerGroupPolicies = []string{
					"affinity",
					"",
				}

				errorList := ValidateCloudProfileConfig(cloudProfileConfig, fldPath)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.serverGroupPolicies[1]"),
				}))))
			})
		})
	})
})

var _ = Describe("LoadBalancerClass validation", func() {
	DescribeTable("LoadBalancerClass",
		func(lbClass api.LoadBalancerClass, expectError bool) {
			errorList := ValidateLoadBalancerClasses(lbClass, field.NewPath("loadBalancerClassTest"))
			if !expectError {
				Expect(errorList).To(BeEmpty())
			}
		},
		Entry("pass as LoadBalancerClass class is valid", api.LoadBalancerClass{Name: "A"}, false),
		Entry("fail as LoadBalancerClass has invalid purpose", api.LoadBalancerClass{Purpose: pointer.StringPtr("invalid")}, true),
		Entry("fail as LoadBalancerClass specifies floating subnet by id, name and tags in parallel", api.LoadBalancerClass{
			FloatingSubnetID:   pointer.StringPtr("floating-subnet-id"),
			FloatingSubnetName: pointer.StringPtr("floating-subnet-name"),
			FloatingSubnetTags: pointer.StringPtr("floating-subnet-tags"),
		}, true),
		Entry("fail as LoadBalancerClass specifies floating subnet by id and name", api.LoadBalancerClass{
			FloatingSubnetID:   pointer.StringPtr("floating-subnet-id"),
			FloatingSubnetName: pointer.StringPtr("floating-subnet-name"),
		}, true),
		Entry("fail as LoadBalancerClass specifies floating subnet by id and tags", api.LoadBalancerClass{
			FloatingSubnetID:   pointer.StringPtr("floating-subnet-id"),
			FloatingSubnetTags: pointer.StringPtr("floating-subnet-tags"),
		}, true),
		Entry("fail as LoadBalancerClass specifies floating subnet by name and tags", api.LoadBalancerClass{
			FloatingSubnetName: pointer.StringPtr("floating-subnet-id"),
			FloatingSubnetTags: pointer.StringPtr("floating-subnet-tags"),
		}, true),
	)
})
