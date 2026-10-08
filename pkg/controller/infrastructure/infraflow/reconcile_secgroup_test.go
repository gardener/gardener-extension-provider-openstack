// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/security/groups"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/security/rules"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	accessmocks "github.com/gardener/gardener-extension-provider-openstack/pkg/controller/infrastructure/infraflow/access/mocks"
)

var _ = Describe("ensureSecGroup", func() {
	var (
		ctrl *gomock.Controller
		ctx  context.Context
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		ctx = context.Background()
	})

	AfterEach(func() { ctrl.Finish() })

	Context("BYO security group (networks.securityGroupId set)", func() {
		It("stores the security group ID and name in state without creating anything", func() {
			sgID := "sgsgsgsg-sgsg-sgsg-sgsg-sgsgsgsgsgsg"
			networkID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
			cfg := defaultInfraConfig()
			cfg.Networks.ID = &networkID
			cfg.Networks.SecurityGroupID = &sgID

			fctx, mockAccess := newFixture(ctrl, cfg)
			mockAccess.EXPECT().
				GetSecurityGroupByID(ctx, sgID).
				Return(&groups.SecGroup{ID: sgID, Name: "my-sg"}, nil)

			Expect(fctx.ensureSecGroup(ctx)).To(Succeed())
			Expect(ptr.Deref(fctx.state.Get(IdentifierSecGroup), "")).To(Equal(sgID))
			Expect(ptr.Deref(fctx.state.Get(NameSecGroup), "")).To(Equal("my-sg"))
			Expect(fctx.state.GetObject(ObjectSecGroup)).NotTo(BeNil())
		})

		It("returns an error when the security group does not exist", func() {
			sgID := "sgsgsgsg-sgsg-sgsg-sgsg-sgsgsgsgsgsg"
			networkID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
			cfg := defaultInfraConfig()
			cfg.Networks.ID = &networkID
			cfg.Networks.SecurityGroupID = &sgID

			fctx, mockAccess := newFixture(ctrl, cfg)
			mockAccess.EXPECT().
				GetSecurityGroupByID(ctx, sgID).
				Return(nil, nil)

			Expect(fctx.ensureSecGroup(ctx)).NotTo(Succeed())
		})

		It("does not call CreateSecurityGroup", func() {
			sgID := "sgsgsgsg-sgsg-sgsg-sgsg-sgsgsgsgsgsg"
			networkID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
			cfg := defaultInfraConfig()
			cfg.Networks.ID = &networkID
			cfg.Networks.SecurityGroupID = &sgID

			fctx, mockAccess := newFixture(ctrl, cfg)
			mockAccess.EXPECT().
				GetSecurityGroupByID(ctx, sgID).
				Return(&groups.SecGroup{ID: sgID, Name: "my-sg"}, nil)
			// CreateSecurityGroup must NOT be called — gomock will fail if it is

			Expect(fctx.ensureSecGroup(ctx)).To(Succeed())
		})
	})

	Context("Gardener-managed security group (no securityGroupId)", func() {
		It("creates a new security group when none exists", func() {
			fctx, mockAccess := newFixture(ctrl, defaultInfraConfig())
			mockAccess.EXPECT().
				GetSecurityGroupByName(ctx, gomock.Any()).
				Return(nil, nil)
			mockAccess.EXPECT().
				CreateSecurityGroup(ctx, gomock.Any()).
				Return(&groups.SecGroup{ID: "new-sg-id", Name: "new-sg"}, nil)

			Expect(fctx.ensureSecGroup(ctx)).To(Succeed())
			Expect(ptr.Deref(fctx.state.Get(IdentifierSecGroup), "")).To(Equal("new-sg-id"))
		})

		It("reuses an existing security group found by name", func() {
			fctx, mockAccess := newFixture(ctrl, defaultInfraConfig())
			existing := &groups.SecGroup{ID: "existing-sg-id", Name: "existing-sg"}
			mockAccess.EXPECT().
				GetSecurityGroupByName(ctx, gomock.Any()).
				Return([]*groups.SecGroup{existing}, nil)

			Expect(fctx.ensureSecGroup(ctx)).To(Succeed())
			Expect(ptr.Deref(fctx.state.Get(IdentifierSecGroup), "")).To(Equal("existing-sg-id"))
		})
	})
})

