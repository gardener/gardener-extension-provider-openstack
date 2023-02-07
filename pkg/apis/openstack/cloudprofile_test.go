// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package openstack_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	. "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
)

var _ = Describe("LoadBalancerClass", func() {
	Context("#IsSemanticallyEqual", func() {
		var (
			loadBalancerClassA LoadBalancerClass
			loadBalancerClassB LoadBalancerClass
		)

		BeforeEach(func() {
			loadBalancerClassA = LoadBalancerClass{
				Name:               "lbclass-A",
				FloatingNetworkID:  pointer.StringPtr("floating-network-id"),
				FloatingSubnetID:   pointer.StringPtr("floating-subnet-id"),
				FloatingSubnetName: pointer.StringPtr("floating-subnet-name"),
				FloatingSubnetTags: pointer.StringPtr("floating-subnet-tags"),
				SubnetID:           pointer.StringPtr("subnet-id"),
			}
			loadBalancerClassB = LoadBalancerClass{
				Name:               "lbclass-B",
				FloatingNetworkID:  pointer.StringPtr("floating-network-id"),
				FloatingSubnetID:   pointer.StringPtr("floating-subnet-id"),
				FloatingSubnetName: pointer.StringPtr("floating-subnet-name"),
				FloatingSubnetTags: pointer.StringPtr("floating-subnet-tags"),
				SubnetID:           pointer.StringPtr("subnet-id"),
			}
		})

		It("should return true as LoadBalancerClass are semantically equal with different names", func() {
			Expect(loadBalancerClassA.IsSemanticallyEqual(loadBalancerClassB)).To(BeTrue())
		})

		It("should return true as LoadBalancerClass are semantically equal with different purposes", func() {
			loadBalancerClassA.Purpose = pointer.StringPtr("purpose-a")
			loadBalancerClassB.Purpose = pointer.StringPtr("purpose-b")
			Expect(loadBalancerClassA.IsSemanticallyEqual(loadBalancerClassB)).To(BeTrue())
		})

		It("should return false as LoadBalancerClass are not semantically due to different floating network ids", func() {
			loadBalancerClassB.FloatingNetworkID = pointer.StringPtr("floating-network-id-2")
			Expect(loadBalancerClassA.IsSemanticallyEqual(loadBalancerClassB)).To(BeFalse())
		})

		It("should return false as LoadBalancerClass are not semantically due to different floating subnet ids", func() {
			loadBalancerClassB.FloatingSubnetID = pointer.StringPtr("floating-subnet-id-2")
			Expect(loadBalancerClassA.IsSemanticallyEqual(loadBalancerClassB)).To(BeFalse())
		})

		It("should return false as LoadBalancerClass are not semantically due to different floating subnet names", func() {
			loadBalancerClassB.FloatingSubnetName = pointer.StringPtr("floating-subnet-name-2")
			Expect(loadBalancerClassA.IsSemanticallyEqual(loadBalancerClassB)).To(BeFalse())
		})

		It("should return false as LoadBalancerClass are not semantically due to different floating subnet tags", func() {
			loadBalancerClassB.FloatingSubnetTags = pointer.StringPtr("floating-subnet-tags-2")
			Expect(loadBalancerClassA.IsSemanticallyEqual(loadBalancerClassB)).To(BeFalse())
		})

		It("should return false as LoadBalancerClass are not semantically due to different subnet ids", func() {
			loadBalancerClassB.SubnetID = pointer.StringPtr("subnet-ids-2")
			Expect(loadBalancerClassA.IsSemanticallyEqual(loadBalancerClassB)).To(BeFalse())
		})
	})
})
