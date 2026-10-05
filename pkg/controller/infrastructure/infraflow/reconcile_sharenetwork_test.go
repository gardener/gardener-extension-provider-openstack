// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	"context"

	"github.com/gophercloud/gophercloud/v2/openstack/sharedfilesystems/v2/sharenetworks"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"k8s.io/utils/ptr"

	openstackapi "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	clientmocks "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client/mocks"
)

var _ = Describe("ensureShareNetwork", func() {
	var (
		ctrl *gomock.Controller
		ctx  context.Context
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		ctx = context.Background()
	})

	AfterEach(func() { ctrl.Finish() })

	Context("ShareNetwork disabled or nil", func() {
		It("does nothing when ShareNetwork config is nil", func() {
			cfg := defaultInfraConfig()
			// cfg.Networks.ShareNetwork is nil by default
			fctx, _ := newFixture(ctrl, cfg)
			// No API calls expected

			Expect(fctx.ensureShareNetwork(ctx)).To(Succeed())
		})

		It("does nothing when ShareNetwork.Enabled is false", func() {
			cfg := defaultInfraConfig()
			cfg.Networks.ShareNetwork = &openstackapi.ShareNetwork{Enabled: false}
			fctx, _ := newFixture(ctrl, cfg)
			// No API calls expected

			Expect(fctx.ensureShareNetwork(ctx)).To(Succeed())
		})
	})

	Context("Gardener-managed share network (ShareNetwork.Enabled = true)", func() {
		It("creates a share network when none exists", func() {
			cfg := defaultInfraConfig()
			cfg.Networks.ShareNetwork = &openstackapi.ShareNetwork{Enabled: true}

			fctx, _ := newFixture(ctrl, cfg)
			fctx.state.Set(IdentifierNetwork, "net-id")
			fctx.state.Set(IdentifierSubnet, "subnet-id")

			mockFactory := clientmocks.NewMockFactory(ctrl)
			mockSharedFS := clientmocks.NewMockSharedFilesystem(ctrl)
			fctx.openstackClientFactory = mockFactory

			mockFactory.EXPECT().
				SharedFilesystem(gomock.Any()).
				Return(mockSharedFS, nil)
			mockSharedFS.EXPECT().
				ListShareNetworks(ctx, gomock.Any()).
				Return(nil, nil)
			mockSharedFS.EXPECT().
				CreateShareNetwork(ctx, gomock.Any()).
				Return(&sharenetworks.ShareNetwork{ID: "new-sn-id", Name: "new-sn"}, nil)

			Expect(fctx.ensureShareNetwork(ctx)).To(Succeed())
			Expect(ptr.Deref(fctx.state.Get(IdentifierShareNetwork), "")).To(Equal("new-sn-id"))
			Expect(ptr.Deref(fctx.state.Get(NameShareNetwork), "")).To(Equal("new-sn"))
		})

		It("reuses an existing share network found by name", func() {
			cfg := defaultInfraConfig()
			cfg.Networks.ShareNetwork = &openstackapi.ShareNetwork{Enabled: true}

			fctx, _ := newFixture(ctrl, cfg)
			fctx.state.Set(IdentifierNetwork, "net-id")
			fctx.state.Set(IdentifierSubnet, "subnet-id")

			mockFactory := clientmocks.NewMockFactory(ctrl)
			mockSharedFS := clientmocks.NewMockSharedFilesystem(ctrl)
			fctx.openstackClientFactory = mockFactory

			existing := sharenetworks.ShareNetwork{ID: "existing-sn-id", Name: "existing-sn"}
			mockFactory.EXPECT().
				SharedFilesystem(gomock.Any()).
				Return(mockSharedFS, nil)
			mockSharedFS.EXPECT().
				ListShareNetworks(ctx, gomock.Any()).
				Return([]sharenetworks.ShareNetwork{existing}, nil)
			// CreateShareNetwork must NOT be called

			Expect(fctx.ensureShareNetwork(ctx)).To(Succeed())
			Expect(ptr.Deref(fctx.state.Get(IdentifierShareNetwork), "")).To(Equal("existing-sn-id"))
		})
	})
})
