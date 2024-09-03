package bastion_test

import (
	"slices"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/controller/bastion"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	core "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
)

var _ = Describe("Bastion VM Details", func() {
	var desired bastion.VmDetails
	var spec core.CloudProfileSpec

	BeforeEach(func() {
		desired = bastion.VmDetails{
			MachineName:   "small_machine",
			Architecture:  "amd64",
			ImageBaseName: "gardenlinux",
			ImageVersion:  "1.2.3",
		}
		spec = core.CloudProfileSpec{
			Bastion: &v1beta1.Bastion{
				MachineImage: &v1beta1.BastionMachineImage{
					Name: desired.ImageBaseName,
				},
				MachineType: &v1beta1.BastionMachineType{
					Name: desired.MachineName,
				},
			},
			MachineTypes: []v1beta1.MachineType{{
				CPU:          resource.MustParse("4"),
				Name:         desired.MachineName,
				Architecture: ptr.To(desired.Architecture),
			}},
			MachineImages: []v1beta1.MachineImage{{
				Name: desired.ImageBaseName,
				Versions: []v1beta1.MachineImageVersion{
					{
						ExpirableVersion: v1beta1.ExpirableVersion{
							Version:        desired.ImageVersion,
							Classification: ptr.To(core.ClassificationSupported),
						},
						Architectures: []string{desired.Architecture, "arm64"},
					}},
			}},
		}
	})

	addImageToCloudProfile := func(imageName, version string, classification core.VersionClassification, archs []string) {
		machineIndex := slices.IndexFunc(spec.MachineImages, func(image core.MachineImage) bool {
			return image.Name == imageName
		})

		newVersion := v1beta1.MachineImageVersion{
			ExpirableVersion: v1beta1.ExpirableVersion{
				Version:        version,
				Classification: ptr.To(classification),
			},
			Architectures: archs,
		}

		// append new machine image
		if machineIndex == -1 {
			spec.MachineImages = append(spec.MachineImages, v1beta1.MachineImage{
				Name:     imageName,
				Versions: []v1beta1.MachineImageVersion{newVersion},
			})
		}

		// add new version
		spec.MachineImages[machineIndex].Versions = append(spec.MachineImages[machineIndex].Versions, newVersion)
	}

	Context("DetermineVmDetails", func() {
		It("should succeed without setting bastion image version", func() {
			details, err := bastion.DetermineVmDetails(spec)
			Expect(err).NotTo(HaveOccurred())
			Expect(details).To(DeepEqual(desired))
		})

		It("should succeed with empty bastion section", func() {
			spec.Bastion = &v1beta1.Bastion{}
			details, err := bastion.DetermineVmDetails(spec)
			Expect(err).NotTo(HaveOccurred())
			Expect(details).To(DeepEqual(desired))
		})

		It("should succeed without setting bastion section", func() {
			spec.Bastion = nil
			details, err := bastion.DetermineVmDetails(spec)
			Expect(err).NotTo(HaveOccurred())
			Expect(details).To(DeepEqual(desired))
		})

		It("should succeed without setting bastion image", func() {
			spec.Bastion.MachineImage = nil
			details, err := bastion.DetermineVmDetails(spec)
			Expect(err).NotTo(HaveOccurred())
			Expect(details).To(DeepEqual(desired))
		})

		It("should succeed without setting machineType", func() {
			spec.Bastion.MachineType = nil
			details, err := bastion.DetermineVmDetails(spec)
			Expect(err).NotTo(HaveOccurred())
			Expect(details).To(DeepEqual(desired))
		})

		It("forbid unknown image name", func() {
			spec.Bastion.MachineImage.Name = "unknown_image"
			_, err := bastion.DetermineVmDetails(spec)
			Expect(err).To(HaveOccurred())
		})

		It("forbid unknown image version", func() {
			spec.Bastion.MachineImage.Version = ptr.To("6.6.6")
			_, err := bastion.DetermineVmDetails(spec)
			Expect(err).To(HaveOccurred())
		})

		It("forbid unknown machineType", func() {
			spec.Bastion.MachineType.Name = "unknown_machine"
			_, err := bastion.DetermineVmDetails(spec)
			Expect(err).To(HaveOccurred())
		})

		It("should find greatest supported version", func() {
			addImageToCloudProfile(desired.ImageBaseName, "1.2.4", core.ClassificationSupported, []string{"amd64"})
			desired.ImageVersion = "1.2.4"
			details, err := bastion.DetermineVmDetails(spec)
			Expect(err).NotTo(HaveOccurred())
			Expect(details).To(DeepEqual(desired))
		})

		It("should find smallest machine", func() {
			spec.Bastion.MachineType = nil
			spec.MachineTypes = append(spec.MachineTypes, v1beta1.MachineType{
				CPU:          resource.MustParse("1"),
				GPU:          resource.MustParse("1"),
				Name:         "smallerMachine",
				Architecture: ptr.To(desired.Architecture),
			})
			details, err := bastion.DetermineVmDetails(spec)
			Expect(err).NotTo(HaveOccurred())
			Expect(details.MachineName).To(DeepEqual("smallerMachine"))
		})

		It("should only use supported version", func() {
			addImageToCloudProfile(desired.ImageBaseName, "1.2.4", core.ClassificationPreview, []string{"amd64"})
			details, err := bastion.DetermineVmDetails(spec)
			Expect(err).NotTo(HaveOccurred())
			Expect(details).To(DeepEqual(desired))
		})

		It("should use version which has been specified", func() {
			addImageToCloudProfile(desired.ImageBaseName, "1.2.4", core.ClassificationSupported, []string{"amd64"})
			spec.Bastion.MachineImage.Version = ptr.To("1.2.3")
			details, err := bastion.DetermineVmDetails(spec)
			Expect(err).NotTo(HaveOccurred())
			Expect(details).To(DeepEqual(desired))
		})

		It("allow preview image if version is specified", func() {
			addImageToCloudProfile(desired.ImageBaseName, "1.2.4", core.ClassificationPreview, []string{"amd64"})
			spec.Bastion.MachineImage.Version = ptr.To("1.2.4")
			desired.ImageVersion = "1.2.4"
			details, err := bastion.DetermineVmDetails(spec)
			Expect(err).NotTo(HaveOccurred())
			Expect(details).To(DeepEqual(desired))
		})

		It("only use images for matching machineType architecture", func() {
			addImageToCloudProfile(desired.ImageBaseName, "1.2.4", core.ClassificationSupported, []string{"x86"})
			details, err := bastion.DetermineVmDetails(spec)
			Expect(err).NotTo(HaveOccurred())
			Expect(details).To(DeepEqual(desired))
		})

		It("fail if no image with matching machineType architecture can be found", func() {
			spec.MachineImages[0].Versions[0].Architectures = []string{"x86"}
			_, err := bastion.DetermineVmDetails(spec)
			Expect(err).To(HaveOccurred())
		})
	})
})
