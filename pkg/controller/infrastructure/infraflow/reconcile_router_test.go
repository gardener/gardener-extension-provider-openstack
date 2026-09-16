// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	"context"

	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/layer3/routers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"k8s.io/utils/ptr"

	openstackapi "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/controller/infrastructure/infraflow/access"
)

var _ = Describe("ensureConfiguredRouter", func() {
	var (
		ctrl *gomock.Controller
		ctx  context.Context
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		ctx = context.Background()
	})

	AfterEach(func() { ctrl.Finish() })

	It("stores router ID and egress CIDRs from an existing BYO router", func() {
		routerID := "rrrrrrrr-rrrr-rrrr-rrrr-rrrrrrrrrrrr"
		cfg := defaultInfraConfig()
		cfg.Networks.Router = &openstackapi.Router{ID: routerID}

		fctx, mockAccess := newFixture(ctrl, cfg)
		fctx.state.SetObject(IdentifierEgressCIDRs, []string{})

		mockAccess.EXPECT().
			GetRouterByID(ctx, routerID).
			Return(&access.Router{
				ID:                routerID,
				ExternalFixedIPs:  []routers.ExternalFixedIP{{IPAddress: "1.2.3.4"}},
				ExternalNetworkID: "ext-net",
			}, nil)

		Expect(fctx.ensureConfiguredRouter(ctx)).To(Succeed())
		Expect(ptr.Deref(fctx.state.Get(IdentifierRouter), "")).To(Equal(routerID))
		cidrs := fctx.state.GetObject(IdentifierEgressCIDRs)
		Expect(cidrs).NotTo(BeNil())
		Expect(cidrs.([]string)).To(ContainElement("1.2.3.4"))
	})

	It("returns an error when the router does not exist", func() {
		routerID := "rrrrrrrr-rrrr-rrrr-rrrr-rrrrrrrrrrrr"
		cfg := defaultInfraConfig()
		cfg.Networks.Router = &openstackapi.Router{ID: routerID}

		fctx, mockAccess := newFixture(ctrl, cfg)
		mockAccess.EXPECT().
			GetRouterByID(ctx, routerID).
			Return(nil, nil)

		Expect(fctx.ensureConfiguredRouter(ctx)).NotTo(Succeed())
	})
})

var _ = Describe("ensureRouterInterface", func() {
	var (
		ctrl *gomock.Controller
		ctx  context.Context
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		ctx = context.Background()
	})

	AfterEach(func() { ctrl.Finish() })

	It("skips AddRouterInterfaceAndWait when the interface already exists", func() {
		routerID := "rrrrrrrr-rrrr-rrrr-rrrr-rrrrrrrrrrrr"
		subnetID := "11111111-2222-3333-4444-555555555555"

		fctx, mockAccess := newFixture(ctrl, defaultInfraConfig())
		fctx.state.Set(IdentifierRouter, routerID)
		fctx.state.Set(IdentifierSubnet, subnetID)

		portID := "port-id"
		mockAccess.EXPECT().
			GetRouterInterfacePortID(ctx, routerID, subnetID).
			Return(&portID, nil)
		// AddRouterInterfaceAndWait must NOT be called

		Expect(fctx.ensureRouterInterface(ctx)).To(Succeed())
	})

	It("calls AddRouterInterfaceAndWait when the interface does not exist", func() {
		routerID := "rrrrrrrr-rrrr-rrrr-rrrr-rrrrrrrrrrrr"
		subnetID := "11111111-2222-3333-4444-555555555555"

		fctx, mockAccess := newFixture(ctrl, defaultInfraConfig())
		fctx.state.Set(IdentifierRouter, routerID)
		fctx.state.Set(IdentifierSubnet, subnetID)

		mockAccess.EXPECT().
			GetRouterInterfacePortID(ctx, routerID, subnetID).
			Return(nil, nil)
		mockAccess.EXPECT().
			AddRouterInterfaceAndWait(ctx, routerID, subnetID).
			Return(nil)

		Expect(fctx.ensureRouterInterface(ctx)).To(Succeed())
	})

	It("returns an error when routerID is not in state", func() {
		fctx, _ := newFixture(ctrl, defaultInfraConfig())
		fctx.state.Set(IdentifierSubnet, "11111111-2222-3333-4444-555555555555")
		Expect(fctx.ensureRouterInterface(ctx)).NotTo(Succeed())
	})
})

