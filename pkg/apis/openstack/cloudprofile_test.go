// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
				FloatingNetworkID:  pointer.String("floating-network-id"),
				FloatingSubnetID:   pointer.String("floating-subnet-id"),
				FloatingSubnetName: pointer.String("floating-subnet-name"),
				FloatingSubnetTags: pointer.String("floating-subnet-tags"),
				SubnetID:           pointer.String("subnet-id"),
			}
			loadBalancerClassB = LoadBalancerClass{
				Name:               "lbclass-B",
				FloatingNetworkID:  pointer.String("floating-network-id"),
				FloatingSubnetID:   pointer.String("floating-subnet-id"),
				FloatingSubnetName: pointer.String("floating-subnet-name"),
				FloatingSubnetTags: pointer.String("floating-subnet-tags"),
				SubnetID:           pointer.String("subnet-id"),
			}
		})

		It("should return true as LoadBalancerClass are semantically equal with different names", func() {
			Expect(loadBalancerClassA.IsSemanticallyEqual(loadBalancerClassB)).To(BeTrue())
		})

		It("should return true as LoadBalancerClass are semantically equal with different purposes", func() {
			loadBalancerClassA.Purpose = pointer.String("purpose-a")
			loadBalancerClassB.Purpose = pointer.String("purpose-b")
			Expect(loadBalancerClassA.IsSemanticallyEqual(loadBalancerClassB)).To(BeTrue())
		})

		It("should return false as LoadBalancerClass are not semantically due to different floating network ids", func() {
			loadBalancerClassB.FloatingNetworkID = pointer.String("floating-network-id-2")
			Expect(loadBalancerClassA.IsSemanticallyEqual(loadBalancerClassB)).To(BeFalse())
		})

		It("should return false as LoadBalancerClass are not semantically due to different floating subnet ids", func() {
			loadBalancerClassB.FloatingSubnetID = pointer.String("floating-subnet-id-2")
			Expect(loadBalancerClassA.IsSemanticallyEqual(loadBalancerClassB)).To(BeFalse())
		})

		It("should return false as LoadBalancerClass are not semantically due to different floating subnet names", func() {
			loadBalancerClassB.FloatingSubnetName = pointer.String("floating-subnet-name-2")
			Expect(loadBalancerClassA.IsSemanticallyEqual(loadBalancerClassB)).To(BeFalse())
		})

		It("should return false as LoadBalancerClass are not semantically due to different floating subnet tags", func() {
			loadBalancerClassB.FloatingSubnetTags = pointer.String("floating-subnet-tags-2")
			Expect(loadBalancerClassA.IsSemanticallyEqual(loadBalancerClassB)).To(BeFalse())
		})

		It("should return false as LoadBalancerClass are not semantically due to different subnet ids", func() {
			loadBalancerClassB.SubnetID = pointer.String("subnet-ids-2")
			Expect(loadBalancerClassA.IsSemanticallyEqual(loadBalancerClassB)).To(BeFalse())
		})
	})
})
