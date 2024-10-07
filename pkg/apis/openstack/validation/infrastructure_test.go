// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	"strings"

	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	. "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/validation"
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

		It("should forbid floating ip subnet when router is specified", func() {
			infrastructureConfig.Networks.Router = &api.Router{ID: "sample-router-id"}
			infrastructureConfig.FloatingPoolSubnetName = ptr.To("sample-floating-pool-subnet-id")

			errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, nilPath)

			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("floatingPoolSubnetName"),
			}))
		})

		It("should forbid subnet id when network id is unspecified", func() {
			infrastructureConfig.Networks.SubnetID = ptr.To(uuid.NewString())

			errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, nilPath)

			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("networks.subnetId"),
				"Detail": Equal("if subnet ID is provided a networkID must be provided"),
			}))
		})

		It("should forbid an invalid subnet id", func() {
			infrastructureConfig.Networks.ID = ptr.To(uuid.NewString())
			infrastructureConfig.Networks.SubnetID = ptr.To("thisiswrong")

			errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, nilPath)

			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("networks.subnetId"),
				"Detail": Equal("if subnet ID is provided it must be a valid OpenStack UUID"),
			}))
		})

		It("should allow an valid OpenStack UUID as subnet ID", func() {
			infrastructureConfig.Networks.ID = ptr.To(uuid.NewString())
			infrastructureConfig.Networks.SubnetID = ptr.To(uuid.NewString())

			errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, nilPath)

			Expect(errorList).To(BeEmpty())
		})

		It("should forbid using user-managed subnet with a shareNetwork", func() {
			infrastructureConfig.Networks.ID = ptr.To(uuid.NewString())
			infrastructureConfig.Networks.SubnetID = ptr.To(uuid.NewString())
			infrastructureConfig.Networks.ShareNetwork = &api.ShareNetwork{
				Enabled: true,
			}

			errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, nilPath)
			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("networks.shareNetwork.enabled"),
				"Detail": Equal("the ShareNetwork can not be enabled when a user provider subnet is used. Please disable this option and ensure the shareNetwork connection with your subnet"),
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
				"Detail": Equal(`must be a subset of "networking.nodes" ("10.250.0.0/16")`),
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

		It("should forbid an invalid network id configuration", func() {
			invalidID := "thisiswrong"
			infrastructureConfig.Networks.ID = &invalidID

			errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, nilPath)

			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("networks.id"),
			}))
		})

		It("should allow an valid OpenStack UUID as network ID", func() {
			id, err := uuid.NewUUID()
			Expect(err).NotTo(HaveOccurred())
			infrastructureConfig.Networks.ID = ptr.To(id.String())

			errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, nilPath)

			Expect(errorList).To(BeEmpty())
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

		It("should allow enabling the share network section", func() {
			newInfrastructureConfig := infrastructureConfig.DeepCopy()
			newInfrastructureConfig.Networks.ShareNetwork = &api.ShareNetwork{Enabled: true}

			errorList := ValidateInfrastructureConfigUpdate(infrastructureConfig, newInfrastructureConfig, nilPath)
			Expect(errorList).To(BeEmpty())
		})
		It("should forbid disabling the share network section", func() {
			infrastructureConfig.Networks.ShareNetwork = &api.ShareNetwork{Enabled: true}
			newInfrastructureConfig := infrastructureConfig.DeepCopy()
			newInfrastructureConfig.Networks.ShareNetwork = nil

			errorList := ValidateInfrastructureConfigUpdate(infrastructureConfig, newInfrastructureConfig, nilPath)
			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("networks"),
			}))))
		})

		It("should forbid changing the floating pool", func() {
			newInfrastructureConfig := infrastructureConfig.DeepCopy()
			newInfrastructureConfig.FloatingPoolName = "test"

			errorList := ValidateInfrastructureConfigUpdate(infrastructureConfig, newInfrastructureConfig, nilPath)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("floatingPoolName"),
			}))))
		})

		It("should forbid changing the floating pool subnet", func() {
			newInfrastructureConfig := infrastructureConfig.DeepCopy()
			newInfrastructureConfig.FloatingPoolSubnetName = ptr.To("test")

			errorList := ValidateInfrastructureConfigUpdate(infrastructureConfig, newInfrastructureConfig, nilPath)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("floatingPoolSubnetName"),
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

			errorList := ValidateInfrastructureConfigAgainstCloudProfile(nil, infrastructureConfig, domain, region, cloudProfileConfig, nilPath)

			Expect(errorList).To(BeEmpty())
		})

		It("should allow using an arbitrary regional floating pool from the same region (wildcard case)", func() {
			cloudProfileConfig.Constraints.FloatingPools[0].Name = "*"
			infrastructureConfig.FloatingPoolName = floatingPoolName1

			errorList := ValidateInfrastructureConfigAgainstCloudProfile(nil, infrastructureConfig, domain, region, cloudProfileConfig, nilPath)

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

			errorList := ValidateInfrastructureConfigAgainstCloudProfile(nil, infrastructureConfig, domain, differentRegion, cloudProfileConfig, nilPath)

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

			errorList := ValidateInfrastructureConfigAgainstCloudProfile(nil, infrastructureConfig, domain, region, cloudProfileConfig, nilPath)

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

			errorList := ValidateInfrastructureConfigAgainstCloudProfile(nil, infrastructureConfig, domain, region, cloudProfileConfig, nilPath)
			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("floatingPoolName"),
			}))
			errorList = ValidateInfrastructureConfigAgainstCloudProfile(nil, infrastructureConfig, domain, differentRegion, cloudProfileConfig, nilPath)
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

			errorList := ValidateInfrastructureConfigAgainstCloudProfile(nil, infrastructureConfig, domain, region, cloudProfileConfig, nilPath)
			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("floatingPoolName"),
			}))
			errorList = ValidateInfrastructureConfigAgainstCloudProfile(nil, infrastructureConfig, differentDomain, region, cloudProfileConfig, nilPath)
			Expect(errorList).To(BeEmpty())
			errorList = ValidateInfrastructureConfigAgainstCloudProfile(nil, infrastructureConfig, differentDomain, differentRegion, cloudProfileConfig, nilPath)
			Expect(errorList).To(BeEmpty())
			errorList = ValidateInfrastructureConfigAgainstCloudProfile(nil, infrastructureConfig, domain, differentRegion, cloudProfileConfig, nilPath)
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

			errorList := ValidateInfrastructureConfigAgainstCloudProfile(nil, infrastructureConfig, domain, region, cloudProfileConfig, nilPath)
			Expect(errorList).To(BeEmpty())
			errorList = ValidateInfrastructureConfigAgainstCloudProfile(nil, infrastructureConfig, differentDomain, region, cloudProfileConfig, nilPath)
			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("floatingPoolName"),
			}))
			errorList = ValidateInfrastructureConfigAgainstCloudProfile(nil, infrastructureConfig, differentDomain, differentRegion, cloudProfileConfig, nilPath)
			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("floatingPoolName"),
			}))
			errorList = ValidateInfrastructureConfigAgainstCloudProfile(nil, infrastructureConfig, domain, differentRegion, cloudProfileConfig, nilPath)
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

			errorList := ValidateInfrastructureConfigAgainstCloudProfile(nil, infrastructureConfig, domain, region, cloudProfileConfig, nilPath)
			Expect(errorList).To(BeEmpty())
			errorList = ValidateInfrastructureConfigAgainstCloudProfile(nil, infrastructureConfig, differentDomain, region, cloudProfileConfig, nilPath)
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

			errorList := ValidateInfrastructureConfigAgainstCloudProfile(nil, infrastructureConfig, domain, differentRegion, cloudProfileConfig, nilPath)

			Expect(errorList).To(BeEmpty())
		})

		It("should not validate anything if the floating pool name was not changed", func() {
			infrastructureConfig.FloatingPoolName = "does-for-sure-not-exist-in-cloudprofile"
			oldInfrastructureConfig := infrastructureConfig.DeepCopy()

			errorList := ValidateInfrastructureConfigAgainstCloudProfile(oldInfrastructureConfig, infrastructureConfig, domain, region, cloudProfileConfig, nilPath)
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