var _ = Describe("ensureRouterInterfaceIPv6", func() {
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
		It("skips all API calls (router interface must pre-exist)", func() {
			ipv6SubnetID := "ffffffff-eeee-dddd-cccc-bbbbbbbbbbbb"
			ipv4SubnetID := "11111111-2222-3333-4444-555555555555"
			networkID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
			cfg := defaultInfraConfig()
			cfg.Networks.Workers = ""
			cfg.Networks.ID = &networkID
			cfg.Networks.SubnetID = &ipv4SubnetID
			cfg.Networks.IPv6 = &openstackapi.IPv6Config{NodeSubnetID: &ipv6SubnetID}

			fctx, _ := newFixture(ctrl, cfg)
			fctx.state.Set(IdentifierRouter, "rrrrrrrr-rrrr-rrrr-rrrr-rrrrrrrrrrrr")
			fctx.state.Set(IdentifierSubnetIPv6, ipv6SubnetID)
			// no EXPECT calls at all

			Expect(fctx.ensureRouterInterfaceIPv6(ctx)).To(Succeed())
		})
	})

	Context("Gardener-managed IPv6 subnet", func() {
		It("creates the interface when it does not exist", func() {
			routerID := "rrrrrrrr-rrrr-rrrr-rrrr-rrrrrrrrrrrr"
			ipv6SubnetID := "ffffffff-eeee-dddd-cccc-bbbbbbbbbbbb"
			cfg := defaultInfraConfig()
			cfg.Networks.IPv6 = &openstackapi.IPv6Config{NodeCIDR: "fd10::/64"}

			fctx, mockAccess := newFixture(ctrl, cfg)
			fctx.state.Set(IdentifierRouter, routerID)
			fctx.state.Set(IdentifierSubnetIPv6, ipv6SubnetID)

			mockAccess.EXPECT().
				GetRouterInterfacePortID(ctx, routerID, ipv6SubnetID).
				Return(nil, nil)
			mockAccess.EXPECT().
				AddRouterInterfaceAndWait(ctx, routerID, ipv6SubnetID).
				Return(nil)

			Expect(fctx.ensureRouterInterfaceIPv6(ctx)).To(Succeed())
		})
	})
})

var _ = Describe("ensureNewRouter", func() {
	var (
		ctrl *gomock.Controller
		ctx  context.Context
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		ctx = context.Background()
	})

	AfterEach(func() { ctrl.Finish() })

	It("creates a router when none exists", func() {
		extNetID := "ext-net-id"
		fctx, mockAccess := newFixture(ctrl, defaultInfraConfig())
		fctx.state.Set(IdentifierFloatingNetwork, extNetID)

		mockAccess.EXPECT().
			GetRouterByName(ctx, gomock.Any()).
			Return(nil, nil)
		mockAccess.EXPECT().
			CreateRouter(ctx, gomock.Any()).
			Return(&access.Router{
				ID:               "new-router-id",
				ExternalFixedIPs: []routers.ExternalFixedIP{{IPAddress: "1.2.3.4"}},
			}, nil)

		Expect(fctx.ensureNewRouter(ctx, extNetID)).To(Succeed())
		Expect(ptr.Deref(fctx.state.Get(IdentifierRouter), "")).To(Equal("new-router-id"))
	})

	It("updates and reuses an existing router found by name", func() {
		extNetID := "ext-net-id"
		fctx, mockAccess := newFixture(ctrl, defaultInfraConfig())

		existing := &access.Router{
			ID:               "existing-router-id",
			ExternalFixedIPs: []routers.ExternalFixedIP{{IPAddress: "5.6.7.8"}},
		}
		mockAccess.EXPECT().
			GetRouterByName(ctx, gomock.Any()).
			Return([]*access.Router{existing}, nil)
		mockAccess.EXPECT().
			UpdateRouter(ctx, gomock.Any(), existing).
			Return(false, existing, nil)

		Expect(fctx.ensureNewRouter(ctx, extNetID)).To(Succeed())
		Expect(ptr.Deref(fctx.state.Get(IdentifierRouter), "")).To(Equal("existing-router-id"))
	})
})
