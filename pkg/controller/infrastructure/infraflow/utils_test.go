// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	"context"
	"errors"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/layer3/routers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"k8s.io/utils/ptr"

	clientmocks "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client/mocks"
)

var _ = Describe("filterDNSServersByIPFamily", func() {
	DescribeTable("filters DNS servers by IP family",
		func(servers []string, family gardencorev1beta1.IPFamily, expected []string) {
			Expect(filterDNSServersByIPFamily(servers, family)).To(Equal(expected))
		},
		Entry("returns only IPv4 addresses when family is IPv4",
			[]string{"8.8.8.8", "2001:4860:4860::8888", "1.1.1.1"},
			gardencorev1beta1.IPFamilyIPv4,
			[]string{"8.8.8.8", "1.1.1.1"},
		),
		Entry("returns only IPv6 addresses when family is IPv6",
			[]string{"8.8.8.8", "2001:4860:4860::8888", "2606:4700:4700::1111"},
			gardencorev1beta1.IPFamilyIPv6,
			[]string{"2001:4860:4860::8888", "2606:4700:4700::1111"},
		),
		Entry("returns nil when no addresses match IPv4 family",
			[]string{"2001:4860:4860::8888"},
			gardencorev1beta1.IPFamilyIPv4,
			nil,
		),
		Entry("returns nil when no addresses match IPv6 family",
			[]string{"8.8.8.8", "1.1.1.1"},
			gardencorev1beta1.IPFamilyIPv6,
			nil,
		),
		Entry("returns nil for empty input",
			[]string{},
			gardencorev1beta1.IPFamilyIPv4,
			nil,
		),
		Entry("returns nil for nil input",
			nil,
			gardencorev1beta1.IPFamilyIPv4,
			nil,
		),
		Entry("handles all-IPv4 input with IPv4 family",
			[]string{"8.8.8.8", "1.1.1.1"},
			gardencorev1beta1.IPFamilyIPv4,
			[]string{"8.8.8.8", "1.1.1.1"},
		),
		Entry("handles all-IPv6 input with IPv6 family",
			[]string{"2001:4860:4860::8888", "2606:4700:4700::1111"},
			gardencorev1beta1.IPFamilyIPv6,
			[]string{"2001:4860:4860::8888", "2606:4700:4700::1111"},
		),
	)
})

var _ = Describe("cleanupKubernetesRoutes", func() {
	const (
		routerID = "rrrrrrrr-rrrr-rrrr-rrrr-rrrrrrrrrrrr"
		podCIDR  = "100.96.0.0/16"
	)

	var (
		ctrl           *gomock.Controller
		ctx            context.Context
		mockNetworking *clientmocks.MockNetworking
		fctx           *FlowContext
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		ctx = context.Background()
		fctx, _ = newFixture(ctrl, defaultInfraConfig())
		mockNetworking = clientmocks.NewMockNetworking(ctrl)
		fctx.networking = mockNetworking
		fctx.shootNetworking = &gardencorev1beta1.Networking{
			Pods: ptr.To(podCIDR),
		}
	})

	AfterEach(func() { ctrl.Finish() })

	It("removes only routes whose destination falls within the pod CIDR", func() {
		// Two CCM routes (destinations inside pod CIDR) and one operator static
		// route (destination outside pod CIDR). Only the CCM routes must be removed.
		routerRoutes := []routers.Route{
			{DestinationCIDR: "100.96.1.0/24", NextHop: "10.250.0.5"},    // CCM — node 1
			{DestinationCIDR: "100.96.2.0/24", NextHop: "10.250.0.6"},    // CCM — node 2
			{DestinationCIDR: "192.168.100.0/24", NextHop: "10.250.0.1"}, // operator static
		}
		mockNetworking.EXPECT().
			GetRouterByID(ctx, routerID).
			Return(&routers.Router{Routes: routerRoutes}, nil)
		mockNetworking.EXPECT().
			UpdateRoutesForRouter(ctx, []routers.Route{
				{DestinationCIDR: "192.168.100.0/24", NextHop: "10.250.0.1"},
			}, routerID).
			Return(&routers.Router{}, nil)

		Expect(fctx.cleanupKubernetesRoutes(ctx, routerID)).To(Succeed())
	})

	It("does not call UpdateRoutesForRouter when no routes match the pod CIDR", func() {
		routerRoutes := []routers.Route{
			{DestinationCIDR: "192.168.100.0/24", NextHop: "10.250.0.1"},
		}
		mockNetworking.EXPECT().
			GetRouterByID(ctx, routerID).
			Return(&routers.Router{Routes: routerRoutes}, nil)
		// UpdateRoutesForRouter must NOT be called

		Expect(fctx.cleanupKubernetesRoutes(ctx, routerID)).To(Succeed())
	})

	It("does not call UpdateRoutesForRouter when the router has no routes", func() {
		mockNetworking.EXPECT().
			GetRouterByID(ctx, routerID).
			Return(&routers.Router{}, nil)

		Expect(fctx.cleanupKubernetesRoutes(ctx, routerID)).To(Succeed())
	})

	It("returns early without any API call when shootNetworking has no pod CIDR", func() {
		fctx.shootNetworking = &gardencorev1beta1.Networking{Pods: nil}
		// neither GetRouterByID nor UpdateRoutesForRouter must be called

		Expect(fctx.cleanupKubernetesRoutes(ctx, routerID)).To(Succeed())
	})

	It("returns early without any API call when shootNetworking is nil", func() {
		fctx.shootNetworking = nil

		Expect(fctx.cleanupKubernetesRoutes(ctx, routerID)).To(Succeed())
	})

	It("preserves an aggregate route whose destination is a supernet of the pod CIDR", func() {
		// 100.96.0.0/15 starts inside the pod CIDR (100.96.0.0/16) but extends to
		// 100.97.0.0/16 which is outside it — must not be removed.
		routerRoutes := []routers.Route{
			{DestinationCIDR: "100.96.1.0/24", NextHop: "10.250.0.5"}, // CCM route — remove
			{DestinationCIDR: "100.96.0.0/15", NextHop: "10.250.0.1"}, // supernet — keep
		}
		mockNetworking.EXPECT().
			GetRouterByID(ctx, routerID).
			Return(&routers.Router{Routes: routerRoutes}, nil)
		mockNetworking.EXPECT().
			UpdateRoutesForRouter(ctx, []routers.Route{
				{DestinationCIDR: "100.96.0.0/15", NextHop: "10.250.0.1"},
			}, routerID).
			Return(&routers.Router{}, nil)

		Expect(fctx.cleanupKubernetesRoutes(ctx, routerID)).To(Succeed())
	})

	It("returns an error when GetRouterByID fails", func() {
		mockNetworking.EXPECT().
			GetRouterByID(ctx, routerID).
			Return(nil, errors.New("neutron unavailable"))

		Expect(fctx.cleanupKubernetesRoutes(ctx, routerID)).NotTo(Succeed())
	})

	It("returns an error when UpdateRoutesForRouter fails", func() {
		routerRoutes := []routers.Route{
			{DestinationCIDR: "100.96.1.0/24", NextHop: "10.250.0.5"},
		}
		mockNetworking.EXPECT().
			GetRouterByID(ctx, routerID).
			Return(&routers.Router{Routes: routerRoutes}, nil)
		mockNetworking.EXPECT().
			UpdateRoutesForRouter(ctx, gomock.Any(), routerID).
			Return(nil, errors.New("neutron unavailable"))

		Expect(fctx.cleanupKubernetesRoutes(ctx, routerID)).NotTo(Succeed())
	})
})
