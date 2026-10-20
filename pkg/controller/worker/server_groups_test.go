// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"testing"

	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servergroups"
	"k8s.io/utils/ptr"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
)

func TestServerGroupDefaultLimit(t *testing.T) {
	for _, group := range []struct {
		name  string
		value *servergroups.ServerGroup
	}{
		{"legacy response", &servergroups.ServerGroup{Policies: []string{"anti-affinity"}}},
		{"modern response without rules", &servergroups.ServerGroup{Policy: ptr.To("anti-affinity")}},
		{"modern response with empty rules", &servergroups.ServerGroup{Policy: ptr.To("anti-affinity"), Rules: &servergroups.Rules{}}},
	} {
		for _, desired := range []struct {
			name  string
			limit *int32
			match bool
		}{
			{"implicit one", nil, true},
			{"explicit one", ptr.To[int32](1), true},
			{"custom limit", ptr.To[int32](2), false},
		} {
			t.Run(group.name+"/"+desired.name, func(t *testing.T) {
				config := &api.ServerGroup{Policy: "anti-affinity", MaxServerPerHost: desired.limit}
				if got := serverGroupMatchesConfig(group.value, config); got != desired.match {
					t.Errorf("matches = %t, want %t", got, desired.match)
				}
			})
		}
	}
}

func TestServerGroupNamesPreserveDefaults(t *testing.T) {
	const uid = "12345678-1234-1234-1234-123456789012"
	// The name produced before maxServerPerHost was supported must remain stable.
	const originalName = "12345678-1234-1234-b4c0b2f0"
	for _, limit := range []*int32{nil, ptr.To[int32](1)} {
		if got := generateServerGroupNameV2(uid, "pool", "anti-affinity", limit); got != originalName {
			t.Errorf("default name = %q, want %q", got, originalName)
		}
	}
	two := generateServerGroupNameV2(uid, "pool", "anti-affinity", ptr.To[int32](2))
	three := generateServerGroupNameV2(uid, "pool", "anti-affinity", ptr.To[int32](3))
	if two == originalName || three == originalName || two == three {
		t.Fatalf("different limits must use distinct names: %q, %q, %q", originalName, two, three)
	}
}
