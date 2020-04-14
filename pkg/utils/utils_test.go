// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package utils_test

import (
	. "github.com/gardener/gardener-extension-provider-openstack/pkg/utils"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
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
})
