// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"bytes"
	"text/template"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Template", func() {

	DescribeTable("dnsServer", func(values interface{}, out string) {
		testTpl := `[{{ dnsServers . }}]`
		parsedTpl, err := template.New("test").Funcs(
			map[string]interface{}{
				"dnsServers": dnsServers,
			}).
			Parse(testTpl)
		Expect(err).NotTo(HaveOccurred())

		var buffer bytes.Buffer
		err = parsedTpl.Execute(&buffer, values)

		Expect(err).NotTo(HaveOccurred())
		Expect(buffer.String()).To(Equal(out))
	},
		Entry("should print correctly", []string{"1"}, `["1"]`),
		Entry("should print correctly for 0 elements", []string{}, `[]`),
		Entry("should print correctly for multiple inputs", []string{"1", "2"}, `["1", "2"]`),
	)
})
