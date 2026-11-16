// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"
)

func TestInfrastructureStateFromRaw(t *testing.T) {
	for _, tc := range []struct {
		name    string
		raw     *runtime.RawExtension
		data    map[string]string
		wantErr bool
	}{
		{name: "absent state", data: map[string]string{}},
		{name: "empty raw extension", raw: &runtime.RawExtension{}, data: map[string]string{}},
		{
			name: "native state preserves resource IDs and existence marker",
			raw:  &runtime.RawExtension{Raw: []byte(`{"apiVersion":"openstack.provider.extensions.gardener.cloud/v1alpha1","kind":"InfrastructureState","data":{"Network":"network-1","Router":"router-1","resource_exist":"true"},"futureField":true}`)},
			data: map[string]string{"Network": "network-1", "Router": "router-1", "resource_exist": "true"},
		},
		{
			name: "native state supplied as an object",
			raw: &runtime.RawExtension{Object: &v1alpha1.InfrastructureState{
				TypeMeta: metav1.TypeMeta{APIVersion: v1alpha1.SchemeGroupVersion.String(), Kind: "InfrastructureState"},
				Data:     map[string]string{"Router": "router-1"},
			}},
			data: map[string]string{"Router": "router-1"},
		},
		{name: "invalid JSON", raw: &runtime.RawExtension{Raw: []byte(`invalid`)}, wantErr: true},
		{name: "legacy Terraform state", raw: &runtime.RawExtension{Raw: []byte(`{"data":"{}","encoding":"none"}`)}, wantErr: true},
		{name: "missing type metadata", raw: &runtime.RawExtension{Raw: []byte(`{"data":{"resource_exist":"true"}}`)}, wantErr: true},
		{name: "unsupported API version", raw: &runtime.RawExtension{Raw: []byte(`{"apiVersion":"example.com/v1","kind":"InfrastructureState"}`)}, wantErr: true},
		{name: "different registered kind", raw: &runtime.RawExtension{Raw: []byte(`{"apiVersion":"openstack.provider.extensions.gardener.cloud/v1alpha1","kind":"InfrastructureConfig"}`)}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			state, err := helper.InfrastructureStateFromRaw(tc.raw)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
				g.Expect(state).To(BeNil())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(state).To(Equal(&api.InfrastructureState{Data: tc.data}))
			}
		})
	}
}
