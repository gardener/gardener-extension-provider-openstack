// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"encoding/json"
	"testing"

	extensionsbastion "github.com/gardener/gardener/extensions/pkg/bastion"
	"github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/security/rules"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	openstackv1alpha1 "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"
)

func TestBastion(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bastion Suite")
}

var _ = Describe("Bastion", func() {
	var (
		cluster        *extensions.Cluster
		bastion        *extensionsv1alpha1.Bastion
		providerImages []openstack.MachineImages

		maxLengthForResource int
	)
	BeforeEach(func() {
		cluster = createOpenstackTestCluster()
		bastion = createTestBastion()
		providerImages = createTestProviderConfig().MachineImages
		maxLengthForResource = 63
	})

	Describe("DetermineOptions", func() {
		It("should return options", func() {
			options, err := DetermineOptions(bastion, cluster)
			Expect(err).To(Not(HaveOccurred()))

			Expect(options.ShootName).To(Equal("cluster1"))
			Expect(options.BastionInstanceName).To(Equal("cluster1-bastionName1-bastion-1cdc8"))
			Expect(options.SecretReference).To(Equal(corev1.SecretReference{
				Namespace: "cluster1",
				Name:      "cloudprovider",
			}))
			Expect(options.Region).To(Equal("eu-nl-1"))
		})
	})

	Describe("#ingressPermissions", func() {
		It("Should return a string array with ipV4 normalized addresses", func() {
			bastion.Spec.Ingress = []extensionsv1alpha1.BastionIngressPolicy{
				{IPBlock: networkingv1.IPBlock{
					CIDR: "0.0.0.0/0",
				}},
			}
			ethers, err := ingressPermissions(bastion)
			Expect(err).To(Not(HaveOccurred()))
			Expect(string(ethers[0].EtherType)).To(Equal("IPv4"))
			Expect(ethers[0].CIDR).To(Equal("0.0.0.0/0"))
		})
		It("Should return a string array with ipV6 normalized addresses", func() {
			bastion.Spec.Ingress = []extensionsv1alpha1.BastionIngressPolicy{
				{IPBlock: networkingv1.IPBlock{
					CIDR: "::/0",
				}},
			}
			ethers, err := ingressPermissions(bastion)
			Expect(err).To(Not(HaveOccurred()))
			Expect(string(ethers[0].EtherType)).To(Equal("IPv6"))
			Expect(ethers[0].CIDR).To(Equal("::/0"))
		})
	})

	Describe("#generateBastionBaseResourceName", func() {
		It("should generate idempotent name", func() {
			expected := "clusterName-shortName-bastion-79641"

			res, err := generateBastionBaseResourceName("clusterName", "shortName")
			Expect(err).To(Not(HaveOccurred()))
			Expect(res).To(Equal(expected))
		})

		It("should generate a name not exceeding a certain length", func() {
			res, err := generateBastionBaseResourceName("clusterName", "LetsExceed63LenLimit012345678901234567890123456789012345678901234567890123456789")
			Expect(err).To(Not(HaveOccurred()))
			Expect(res).To(Equal("clusterName-LetsExceed63LenLimit0-bastion-139c4"))
		})

		It("should generate a unique name even if inputs values have minor deviations", func() {
			res, _ := generateBastionBaseResourceName("1", "1")
			res2, _ := generateBastionBaseResourceName("1", "2")
			Expect(res).ToNot(Equal(res2))
		})

		baseName, _ := generateBastionBaseResourceName("clusterName", "LetsExceed63LenLimit012345678901234567890123456789012345678901234567890123456789")
		DescribeTable("should generate names and fit maximum length",
			func(input string, expectedOut string) {
				Expect(len(input)).Should(BeNumerically("<", maxLengthForResource))
				Expect(input).Should(Equal(expectedOut))
			},

			Entry("security group name", securityGroupName(baseName), "clusterName-LetsExceed63LenLimit0-bastion-139c4-sg"),
			Entry("ingress allow ssh resource name", ingressAllowSSHResourceName(baseName), "clusterName-LetsExceed63LenLimit0-bastion-139c4-allow-ssh"),
		)
	})

	Describe("#ingressAllowSSHResourceName,#rulesSymmetricDifference ", func() {
		It("should return rulestoAdd, rulesToDelete", func() {
			currentRules := []rules.SecGroupRule{
				{Direction: "ingress",
					Description:    ingressAllowSSHResourceName("rule A"),
					EtherType:      "IPv4",
					SecGroupID:     "sg-ruleA",
					PortRangeMax:   sshPort,
					PortRangeMin:   sshPort,
					Protocol:       "tcp",
					RemoteIPPrefix: "10.250.0.0/16",
				},
				{Direction: "ingress",
					Description:    ingressAllowSSHResourceName("rule A"),
					EtherType:      "IPv4",
					SecGroupID:     "sg-ruleA",
					PortRangeMax:   sshPort,
					PortRangeMin:   sshPort,
					Protocol:       "tcp",
					RemoteIPPrefix: "10.250.0.0/32",
				},
			}
			wantedRules := []rules.CreateOpts{
				{
					Direction:      "ingress",
					Description:    ingressAllowSSHResourceName("rule A"),
					PortRangeMin:   sshPort,
					EtherType:      rules.EtherType4,
					PortRangeMax:   sshPort,
					Protocol:       "tcp",
					SecGroupID:     "sg-ruleA",
					RemoteIPPrefix: "10.250.0.0/16"},
				{
					Direction:      "ingress",
					Description:    ingressAllowSSHResourceName("rule A"),
					PortRangeMin:   sshPort,
					EtherType:      rules.EtherType6,
					PortRangeMax:   sshPort,
					Protocol:       "tcp",
					SecGroupID:     "sg-ruleA",
					RemoteIPPrefix: "::/0"},
			}
			rulesToAdd, rulesToDelete := rulesSymmetricDifference(wantedRules, currentRules)
			Expect(rulesToAdd).To(HaveLen(1))
			Expect(rulesToDelete).To(HaveLen(1))
		})
	})

	Describe("#ruleEqual", func() {
		It("should return bool for rule equal", func() {
			a := rules.SecGroupRule{
				Direction:      "ingress",
				Description:    ingressAllowSSHResourceName("rule A"),
				EtherType:      "IPv4",
				SecGroupID:     "sg-ruleA",
				PortRangeMax:   sshPort,
				PortRangeMin:   sshPort,
				Protocol:       "tcp",
				RemoteIPPrefix: "10.250.0.0/16",
			}

			b := rules.SecGroupRule{
				Direction:      "ingress",
				Description:    ingressAllowSSHResourceName("rule B"),
				EtherType:      "IPv6",
				SecGroupID:     "sg-ruleB",
				PortRangeMax:   21,
				PortRangeMin:   21,
				Protocol:       "tcp",
				RemoteIPPrefix: "::/0",
			}
			c := rules.CreateOpts{
				Direction:      "ingress",
				Description:    ingressAllowSSHResourceName("rule A"),
				PortRangeMin:   sshPort,
				EtherType:      rules.EtherType4,
				PortRangeMax:   sshPort,
				Protocol:       "tcp",
				SecGroupID:     "sg-ruleA",
				RemoteIPPrefix: "10.250.0.0/16"}

			Expect(ruleEqual(c, a)).To(BeTrue())
			Expect(ruleEqual(c, b)).To(BeFalse())
		})
	})

	Describe("#getProviderSpecificImage", func() {
		var desiredVM = extensionsbastion.MachineSpec{
			MachineTypeName: "small_machine",
			Architecture:    "amd64",
			ImageBaseName:   "gardenlinux",
			ImageVersion:    "1.2.3",
		}

		It("should succeed for existing image", func() {
			machineImage, err := getProviderSpecificImage(providerImages, desiredVM)
			Expect(err).ToNot(HaveOccurred())
			Expect(machineImage).To(DeepEqual(providerImages[0].Versions[0]))
		})

		It("fail if image name does not exist", func() {
			desiredVM.ImageBaseName = "unknown"
			_, err := getProviderSpecificImage(providerImages, desiredVM)
			Expect(err).To(HaveOccurred())
		})

		It("fail if image version does not exist", func() {
			desiredVM.ImageVersion = "6.6.6"
			_, err := getProviderSpecificImage(providerImages, desiredVM)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#findImageIdByRegion", func() {
		It("should succeed for existing image", func() {
			imageID, err := findImageIdByRegion(providerImages[0].Versions[0], "suse", "eu-nl-1")
			Expect(err).ToNot(HaveOccurred())
			Expect(imageID).To(Equal(providerImages[0].Versions[0].Regions[0].ID))
		})

		It("should fail for unknown region", func() {
			_, err := findImageIdByRegion(providerImages[0].Versions[0], "suse", "unknown")
			Expect(err).To(HaveOccurred())
		})
	})
})

func createTestBastion() *extensionsv1alpha1.Bastion {
	return &extensionsv1alpha1.Bastion{
		ObjectMeta: metav1.ObjectMeta{
			Name: "bastionName1",
		},
		Spec: extensionsv1alpha1.BastionSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{},
			UserData:    nil,
			Ingress: []extensionsv1alpha1.BastionIngressPolicy{
				{IPBlock: networkingv1.IPBlock{
					CIDR: "213.69.151.0/24",
				}},
			},
		},
	}
}

func createOpenstackTestCluster() *extensions.Cluster {
	return &controller.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster1"},
		Shoot:      createShootTestStruct(),
		CloudProfile: &gardencorev1beta1.CloudProfile{
			Spec: gardencorev1beta1.CloudProfileSpec{
				Regions: []gardencorev1beta1.Region{
					{Name: "eu-nl-1"},
				},
				MachineImages: createTestMachineImages(),
				MachineTypes:  createTestMachineTypes(),
				ProviderConfig: &runtime.RawExtension{
					Raw: mustEncode(createTestProviderConfig()),
				},
			},
		},
	}
}

