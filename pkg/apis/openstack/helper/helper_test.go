// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	. "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
)

var _ = Describe("Helper", func() {
	var (
		purpose      api.Purpose = "foo"
		purposeWrong api.Purpose = "baz"
	)

	DescribeTable("#FindSubnetByPurpose",
		func(subnets []api.Subnet, purpose api.Purpose, expectedSubnet *api.Subnet, expectErr bool) {
			subnet, err := FindSubnetByPurpose(subnets, purpose)
			expectResults(subnet, expectedSubnet, err, expectErr)
		},

		Entry("list is nil", nil, purpose, nil, true),
		Entry("empty list", []api.Subnet{}, purpose, nil, true),
		Entry("entry not found", []api.Subnet{{ID: "bar", Purpose: purposeWrong}}, purpose, nil, true),
		Entry("entry exists", []api.Subnet{{ID: "bar", Purpose: purpose}}, purpose, &api.Subnet{ID: "bar", Purpose: purpose}, false),
	)

	DescribeTable("#FindSecurityGroupByPurpose",
		func(securityGroups []api.SecurityGroup, purpose api.Purpose, expectedSecurityGroup *api.SecurityGroup, expectErr bool) {
			securityGroup, err := FindSecurityGroupByPurpose(securityGroups, purpose)
			expectResults(securityGroup, expectedSecurityGroup, err, expectErr)
		},

		Entry("list is nil", nil, purpose, nil, true),
		Entry("empty list", []api.SecurityGroup{}, purpose, nil, true),
		Entry("entry not found", []api.SecurityGroup{{Name: "bar", Purpose: purposeWrong}}, purpose, nil, true),
		Entry("entry exists", []api.SecurityGroup{{Name: "bar", Purpose: purpose}}, purpose, &api.SecurityGroup{Name: "bar", Purpose: purpose}, false),
	)

	DescribeTable("#FindMachineImage",
		func(machineImages []api.MachineImage, name, version, architecture string, expectedMachineImage *api.MachineImage, expectErr bool) {
			machineImage, err := FindMachineImage(machineImages, name, version, architecture)
			expectResults(machineImage, expectedMachineImage, err, expectErr)
		},

		Entry("list is nil",
			nil,
			"foo", "1.2.3", "",
			nil, true,
		),
		Entry("empty list",
			[]api.MachineImage{},
			"foo", "1.2.3", "",
			nil, true,
		),
		Entry("entry not found (name mismatch)",
			[]api.MachineImage{{Name: "bar", Version: "1.2.3"}},
			"foo", "1.2.3", "",
			nil, true,
		),
		Entry("entry not found (version mismatch)",
			[]api.MachineImage{{Name: "bar", Version: "1.2.3"}},
			"foo", "1.2.3", "",
			nil, true,
		),
		Entry("entry not found (architecture mismatch)",
			[]api.MachineImage{{Name: "bar", Version: "1.2.3", Architecture: ptr.To("amd64")}},
			"bar", "1.2.3", "arm64",
			nil, true,
		),
		Entry("entry exists (architecture is ignored, amd64)",
			[]api.MachineImage{{Name: "bar", Version: "1.2.3"}},
			"bar", "1.2.3", "amd64",
			&api.MachineImage{Name: "bar", Version: "1.2.3"}, false,
		),
		Entry("entry exists (architecture is ignored, arm64)",
			[]api.MachineImage{{Name: "bar", Version: "1.2.3"}},
			"bar", "1.2.3", "arm64",
			&api.MachineImage{Name: "bar", Version: "1.2.3"}, false,
		),
		Entry("entry exists (architecture amd64)",
			[]api.MachineImage{{Name: "bar", Version: "1.2.3", Architecture: ptr.To("amd64")}},
			"bar", "1.2.3", "amd64",
			&api.MachineImage{Name: "bar", Version: "1.2.3", Architecture: ptr.To("amd64")}, false,
		),
		Entry("entry exists (architecture arm64)",
			[]api.MachineImage{{Name: "bar", Version: "1.2.3", Architecture: ptr.To("arm64")}},
			"bar", "1.2.3", "arm64",
			&api.MachineImage{Name: "bar", Version: "1.2.3", Architecture: ptr.To("arm64")}, false,
		),
		Entry("entry exists (multiple architectures)",
			[]api.MachineImage{
				{Name: "bar", Version: "1.2.3", ID: "amd", Architecture: ptr.To("amd64")},
				{Name: "bar", Version: "1.2.3", ID: "arm", Architecture: ptr.To("arm64")},
			},
			"bar", "1.2.3", "amd64",
			&api.MachineImage{Name: "bar", Version: "1.2.3", ID: "amd", Architecture: ptr.To("amd64")}, false,
		),
	)

	regionName := "eu-de-1"

	Describe("#FindImageForCloudProfile", func() {
		var (
			cfg *api.CloudProfileConfig
		)

		BeforeEach(func() {
			cfg = &api.CloudProfileConfig{
				MachineImages: []api.MachineImages{
					{
						Name: "flatcar",
						Versions: []api.MachineImageVersion{
							{
								Version: "1.0",
								Image:   "flatcar_1.0",
							},
							{
								Version: "2.0",
								Image:   "flatcar_2.0",
								Regions: []api.RegionIDMapping{
									{
										Name: "eu01",
										ID:   "flatcar_eu01_2.0",
									},
								},
							},
							{
								Version: "3.0",
								Regions: []api.RegionIDMapping{
									{
										Name:         "eu01",
										ID:           "flatcar_eu01_3.0_amd64",
										Architecture: ptr.To("amd64"),
									},
									{
										Name:         "eu01",
										ID:           "flatcar_eu01_3.0_arm64",
										Architecture: ptr.To("arm64"),
									},
								},
							},
						},
					},
				},
			}
		})

		Context("no image found", func() {
			It("should not find image in nil list", func() {
				cfg.MachineImages = nil

				image, err := FindImageFromCloudProfile(cfg, "flatcar", "1.0", "eu01", "amd64")
				Expect(image).To(BeNil())
				Expect(err).To(MatchError(ContainSubstring("could not find an image")))
			})

			It("should not find image in empty list", func() {
				cfg.MachineImages = []api.MachineImages{}

				image, err := FindImageFromCloudProfile(cfg, "flatcar", "1.0", "eu01", "amd64")
				Expect(image).To(BeNil())
				Expect(err).To(MatchError(ContainSubstring("could not find an image")))
			})

			It("should not find image for wrong image name", func() {
				image, err := FindImageFromCloudProfile(cfg, "gardenlinux", "1.0", "eu01", "amd64")
				Expect(image).To(BeNil())
				Expect(err).To(MatchError(ContainSubstring("could not find an image")))
			})

			It("should not find image for wrong version", func() {
				image, err := FindImageFromCloudProfile(cfg, "flatcar", "1.1", "eu01", "amd64")
				Expect(image).To(BeNil())
				Expect(err).To(MatchError(ContainSubstring("could not find an image")))
			})

		})

		Context("without region mapping", func() {
			It("should fallback to image name (amd64)", func() {
				image, err := FindImageFromCloudProfile(cfg, "flatcar", "1.0", "eu01", "amd64")
				Expect(err).NotTo(HaveOccurred())
				Expect(image).To(Equal(&api.MachineImage{
					Name:         "flatcar",
					Version:      "1.0",
					Image:        "flatcar_1.0",
					Architecture: ptr.To("amd64"),
				}))
			})

			It("should not fallback to image name (not amd64)", func() {
				image, err := FindImageFromCloudProfile(cfg, "flatcar", "1.0", "eu01", "arm64")
				Expect(image).To(BeNil())
				Expect(err).To(MatchError(ContainSubstring("could not find an image")))
			})
		})

		Context("with region mapping, without architectures", func() {
			It("should fallback to image name if region is not mapped", func() {
				image, err := FindImageFromCloudProfile(cfg, "flatcar", "2.0", "eu02", "amd64")
				Expect(err).NotTo(HaveOccurred())
				Expect(image).To(Equal(&api.MachineImage{
					Name:         "flatcar",
					Version:      "2.0",
					Image:        "flatcar_2.0",
					Architecture: ptr.To("amd64"),
				}))
			})

			It("should use the correct mapping (without architecture)", func() {
				image, err := FindImageFromCloudProfile(cfg, "flatcar", "2.0", "eu01", "amd64")
				Expect(err).NotTo(HaveOccurred())
				Expect(image).To(Equal(&api.MachineImage{
					Name:         "flatcar",
					Version:      "2.0",
					ID:           "flatcar_eu01_2.0",
					Architecture: ptr.To("amd64"),
				}))
			})

			It("should not find image because of non-amd64 architecture", func() {
				image, err := FindImageFromCloudProfile(cfg, "flatcar", "2.0", "eu01", "arm64")
				Expect(image).To(BeNil())
				Expect(err).To(MatchError(ContainSubstring("could not find an image")))
			})
		})

		Context("with region mapping and architectures", func() {
			It("should not find image if architecture is not mapped", func() {
				image, err := FindImageFromCloudProfile(cfg, "flatcar", "3.0", "eu01", "ppc64")
				Expect(image).To(BeNil())
				Expect(err).To(MatchError(ContainSubstring("could not find an image")))
			})

			It("should pick the correctly mapped architecture", func() {
				image, err := FindImageFromCloudProfile(cfg, "flatcar", "3.0", "eu01", "arm64")
				Expect(err).NotTo(HaveOccurred())
				Expect(image).To(Equal(&api.MachineImage{
					Name:         "flatcar",
					Version:      "3.0",
					ID:           "flatcar_eu01_3.0_arm64",
					Architecture: ptr.To("arm64"),
				}))
			})
		})
	})

	DescribeTableSubtree("Select Worker Images", func(hasCapabilities bool) {
		var capabilityDefinitions []gardencorev1beta1.CapabilityDefinition
		var machineTypeCapabilities gardencorev1beta1.Capabilities
		var imageCapabilities gardencorev1beta1.Capabilities
		region := "europe"

		if hasCapabilities {
			capabilityDefinitions = []gardencorev1beta1.CapabilityDefinition{
				{Name: "architecture", Values: []string{"amd64", "arm64"}},
				{Name: "capability1", Values: []string{"value1", "value2", "value3"}},
			}
			machineTypeCapabilities = gardencorev1beta1.Capabilities{
				"architecture": []string{"amd64"},
				"capability1":  []string{"value2"},
			}
			imageCapabilities = gardencorev1beta1.Capabilities{
				"architecture": []string{"amd64"},
				"capability1":  []string{"value2"},
			}
		}

		DescribeTable("#FindImageInWorkerStatus",
			func(machineImages []api.MachineImage, name, version string, arch *string, expectedMachineImage *api.MachineImage, expectErr bool) {
				if hasCapabilities {
					machineTypeCapabilities["architecture"] = []string{*arch}
					if expectedMachineImage != nil {
						expectedMachineImage.Capabilities = imageCapabilities
						expectedMachineImage.Architecture = nil
					}
				}
				machineImage, err := FindImageInWorkerStatus(machineImages, name, version, arch, machineTypeCapabilities, capabilityDefinitions)
				expectResults(machineImage, expectedMachineImage, err, expectErr)
			},

			Entry("list is nil", nil, "bar", "1.2.3", ptr.To("amd64"), nil, true),
			Entry("empty list", []api.MachineImage{}, "image", "1.2.3", ptr.To("amd64"), nil, true),
			Entry("entry not found (no name)", makeStatusMachineImages("bar", "1.2.3", "id-1234", ptr.To("amd64"), imageCapabilities), "foo", "1.2.3", ptr.To("amd64"), nil, true),
			Entry("entry not found (no version)", makeStatusMachineImages("bar", "1.2.3", "id-1234", ptr.To("amd64"), imageCapabilities), "bar", "1.2.ś", ptr.To("amd64"), nil, true),
			Entry("entry not found (no architecture)", []api.MachineImage{{Name: "bar", Version: "1.2.3", Architecture: ptr.To("arm64"), Capabilities: gardencorev1beta1.Capabilities{"architecture": []string{"arm64"}}}}, "bar", "1.2.3", ptr.To("amd64"), nil, true),
			Entry("entry exists if architecture is nil", makeStatusMachineImages("bar", "1.2.3", "id-1234", nil, imageCapabilities), "bar", "1.2.3", ptr.To("amd64"), &api.MachineImage{Name: "bar", Version: "1.2.3", ID: "id-1234", Architecture: ptr.To("amd64")}, false),
			Entry("entry exists", makeStatusMachineImages("bar", "1.2.3", "id-1234", ptr.To("amd64"), imageCapabilities), "bar", "1.2.3", ptr.To("amd64"), &api.MachineImage{Name: "bar", Version: "1.2.3", ID: "id-1234", Architecture: ptr.To("amd64")}, false),
		)

		DescribeTable("#FindImageInCloudProfile",
			func(profileImages []api.MachineImages, imageName, version, regionName string, arch *string, expectedID string) {
				if hasCapabilities {
					machineTypeCapabilities["architecture"] = []string{*arch}
				}
				cfg := &api.CloudProfileConfig{}
				cfg.MachineImages = profileImages

				imageFlavor, err := FindImageInCloudProfile(cfg, imageName, version, regionName, arch, machineTypeCapabilities, capabilityDefinitions)

				if expectedID != "" {
					Expect(err).NotTo(HaveOccurred())
					Expect(imageFlavor.Regions[0].ID).To(Equal(expectedID))
				} else {
					Expect(err).To(HaveOccurred())
				}
			},

			Entry("list is nil", nil, "ubuntu", "1", region, ptr.To("amd64"), ""),

			Entry("profile empty list", []api.MachineImages{}, "ubuntu", "1", region, ptr.To("amd64"), ""),
			Entry("profile entry not found (image does not exist)", makeProfileMachineImages("debian", "1", region, "0", ptr.To("amd64"), imageCapabilities), "ubuntu", "1", region, ptr.To("amd64"), ""),
			Entry("profile entry not found (version does not exist)", makeProfileMachineImages("ubuntu", "2", region, "0", ptr.To("amd64"), imageCapabilities), "ubuntu", "1", region, ptr.To("amd64"), ""),
			Entry("profile entry not found (architecture does not exist)", makeProfileMachineImages("ubuntu", "1", region, "0", ptr.To("amd64"), imageCapabilities), "ubuntu", "1", region, ptr.To("arm64"), ""),
			Entry("profile entry", makeProfileMachineImages("ubuntu", "1", region, "id-1234", ptr.To("amd64"), imageCapabilities), "ubuntu", "1", region, ptr.To("amd64"), "id-1234"),
			Entry("profile entry (architecture not defined)", makeProfileMachineImages("ubuntu", "1", region, "id-1234", nil, imageCapabilities), "ubuntu", "1", region, ptr.To("amd64"), "id-1234"),
			Entry("profile non matching region", makeProfileMachineImages("ubuntu", "1", region, "id-1234", ptr.To("amd64"), imageCapabilities), "ubuntu", "1", "china", ptr.To("amd64"), ""),
		)

	},
		Entry("without capabilities", false),
		Entry("with capabilities", true),
	)

	DescribeTable("#FindKeyStoneURL",
		func(keyStoneURLs []api.KeyStoneURL, keystoneURL, region, expectedKeyStoneURL string, expectErr bool) {
			result, err := FindKeyStoneURL(keyStoneURLs, keystoneURL, region)

			if !expectErr {
				Expect(result).To(Equal(expectedKeyStoneURL))
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(result).To(BeEmpty())
				Expect(err).To(HaveOccurred())
			}
		},

		Entry("list is nil", nil, "default", "europe", "default", false),
		Entry("empty list", []api.KeyStoneURL{}, "default", "europe", "default", false),
		Entry("region not found", []api.KeyStoneURL{{URL: "bar", Region: "asia"}}, "default", "europe", "default", false),
		Entry("region exists", []api.KeyStoneURL{{URL: "bar", Region: "europe"}}, "default", "europe", "bar", false),
		Entry("no default URL", []api.KeyStoneURL{{URL: "bar", Region: "europe"}}, "", "asia", "", true),
	)

	DescribeTable("#FindFloatingPool",
		func(floatingPools []api.FloatingPool, floatingPoolNamePattern, region string, domain, expectedFloatingPoolName *string) {
			result, err := FindFloatingPool(floatingPools, floatingPoolNamePattern, region, domain)
			if expectedFloatingPoolName == nil {
				Expect(err).To(HaveOccurred())
				return
			}
			Expect(result.Name).To(Equal(*expectedFloatingPoolName))
		},

		Entry("no fip as list is empty", []api.FloatingPool{}, "fip-1", regionName, nil, nil),
		Entry("return fip as there only one match in the list", []api.FloatingPool{{Name: "fip-*", Region: &regionName}}, "fip-1", regionName, nil, ptr.To("fip-*")),
		Entry("return best matching fip", []api.FloatingPool{{Name: "fip-*", Region: &regionName}, {Name: "fip-1", Region: &regionName}}, "fip-1", regionName, nil, ptr.To("fip-1")),
		Entry("no fip as there no entry for the same region", []api.FloatingPool{{Name: "fip-*", Region: ptr.To("somewhere-else")}}, "fip-1", regionName, nil, nil),
		Entry("no fip as there is no entry with domain", []api.FloatingPool{{Name: "fip-*", Region: &regionName}}, "fip-1", regionName, ptr.To("net-1"), nil),
		Entry("return fip even if there is a non-constraing fip with better score", []api.FloatingPool{{Name: "fip-*", Region: &regionName}, {Name: "fip-1", Region: &regionName, NonConstraining: ptr.To(true)}}, "fip-1", regionName, nil, ptr.To("fip-*")),
		Entry("return non-constraing fip as there is no other matching fip", []api.FloatingPool{{Name: "nofip-1", Region: &regionName}, {Name: "fip-1", Region: &regionName, NonConstraining: ptr.To(true)}}, "fip-1", regionName, nil, ptr.To("fip-1")),
	)
})

