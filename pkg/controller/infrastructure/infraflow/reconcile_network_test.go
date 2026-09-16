// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	"context"
	"errors"

	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/networks"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/controller/infrastructure/infraflow/access"
	clientmocks "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client/mocks"
)

var _ = Describe("ensureNetwork", func() {
	var (
		ctrl *gomock.Controller
		ctx  context.Context
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		ctx = context.Background()
	})

	AfterEach(func() { ctrl.Finish() })

	Context("BYO network (networks.id set)", func() {
		It("stores the network ID and name in state when network is found", func() {
			networkID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
			cfg := defaultInfraConfig()
			cfg.Networks.ID = &networkID

			fctx, mockAccess := newFixture(ctrl, cfg)
			mockAccess.EXPECT().
				GetNetworkByID(ctx, networkID).
				Return(&access.Network{ID: networkID, Name: "my-network"}, nil)

			Expect(fctx.ensureNetwork(ctx)).To(Succeed())
			Expect(ptr.Deref(fctx.state.Get(IdentifierNetwork), "")).To(Equal(networkID))
			Expect(ptr.Deref(fctx.state.Get(NameNetwork), "")).To(Equal("my-network"))
		})

		It("returns an error when the network does not exist", func() {
			networkID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
			cfg := defaultInfraConfig()
			cfg.Networks.ID = &networkID

			fctx, mockAccess := newFixture(ctrl, cfg)
			mockAccess.EXPECT().
				GetNetworkByID(ctx, networkID).
				Return(nil, nil)

			Expect(fctx.ensureNetwork(ctx)).NotTo(Succeed())
		})

		It("propagates a lookup error", func() {
			networkID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
			cfg := defaultInfraConfig()
			cfg.Networks.ID = &networkID

			fctx, mockAccess := newFixture(ctrl, cfg)
			mockAccess.EXPECT().
				GetNetworkByID(ctx, networkID).
				Return(nil, errors.New("neutron unavailable"))

			Expect(fctx.ensureNetwork(ctx)).NotTo(Succeed())
		})
	})

	Context("Gardener-managed network (no networks.id)", func() {
		It("creates a new network when none exists", func() {
			fctx, mockAccess := newFixture(ctrl, defaultInfraConfig())
			mockAccess.EXPECT().
				GetNetworkByName(ctx, gomock.Any()).
				Return(nil, nil)
			mockAccess.EXPECT().
				CreateNetwork(ctx, gomock.Any()).
				Return(&access.Network{ID: "new-net-id", Name: "new-net"}, nil)

			Expect(fctx.ensureNetwork(ctx)).To(Succeed())
			Expect(ptr.Deref(fctx.state.Get(IdentifierNetwork), "")).To(Equal("new-net-id"))
		})

		It("updates and reuses an existing network found by name", func() {
			fctx, mockAccess := newFixture(ctrl, defaultInfraConfig())
			existing := &access.Network{ID: "existing-net-id", Name: "existing-net"}
			mockAccess.EXPECT().
				GetNetworkByName(ctx, gomock.Any()).
				Return([]*access.Network{existing}, nil)
			mockAccess.EXPECT().
				UpdateNetwork(ctx, gomock.Any(), existing).
				Return(false, nil)

			Expect(fctx.ensureNetwork(ctx)).To(Succeed())
			Expect(ptr.Deref(fctx.state.Get(IdentifierNetwork), "")).To(Equal("existing-net-id"))
		})
	})
})

var _ = Describe("ensureExternalNetwork", func() {
	var (
		ctrl *gomock.Controller
		ctx  context.Context
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		ctx = context.Background()
	})

	AfterEach(func() { ctrl.Finish() })

	It("stores the floating network ID and name in state when found", func() {
		fctx, _ := newFixture(ctrl, defaultInfraConfig())
		mockNetworking := clientmocks.NewMockNetworking(ctrl)
		fctx.networking = mockNetworking

		mockNetworking.EXPECT().
			GetExternalNetworkByName(ctx, "my-pool").
			Return(&networks.Network{ID: "ext-net-id", Name: "my-pool"}, nil)

		Expect(fctx.ensureExternalNetwork(ctx)).To(Succeed())
		Expect(ptr.Deref(fctx.state.Get(IdentifierFloatingNetwork), "")).To(Equal("ext-net-id"))
		Expect(ptr.Deref(fctx.state.Get(NameFloatingNetwork), "")).To(Equal("my-pool"))
	})

	It("returns an error when the external network is not found", func() {
		fctx, _ := newFixture(ctrl, defaultInfraConfig())
		mockNetworking := clientmocks.NewMockNetworking(ctrl)
		fctx.networking = mockNetworking

		mockNetworking.EXPECT().
			GetExternalNetworkByName(ctx, "my-pool").
			Return(nil, nil)

		Expect(fctx.ensureExternalNetwork(ctx)).NotTo(Succeed())
	})

	It("propagates a lookup error", func() {
		fctx, _ := newFixture(ctrl, defaultInfraConfig())
		mockNetworking := clientmocks.NewMockNetworking(ctrl)
		fctx.networking = mockNetworking

		mockNetworking.EXPECT().
			GetExternalNetworkByName(ctx, "my-pool").
			Return(nil, errors.New("neutron unavailable"))

		Expect(fctx.ensureExternalNetwork(ctx)).NotTo(Succeed())
	})
})
