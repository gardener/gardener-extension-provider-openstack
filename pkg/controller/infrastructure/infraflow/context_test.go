// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	openstackv1alpha1 "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"
)

var _ = Describe("computeInfrastructureStatus", func() {
	const (
		nodesSubnetID = "nodes-ipv6"
		nodesCIDR     = "2001:db8:1::/64"
	)

	It("reports the IPv6 nodes CIDR from state alongside the configured IPv4 CIDR", func() {
		ctrl := gomock.NewController(GinkgoT())
		defer ctrl.Finish()
		cfg := defaultInfraConfig()
		fctx, _ := newFixture(ctrl, cfg)
		fctx.state.SetObject(IdentifierEgressCIDRs, []string{})
		fctx.state.Set(IdentifierSubnet, "nodes-ipv4")
		fctx.state.Set(IdentifierSubnetIPv6, nodesSubnetID)
		fctx.state.Set(IdentifierNodeSubnetIPv6CIDR, nodesCIDR)

		Expect(fctx.computeInfrastructureStatus().Networks.Subnets).To(ConsistOf(
			openstackv1alpha1.Subnet{Purpose: openstackv1alpha1.PurposeNodesIPv6, ID: nodesSubnetID, CIDR: nodesCIDR},
			openstackv1alpha1.Subnet{Purpose: openstackv1alpha1.PurposeNodes, ID: "nodes-ipv4", CIDR: cfg.Networks.Workers},
		))
	})

	It("does not invent an IPv6 CIDR when state is missing", func() {
		ctrl := gomock.NewController(GinkgoT())
		defer ctrl.Finish()
		fctx, _ := newFixture(ctrl, defaultInfraConfig())
		fctx.state.SetObject(IdentifierEgressCIDRs, []string{})
		fctx.state.Set(IdentifierSubnetIPv6, nodesSubnetID)

		Expect(fctx.computeInfrastructureStatus().Networks.Subnets).To(ConsistOf(
			openstackv1alpha1.Subnet{Purpose: openstackv1alpha1.PurposeNodesIPv6, ID: nodesSubnetID},
		))
	})

	It("preserves the allocated IPv4 nodes CIDR", func() {
		ctrl := gomock.NewController(GinkgoT())
		defer ctrl.Finish()
		fctx, _ := newFixture(ctrl, defaultInfraConfig())
		fctx.state.SetObject(IdentifierEgressCIDRs, []string{})
		fctx.state.Set(IdentifierSubnet, "nodes-ipv4")
		fctx.state.Set(IdentifierWorkersCIDR, "10.20.0.0/24")

		Expect(fctx.computeInfrastructureStatus().Networks.Subnets).To(ConsistOf(
			openstackv1alpha1.Subnet{Purpose: openstackv1alpha1.PurposeNodes, ID: "nodes-ipv4", CIDR: "10.20.0.0/24"},
		))
	})
})
