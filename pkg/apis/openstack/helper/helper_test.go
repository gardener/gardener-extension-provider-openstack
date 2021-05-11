// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package helper_test

import (
	"github.com/Masterminds/semver"
	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	. "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"k8s.io/utils/pointer"
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
		func(machineImages []api.MachineImage, name, version string, expectedMachineImage *api.MachineImage, expectErr bool) {
			machineImage, err := FindMachineImage(machineImages, name, version)
			expectResults(machineImage, expectedMachineImage, err, expectErr)
		},

		Entry("list is nil", nil, "foo", "1.2.3", nil, true),
		Entry("empty list", []api.MachineImage{}, "foo", "1.2.3", nil, true),
		Entry("entry not found (no name)", []api.MachineImage{{Name: "bar", Version: "1.2.3"}}, "foo", "1.2.3", nil, true),
		Entry("entry not found (no version)", []api.MachineImage{{Name: "bar", Version: "1.2.3"}}, "foo", "1.2.3", nil, true),
		Entry("entry exists", []api.MachineImage{{Name: "bar", Version: "1.2.3"}}, "bar", "1.2.3", &api.MachineImage{Name: "bar", Version: "1.2.3"}, false),
	)

	regionName := "eu-de-1"

	DescribeTable("#FindImageForCloudProfile",
		func(profileImages []api.MachineImages, imageName, version, region, expectedImage string) {
			cfg := &api.CloudProfileConfig{}
			cfg.MachineImages = profileImages
			image, err := FindImageFromCloudProfile(cfg, imageName, version, region)

			if expectedImage == "" {
				Expect(err).To(HaveOccurred())
				Expect(image).To(BeNil())
				return
			}
			Expect(err).NotTo(HaveOccurred())
			Expect(image).NotTo(BeNil())
			if image.ID != "" {
				Expect(image.ID).To(Equal(expectedImage))
			} else {
				Expect(image.Image).To(Equal(expectedImage))
			}
		},

		Entry("list is nil", nil, "ubuntu", "1", regionName, ""),

		Entry("profile empty list", []api.MachineImages{}, "ubuntu", "1", regionName, ""),
		Entry("profile entry not found (image does not exist)", makeProfileMachineImages("debian", "1", "0"), "ubuntu", "1", regionName, ""),
		Entry("profile entry not found (version does not exist)", makeProfileMachineImages("ubuntu", "2", "0"), "ubuntu", "1", regionName, ""),
		Entry("profile entry", makeProfileMachineImages("ubuntu", "1", "image-1234"), "ubuntu", "1", regionName, "image-1234"),
		Entry("profile region entry", makeProfileRegionMachineImages("ubuntu", "1", "image-1234", regionName), "ubuntu", "1", regionName, "image-1234"),
		Entry("profile region not found", makeProfileRegionMachineImages("ubuntu", "1", "image-1234", regionName+"x"), "ubuntu", "1", regionName, ""),
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
		Entry("return fip as there only one match in the list", []api.FloatingPool{{Name: "fip-*", Region: &regionName}}, "fip-1", regionName, nil, pointer.StringPtr("fip-*")),
		Entry("return best matching fip", []api.FloatingPool{{Name: "fip-*", Region: &regionName}, {Name: "fip-1", Region: &regionName}}, "fip-1", regionName, nil, pointer.StringPtr("fip-1")),
		Entry("no fip as there no entry for the same region", []api.FloatingPool{{Name: "fip-*", Region: pointer.StringPtr("somewhere-else")}}, "fip-1", regionName, nil, nil),
		Entry("no fip as there is no entry with domain", []api.FloatingPool{{Name: "fip-*", Region: &regionName}}, "fip-1", regionName, pointer.StringPtr("net-1"), nil),
		Entry("return fip even if there is a non-constraing fip with better score", []api.FloatingPool{{Name: "fip-*", Region: &regionName}, {Name: "fip-1", Region: &regionName, NonConstraining: pointer.BoolPtr(true)}}, "fip-1", regionName, nil, pointer.StringPtr("fip-*")),
		Entry("return non-constraing fip as there is no other matching fip", []api.FloatingPool{{Name: "nofip-1", Region: &regionName}, {Name: "fip-1", Region: &regionName, NonConstraining: pointer.BoolPtr(true)}}, "fip-1", regionName, nil, pointer.StringPtr("fip-1")),
	)

	DescribeTable("#FilterLoadBalancerClassByVersionContraints",
		func(k8sVersion string, lbClasses, expectedLbClasses []api.LoadBalancerClass) {
			version, err := semver.NewVersion(k8sVersion)
			Expect(err).NotTo(HaveOccurred())

			filterLbClassList := FilterLoadBalancerClassByVersionContraints(lbClasses, version)

			Expect(filterLbClassList).To(Equal(expectedLbClasses))
		},
		Entry("should return input list for k8s version >= v1.21", "v1.21.0",
			[]api.LoadBalancerClass{{Name: "default", FloatingSubnetTags: pointer.StringPtr("test1,test2"), FloatingSubnetName: pointer.StringPtr("*pattern*")}},
			[]api.LoadBalancerClass{{Name: "default", FloatingSubnetTags: pointer.StringPtr("test1,test2"), FloatingSubnetName: pointer.StringPtr("*pattern*")}},
		),
		Entry("should return input list for k8s version < v1.21 as there are no entries with unsupporeted fields", "v1.20.0",
			[]api.LoadBalancerClass{{Name: "default", FloatingNetworkID: pointer.StringPtr("fip-1")}},
			[]api.LoadBalancerClass{{Name: "default", FloatingNetworkID: pointer.StringPtr("fip-1")}},
		),
		Entry("should return empty list for k8s version < v1.21 as entries with not supported field floatingSubnetName are filtered", "v1.20.0",
			[]api.LoadBalancerClass{
				{Name: "default", FloatingSubnetName: pointer.StringPtr("*pattern*")}},
			[]api.LoadBalancerClass{},
		),
		Entry("should return empty list for k8s version < v1.21 as entries with not supported field floatingSubnetTags are filtered", "v1.20.0",
			[]api.LoadBalancerClass{{Name: "default", FloatingSubnetTags: pointer.StringPtr("test1,test2")}},
			[]api.LoadBalancerClass{},
		),
	)

	DescribeTable("DetermineShootVersionFromCluster",
		func(cluster *extensionscontroller.Cluster, expectError bool, expectedVersion string) {
			version, err := DetermineShootVersionFromCluster(cluster)
			if expectError {
				Expect(err).To(HaveOccurred())
				return
			}
			Expect(err).NotTo(HaveOccurred())
			Expect(version.String()).To(Equal(expectedVersion))
		},
		Entry("should return shoot version", makeCluster("v1.20.0"), false, "1.20.0"),
		Entry("should return error as shoot verison is invalid", makeCluster("x.x.x"), true, ""),
		Entry("should return error as cluster resource is empty", nil, true, ""),
		Entry("should return error as cluster shoot resource is empty", &extensionscontroller.Cluster{}, true, ""),
	)
})

func makeProfileMachineImages(name, version, image string) []api.MachineImages {
	var versions []api.MachineImageVersion
	if len(image) != 0 {
		versions = append(versions, api.MachineImageVersion{
			Version: version,
			Image:   image,
		})
	}

	return []api.MachineImages{
		{
			Name:     name,
			Versions: versions,
		},
	}
}

func makeProfileRegionMachineImages(name, version, image, region string) []api.MachineImages {
	var versions []api.MachineImageVersion
	if len(image) != 0 {
		versions = append(versions, api.MachineImageVersion{
			Version: version,
			Regions: []api.RegionIDMapping{
				{
					Name: region,
					ID:   image,
				},
			},
		})
	}

	return []api.MachineImages{
		{
			Name:     name,
			Versions: versions,
		},
	}
}

func makeCluster(version string) *extensionscontroller.Cluster {
	return &extensionscontroller.Cluster{
		Shoot: &gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: version,
				},
			},
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
