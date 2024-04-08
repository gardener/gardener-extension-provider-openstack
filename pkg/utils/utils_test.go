// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener-extension-provider-openstack/pkg/utils"
)

var _ = Describe("Utils", func() {
	DescribeTable("#SimpleMatch", func(pattern, text string, expected bool, expectedSocre int) {
		match, score := SimpleMatch(pattern, text)
		Expect(match).To(Equal(expected))
		Expect(score).To(Equal(expectedSocre))
	},
		Entry("should match wildcard", "*", "test", true, 0),
		Entry("should not match empty string", "", "test", false, 0),
		Entry("should match text", "test", "test", true, 4),
		Entry("should not match text", "tet", "test", false, 0),
		Entry("should match wildcard suffix", "t*", "test", true, 1),
		Entry("should match wildcard suffix score 2", "te*", "test", true, 2),
		Entry("should match wildcard suffix score 3", "tes*", "test", true, 3),
		Entry("should match wildcard suffix score 4", "test*", "test", true, 4),
		Entry("should not match wildcard suffix", "e*", "test", false, 0),
		Entry("should not match wildcard suffix", "teste*", "test", false, 0),
		Entry("should match wildcard prefix", "*t", "test", true, 1),
		Entry("should match wildcard prefix score 2", "*st", "test", true, 2),
		Entry("should match wildcard prefix score 3", "*est", "test", true, 3),
		Entry("should match wildcard prefix score 4", "*test", "test", true, 4),
		Entry("should not match wildcard prefix", "*d", "test", false, 0),
		Entry("should not match wildcard prefix", "*teste", "test", false, 0),
		Entry("should not match wildcard arbitrary", "te*t", "test", false, 0),
	)

	DescribeTable("#IsStringPtrValueEqual", func(a *string, b string, expected bool) {
		Expect(IsStringPtrValueEqual(a, b)).To(Equal(expected))
	},
		Entry("should be false as pointer points to nil", nil, "test", false),
		Entry("should be false as pointer value is different", ptr.To("different"), "test", false),
		Entry("should be true as pointer value is equal", ptr.To("test"), "test", true),
	)
})
