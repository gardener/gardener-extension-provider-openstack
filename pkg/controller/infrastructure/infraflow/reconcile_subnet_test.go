// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	"context"
	"errors"

	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/subnets"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"k8s.io/utils/ptr"

	openstackapi "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
)

var _ = Describe("ensureSubnet", func() {
	var (
		ctrl *gomock.Controller
		ctx  context.Context
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		ctx = context.Background()
	})

	AfterEach(func() { ctrl.Finish() })

	Context("BYO subnet (networks.subnetId set)", func() {
		It("stores the subnet ID and CIDR in state without creating anything", func() {
			subnetID := "11111111-2222-3333-4444-555555555555"
			networkID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
			cfg := defaultInfraConfig()
			cfg.Networks.Workers = ""
			cfg.Networks.ID = &networkID
			cfg.Networks.SubnetID = &subnetID

			fctx, mockAccess := newFixture(ctrl, cfg)
			fctx.state.Set(IdentifierNetwork, networkID)

			mockAccess.EXPECT().
				GetSubnetByID(ctx, subnetID).
				Return(&subnets.Subnet{ID: subnetID, CIDR: "10.250.0.0/19"}, nil)

			Expect(fctx.ensureSubnet(ctx)).To(Succeed())
			Expect(ptr.Deref(fctx.state.Get(IdentifierSubnet), "")).To(Equal(subnetID))
			Expect(ptr.Deref(fctx.state.Get(IdentifierWorkersCIDR), "")).To(Equal("10.250.0.0/19"))
		})

		It("returns an error when the subnet does not exist", func() {
			subnetID := "11111111-2222-3333-4444-555555555555"
			networkID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
			cfg := defaultInfraConfig()
			cfg.Networks.Workers = ""
			cfg.Networks.ID = &networkID
			cfg.Networks.SubnetID = &subnetID

			fctx, mockAccess := newFixture(ctrl, cfg)
			fctx.state.Set(IdentifierNetwork, networkID)

			mockAccess.EXPECT().
				GetSubnetByID(ctx, subnetID).
				Return(nil, nil)

			Expect(fctx.ensureSubnet(ctx)).NotTo(Succeed())
		})

		It("propagates a lookup error", func() {
			subnetID := "11111111-2222-3333-4444-555555555555"
			networkID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
			cfg := defaultInfraConfig()
			cfg.Networks.Workers = ""
			cfg.Networks.ID = &networkID
			cfg.Networks.SubnetID = &subnetID

			fctx, mockAccess := newFixture(ctrl, cfg)
			fctx.state.Set(IdentifierNetwork, networkID)

			mockAccess.EXPECT().
				GetSubnetByID(ctx, subnetID).
				Return(nil, errors.New("neutron unavailable"))

			Expect(fctx.ensureSubnet(ctx)).NotTo(Succeed())
		})

		It("does not call CreateSubnet or UpdateSubnet", func() {
			subnetID := "11111111-2222-3333-4444-555555555555"
			networkID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
			cfg := defaultInfraConfig()
			cfg.Networks.Workers = ""
			cfg.Networks.ID = &networkID
			cfg.Networks.SubnetID = &subnetID

			fctx, mockAccess := newFixture(ctrl, cfg)
			fctx.state.Set(IdentifierNetwork, networkID)

			mockAccess.EXPECT().
				GetSubnetByID(ctx, subnetID).
				Return(&subnets.Subnet{ID: subnetID, CIDR: "10.250.0.0/19"}, nil)
			// CreateSubnet and UpdateSubnet must NOT be called — gomock will fail if they are.

			Expect(fctx.ensureSubnet(ctx)).To(Succeed())
		})
	})

	Context("Gardener-managed subnet (workers CIDR set)", func() {
		It("creates a subnet when none exists", func() {
			networkID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
			fctx, mockAccess := newFixture(ctrl, defaultInfraConfig())
			fctx.state.Set(IdentifierNetwork, networkID)

			mockAccess.EXPECT().
				GetSubnetByName(ctx, networkID, gomock.Any()).
				Return(nil, nil)
			mockAccess.EXPECT().
				CreateSubnet(ctx, gomock.Any(), nil).
				Return(&subnets.Subnet{ID: "new-subnet-id", CIDR: "10.250.0.0/19"}, nil)

			Expect(fctx.ensureSubnet(ctx)).To(Succeed())
			Expect(ptr.Deref(fctx.state.Get(IdentifierSubnet), "")).To(Equal("new-subnet-id"))
		})

		It("updates an existing subnet found by name", func() {
			networkID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
			fctx, mockAccess := newFixture(ctrl, defaultInfraConfig())
			fctx.state.Set(IdentifierNetwork, networkID)

			existing := &subnets.Subnet{ID: "existing-subnet-id", CIDR: "10.250.0.0/19"}
			mockAccess.EXPECT().
				GetSubnetByName(ctx, networkID, gomock.Any()).
				Return([]*subnets.Subnet{existing}, nil)
			mockAccess.EXPECT().
				UpdateSubnet(ctx, gomock.Any(), existing).
				Return(false, nil)

			Expect(fctx.ensureSubnet(ctx)).To(Succeed())
			Expect(ptr.Deref(fctx.state.Get(IdentifierSubnet), "")).To(Equal("existing-subnet-id"))
		})

		It("returns an error when IdentifierNetwork is not set in state", func() {
			fctx, _ := newFixture(ctrl, defaultInfraConfig())
			Expect(fctx.ensureSubnet(ctx)).NotTo(Succeed())
		})
	})
})

