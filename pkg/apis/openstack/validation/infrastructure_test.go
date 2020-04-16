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

	. "github.com/gardener/gardener/pkg/utils/validation/gomega"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var _ = Describe("InfrastructureConfig validation", func() {
	var (
		nilPath *field.Path

		floatingPoolName1 = "foo"

		infrastructureConfig *api.InfrastructureConfig

		nodes       = "10.250.0.0/16"
		invalidCIDR = "invalid-cidr"
	)

	BeforeEach(func() {
		infrastructureConfig = &api.InfrastructureConfig{
			FloatingPoolName: floatingPoolName1,
			Networks: api.Networks{
				Router: &api.Router{
					ID: "hugo",
				},
				Workers: "10.250.0.0/16",
			},
		}
	})

	Describe("#ValidateInfrastructureConfig", func() {
		It("should forbid invalid floating pool name configuration", func() {
			infrastructureConfig.FloatingPoolName = ""

			errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, nilPath)

			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("floatingPoolName"),
			}))
		})

		It("should forbid invalid router id configuration", func() {
			infrastructureConfig.Networks.Router = &api.Router{ID: ""}

			errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, nilPath)

			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("networks.router.id"),
			}))
		})
	})

	Context("CIDR", func() {
		It("should forbid empty workers CIDR", func() {
			infrastructureConfig.Networks.Workers = ""

			errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, nilPath)

			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":   Equal(field.ErrorTypeRequired),
				"Field":  Equal("networks.workers"),
				"Detail": Equal("must specify the network range for the worker network"),
			}))
		})

		It("should forbid invalid workers CIDR", func() {
			infrastructureConfig.Networks.Workers = invalidCIDR

			errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, nilPath)

			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("networks.workers"),
				"Detail": Equal("invalid CIDR address: invalid-cidr"),
			}))
		})

		It("should forbid workers CIDR which are not in Nodes CIDR", func() {
			infrastructureConfig.Networks.Workers = "1.1.1.1/32"

			errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, nilPath)

			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("networks.workers"),
				"Detail": Equal(`must be a subset of "" ("10.250.0.0/16")`),
			}))
		})

		It("should forbid non canonical CIDRs", func() {
			nodeCIDR := "10.250.0.3/16"

			infrastructureConfig.Networks.Workers = "10.250.3.8/24"

			errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodeCIDR, nilPath)
			Expect(errorList).To(HaveLen(1))

			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("networks.workers"),
				"Detail": Equal("must be valid canonical CIDR"),
			}))
		})
	})

	Describe("#ValidateInfrastructureConfigUpdate", func() {
		It("should return no errors for an unchanged config", func() {
			Expect(ValidateInfrastructureConfigUpdate(infrastructureConfig, infrastructureConfig, nilPath)).To(BeEmpty())
		})

		It("should forbid changing the network section", func() {
			newInfrastructureConfig := infrastructureConfig.DeepCopy()
			newInfrastructureConfig.Networks.Router = &api.Router{ID: "name"}

			errorList := ValidateInfrastructureConfigUpdate(infrastructureConfig, newInfrastructureConfig, nilPath)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("networks"),
			}))))
		})
	})

	Describe("#ValidateInfrastructureConfigAgainstCloudProfile", func() {
		var (
			region             = "europe"
			cloudProfileConfig *api.CloudProfileConfig
		)

		BeforeEach(func() {
			cloudProfileConfig = &api.CloudProfileConfig{
				Constraints: api.Constraints{
					FloatingPools: []api.FloatingPool{
						{
							Name:   floatingPoolName1,
							Region: &region,
						},
					},
				},
			}
		})

		It("should allow using a regional floating pool from the same region", func() {
			infrastructureConfig.FloatingPoolName = floatingPoolName1

			errorList := ValidateInfrastructureConfigAgainstCloudProfile(infrastructureConfig, region, cloudProfileConfig, nilPath)

			Expect(errorList).To(BeEmpty())
		})

		It("should allow using an arbitrary regional floating pool from the same region (wildcard case)", func() {
			cloudProfileConfig.Constraints.FloatingPools[0].Name = "*"
			infrastructureConfig.FloatingPoolName = floatingPoolName1

			errorList := ValidateInfrastructureConfigAgainstCloudProfile(infrastructureConfig, region, cloudProfileConfig, nilPath)

			Expect(errorList).To(BeEmpty())
		})

		It("should forbid using a floating pool name for different region", func() {
			differentRegion := "asia"
			cloudProfileConfig.Constraints = api.Constraints{
				FloatingPools: []api.FloatingPool{
					{
						Name:   floatingPoolName1,
						Region: &region,
					},
					{
						Name:   "other",
						Region: &differentRegion,
					},
				},
			}
			infrastructureConfig.FloatingPoolName = floatingPoolName1

			errorList := ValidateInfrastructureConfigAgainstCloudProfile(infrastructureConfig, differentRegion, cloudProfileConfig, nilPath)

			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("floatingPoolName"),
			}))
		})

		It("should forbid using the non-regional floating pool name if region is specified", func() {
			floatingPoolName2 := "fp2"

			cloudProfileConfig.Constraints = api.Constraints{
				FloatingPools: []api.FloatingPool{
					{
						Name: floatingPoolName2,
					},
					{
						Name:   floatingPoolName1,
						Region: &region,
					},
				},
			}
			infrastructureConfig.FloatingPoolName = floatingPoolName2

			errorList := ValidateInfrastructureConfigAgainstCloudProfile(infrastructureConfig, region, cloudProfileConfig, nilPath)

			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("floatingPoolName"),
			}))
		})

		It("should forbid using floating pool name if no floating pools are specified", func() {
			cloudProfileConfig.Constraints = api.Constraints{
				FloatingPools: nil,
			}
			infrastructureConfig.FloatingPoolName = floatingPoolName1

			errorList := ValidateInfrastructureConfigAgainstCloudProfile(infrastructureConfig, region, cloudProfileConfig, nilPath)

			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("floatingPoolName"),
			}))
		})

		It("should allow using the non-regional floating pool name if region not specified", func() {
			differentRegion := "asia"
			floatingPoolName2 := "fp2"

			cloudProfileConfig.Constraints = api.Constraints{
				FloatingPools: []api.FloatingPool{
					{
						Name: floatingPoolName2,
					},
					{
						Name:   floatingPoolName1,
						Region: &region,
					},
				},
			}
			infrastructureConfig.FloatingPoolName = floatingPoolName2

			errorList := ValidateInfrastructureConfigAgainstCloudProfile(infrastructureConfig, differentRegion, cloudProfileConfig, nilPath)

			Expect(errorList).To(BeEmpty())
		})

		It("should allow using an arbitrary non-regional floating pool name if region not specified (wildcard case)", func() {
			differentRegion := "asia"
			someFloatingPool := "fp2"

			cloudProfileConfig.Constraints = api.Constraints{
				FloatingPools: []api.FloatingPool{
					{
						Name: "*",
					},
					{
						Name:   floatingPoolName1,
						Region: &region,
					},
				},
			}
			infrastructureConfig.FloatingPoolName = someFloatingPool

			errorList := ValidateInfrastructureConfigAgainstCloudProfile(infrastructureConfig, differentRegion, cloudProfileConfig, nilPath)

			Expect(errorList).To(BeEmpty())
		})
	})
})
