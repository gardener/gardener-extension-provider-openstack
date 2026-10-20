// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package openstack_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/install"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"
)

var _ = Describe("WorkerConfig", func() {
	DescribeTable("should preserve the per-host limit through API conversion", func(raw string, limit *int32) {
		scheme := runtime.NewScheme()
		Expect(install.AddToScheme(scheme)).To(Succeed())
		codecs := serializer.NewCodecFactory(scheme, serializer.EnableStrict)

		config := &openstack.WorkerConfig{}
		_, _, err := codecs.UniversalDecoder().Decode([]byte(raw), nil, config)
		Expect(err).NotTo(HaveOccurred())
		Expect(config.ServerGroup.MaxServerPerHost).To(Equal(limit))

		encoded, err := runtime.Encode(codecs.LegacyCodec(v1alpha1.SchemeGroupVersion), config)
		Expect(err).NotTo(HaveOccurred())
		Expect(encoded).To(MatchJSON(raw))
	},
		Entry("omitted", `{"apiVersion":"openstack.provider.extensions.gardener.cloud/v1alpha1","kind":"WorkerConfig","serverGroup":{"policy":"anti-affinity"}}`, nil),
		Entry("set", `{"apiVersion":"openstack.provider.extensions.gardener.cloud/v1alpha1","kind":"WorkerConfig","serverGroup":{"policy":"anti-affinity","maxServerPerHost":2}}`, ptr.To(int32(2))),
	)

	It("should deep copy the per-host limit in internal and versioned worker configurations", func() {
		config := &openstack.WorkerConfig{
			ServerGroup: &openstack.ServerGroup{Policy: "anti-affinity", MaxServerPerHost: ptr.To(int32(2))},
		}
		copied := config.DeepCopy()
		*copied.ServerGroup.MaxServerPerHost = 3
		Expect(*config.ServerGroup.MaxServerPerHost).To(Equal(int32(2)))

		versioned := &v1alpha1.WorkerConfig{
			ServerGroup: &v1alpha1.ServerGroup{Policy: "anti-affinity", MaxServerPerHost: ptr.To(int32(2))},
		}
		versionedCopy := versioned.DeepCopy()
		*versionedCopy.ServerGroup.MaxServerPerHost = 3
		Expect(*versioned.ServerGroup.MaxServerPerHost).To(Equal(int32(2)))
	})
})
