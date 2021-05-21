// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package infrastructure

import (
	"bytes"
	"text/template"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
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
