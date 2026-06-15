// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
