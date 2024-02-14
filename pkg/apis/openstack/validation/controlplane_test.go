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
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	. "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/validation"
)

var _ = Describe("ControlPlaneConfig validation", func() {
	var (
		lbProvider1  = "foo"
		nilPath      *field.Path
		controlPlane *api.ControlPlaneConfig
		infraConfig  *api.InfrastructureConfig
	)

	BeforeEach(func() {
		controlPlane = &api.ControlPlaneConfig{
			LoadBalancerProvider: lbProvider1,
		}
		infraConfig = &api.InfrastructureConfig{}
	})

	Describe("#ValidateControlPlaneConfig", func() {

		It("should return no errors for a valid configuration", func() {
			Expect(ValidateControlPlaneConfig(controlPlane, infraConfig, "", nilPath)).To(BeEmpty())
		})

		It("should require the name of a load balancer provider", func() {
			controlPlane.LoadBalancerProvider = ""

			errorList := ValidateControlPlaneConfig(controlPlane, infraConfig, "", nilPath)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("loadBalancerProvider"),
			}))))
		})

		It("should fail with invalid CCM feature gates", func() {
			controlPlane.CloudControllerManager = &api.CloudControllerManagerConfig{
				FeatureGates: map[string]bool{
					"AnyVolumeDataSource": true,
					"Foo":                 true,
				},
			}

			errorList := ValidateControlPlaneConfig(controlPlane, infraConfig, "1.28.2", nilPath)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("cloudControllerManager.featureGates.Foo"),
				})),
			))
		})

		It("should fail if CSI Manila is enabled, but no share network is created", func() {
			controlPlane.Storage = &api.Storage{CSIManila: &api.CSIManila{Enabled: true}}
			infraConfig.Networks.ShareNetwork = &api.ShareNetwork{Enabled: false}

			errorList := ValidateControlPlaneConfig(controlPlane, infraConfig, "1.28.2", nilPath)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("storage.csiManila.enabled"),
				})),
			))
		})

		It("should return no error if CSI Manila is enabled and share network is created", func() {
			controlPlane.Storage = &api.Storage{CSIManila: &api.CSIManila{Enabled: true}}
			infraConfig.Networks.ShareNetwork = &api.ShareNetwork{Enabled: true}

			errorList := ValidateControlPlaneConfig(controlPlane, infraConfig, "1.28.2", nilPath)

			Expect(errorList).To(BeEmpty())
		})
	})

	Describe("#ValidateControlPlaneConfigUpdate", func() {
		It("should return no errors for an unchanged config", func() {
			Expect(ValidateControlPlaneConfigUpdate(controlPlane, controlPlane, nilPath)).To(BeEmpty())
		})
	})

	Describe("#ValidateControlPlaneConfigAgainstCloudProfile", func() {
		var (
			region       = "foo"
			domain       = "dummy"
			zone         = "some-zone"
			floatingPool = "fp"

			cloudProfileConfig *api.CloudProfileConfig
			cloudProfile       *gardencorev1beta1.CloudProfile
			loadBalancerClass  api.LoadBalancerClass
		)

		BeforeEach(func() {
			cloudProfile = &gardencorev1beta1.CloudProfile{
				Spec: gardencorev1beta1.CloudProfileSpec{
					Regions: []gardencorev1beta1.Region{
						{
							Name: region,
							Zones: []gardencorev1beta1.AvailabilityZone{
								{Name: zone},
							},
						},
					},
				},
			}

			loadBalancerClass = api.LoadBalancerClass{
				Name:              "LBCLass",
				FloatingSubnetID:  pointer.String("1"),
				FloatingNetworkID: pointer.String("1"),
				SubnetID:          pointer.String("1"),
			}

			cloudProfileConfig = &api.CloudProfileConfig{
				Constraints: api.Constraints{
					LoadBalancerProviders: []api.LoadBalancerProvider{
						{Name: lbProvider1},
					},
					FloatingPools: []api.FloatingPool{
						{
							Name:                floatingPool,
							Region:              &region,
							LoadBalancerClasses: []api.LoadBalancerClass{loadBalancerClass},
						},
					},
				},
			}
		})

		It("should require a name of a load balancer provider that is part of the constraints", func() {
			controlPlane.LoadBalancerProvider = "bar"

			errorList := ValidateControlPlaneConfigAgainstCloudProfile(nil, controlPlane, domain, region, floatingPool, cloudProfileConfig, nilPath)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("loadBalancerProvider"),
			}))))
		})

		It("should forbid using a load balancer provider from a different region", func() {
			differentRegion := "asia"
			cloudProfileConfig.Constraints = api.Constraints{
				LoadBalancerProviders: []api.LoadBalancerProvider{
					{
						Name:   lbProvider1,
						Region: &region,
					},
					{
						Name:   "other",
						Region: &differentRegion,
					},
				},
			}
			cloudProfile.Spec.Regions = []gardencorev1beta1.Region{
				{
					Name: differentRegion,
					Zones: []gardencorev1beta1.AvailabilityZone{
						{Name: zone},
					},
				},
			}
			controlPlane.LoadBalancerProvider = lbProvider1

			errorList := ValidateControlPlaneConfigAgainstCloudProfile(nil, controlPlane, domain, differentRegion, "", cloudProfileConfig, nilPath)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("loadBalancerProvider"),
			}))))
		})

		It("should forbid using a load balancer provider from a different region even if none local is specified", func() {
			differentRegion := "asia"
			lbProvider2 := "lb2"

			cloudProfileConfig.Constraints = api.Constraints{
				LoadBalancerProviders: []api.LoadBalancerProvider{
					{
						Name: lbProvider2,
					},
					{
						Name:   lbProvider1,
						Region: &differentRegion,
					},
				},
			}
			cloudProfile.Spec.Regions = []gardencorev1beta1.Region{
				{
					Name: region,
					Zones: []gardencorev1beta1.AvailabilityZone{
						{Name: zone},
					},
				},
			}
			controlPlane.LoadBalancerProvider = lbProvider1

			errorList := ValidateControlPlaneConfigAgainstCloudProfile(nil, controlPlane, domain, region, floatingPool, cloudProfileConfig, nilPath)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("loadBalancerProvider"),
			}))))
		})

		It("should allow using the non-regional load balancer provider name if region not specified", func() {
			differentRegion := "asia"
			lbProvider2 := "lb2"

			cloudProfileConfig.Constraints = api.Constraints{
				LoadBalancerProviders: []api.LoadBalancerProvider{
					{
						Name: lbProvider2,
					},
					{
						Name:   lbProvider1,
						Region: &region,
					},
				},
			}
			cloudProfile.Spec.Regions = []gardencorev1beta1.Region{
				{
					Name: differentRegion,
					Zones: []gardencorev1beta1.AvailabilityZone{
						{Name: zone},
					},
				},
			}
			controlPlane.LoadBalancerProvider = lbProvider2

			errorList := ValidateControlPlaneConfigAgainstCloudProfile(nil, controlPlane, domain, differentRegion, "", cloudProfileConfig, nilPath)

			Expect(errorList).To(BeEmpty())
		})

		It("should pass because no load balancer class is configured in cloud profile", func() {
			cloudProfileConfig.Constraints.FloatingPools[0].LoadBalancerClasses = nil

			errorList := ValidateControlPlaneConfigAgainstCloudProfile(nil, controlPlane, domain, region, floatingPool, cloudProfileConfig, nilPath)

			Expect(errorList).To(BeEmpty())
		})

		It("should pass because load balancer class is configured correctly in control plane", func() {
			controlPlane.LoadBalancerClasses = []api.LoadBalancerClass{loadBalancerClass}

			errorList := ValidateControlPlaneConfigAgainstCloudProfile(nil, controlPlane, domain, region, floatingPool, cloudProfileConfig, nilPath)

			Expect(errorList).To(BeEmpty())
		})

		It("should forbid using the non-regional floating pool's load balancer class if region is specified", func() {
			lbClasses := []api.LoadBalancerClass{
				{Name: "lbClass1"},
				{Name: "lbClass2"},
				{Name: "lbClass2"},
			}

			cloudProfileConfig.Constraints.FloatingPools = append(cloudProfileConfig.Constraints.FloatingPools, api.FloatingPool{
				Name:                "No region",
				LoadBalancerClasses: lbClasses,
			})

			controlPlane.LoadBalancerClasses = []api.LoadBalancerClass{lbClasses[1], lbClasses[0]}

			errorList := ValidateControlPlaneConfigAgainstCloudProfile(nil, controlPlane, domain, region, floatingPool, cloudProfileConfig, nilPath)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeNotSupported),
					"Field": Equal("loadBalancerClasses[0]"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeNotSupported),
					"Field": Equal("loadBalancerClasses[1]"),
				}))),
			)
		})

		It("should pass for a non-regional load balancer class to be configured in the control plane w/ different shoot region", func() {
			differentRegion := "asia"
			fpName := "No region"
			lbClasses := []api.LoadBalancerClass{
				{Name: "lbClass1"},
				{Name: "lbClass2"},
				{Name: "lbClass2"},
			}

			cloudProfileConfig.Constraints.FloatingPools = append(cloudProfileConfig.Constraints.FloatingPools, api.FloatingPool{
				Name:                "No region",
				LoadBalancerClasses: lbClasses,
			})

			cloudProfile.Spec.Regions = []gardencorev1beta1.Region{
				{
					Name: differentRegion,
					Zones: []gardencorev1beta1.AvailabilityZone{
						{Name: zone},
					},
				},
			}

			controlPlane.LoadBalancerClasses = []api.LoadBalancerClass{lbClasses[1]}

			errorList := ValidateControlPlaneConfigAgainstCloudProfile(nil, controlPlane, domain, differentRegion, fpName, cloudProfileConfig, nilPath)

			Expect(errorList).To(BeEmpty())
		})

		It("should pass for a load balancer class which is assigned by a wildcard FloatingPoolName", func() {
			fpName := "some_name"
			lbClasses := []api.LoadBalancerClass{
				{Name: "lbClass1"},
				{Name: "lbClass2"},
				{Name: "lbClass2"},
			}

			cloudProfileConfig.Constraints.FloatingPools = append(cloudProfileConfig.Constraints.FloatingPools, api.FloatingPool{
				Name:                "*",
				Region:              &region,
				LoadBalancerClasses: lbClasses,
			})

			controlPlane.LoadBalancerClasses = []api.LoadBalancerClass{lbClasses[1]}

			errorList := ValidateControlPlaneConfigAgainstCloudProfile(nil, controlPlane, domain, region, fpName, cloudProfileConfig, nilPath)

			Expect(errorList).To(BeEmpty())
		})

		It("should forbid for a load balancer class because a closer match exists", func() {
			lbClasses := []api.LoadBalancerClass{
				{Name: "lbClass1"},
			}

			cloudProfileConfig.Constraints.FloatingPools = append(cloudProfileConfig.Constraints.FloatingPools, api.FloatingPool{
				Name:                "*",
				Region:              &region,
				LoadBalancerClasses: lbClasses,
			})

			controlPlane.LoadBalancerClasses = []api.LoadBalancerClass{lbClasses[0]}

			errorList := ValidateControlPlaneConfigAgainstCloudProfile(nil, controlPlane, domain, region, floatingPool, cloudProfileConfig, nilPath)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("loadBalancerClasses[0]"),
			}))))
		})

		It("should forbid using a load balancer class from another region", func() {
			differentRegion := "asia"
			lbClasses := []api.LoadBalancerClass{
				{Name: "asia-lbClass1"},
				{Name: "asia-lbClass2"},
				{Name: "asia-lbClass2"},
			}
			cloudProfileConfig.Constraints.FloatingPools = append(cloudProfileConfig.Constraints.FloatingPools, api.FloatingPool{
				Region:              &differentRegion,
				LoadBalancerClasses: lbClasses,
			})

			controlPlane.LoadBalancerClasses = append(controlPlane.LoadBalancerClasses, lbClasses[0])

			errorList := ValidateControlPlaneConfigAgainstCloudProfile(nil, controlPlane, domain, region, floatingPool, cloudProfileConfig, nilPath)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("loadBalancerClasses[0]"),
			}))))
		})

		It("should forbid using a load balancer class from another domain", func() {
			differentDomain := "domain2"
			lbClasses := []api.LoadBalancerClass{
				{Name: "asia-lbClass1"},
				{Name: "asia-lbClass2"},
				{Name: "asia-lbClass2"},
			}
			cloudProfileConfig.Constraints.FloatingPools = []api.FloatingPool{
				{
					Name:                floatingPool,
					LoadBalancerClasses: []api.LoadBalancerClass{loadBalancerClass},
				},
				{
					Name:                floatingPool,
					Domain:              &domain,
					LoadBalancerClasses: lbClasses,
				},
			}

			controlPlane.LoadBalancerClasses = append(controlPlane.LoadBalancerClasses, lbClasses[0])

			errorList := ValidateControlPlaneConfigAgainstCloudProfile(nil, controlPlane, domain, region, floatingPool, cloudProfileConfig, nilPath)
			Expect(errorList).To(BeEmpty())
			errorList = ValidateControlPlaneConfigAgainstCloudProfile(nil, controlPlane, differentDomain, region, floatingPool, cloudProfileConfig, nilPath)
			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("loadBalancerClasses[0]"),
			}))))
		})

		It("should forbid using a load balancer class if none is configured in cloud profile", func() {
			controlPlane.LoadBalancerClasses = []api.LoadBalancerClass{loadBalancerClass}
			cloudProfileConfig.Constraints.FloatingPools = nil

			errorList := ValidateControlPlaneConfigAgainstCloudProfile(nil, controlPlane, domain, region, floatingPool, cloudProfileConfig, nilPath)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("floatingPoolName"),
			}))))
		})

		It("should forbid using a load balancer class that is not available", func() {
			lbClass := loadBalancerClass
			lbClass.FloatingNetworkID = nil
			controlPlane.LoadBalancerClasses = []api.LoadBalancerClass{lbClass}

			errorList := ValidateControlPlaneConfigAgainstCloudProfile(nil, controlPlane, domain, region, floatingPool, cloudProfileConfig, nilPath)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("loadBalancerClasses[0]"),
			}))))
		})

		It("should not validate anything if the load balancer provider was not changed", func() {
			controlPlane.LoadBalancerProvider = "does-for-sure-not-exist-in-cloudprofile"
			oldControlPlane := controlPlane.DeepCopy()

			errorList := ValidateControlPlaneConfigAgainstCloudProfile(oldControlPlane, controlPlane, domain, region, floatingPool, cloudProfileConfig, nilPath)
			Expect(errorList).To(BeEmpty())
		})

		It("should not validate anything if the load balancer classes were not changed", func() {
			loadBalancerClassNotInCloudProfile := loadBalancerClass.DeepCopy()
			loadBalancerClassNotInCloudProfile.Name = "does-for-sure-not-exist-in-cloudprofile"

			controlPlane.LoadBalancerClasses = []api.LoadBalancerClass{*loadBalancerClassNotInCloudProfile}
			oldControlPlane := controlPlane.DeepCopy()

			errorList := ValidateControlPlaneConfigAgainstCloudProfile(oldControlPlane, controlPlane, domain, region, floatingPool, cloudProfileConfig, nilPath)
			Expect(errorList).To(BeEmpty())
		})
	})
})