var _ = Describe("ensureSubnetIPv6", func() {
	var (
		ctrl *gomock.Controller
		ctx  context.Context
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		ctx = context.Background()
	})

	AfterEach(func() { ctrl.Finish() })

	Context("BYO IPv6 node subnet (isByoDualStack)", func() {
		It("stores the node subnet ID in state without any API calls", func() {
			ipv6SubnetID := "ffffffff-eeee-dddd-cccc-bbbbbbbbbbbb"
			ipv4SubnetID := "11111111-2222-3333-4444-555555555555"
			networkID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
			cfg := defaultInfraConfig()
			cfg.Networks.Workers = ""
			cfg.Networks.ID = &networkID
			cfg.Networks.SubnetID = &ipv4SubnetID
			cfg.Networks.IPv6 = &openstackapi.IPv6Config{
				NodeSubnetID: &ipv6SubnetID,
				PodCIDR:      "fd00::/56",
				ServiceCIDR:  "fd01::/112",
			}

			fctx, _ := newFixture(ctrl, cfg)
			// no EXPECT calls — no API calls must happen

			Expect(fctx.ensureSubnetIPv6(ctx)).To(Succeed())
			Expect(ptr.Deref(fctx.state.Get(IdentifierSubnetIPv6), "")).To(Equal(ipv6SubnetID))
		})
	})

	Context("Gardener-managed IPv6 subnets (explicit CIDRs)", func() {
		It("creates all three IPv6 subnets (node, pod, service) when none exist", func() {
			networkID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
			cfg := defaultInfraConfig()
			cfg.Networks.Workers = ""
			cfg.Networks.ID = &networkID
			cfg.Networks.IPv6 = &openstackapi.IPv6Config{
				NodeCIDR:    "fd10::/64",
				PodCIDR:     "fd00::/56",
				ServiceCIDR: "fd01::/112",
			}

			fctx, mockAccess := newFixture(ctrl, cfg)
			fctx.state.Set(IdentifierNetwork, networkID)

			mockAccess.EXPECT().
				GetSubnetByName(ctx, networkID, gomock.Any()).
				Return(nil, nil).
				Times(3)
			mockAccess.EXPECT().
				CreateSubnet(ctx, gomock.Any(), nil).
				DoAndReturn(func(_ context.Context, desired *subnets.Subnet, _ *int) (*subnets.Subnet, error) {
					return &subnets.Subnet{ID: "created-" + desired.Name, CIDR: desired.CIDR}, nil
				}).
				Times(3)

			Expect(fctx.ensureSubnetIPv6(ctx)).To(Succeed())
			Expect(fctx.state.Get(IdentifierSubnetIPv6)).NotTo(BeNil())
			Expect(fctx.state.Get(IdentifierSubnetIPv6Pod)).NotTo(BeNil())
			Expect(fctx.state.Get(IdentifierSubnetIPv6Svc)).NotTo(BeNil())
		})

		It("updates all three existing IPv6 subnets found by name", func() {
			networkID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
			cfg := defaultInfraConfig()
			cfg.Networks.Workers = ""
			cfg.Networks.ID = &networkID
			cfg.Networks.IPv6 = &openstackapi.IPv6Config{
				NodeCIDR:    "fd10::/64",
				PodCIDR:     "fd00::/56",
				ServiceCIDR: "fd01::/112",
			}

			fctx, mockAccess := newFixture(ctrl, cfg)
			fctx.state.Set(IdentifierNetwork, networkID)

			mockAccess.EXPECT().
				GetSubnetByName(ctx, networkID, gomock.Any()).
				DoAndReturn(func(_ context.Context, _ string, name string) ([]*subnets.Subnet, error) {
					return []*subnets.Subnet{{ID: "existing-" + name, CIDR: "fd00::/56"}}, nil
				}).
				Times(3)
			mockAccess.EXPECT().
				UpdateSubnet(ctx, gomock.Any(), gomock.Any()).
				Return(false, nil).
				Times(3)

			Expect(fctx.ensureSubnetIPv6(ctx)).To(Succeed())
			Expect(fctx.state.Get(IdentifierSubnetIPv6)).NotTo(BeNil())
		})
	})
})

