// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"fmt"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"k8s.io/utils/ptr"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/utils"
)

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
func FindMachineImage(machineImages []api.MachineImage, name, version, architecture string) (*api.MachineImage, error) {
	for _, machineImage := range machineImages {
		// If the architecture field is not present, ignore it for backwards-compatibility.
		if machineImage.Name == name && machineImage.Version == version &&
			(machineImage.Architecture == nil || *machineImage.Architecture == architecture) {
			return &machineImage, nil
		}
	}
	return nil, fmt.Errorf("no machine image with name %q, version %q found", name, version)
}

// FindImageFromCloudProfile takes a list of machine images, and the desired image name and version. It tries
// to find the image with the given name and version in the desired cloud profile. If it cannot be found then an error
// is returned.
func FindImageFromCloudProfile(cloudProfileConfig *api.CloudProfileConfig, imageName, imageVersion, regionName, architecture string) (*api.MachineImage, error) {
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
					if regionName == region.Name && architecture == ptr.Deref(region.Architecture, v1beta1constants.ArchitectureAMD64) {
						return &api.MachineImage{
							Name:         imageName,
							Version:      imageVersion,
							Architecture: &architecture,
							ID:           region.ID,
						}, nil
					}
				}

				// if we haven't found a region mapping, fallback to the image name
				if version.Image != "" && architecture == v1beta1constants.ArchitectureAMD64 {
					// The fallback image name doesn't specify an architecture, but we assume it is amd64 as arm was not supported
					// previously.
					// Referencing images by name is error-prone and is highly discouraged anyways.
					// If people want to use arm images in their CloudProfile, they need to specify a region mapping and can't
					// use the fallback MachineImage by name.
					return &api.MachineImage{
						Name:         imageName,
						Version:      imageVersion,
						Architecture: ptr.To(v1beta1constants.ArchitectureAMD64),
						Image:        version.Image,
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

// FindKeyStoneCACert takes a list of keystone URLs and tries to find the first entry
// whose region matches with the given region and returns the CA cert for this region. If no such entry is found then it
// tries to use the non-regional value.
func FindKeyStoneCACert(keyStoneURLs []api.KeyStoneURL, keystoneCABundle *string, region string) *string {
	for _, keyStoneURL := range keyStoneURLs {
		if keyStoneURL.Region == region && keyStoneURL.CACert != nil && len(*keyStoneURL.CACert) > 0 {
			return keyStoneURL.CACert
		}
	}

	return keystoneCABundle
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

		// Check non-constraining floating pools with second priority
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
	// therefore try now if there is a non-constraining floating pool matching.
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