var _ = Describe("ensureSecGroupRules", func() {
	var (
		ctrl *gomock.Controller
		ctx  context.Context
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		ctx = context.Background()
	})

	AfterEach(func() { ctrl.Finish() })

	It("skips UpdateSecurityGroupRules for BYO security group", func() {
		sgID := "sgsgsgsg-sgsg-sgsg-sgsg-sgsgsgsgsgsg"
		networkID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
		cfg := defaultInfraConfig()
		cfg.Networks.ID = &networkID
		cfg.Networks.SecurityGroupID = &sgID

		fctx, _ := newFixture(ctrl, cfg)
		// UpdateSecurityGroupRules must NOT be called — gomock will fail if it is

		Expect(fctx.ensureSecGroupRules(ctx)).To(Succeed())
	})

	It("calls UpdateSecurityGroupRules for a Gardener-managed security group", func() {
		fctx, mockAccess := newFixture(ctrl, defaultInfraConfig())
		sg := &groups.SecGroup{ID: "sg-id", Name: "sg"}
		fctx.state.SetObject(ObjectSecGroup, sg)

		mockAccess.EXPECT().
			UpdateSecurityGroupRules(ctx, sg, gomock.Any(), gomock.Any()).
			Return(false, nil)

		Expect(fctx.ensureSecGroupRules(ctx)).To(Succeed())
	})

	It("returns an internal error when ObjectSecGroup is not in state", func() {
		fctx, _ := newFixture(ctrl, defaultInfraConfig())
		Expect(fctx.ensureSecGroupRules(ctx)).NotTo(Succeed())
	})

	Context("pod network rules", func() {
		var (
			fctx         *FlowContext
			sg           *groups.SecGroup
			desiredRules []rules.SecGroupRule
		)

		networkingWithProviderConfig := func(providerConfig string) *gardencorev1beta1.Networking {
			n := &gardencorev1beta1.Networking{
				Pods:       ptr.To("100.96.0.0/11"),
				IPFamilies: []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4},
			}
			if providerConfig != "" {
				n.ProviderConfig = &runtime.RawExtension{Raw: []byte(providerConfig)}
			}
			return n
		}

		podRules := func() []rules.SecGroupRule {
			var result []rules.SecGroupRule
			for _, r := range desiredRules {
				if r.RemoteIPPrefix == "100.96.0.0/11" || r.RemoteIPPrefix == "2001:db8:1::/64" {
					result = append(result, r)
				}
			}
			return result
		}

		BeforeEach(func() {
			var mockAccess *accessmocks.MockNetworkingAccess
			fctx, mockAccess = newFixture(ctrl, defaultInfraConfig())
			sg = &groups.SecGroup{ID: "sg-id", Name: "sg"}
			fctx.state.SetObject(ObjectSecGroup, sg)
			desiredRules = nil

			mockAccess.EXPECT().
				UpdateSecurityGroupRules(ctx, sg, gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, _ *groups.SecGroup, r []rules.SecGroupRule, _ func(*rules.SecGroupRule) bool) (bool, error) {
					desiredRules = r
					return false, nil
				}).
				AnyTimes()
		})

		It("adds an ingress rule for the pod CIDR when the overlay is disabled", func() {
			fctx.shootNetworking = networkingWithProviderConfig(`{"overlay":{"enabled":false}}`)

			Expect(fctx.ensureSecGroupRules(ctx)).To(Succeed())
			Expect(podRules()).To(ConsistOf(rules.SecGroupRule{
				Direction:      string(rules.DirIngress),
				EtherType:      string(rules.EtherType4),
				RemoteIPPrefix: "100.96.0.0/11",
				Description:    "IPv4: allow all incoming traffic from the pod network (overlay disabled)",
			}))
		})

		It("adds ingress rules for the IPv4 and IPv6 pod CIDRs when the overlay is disabled in a dual-stack shoot", func() {
			fctx.shootNetworking = networkingWithProviderConfig(`{"overlay":{"enabled":false}}`)
			fctx.shootNetworking.IPFamilies = []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4, gardencorev1beta1.IPFamilyIPv6}
			fctx.state.Set(IdentifierPodSubnetIPv6CIDR, "2001:db8:1::/64")

			Expect(fctx.ensureSecGroupRules(ctx)).To(Succeed())
			Expect(podRules()).To(ConsistOf(
				rules.SecGroupRule{
					Direction:      string(rules.DirIngress),
					EtherType:      string(rules.EtherType4),
					RemoteIPPrefix: "100.96.0.0/11",
					Description:    "IPv4: allow all incoming traffic from the pod network (overlay disabled)",
				},
				rules.SecGroupRule{
					Direction:      string(rules.DirIngress),
					EtherType:      string(rules.EtherType6),
					RemoteIPPrefix: "2001:db8:1::/64",
					Description:    "IPv6: allow all incoming traffic from the pod network (overlay disabled)",
				},
			))
		})

		It("does not add a pod CIDR rule when the overlay is enabled", func() {
			fctx.shootNetworking = networkingWithProviderConfig(`{"overlay":{"enabled":true}}`)

			Expect(fctx.ensureSecGroupRules(ctx)).To(Succeed())
			Expect(desiredRules).NotTo(BeEmpty())
			Expect(podRules()).To(BeEmpty())
		})

		It("does not add a pod CIDR rule when no overlay is configured", func() {
			fctx.shootNetworking = networkingWithProviderConfig("")

			Expect(fctx.ensureSecGroupRules(ctx)).To(Succeed())
			Expect(desiredRules).NotTo(BeEmpty())
			Expect(podRules()).To(BeEmpty())
		})

		It("returns an error when the overlay configuration is invalid", func() {
			fctx.shootNetworking = networkingWithProviderConfig(`{"overlay":{"enabled":"no"}}`)

			Expect(fctx.ensureSecGroupRules(ctx)).NotTo(Succeed())
		})
	})
})
