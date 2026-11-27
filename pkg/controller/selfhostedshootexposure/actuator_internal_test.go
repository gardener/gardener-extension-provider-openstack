// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package selfhostedshootexposure

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/security/rules"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	openstackapi "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client/mocks"
)

func TestEnsureSecurityGroupRule(t *testing.T) {
	const (
		namespace = "shoot--project--test"
		groupID   = "nodes-sg"
		ipv4CIDR  = "10.0.0.0/24"
		ipv6CIDR  = "2001:db8:1::/64"
	)

	exposure := &extensionsv1alpha1.SelfHostedShootExposure{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace},
		Spec:       extensionsv1alpha1.SelfHostedShootExposureSpec{Port: 6443},
	}
	subnets := []openstackapi.Subnet{
		{Purpose: openstackapi.PurposeNodes, ID: "nodes-ipv4", CIDR: ipv4CIDR},
		{Purpose: openstackapi.PurposeNodesIPv6, ID: "nodes-ipv6", CIDR: ipv6CIDR},
	}

	ipv4Opts := rules.CreateOpts{
		Direction:      rules.DirIngress,
		Description:    "shoot-exposure-" + namespace,
		EtherType:      rules.EtherType4,
		Protocol:       rules.ProtocolTCP,
		PortRangeMin:   6443,
		PortRangeMax:   6443,
		SecGroupID:     groupID,
		RemoteIPPrefix: ipv4CIDR,
	}
	ipv6Opts := ipv4Opts
	ipv6Opts.EtherType = rules.EtherType6
	ipv6Opts.RemoteIPPrefix = ipv6CIDR

	scopedIPv4 := rules.SecGroupRule{
		ID: "scoped-rule", Direction: "ingress", EtherType: "IPv4", Protocol: "tcp",
		PortRangeMin: 6443, PortRangeMax: 6443, RemoteIPPrefix: ipv4CIDR,
	}
	scopedIPv6 := scopedIPv4
	scopedIPv6.EtherType = "IPv6"
	scopedIPv6.RemoteIPPrefix = ipv6CIDR
	emptyIPv4 := scopedIPv4
	emptyIPv4.ID, emptyIPv4.RemoteIPPrefix = "empty-prefix-rule", ""
	emptyIPv6 := scopedIPv6
	emptyIPv6.ID, emptyIPv6.RemoteIPPrefix = "empty-prefix-rule", ""
	anyIPv4 := scopedIPv4
	anyIPv4.ID, anyIPv4.RemoteIPPrefix = "any-source-rule", "0.0.0.0/0"
	anyIPv6 := scopedIPv6
	anyIPv6.ID, anyIPv6.RemoteIPPrefix = "any-source-rule", "::/0"

	tests := []struct {
		name       string
		family     gardencorev1beta1.IPFamily
		existing   []rules.SecGroupRule
		deleteID   string
		deleteErr  error
		wantCreate *rules.CreateOpts // nil means no rule should be created.
		wantErr    bool
	}{
		{
			name: "creates a scoped IPv4 rule", family: gardencorev1beta1.IPFamilyIPv4,
			wantCreate: &ipv4Opts,
		},
		{
			name: "creates a scoped IPv6 rule", family: gardencorev1beta1.IPFamilyIPv6,
			wantCreate: &ipv6Opts,
		},
		{
			name: "replaces an empty IPv4 prefix", family: gardencorev1beta1.IPFamilyIPv4,
			existing: []rules.SecGroupRule{emptyIPv4}, deleteID: emptyIPv4.ID, wantCreate: &ipv4Opts,
		},
		{
			name: "replaces an empty IPv6 prefix", family: gardencorev1beta1.IPFamilyIPv6,
			existing: []rules.SecGroupRule{emptyIPv6}, deleteID: emptyIPv6.ID, wantCreate: &ipv6Opts,
		},
		{
			name: "replaces an explicit IPv4 any-source rule", family: gardencorev1beta1.IPFamilyIPv4,
			existing: []rules.SecGroupRule{anyIPv4}, deleteID: anyIPv4.ID, wantCreate: &ipv4Opts,
		},
		{
			name: "replaces an explicit IPv6 any-source rule", family: gardencorev1beta1.IPFamilyIPv6,
			existing: []rules.SecGroupRule{anyIPv6}, deleteID: anyIPv6.ID, wantCreate: &ipv6Opts,
		},
		{
			name: "preserves a correct IPv4 rule", family: gardencorev1beta1.IPFamilyIPv4,
			existing: []rules.SecGroupRule{scopedIPv4},
		},
		{
			name: "preserves a correct IPv6 rule", family: gardencorev1beta1.IPFamilyIPv6,
			existing: []rules.SecGroupRule{scopedIPv6},
		},
		{
			name: "removes a broad IPv4 rule alongside a correct rule", family: gardencorev1beta1.IPFamilyIPv4,
			existing: []rules.SecGroupRule{scopedIPv4, emptyIPv4}, deleteID: emptyIPv4.ID,
		},
		{
			name: "removes a broad IPv6 rule alongside a correct rule", family: gardencorev1beta1.IPFamilyIPv6,
			existing: []rules.SecGroupRule{scopedIPv6, emptyIPv6}, deleteID: emptyIPv6.ID,
		},
		{
			name: "propagates IPv4 deletion failures", family: gardencorev1beta1.IPFamilyIPv4,
			existing: []rules.SecGroupRule{emptyIPv4}, deleteID: emptyIPv4.ID,
			deleteErr: errors.New("neutron unavailable"), wantErr: true,
		},
		{
			name: "propagates IPv6 deletion failures", family: gardencorev1beta1.IPFamilyIPv6,
			existing: []rules.SecGroupRule{emptyIPv6}, deleteID: emptyIPv6.ID,
			deleteErr: errors.New("neutron unavailable"), wantErr: true,
		},
		{
			name: "propagates forbidden IPv4 deletion", family: gardencorev1beta1.IPFamilyIPv4,
			existing: []rules.SecGroupRule{emptyIPv4}, deleteID: emptyIPv4.ID,
			deleteErr: gophercloud.ErrUnexpectedResponseCode{Actual: http.StatusForbidden}, wantErr: true,
		},
		{
			name: "propagates forbidden IPv6 deletion", family: gardencorev1beta1.IPFamilyIPv6,
			existing: []rules.SecGroupRule{emptyIPv6}, deleteID: emptyIPv6.ID,
			deleteErr: gophercloud.ErrUnexpectedResponseCode{Actual: http.StatusForbidden}, wantErr: true,
		},
		{
			name: "ignores an already-deleted IPv4 rule", family: gardencorev1beta1.IPFamilyIPv4,
			existing: []rules.SecGroupRule{emptyIPv4}, deleteID: emptyIPv4.ID,
			deleteErr: gophercloud.ErrUnexpectedResponseCode{Actual: http.StatusNotFound}, wantCreate: &ipv4Opts,
		},
		{
			name: "ignores an already-deleted IPv6 rule", family: gardencorev1beta1.IPFamilyIPv6,
			existing: []rules.SecGroupRule{emptyIPv6}, deleteID: emptyIPv6.ID,
			deleteErr: gophercloud.ErrUnexpectedResponseCode{Actual: http.StatusNotFound}, wantCreate: &ipv6Opts,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			networking := mocks.NewMockNetworking(gomock.NewController(t))
			lastCall := networking.EXPECT().ListRules(ctx, rules.ListOpts{
				SecGroupID: groupID, Description: "shoot-exposure-" + namespace,
			}).Return(tt.existing, nil)
			if tt.deleteID != "" {
				lastCall = networking.EXPECT().DeleteRule(ctx, tt.deleteID).Return(tt.deleteErr).After(lastCall)
			}
			if tt.wantCreate != nil {
				networking.EXPECT().CreateRule(ctx, *tt.wantCreate).After(lastCall).
					DoAndReturn(func(_ context.Context, opts rules.CreateOpts) (*rules.SecGroupRule, error) {
						body, err := opts.ToSecGroupRuleCreateMap()
						if err != nil {
							t.Fatal(err)
						}
						ruleBody, ok := body["security_group_rule"].(map[string]any)
						if !ok || ruleBody["remote_ip_prefix"] != tt.wantCreate.RemoteIPPrefix {
							t.Fatalf("expected request with remote_ip_prefix %q, got %#v", tt.wantCreate.RemoteIPPrefix, body)
						}
						return &rules.SecGroupRule{ID: "scoped-rule"}, nil
					})
			}

			err := ensureSecurityGroupRule(ctx, networking, exposure, groupID, tt.family, subnets)
			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), tt.deleteErr.Error()) {
					t.Fatalf("expected deletion error %v, got %v", tt.deleteErr, err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestEnsureSecurityGroupRuleRejectsInvalidCIDR(t *testing.T) {
	tests := []struct {
		name    string
		family  gardencorev1beta1.IPFamily
		purpose openstackapi.Purpose
		cidr    string
	}{
		{"missing IPv4 CIDR", gardencorev1beta1.IPFamilyIPv4, openstackapi.PurposeNodes, ""},
		{"missing IPv6 CIDR", gardencorev1beta1.IPFamilyIPv6, openstackapi.PurposeNodesIPv6, ""},
		{"malformed IPv4 CIDR", gardencorev1beta1.IPFamilyIPv4, openstackapi.PurposeNodes, "invalid"},
		{"malformed IPv6 CIDR", gardencorev1beta1.IPFamilyIPv6, openstackapi.PurposeNodesIPv6, "invalid"},
		{"IPv6 CIDR for IPv4", gardencorev1beta1.IPFamilyIPv4, openstackapi.PurposeNodes, "2001:db8::/64"},
		{"IPv4 CIDR for IPv6", gardencorev1beta1.IPFamilyIPv6, openstackapi.PurposeNodesIPv6, "10.0.0.0/24"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// No cloud calls are expected, including deletion of existing rules.
			networking := mocks.NewMockNetworking(gomock.NewController(t))
			err := ensureSecurityGroupRule(context.Background(), networking,
				&extensionsv1alpha1.SelfHostedShootExposure{}, "nodes-sg", tt.family,
				[]openstackapi.Subnet{{Purpose: tt.purpose, ID: "nodes-subnet", CIDR: tt.cidr}})
			if err == nil {
				t.Fatal("expected invalid nodes CIDR to prevent security group rule reconciliation")
			}
		})
	}
}