//nolint:unparam
func makeProfileMachineImages(name, version, region, id string, arch *string, capabilities gardencorev1beta1.Capabilities) []api.MachineImages {
	versions := []api.MachineImageVersion{{
		Version: version,
	}}

	if capabilities == nil {
		versions[0].Regions = []api.RegionIDMapping{{
			Name:         region,
			ID:           id,
			Architecture: arch,
		}}
	} else {
		versions[0].CapabilityFlavors = []api.MachineImageFlavor{{
			Capabilities: capabilities,
			Regions: []api.RegionIDMapping{{
				Name: region,
				ID:   id,
			}},
		}}
	}

	return []api.MachineImages{
		{
			Name:     name,
			Versions: versions,
		},
	}
}

//nolint:unparam
func makeStatusMachineImages(name, version, id string, arch *string, capabilities gardencorev1beta1.Capabilities) []api.MachineImage {
	if capabilities != nil {
		capabilities["architecture"] = []string{ptr.Deref(arch, "")}
		return []api.MachineImage{
			{
				Name:         name,
				Version:      version,
				ID:           id,
				Capabilities: capabilities,
			},
		}
	}
	return []api.MachineImage{
		{
			Name:         name,
			Version:      version,
			ID:           id,
			Architecture: arch,
		},
	}
}

func expectResults(result, expected interface{}, err error, expectErr bool) {
	if !expectErr {
		Expect(result).To(Equal(expected))
		Expect(err).NotTo(HaveOccurred())
	} else {
		Expect(result).To(BeNil())
		Expect(err).To(HaveOccurred())
	}
}