func createTestMachineImages() []gardencorev1beta1.MachineImage {
	return []gardencorev1beta1.MachineImage{{
		Name: "gardenlinux",
		Versions: []gardencorev1beta1.MachineImageVersion{{
			ExpirableVersion: gardencorev1beta1.ExpirableVersion{
				Version:        "1.2.3",
				Classification: ptr.To(gardencorev1beta1.ClassificationSupported),
			},
			Architectures: []string{"amd64"},
		}},
	}}
}

func createTestMachineTypes() []gardencorev1beta1.MachineType {
	return []gardencorev1beta1.MachineType{{
		CPU:          resource.MustParse("4"),
		Name:         "machineName",
		Architecture: ptr.To("amd64"),
	}}
}

func createTestProviderConfig() *openstack.CloudProfileConfig {
	return &openstack.CloudProfileConfig{MachineImages: []openstack.MachineImages{{
		Name: "gardenlinux",
		Versions: []openstack.MachineImageVersion{{
			Version: "1.2.3",
			Regions: []openstack.RegionIDMapping{{
				Name: "eu-nl-1",
				ID:   "bfcaecc3-e6f1-46b1-8e1a-a2b7fdeab17d",
			}},
		}},
	}}}
}

func createShootTestStruct() *gardencorev1beta1.Shoot {
	return &gardencorev1beta1.Shoot{
		Spec: gardencorev1beta1.ShootSpec{
			Region:            "eu-nl-1",
			SecretBindingName: ptr.To(v1beta1constants.SecretNameCloudProvider),
			Provider: gardencorev1beta1.Provider{
				InfrastructureConfig: &runtime.RawExtension{
					Object: &openstackv1alpha1.InfrastructureConfig{
						TypeMeta: metav1.TypeMeta{
							APIVersion: openstackv1alpha1.SchemeGroupVersion.String(),
							Kind:       "InfrastructureConfig",
						},
						FloatingPoolName: "FloatingIP-external-monsoon-testing",
					},
				},
			},
		},
	}
}

func mustEncode(object any) []byte {
	data, err := json.Marshal(object)
	Expect(err).ToNot(HaveOccurred())
	return data
}
