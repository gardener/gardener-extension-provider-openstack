// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"
)

var _ = Describe("#TransformProviderConfigToParentFormat", func() {
	var (
		capabilityDefinitions []v1beta1.CapabilityDefinition
	)

	BeforeEach(func() {
		capabilityDefinitions = []v1beta1.CapabilityDefinition{{
			Name:   "architecture",
			Values: []string{"amd64", "arm64"},
		}}
	})

	Context("when config is empty", func() {
		It("should return empty config", func() {
			result := TransformProviderConfigToParentFormat(nil, capabilityDefinitions)

			Expect(result).NotTo(BeNil())
			Expect(result.MachineImages).To(BeEmpty())
		})

		It("should return empty config with proper structure", func() {
			config := &v1alpha1.CloudProfileConfig{}
			result := TransformProviderConfigToParentFormat(config, capabilityDefinitions)

			Expect(result).NotTo(BeNil())
			Expect(result.MachineImages).To(BeEmpty())
			Expect(result.TypeMeta).To(Equal(config.TypeMeta))
		})
	})

	Context("when transforming to capability format", func() {
		It("should transform legacy format with single architecture to capability format", func() {
			config := &v1alpha1.CloudProfileConfig{
				MachineImages: []v1alpha1.MachineImages{
					{
						Name: "ubuntu",
						Versions: []v1alpha1.MachineImageVersion{
							{
								Version: "20.04",
								Regions: []v1alpha1.RegionIDMapping{
									{Name: "eu1", ID: "image-id-123", Architecture: ptr.To("amd64")},
									{Name: "us1", ID: "image-id-456", Architecture: ptr.To("amd64")},
								},
							},
						},
					},
				},
			}

			result := TransformProviderConfigToParentFormat(config, capabilityDefinitions)

			Expect(result.MachineImages).To(HaveLen(1))
			Expect(result.MachineImages[0].Name).To(Equal("ubuntu"))
			Expect(result.MachineImages[0].Versions).To(HaveLen(1))

			version := result.MachineImages[0].Versions[0]
			Expect(version.Version).To(Equal("20.04"))
			Expect(version.Regions).To(BeEmpty()) // Should be empty in capability format
			Expect(version.CapabilityFlavors).To(HaveLen(1))

			flavor := version.CapabilityFlavors[0]
			Expect(flavor.Capabilities).To(Equal(v1beta1.Capabilities{"architecture": []string{"amd64"}}))
			Expect(flavor.Regions).To(ConsistOf(
				v1alpha1.RegionIDMapping{Name: "eu1", ID: "image-id-123"},
				v1alpha1.RegionIDMapping{Name: "us1", ID: "image-id-456"},
			))
		})

		It("should transform legacy format with multiple architectures to capability format", func() {
			config := &v1alpha1.CloudProfileConfig{
				MachineImages: []v1alpha1.MachineImages{
					{
						Name: "ubuntu",
						Versions: []v1alpha1.MachineImageVersion{
							{
								Version: "20.04",
								Regions: []v1alpha1.RegionIDMapping{
									{Name: "eu1", ID: "image-id-123", Architecture: ptr.To("amd64")},
									{Name: "eu1", ID: "image-id-124", Architecture: ptr.To("arm64")},
									{Name: "us1", ID: "image-id-456", Architecture: ptr.To("amd64")},
								},
							},
						},
					},
				},
			}

			result := TransformProviderConfigToParentFormat(config, capabilityDefinitions)

			Expect(result.MachineImages).To(HaveLen(1))
			version := result.MachineImages[0].Versions[0]
			Expect(version.CapabilityFlavors).To(HaveLen(2))

			// Check both architecture flavors are present
			var amd64Flavor, arm64Flavor *v1alpha1.MachineImageFlavor
			for i := range version.CapabilityFlavors {
				switch version.CapabilityFlavors[i].Capabilities["architecture"][0] {
				case "amd64":
					amd64Flavor = &version.CapabilityFlavors[i]
				case "arm64":
					arm64Flavor = &version.CapabilityFlavors[i]
				}
			}

			Expect(amd64Flavor).NotTo(BeNil())
			Expect(amd64Flavor.Regions).To(ConsistOf(
				v1alpha1.RegionIDMapping{Name: "eu1", ID: "image-id-123"},
				v1alpha1.RegionIDMapping{Name: "us1", ID: "image-id-456"},
			))

			Expect(arm64Flavor).NotTo(BeNil())
			Expect(arm64Flavor.Regions).To(ConsistOf(
				v1alpha1.RegionIDMapping{Name: "eu1", ID: "image-id-124"},
			))
		})

		It("should default to amd64 when architecture is not specified", func() {
			config := &v1alpha1.CloudProfileConfig{
				MachineImages: []v1alpha1.MachineImages{
					{
						Name: "ubuntu",
						Versions: []v1alpha1.MachineImageVersion{
							{
								Version: "20.04",
								Regions: []v1alpha1.RegionIDMapping{
									{Name: "eu1", ID: "image-id-123"}, // No architecture specified
								},
							},
						},
					},
				},
			}

			result := TransformProviderConfigToParentFormat(config, capabilityDefinitions)

			version := result.MachineImages[0].Versions[0]
			Expect(version.CapabilityFlavors).To(HaveLen(1))
			Expect(version.CapabilityFlavors[0].Capabilities).To(Equal(v1beta1.Capabilities{"architecture": []string{"amd64"}}))
		})

		It("should preserve already capability-formatted data", func() {
			config := &v1alpha1.CloudProfileConfig{
				MachineImages: []v1alpha1.MachineImages{
					{
						Name: "ubuntu",
						Versions: []v1alpha1.MachineImageVersion{
							{
								Version: "20.04",
								CapabilityFlavors: []v1alpha1.MachineImageFlavor{
									{
										Capabilities: v1beta1.Capabilities{"architecture": []string{"amd64"}},
										Regions:      []v1alpha1.RegionIDMapping{{Name: "eu1", ID: "image-id-123"}},
									},
								},
							},
						},
					},
				},
			}

			result := TransformProviderConfigToParentFormat(config, capabilityDefinitions)

			version := result.MachineImages[0].Versions[0]
			Expect(version.CapabilityFlavors).To(HaveLen(1))
			Expect(version.CapabilityFlavors[0].Capabilities).To(Equal(v1beta1.Capabilities{"architecture": []string{"amd64"}}))
			Expect(version.CapabilityFlavors[0].Regions).To(ConsistOf(
				v1alpha1.RegionIDMapping{Name: "eu1", ID: "image-id-123"},
			))
		})
	})

	Context("when transforming to legacy format", func() {
		BeforeEach(func() {
			capabilityDefinitions = nil // No capability definitions means legacy format
		})

		It("should transform capability format to legacy format", func() {
			config := &v1alpha1.CloudProfileConfig{
				MachineImages: []v1alpha1.MachineImages{
					{
						Name: "ubuntu",
						Versions: []v1alpha1.MachineImageVersion{
							{
								Version: "20.04",
								CapabilityFlavors: []v1alpha1.MachineImageFlavor{
									{
										Capabilities: v1beta1.Capabilities{"architecture": []string{"amd64"}},
										Regions:      []v1alpha1.RegionIDMapping{{Name: "eu1", ID: "image-id-123"}},
									},
									{
										Capabilities: v1beta1.Capabilities{"architecture": []string{"arm64"}},
										Regions:      []v1alpha1.RegionIDMapping{{Name: "eu1", ID: "image-id-124"}},
									},
								},
							},
						},
					},
				},
			}

			result := TransformProviderConfigToParentFormat(config, capabilityDefinitions)

			version := result.MachineImages[0].Versions[0]
			Expect(version.CapabilityFlavors).To(BeEmpty()) // Should be empty in legacy format
			Expect(version.Regions).To(HaveLen(2))
			Expect(version.Regions).To(ConsistOf(
				v1alpha1.RegionIDMapping{Name: "eu1", ID: "image-id-123", Architecture: ptr.To("amd64")},
				v1alpha1.RegionIDMapping{Name: "eu1", ID: "image-id-124", Architecture: ptr.To("arm64")},
			))
		})

		It("should preserve already legacy-formatted data", func() {
			config := &v1alpha1.CloudProfileConfig{
				MachineImages: []v1alpha1.MachineImages{
					{
						Name: "ubuntu",
						Versions: []v1alpha1.MachineImageVersion{
							{
								Version: "20.04",
								Regions: []v1alpha1.RegionIDMapping{
									{Name: "eu1", ID: "image-id-123", Architecture: ptr.To("amd64")},
								},
							},
						},
					},
				},
			}

			result := TransformProviderConfigToParentFormat(config, capabilityDefinitions)

			version := result.MachineImages[0].Versions[0]
			Expect(version.Regions).To(HaveLen(1))
			Expect(version.Regions[0]).To(Equal(v1alpha1.RegionIDMapping{
				Name: "eu1", ID: "image-id-123", Architecture: ptr.To("amd64"),
			}))
		})

		It("should default to amd64 when no architecture capability is found", func() {
			config := &v1alpha1.CloudProfileConfig{
				MachineImages: []v1alpha1.MachineImages{
					{
						Name: "ubuntu",
						Versions: []v1alpha1.MachineImageVersion{
							{
								Version: "20.04",
								CapabilityFlavors: []v1alpha1.MachineImageFlavor{
									{
										Capabilities: v1beta1.Capabilities{"other": []string{"value"}},
										Regions:      []v1alpha1.RegionIDMapping{{Name: "eu1", ID: "image-id-123"}},
									},
								},
							},
						},
					},
				},
			}

			result := TransformProviderConfigToParentFormat(config, capabilityDefinitions)

			version := result.MachineImages[0].Versions[0]
			Expect(version.Regions).To(HaveLen(1))
			Expect(version.Regions[0].Architecture).To(Equal(ptr.To("amd64")))
		})
	})

	Context("when handling edge cases", func() {
		It("should handle empty machine images list", func() {
			config := &v1alpha1.CloudProfileConfig{
				MachineImages: []v1alpha1.MachineImages{},
			}

			result := TransformProviderConfigToParentFormat(config, capabilityDefinitions)

			Expect(result.MachineImages).To(BeEmpty())
		})

		It("should handle machine image with no versions", func() {
			config := &v1alpha1.CloudProfileConfig{
				MachineImages: []v1alpha1.MachineImages{
					{
						Name:     "ubuntu",
						Versions: []v1alpha1.MachineImageVersion{},
					},
				},
			}

			result := TransformProviderConfigToParentFormat(config, capabilityDefinitions)

			Expect(result.MachineImages).To(HaveLen(1))
			Expect(result.MachineImages[0].Name).To(Equal("ubuntu"))
			Expect(result.MachineImages[0].Versions).To(BeEmpty())
		})

		It("should handle version with no regions or capability flavors", func() {
			config := &v1alpha1.CloudProfileConfig{
				MachineImages: []v1alpha1.MachineImages{
					{
						Name: "ubuntu",
						Versions: []v1alpha1.MachineImageVersion{
							{
								Version: "20.04",
								// No regions or capability flavors
							},
						},
					},
				},
			}

			result := TransformProviderConfigToParentFormat(config, capabilityDefinitions)

			version := result.MachineImages[0].Versions[0]
			Expect(version.Version).To(Equal("20.04"))
			Expect(version.Regions).To(BeEmpty())
			Expect(version.CapabilityFlavors).To(BeEmpty())
		})

		It("should handle multiple machine images", func() {
			config := &v1alpha1.CloudProfileConfig{
				MachineImages: []v1alpha1.MachineImages{
					{
						Name: "ubuntu",
						Versions: []v1alpha1.MachineImageVersion{
							{
								Version: "20.04",
								Regions: []v1alpha1.RegionIDMapping{{Name: "eu1", ID: "image-id-123"}},
							},
						},
					},
					{
						Name: "rhel",
						Versions: []v1alpha1.MachineImageVersion{
							{
								Version: "8.5",
								Regions: []v1alpha1.RegionIDMapping{{Name: "us1", ID: "image-id-456"}},
							},
						},
					},
				},
			}

			result := TransformProviderConfigToParentFormat(config, capabilityDefinitions)

			Expect(result.MachineImages).To(HaveLen(2))

			ubuntuImg := result.MachineImages[0]
			if ubuntuImg.Name != "ubuntu" {
				ubuntuImg = result.MachineImages[1]
			}
			Expect(ubuntuImg.Name).To(Equal("ubuntu"))

			rhelImg := result.MachineImages[0]
			if rhelImg.Name != "rhel" {
				rhelImg = result.MachineImages[1]
			}
			Expect(rhelImg.Name).To(Equal("rhel"))
		})
	})
})
