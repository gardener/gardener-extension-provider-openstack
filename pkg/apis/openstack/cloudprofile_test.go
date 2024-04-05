// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package openstack_test

import (
	. "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
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
				FloatingNetworkID:  ptr.To("floating-network-id"),
				FloatingSubnetID:   ptr.To("floating-subnet-id"),
				FloatingSubnetName: ptr.To("floating-subnet-name"),
				FloatingSubnetTags: ptr.To("floating-subnet-tags"),
				SubnetID:           ptr.To("subnet-id"),
			}
			loadBalancerClassB = LoadBalancerClass{
				Name:               "lbclass-B",
				FloatingNetworkID:  ptr.To("floating-network-id"),
				FloatingSubnetID:   ptr.To("floating-subnet-id"),
				FloatingSubnetName: ptr.To("floating-subnet-name"),
				FloatingSubnetTags: ptr.To("floating-subnet-tags"),
				SubnetID:           ptr.To("subnet-id"),
			}
		})

		It("should return true as LoadBalancerClass are semantically equal with different names", func() {
			Expect(loadBalancerClassA.IsSemanticallyEqual(loadBalancerClassB)).To(BeTrue())
		})

		It("should return true as LoadBalancerClass are semantically equal with different purposes", func() {
			loadBalancerClassA.Purpose = ptr.To("purpose-a")
			loadBalancerClassB.Purpose = ptr.To("purpose-b")
			Expect(loadBalancerClassA.IsSemanticallyEqual(loadBalancerClassB)).To(BeTrue())
		})

		It("should return false as LoadBalancerClass are not semantically due to different floating network ids", func() {
			loadBalancerClassB.FloatingNetworkID = ptr.To("floating-network-id-2")
			Expect(loadBalancerClassA.IsSemanticallyEqual(loadBalancerClassB)).To(BeFalse())
		})

		It("should return false as LoadBalancerClass are not semantically due to different floating subnet ids", func() {
			loadBalancerClassB.FloatingSubnetID = ptr.To("floating-subnet-id-2")
			Expect(loadBalancerClassA.IsSemanticallyEqual(loadBalancerClassB)).To(BeFalse())
		})

		It("should return false as LoadBalancerClass are not semantically due to different floating subnet names", func() {
			loadBalancerClassB.FloatingSubnetName = ptr.To("floating-subnet-name-2")
			Expect(loadBalancerClassA.IsSemanticallyEqual(loadBalancerClassB)).To(BeFalse())
		})

		It("should return false as LoadBalancerClass are not semantically due to different floating subnet tags", func() {
			loadBalancerClassB.FloatingSubnetTags = ptr.To("floating-subnet-tags-2")
			Expect(loadBalancerClassA.IsSemanticallyEqual(loadBalancerClassB)).To(BeFalse())
		})

		It("should return false as LoadBalancerClass are not semantically due to different subnet ids", func() {
			loadBalancerClassB.SubnetID = ptr.To("subnet-ids-2")
			Expect(loadBalancerClassA.IsSemanticallyEqual(loadBalancerClassB)).To(BeFalse())
		})
	})
})
