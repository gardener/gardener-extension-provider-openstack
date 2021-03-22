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

package helper

import (
	"fmt"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/utils"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"

	"github.com/Masterminds/semver"
)

// K8sVersionV121 represents the Kubernetes v1.21.0 version.
var K8sVersionV121 = semver.MustParse("v1.21.0")

// FindSubnetByPurpose takes a list of subnets and tries to find the first entry
// whose purpose matches with the given purpose. If no such entry is found then an error will be
// returned.
func FindSubnetByPurpose(subnets []api.Subnet, purpose api.Purpose) (*api.Subnet, error) {
	for _, subnet := range subnets {
		if subnet.Purpose == purpose {
			return &subnet, nil
		}
	}
	return nil, fmt.Errorf("cannot find subnet with purpose %q", purpose)
}

// FindSecurityGroupByPurpose takes a list of security groups and tries to find the first entry
// whose purpose matches with the given purpose. If no such entry is found then an error will be
// returned.
func FindSecurityGroupByPurpose(securityGroups []api.SecurityGroup, purpose api.Purpose) (*api.SecurityGroup, error) {
	for _, securityGroup := range securityGroups {
		if securityGroup.Purpose == purpose {
			return &securityGroup, nil
		}
	}
	return nil, fmt.Errorf("cannot find security group with purpose %q", purpose)
}

// FindMachineImage takes a list of machine images and tries to find the first entry
// whose name, version, and zone matches with the given name, version, and cloud profile. If no such
// entry is found then an error will be returned.
func FindMachineImage(machineImages []api.MachineImage, name, version string) (*api.MachineImage, error) {
	for _, machineImage := range machineImages {
		if machineImage.Name == name && machineImage.Version == version {
			return &machineImage, nil
		}
	}
	return nil, fmt.Errorf("no machine image with name %q, version %q found", name, version)
}

// FindImageFromCloudProfile takes a list of machine images, and the desired image name and version. It tries
// to find the image with the given name and version in the desired cloud profile. If it cannot be found then an error
// is returned.
func FindImageFromCloudProfile(cloudProfileConfig *api.CloudProfileConfig, imageName, imageVersion, regionName string) (*api.MachineImage, error) {
	if cloudProfileConfig != nil {
		for _, machineImage := range cloudProfileConfig.MachineImages {
			if machineImage.Name != imageName {
				continue
			}
			for _, version := range machineImage.Versions {
				if imageVersion != version.Version {
					continue
				}
				for _, region := range version.Regions {
					if regionName == region.Name {
						return &api.MachineImage{
							Name:    imageName,
							Version: imageVersion,
							ID:      region.ID,
						}, nil
					}
				}
				if version.Image != "" {
					return &api.MachineImage{
						Name:    imageName,
						Version: imageVersion,
						Image:   version.Image,
					}, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("could not find an image for name %q in version %q for region %q", imageName, imageVersion, regionName)
}

// FindKeyStoneURL takes a list of keystone URLs and tries to find the first entry
// whose region matches with the given region. If no such entry is found then it tries to use the non-regional
// keystone URL. If this is not specified then an error will be returned.
func FindKeyStoneURL(keyStoneURLs []api.KeyStoneURL, keystoneURL, region string) (string, error) {
	for _, keyStoneURL := range keyStoneURLs {
		if keyStoneURL.Region == region {
			return keyStoneURL.URL, nil
		}
	}

	if len(keystoneURL) > 0 {
		return keystoneURL, nil
	}

	return "", fmt.Errorf("cannot find keystone URL for region %q", region)
}

// FindFloatingPool receives a list of floating pools and tries to find the best
// match for a given `floatingPoolNamePattern` considering constraints like
// `region` and `domain`. If no matching floating pool was found then an error will be returned.
func FindFloatingPool(floatingPools []api.FloatingPool, floatingPoolNamePattern, region string, domain *string) (*api.FloatingPool, error) {
	var (
		floatingPoolCandidate        *api.FloatingPool
		maxCandidateScore            int
		nonConstrainingFloatingPools []api.FloatingPool
	)

	for _, f := range floatingPools {
		var fip = f

		// Check non constraining floating pools with second priority
		// which means only when no other floating pool is matching.
		if fip.NonConstraining != nil && *fip.NonConstraining {
			nonConstrainingFloatingPools = append(nonConstrainingFloatingPools, fip)
			continue
		}

		if candidate, score := checkFloatingPoolCandidate(&fip, floatingPoolNamePattern, region, domain); candidate != nil && score > maxCandidateScore {
			floatingPoolCandidate = candidate
			maxCandidateScore = score
		}
	}

	if floatingPoolCandidate != nil {
		return floatingPoolCandidate, nil
	}

	// So far no floating pool was matching to the `floatingPoolNamePattern`
	// therefore try now if there is a non contraining floating pool matching.
	for _, f := range nonConstrainingFloatingPools {
		var fip = f
		if candidate, score := checkFloatingPoolCandidate(&fip, floatingPoolNamePattern, region, domain); candidate != nil && score > maxCandidateScore {
			floatingPoolCandidate = candidate
			maxCandidateScore = score
		}
	}

	if floatingPoolCandidate != nil {
		return floatingPoolCandidate, nil
	}

	return nil, fmt.Errorf("cannot find a matching floating pool for pattern %q", floatingPoolNamePattern)
}

func checkFloatingPoolCandidate(floatingPool *api.FloatingPool, floatingPoolNamePattern, region string, domain *string) (*api.FloatingPool, int) {
	// If the domain should be considered then only floating pools
	// in the same domain will be considered.
	if domain != nil && !utils.IsStringPtrValueEqual(floatingPool.Domain, *domain) {
		return nil, 0
	}

	// Require floating pools are in the same region.
	if !utils.IsStringPtrValueEqual(floatingPool.Region, region) {
		return nil, 0
	}

	// Check that the name of the current floatingPool is matching to the `floatingPoolNamePattern`.
	if isMatching, score := utils.SimpleMatch(floatingPool.Name, floatingPoolNamePattern); isMatching {
		return floatingPool, score
	}

	return nil, 0
}

// FilterLoadBalancerClassByVersionContraints filters a given list of load balancer classes
// to consider the only usable load balancer classes with version compatible features
// based on a given kubernetes version.
func FilterLoadBalancerClassByVersionContraints(lbClasses []api.LoadBalancerClass, version *semver.Version) []api.LoadBalancerClass {
	var (
		usableLoadBalancerClasses = []api.LoadBalancerClass{}
		lessThenK8sV121           = version.LessThan(K8sVersionV121)
	)

	for _, lbClass := range lbClasses {
		if lessThenK8sV121 && (lbClass.FloatingSubnetName != nil || lbClass.FloatingSubnetTags != nil) {
			continue
		}
		lbClassRef := lbClass
		usableLoadBalancerClasses = append(usableLoadBalancerClasses, lbClassRef)
	}

	return usableLoadBalancerClasses
}

// DetermineShootVersionFromCluster receives a cluster resource and determine the
// Shoot version out of it. If it can't be determined an error will be returned.
func DetermineShootVersionFromCluster(cluster *extensionscontroller.Cluster) (*semver.Version, error) {
	if cluster == nil || cluster.Shoot == nil {
		return nil, fmt.Errorf("cluster or shoot object in cluster is empty")
	}
	version, err := semver.NewVersion(cluster.Shoot.Spec.Kubernetes.Version)
	if err != nil {
		return nil, err
	}
	return version, nil
}
