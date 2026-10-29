// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servergroups"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
)

var _ = Describe("#serverGroupMatchesConfig", func() {
	for _, group := range []struct {
		name  string
		value *servergroups.ServerGroup
	}{
		{"legacy response", &servergroups.ServerGroup{Policies: []string{"anti-affinity"}}},
		{"modern response without rules", &servergroups.ServerGroup{Policy: ptr.To("anti-affinity")}},
		{"modern response with empty rules", &servergroups.ServerGroup{Policy: ptr.To("anti-affinity"), Rules: &servergroups.Rules{}}},
	} {
		Context(group.name, func() {
			DescribeTable("should compare the effective per-host limit", func(limit *int32, matches bool) {
				config := &api.ServerGroup{Policy: "anti-affinity", MaxServerPerHost: limit}
				Expect(serverGroupMatchesConfig(group.value, config)).To(Equal(matches))
			},
				Entry("implicit one", nil, true),
				Entry("explicit one", ptr.To[int32](1), true),
				Entry("custom limit", ptr.To[int32](2), false),
			)
		})
	}
})

var _ = Describe("#generateServerGroupNameV2", func() {
	const uid = "12345678-1234-1234-1234-123456789012"
	// The name produced before maxServerPerHost was supported must remain stable.
	const originalName = "12345678-1234-1234-b4c0b2f0"

	DescribeTable("should preserve the original name for the default limit", func(limit *int32) {
		Expect(generateServerGroupNameV2(uid, "pool", "anti-affinity", limit)).To(Equal(originalName))
	},
		Entry("implicit default", nil),
		Entry("explicit default", ptr.To[int32](1)),
	)

	It("should use distinct names for different effective limits", func() {
		two := generateServerGroupNameV2(uid, "pool", "anti-affinity", ptr.To[int32](2))
		three := generateServerGroupNameV2(uid, "pool", "anti-affinity", ptr.To[int32](3))
		Expect(two).NotTo(Equal(originalName))
		Expect(three).NotTo(Equal(originalName))
		Expect(two).NotTo(Equal(three))
	})
})
