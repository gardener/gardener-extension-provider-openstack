// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"context"

	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/layer3/floatingips"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/layer3/routers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	openstackapi "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	mockopenstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client/mocks"
)

var _ = Describe("EnsurePublicIPAddress", func() {
	var (
		ctrl                *gomock.Controller
		networkingClient    *mockopenstackclient.MockNetworking
		ctx                 context.Context
		opts                Options
		infraStatus         *openstackapi.InfrastructureStatus
		bastionInstanceName string
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		networkingClient = mockopenstackclient.NewMockNetworking(ctrl)
		ctx = context.Background()
		bastionInstanceName = "test-bastion-instance"

		opts = Options{
			BaseOptions: BaseOptions{
				BastionInstanceName: bastionInstanceName,
				Logr:                logf.Log.WithName("test"),
			},
		}

		infraStatus = &openstackapi.InfrastructureStatus{
			Networks: openstackapi.NetworkStatus{
				FloatingPool: openstackapi.FloatingPoolStatus{
					ID:   "floating-pool-id",
					Name: "floating-pool-name",
				},
				Router: openstackapi.RouterStatus{
					ID: "router-id",
				},
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("reusing existing FIP", func() {
		It("should return existing FIP when status is ACTIVE", func() {
			existingFips := []floatingips.FloatingIP{{
				ID:         "fip-1",
				FloatingIP: "1.2.3.4",
				Status:     "ACTIVE",
				PortID:     "port-id",
			}}

			networkingClient.EXPECT().
				GetFipByName(ctx, bastionInstanceName).
				Return(existingFips, nil)

			fip, err := ensurePublicIPAddress(ctx, opts, networkingClient, infraStatus)
			Expect(err).NotTo(HaveOccurred())
			Expect(fip.ID).To(Equal("fip-1"))
			Expect(fip.FloatingIP).To(Equal("1.2.3.4"))
		})

		It("should return existing FIP when status is DOWN and PortID is empty", func() {
			existingFips := []floatingips.FloatingIP{{
				ID:         "fip-2",
				FloatingIP: "2.3.4.5",
				Status:     "DOWN",
				PortID:     "",
			}}

			networkingClient.EXPECT().
				GetFipByName(ctx, bastionInstanceName).
				Return(existingFips, nil)

			fip, err := ensurePublicIPAddress(ctx, opts, networkingClient, infraStatus)
			Expect(err).NotTo(HaveOccurred())
			Expect(fip.ID).To(Equal("fip-2"))
			Expect(fip.FloatingIP).To(Equal("2.3.4.5"))
		})

		It("should error when existing FIP is DOWN and PortID is not empty", func() {
			existingFips := []floatingips.FloatingIP{{
				ID:         "fip-3",
				FloatingIP: "3.4.5.6",
				Status:     "DOWN",
				PortID:     "port-id",
			}}

			networkingClient.EXPECT().
				GetFipByName(ctx, bastionInstanceName).
				Return(existingFips, nil)

			_, err := ensurePublicIPAddress(ctx, opts, networkingClient, infraStatus)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not active"))
		})

		It("should error when existing FIP has ERROR status", func() {
			existingFip := floatingips.FloatingIP{
				ID:         "fip-4",
				FloatingIP: "4.5.6.7",
				Status:     "ERROR",
				PortID:     "",
			}

			networkingClient.EXPECT().
				GetFipByName(ctx, bastionInstanceName).
				Return([]floatingips.FloatingIP{existingFip}, nil)

			_, err := ensurePublicIPAddress(ctx, opts, networkingClient, infraStatus)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not active"))
		})
	})

	Describe("creating new FIP", func() {
		It("should create new FIP and accept ACTIVE status", func() {
			networkingClient.EXPECT().
				GetFipByName(ctx, bastionInstanceName).
				Return([]floatingips.FloatingIP{}, nil)

			router := &routers.Router{
				ID:     "router-id",
				Status: "ACTIVE",
				GatewayInfo: routers.GatewayInfo{
					NetworkID: "ext-net-id",
					ExternalFixedIPs: []routers.ExternalFixedIP{{
						IPAddress: "10.0.0.1",
						SubnetID:  "ext-subnet-id",
					}},
				},
			}
			networkingClient.EXPECT().
				GetRouterByID(ctx, infraStatus.Networks.Router.ID).
				Return(router, nil)

			mockFip := floatingips.FloatingIP{
				ID:         "fip-new",
				FloatingIP: "5.6.7.8",
				Status:     "ACTIVE",
			}
			networkingClient.EXPECT().
				CreateFloatingIP(ctx, gomock.Any()).
				Return(&mockFip, nil)
			networkingClient.EXPECT().
				GetFloatingIP(ctx, gomock.Any()).
				Return(mockFip, nil)

			fip, err := ensurePublicIPAddress(ctx, opts, networkingClient, infraStatus)
			Expect(err).NotTo(HaveOccurred())
			Expect(fip.ID).To(Equal("fip-new"))
		})

		It("should create new FIP and accept DOWN status after creation", func() {
			networkingClient.EXPECT().
				GetFipByName(ctx, bastionInstanceName).
				Return([]floatingips.FloatingIP{}, nil)

			router := &routers.Router{
				ID:     "router-id",
				Status: "ACTIVE",
				GatewayInfo: routers.GatewayInfo{
					NetworkID: "ext-net-id",
					ExternalFixedIPs: []routers.ExternalFixedIP{{
						IPAddress: "10.0.0.1",
						SubnetID:  "ext-subnet-id",
					}},
				},
			}
			networkingClient.EXPECT().
				GetRouterByID(ctx, infraStatus.Networks.Router.ID).
				Return(router, nil)

			mockFip := floatingips.FloatingIP{
				ID:         "fip-new-down",
				FloatingIP: "6.7.8.9",
				Status:     "DOWN",
			}
			networkingClient.EXPECT().
				CreateFloatingIP(ctx, gomock.Any()).
				Return(&mockFip, nil)
			networkingClient.EXPECT().
				GetFloatingIP(ctx, gomock.Any()).
				Return(mockFip, nil)

			fip, err := ensurePublicIPAddress(ctx, opts, networkingClient, infraStatus)
			Expect(err).NotTo(HaveOccurred())
			Expect(fip.ID).To(Equal("fip-new-down"))
		})
	})
})