var _ = Describe("ensureIPv6CIDRs", func() {
	var (
		ctrl *gomock.Controller
		ctx  context.Context
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		ctx = context.Background()
	})

	AfterEach(func() { ctrl.Finish() })

	Context("BYO IPv6 node subnet", func() {
		It("reads the node CIDR from the existing subnet and stores pod/service CIDRs from config", func() {
			ipv6SubnetID := "ffffffff-eeee-dddd-cccc-bbbbbbbbbbbb"
			ipv4SubnetID := "11111111-2222-3333-4444-555555555555"
			networkID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
			cfg := defaultInfraConfig()
			cfg.Networks.Workers = ""
			cfg.Networks.ID = &networkID
			cfg.Networks.SubnetID = &ipv4SubnetID
			cfg.Networks.IPv6 = &openstackapi.IPv6Config{
				NodeSubnetID: &ipv6SubnetID,
				PodCIDR:      "fd00::/56",
				ServiceCIDR:  "fd01::/112",
			}

			fctx, mockAccess := newFixture(ctrl, cfg)
			fctx.state.Set(IdentifierSubnetIPv6, ipv6SubnetID)

			mockAccess.EXPECT().
				GetSubnetByID(ctx, ipv6SubnetID).
				Return(&subnets.Subnet{ID: ipv6SubnetID, CIDR: "fd10::/64"}, nil)

			Expect(fctx.ensureIPv6CIDRs(ctx)).To(Succeed())
			Expect(ptr.Deref(fctx.state.Get(IdentifierNodeSubnetIPv6CIDR), "")).To(Equal("fd10::/64"))
			Expect(ptr.Deref(fctx.state.Get(IdentifierPodSubnetIPv6CIDR), "")).To(Equal("fd00::/56"))
			// service CIDR is normalised to /112 based on the network address
			Expect(ptr.Deref(fctx.state.Get(IdentifierServiceSubnetIPv6CIDR), "")).To(HaveSuffix("/112"))
		})

		It("returns an error when the IPv6 subnet does not exist", func() {
			ipv6SubnetID := "ffffffff-eeee-dddd-cccc-bbbbbbbbbbbb"
			ipv4SubnetID := "11111111-2222-3333-4444-555555555555"
			networkID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
			cfg := defaultInfraConfig()
			cfg.Networks.Workers = ""
			cfg.Networks.ID = &networkID
			cfg.Networks.SubnetID = &ipv4SubnetID
			cfg.Networks.IPv6 = &openstackapi.IPv6Config{
				NodeSubnetID: &ipv6SubnetID,
				PodCIDR:      "fd00::/56",
				ServiceCIDR:  "fd01::/112",
			}

			fctx, mockAccess := newFixture(ctrl, cfg)
			fctx.state.Set(IdentifierSubnetIPv6, ipv6SubnetID)

			mockAccess.EXPECT().
				GetSubnetByID(ctx, ipv6SubnetID).
				Return(nil, nil)

			Expect(fctx.ensureIPv6CIDRs(ctx)).NotTo(Succeed())
		})
	})

	Context("explicit IPv6 CIDRs (no BYO subnet)", func() {
		It("stores all three CIDRs in state without any API calls", func() {
			networkID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
			cfg := defaultInfraConfig()
			cfg.Networks.IPv6 = &openstackapi.IPv6Config{
				NodeCIDR:    "fd10::/64",
				PodCIDR:     "fd00::/56",
				ServiceCIDR: "fd01::/112",
			}

			fctx, _ := newFixture(ctrl, cfg)
			fctx.state.Set(IdentifierNetwork, networkID)
			// no EXPECT calls — GetSubnetByID must NOT be called

			Expect(fctx.ensureIPv6CIDRs(ctx)).To(Succeed())
			Expect(ptr.Deref(fctx.state.Get(IdentifierNodeSubnetIPv6CIDR), "")).To(Equal("fd10::/64"))
			Expect(ptr.Deref(fctx.state.Get(IdentifierPodSubnetIPv6CIDR), "")).To(Equal("fd00::/56"))
			Expect(ptr.Deref(fctx.state.Get(IdentifierServiceSubnetIPv6CIDR), "")).To(HaveSuffix("/112"))
		})
	})
})
