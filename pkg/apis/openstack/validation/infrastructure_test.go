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
	"strings"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	. "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/validation"

	. "github.com/gardener/gardener/pkg/utils/validation/gomega"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
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
					ID: makeStringPointer("hugo"),
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
			infrastructureConfig.Networks.Router = &api.Router{ID: makeStringPointer("")}

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
			newInfrastructureConfig.Networks.Router = &api.Router{ID: makeStringPointer("name")}

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
			domain             = "dummy"
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

			errorList := ValidateInfrastructureConfigAgainstCloudProfile(infrastructureConfig, domain, region, cloudProfileConfig, nilPath)

			Expect(errorList).To(BeEmpty())
		})

		It("should allow using an arbitrary regional floating pool from the same region (wildcard case)", func() {
			cloudProfileConfig.Constraints.FloatingPools[0].Name = "*"
			infrastructureConfig.FloatingPoolName = floatingPoolName1

			errorList := ValidateInfrastructureConfigAgainstCloudProfile(infrastructureConfig, domain, region, cloudProfileConfig, nilPath)

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

			errorList := ValidateInfrastructureConfigAgainstCloudProfile(infrastructureConfig, domain, differentRegion, cloudProfileConfig, nilPath)

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

			errorList := ValidateInfrastructureConfigAgainstCloudProfile(infrastructureConfig, domain, region, cloudProfileConfig, nilPath)

			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("floatingPoolName"),
			}))
		})

		It("should only allow using a global floating pool name if different from other regional fp", func() {
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

			errorList := ValidateInfrastructureConfigAgainstCloudProfile(infrastructureConfig, domain, region, cloudProfileConfig, nilPath)
			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("floatingPoolName"),
			}))
			errorList = ValidateInfrastructureConfigAgainstCloudProfile(infrastructureConfig, domain, differentRegion, cloudProfileConfig, nilPath)
			Expect(errorList).To(BeEmpty())
		})

		It("should only allow using a global floating pool name if no restricted fp are present", func() {
			differentRegion := "asia"
			differentDomain := "domain2"
			floatingPoolName2 := "fp2"

			cloudProfileConfig.Constraints = api.Constraints{
				FloatingPools: []api.FloatingPool{
					{
						Name: floatingPoolName2,
					},
					{
						Name:   floatingPoolName1,
						Region: &region,
						Domain: &domain,
					},
				},
			}
			infrastructureConfig.FloatingPoolName = floatingPoolName2

			errorList := ValidateInfrastructureConfigAgainstCloudProfile(infrastructureConfig, domain, region, cloudProfileConfig, nilPath)
			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("floatingPoolName"),
			}))
			errorList = ValidateInfrastructureConfigAgainstCloudProfile(infrastructureConfig, differentDomain, region, cloudProfileConfig, nilPath)
			Expect(errorList).To(BeEmpty())
			errorList = ValidateInfrastructureConfigAgainstCloudProfile(infrastructureConfig, differentDomain, differentRegion, cloudProfileConfig, nilPath)
			Expect(errorList).To(BeEmpty())
			errorList = ValidateInfrastructureConfigAgainstCloudProfile(infrastructureConfig, domain, differentRegion, cloudProfileConfig, nilPath)
			Expect(errorList).To(BeEmpty())
		})

		It("should only allow domain and region specific floating pool name for this domain and region", func() {
			differentRegion := "asia"
			differentDomain := "domain2"
			floatingPoolName2 := "fp2"

			cloudProfileConfig.Constraints = api.Constraints{
				FloatingPools: []api.FloatingPool{
					{
						Name: floatingPoolName2,
					},
					{
						Name:   floatingPoolName1,
						Region: &region,
						Domain: &domain,
					},
				},
			}
			infrastructureConfig.FloatingPoolName = floatingPoolName1

			errorList := ValidateInfrastructureConfigAgainstCloudProfile(infrastructureConfig, domain, region, cloudProfileConfig, nilPath)
			Expect(errorList).To(BeEmpty())
			errorList = ValidateInfrastructureConfigAgainstCloudProfile(infrastructureConfig, differentDomain, region, cloudProfileConfig, nilPath)
			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("floatingPoolName"),
			}))
			errorList = ValidateInfrastructureConfigAgainstCloudProfile(infrastructureConfig, differentDomain, differentRegion, cloudProfileConfig, nilPath)
			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("floatingPoolName"),
			}))
			errorList = ValidateInfrastructureConfigAgainstCloudProfile(infrastructureConfig, domain, differentRegion, cloudProfileConfig, nilPath)
			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("floatingPoolName"),
			}))
		})

		It("should only allow domain specific floating pool name for this domain", func() {
			differentDomain := "domain2"
			floatingPoolName2 := "fp2"

			cloudProfileConfig.Constraints = api.Constraints{
				FloatingPools: []api.FloatingPool{
					{
						Name: floatingPoolName2,
					},
					{
						Name:   floatingPoolName1,
						Domain: &domain,
					},
				},
			}
			infrastructureConfig.FloatingPoolName = floatingPoolName1

			errorList := ValidateInfrastructureConfigAgainstCloudProfile(infrastructureConfig, domain, region, cloudProfileConfig, nilPath)
			Expect(errorList).To(BeEmpty())
			errorList = ValidateInfrastructureConfigAgainstCloudProfile(infrastructureConfig, differentDomain, region, cloudProfileConfig, nilPath)
			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("floatingPoolName"),
			}))
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

			errorList := ValidateInfrastructureConfigAgainstCloudProfile(infrastructureConfig, domain, differentRegion, cloudProfileConfig, nilPath)

			Expect(errorList).To(BeEmpty())
		})
	})

	Describe("#FindFloatingPool", func() {
		domain1 := "domain1"
		domain2 := "domain2"
		domain3 := "domain3"
		domain4 := "domain4"
		regionA := "regionA"
		regionB := "regionB"
		regionC := "regionC"
		regionD := "regionD"

		fpglobal := api.FloatingPool{Name: "fpglobal"}
		fp1 := api.FloatingPool{Name: "fp1", Domain: &domain1}
		fpA := api.FloatingPool{Name: "fpA", Region: &regionA}
		fp1A := api.FloatingPool{Name: "fp1A", Domain: &domain1, Region: &regionA}
		fp3 := api.FloatingPool{Name: "fp3", Domain: &domain3}
		fpC := api.FloatingPool{Name: "fpC", Region: &regionC}
		fp4 := api.FloatingPool{Name: "fp4D", Domain: &domain4}
		fpD := api.FloatingPool{Name: "fp4D", Region: &regionD}
		fpWild := api.FloatingPool{Name: "fpwild*", Domain: &domain1, Region: &regionA}

		pools12 := []api.FloatingPool{fpglobal, fp1, fpA, fp1A, fpWild}
		pools34 := []api.FloatingPool{fp3, fpC, fp4, fpD}

		nonConstraining := true
		fpglobaladd := api.FloatingPool{Name: "fpglobaladd*", NonConstraining: &nonConstraining}
		fp1add := api.FloatingPool{Name: "fp1add", Domain: &domain1, NonConstraining: &nonConstraining}
		pools12add := []api.FloatingPool{fpglobal, fp1, fpA, fp1A, fp1add, fpglobaladd}

		vvNone := []string{}
		vvGlobal := []string{"fpglobal"}
		vv1 := []string{"fp1"}
		vvA := []string{"fpA"}
		vv1A := []string{"fp1A", "fpwild*"}
		vvGlobalAdd := []string{"fpglobal", "fpglobaladd*"}

		valuesToString := func(values []string) string {
			if len(values) == 0 {
				return ""
			}
			return "supported values: \"" + strings.Join(values, "\", \"") + "\""
		}

		DescribeTable("FindFloatingPool table",
			func(pools []api.FloatingPool, fpName string, domain string, region string, expectedFp *api.FloatingPool, expectedValidValues []string) {
				found, errorList := FindFloatingPool(pools, domain, region, fpName, nilPath)
				if expectedFp != nil {
					Expect(found).To(Equal(expectedFp))
					Expect(errorList).To(BeEmpty())
				} else {
					Expect(found).To(BeNil())
					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":     Equal(field.ErrorTypeNotSupported),
						"BadValue": Equal(fpName),
						"Detail":   Equal(valuesToString(expectedValidValues)),
					}))
				}
			},

			Entry("global unrestricted 1A", pools12, "fpglobal", domain1, regionA, nil, vv1A),
			Entry("global unrestricted 1B", pools12, "fpglobal", domain1, regionB, nil, vv1),
			Entry("global unrestricted 2A", pools12, "fpglobal", domain2, regionA, nil, vvA),
			Entry("global unrestricted 2B", pools12, "fpglobal", domain2, regionB, &fpglobal, nil),

			Entry("domain restricted 1A", pools12, "fp1", domain1, regionA, nil, vv1A),
			Entry("domain restricted 1B", pools12, "fp1", domain1, regionB, &fp1, nil),
			Entry("domain restricted 2A", pools12, "fp1", domain2, regionA, nil, vvA),
			Entry("domain restricted 2B", pools12, "fp1", domain2, regionB, nil, vvGlobal),

			Entry("region restricted 1A", pools12, "fpA", domain1, regionA, nil, vv1A),
			Entry("region restricted 1B", pools12, "fpA", domain1, regionB, nil, vv1),
			Entry("region restricted 2A", pools12, "fpA", domain2, regionA, &fpA, nil),
			Entry("region restricted 2B", pools12, "fpA", domain2, regionB, nil, vvGlobal),

			Entry("domain&region restricted 1A", pools12, "fp1A", domain1, regionA, &fp1A, nil),
			Entry("domain&region restricted 1B", pools12, "fp1A", domain1, regionB, nil, vv1),
			Entry("domain&region restricted 2A", pools12, "fp1A", domain2, regionA, nil, vvA),
			Entry("domain&region restricted 2B", pools12, "fp1A", domain2, regionB, nil, vvGlobal),

			Entry("wildcard 1A", pools12, "fpwildfoo", domain1, regionA, &fpWild, nil),
			Entry("wildcard 1B", pools12, "fpwildfoo", domain1, regionB, nil, vv1),
			Entry("wildcard 2A", pools12, "fpwildfoo", domain2, regionA, nil, vvA),
			Entry("wildcard 2B", pools12, "fpwildfoo", domain2, regionB, nil, vvGlobal),

			Entry("unknown", pools12, "fpunknown", domain1, regionA, nil, vv1A),
			Entry("unknown", []api.FloatingPool{}, "fp", domain1, regionA, nil, vvNone),

			Entry("domain restricted 3B", pools34, "fp3", domain3, regionB, &fp3, nil),
			Entry("domain restricted 3C", pools34, "fp3", domain3, regionC, nil, vvNone),
			Entry("domain restricted 4D", pools34, "fp4D", domain4, regionD, &fpD, nil),

			Entry("region restricted 2C", pools34, "fpC", domain2, regionC, &fpC, nil),
			Entry("region restricted 3C", pools34, "fpC", domain3, regionC, nil, vvNone),
			Entry("region restricted 4D", pools34, "fp4D", domain4, regionD, &fpD, nil), // result is fpD because region preferred in intersection cause, but same fp name as &fp4

			Entry("domain/region restricted 1D unknown", pools34, "fpunknown", domain1, regionD, nil, []string{"fp4D"}),
			Entry("domain/region restricted 3D unknown", pools34, "fpunknown", domain3, regionD, nil, vvNone),
			Entry("domain/region restricted 4A unknown", pools34, "fpunknown", domain4, regionA, nil, []string{"fp4D"}),
			Entry("domain/region restricted 4C unknown", pools34, "fpunknown", domain4, regionC, nil, vvNone),
			Entry("domain/region restricted 4D unknown", pools34, "fpunknown", domain4, regionD, nil, []string{"fp4D"}),

			Entry("no non-constraining 1A", pools12, "fp1add", domain1, regionA, nil, vv1A),
			Entry("non-constraining 1A", pools12add, "fp1add", domain1, regionA, &fp1add, nil),
			Entry("non-constraining 1B", pools12add, "fp1add", domain1, regionB, &fp1add, nil),
			Entry("non-constraining 2A", pools12add, "fp1add", domain2, regionA, nil, []string{"fpA", "fpglobaladd*"}),
			Entry("non-constraining 2B", pools12add, "fp1add", domain2, regionB, nil, vvGlobalAdd),

			Entry("no non-constraining - global 1A", pools12, "fpglobaladdfoo", domain1, regionA, nil, vv1A),
			Entry("non-constraining - global 1A", pools12add, "fpglobaladdfoo", domain1, regionA, &fpglobaladd, nil),
			Entry("non-constraining - global 1B", pools12add, "fpglobaladdfoo", domain1, regionB, &fpglobaladd, nil),
			Entry("non-constraining - global 2A", pools12add, "fpglobaladdfoo", domain2, regionA, &fpglobaladd, nil),
			Entry("non-constraining - unknown 1A", pools12add, "fpunknown", domain1, regionA, nil, []string{"fp1A", "fp1add", "fpglobaladd*"}),
			Entry("non-constraining - unknown 1B", pools12add, "fpunknown", domain1, regionB, nil, []string{"fp1", "fp1add", "fpglobaladd*"}),
			Entry("non-constraining - unknown 2A", pools12add, "fpunknown", domain2, regionA, nil, []string{"fpA", "fpglobaladd*"}),
			Entry("non-constraining - unknown 2B", pools12add, "fpunknown", domain2, regionB, nil, []string{"fpglobal", "fpglobaladd*"}),
		)
	})
})
