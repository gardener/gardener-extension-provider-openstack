// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/controller/infrastructure/infraflow/shared"
)

var _ = Describe("Whiteboard", func() {
	It("should create hierarchical whiteboard from flat map", func() {
		var (
			data = shared.FlatMap{
				"key1":                  "id1",
				"key2":                  "id2",
				"child1/subchild1/key1": "id111",
				"child1/subchild2/key1": "id121",
				"child2/key2":           "id22",
			}
			expectedData = shared.FlatMap{
				"key1":                  "<deleted>",
				"key2":                  "id2a",
				"key3":                  "id3",
				"child1/subchild1/key1": "id111b",
				"child2/key2":           "id22",
			}
			expectedMap = map[string]string{
				"key2": "id2a",
				"key3": "id3",
			}
			expectedKeys = []string{"key1", "key2", "key3"}
		)

		w := shared.NewWhiteboard()
		w.IsEmpty()
		w.ImportFromFlatMap(data)
		Expect(w.Get("key1")).NotTo(BeNil())
		Expect(*w.Get("key1")).To(Equal("id1"))
		Expect(w.Get("key2")).NotTo(BeNil())
		Expect(*w.Get("key2")).To(Equal("id2"))
		Expect(w.Get("child1")).To(BeNil())
		Expect(w.GetChild("child1").IsEmpty()).To(BeFalse())
		Expect(w.GetChild("child1").GetChild("subchild1").Get("key1")).To(Equal(ptr.To("id111")))
		Expect(w.GetChild("child1").GetChild("subchild2").Get("key1")).To(Equal(ptr.To("id121")))
		Expect(w.GetChild("child2").IsEmpty()).To(BeFalse())
		Expect(w.GetChild("child2").Get("key2")).To(Equal(ptr.To("id22")))
		Expect(w.GetChild("child3").IsEmpty()).To(BeTrue())
		generation1 := w.CurrentGeneration()
		w.Set("key2", "id2")
		Expect(w.CurrentGeneration()).To(Equal(generation1))
		w.GetChild("child1").GetChild("subchild1").Set("key1", "id111")
		Expect(w.CurrentGeneration()).To(Equal(generation1))
		w.Set("key2", "id2a")
		generation2 := w.CurrentGeneration()
		Expect(generation2 > generation1).To(BeTrue())
		w.GetChild("child1").GetChild("subchild1").Set("key1", "id111b")
		Expect(w.CurrentGeneration() > generation2).To(BeTrue())

		Expect(w.GetChild("child1").GetChild("subchild2").IsAlreadyDeleted("key1")).To(BeFalse())
		w.GetChild("child1").GetChild("subchild2").Set("key1", "")
		Expect(w.GetChild("child1").GetChild("subchild2").IsAlreadyDeleted("key1")).To(BeFalse())

		Expect(w.Get("key1")).NotTo(BeNil())
		Expect(w.IsAlreadyDeleted("key1")).To(BeFalse())
		w.SetAsDeleted("key1")
		Expect(w.Get("key1")).To(BeNil())
		Expect(w.IsAlreadyDeleted("key1")).To(BeTrue())

		Expect(w.Get("key3")).To(BeNil())
		w.SetPtr("key3", ptr.To("id3"))
		Expect(w.Get("key3")).NotTo(BeNil())

		Expect(w.HasChild("child1")).To(BeTrue())
		Expect(w.HasChild("child3")).To(BeFalse())

		Expect(w.AsMap()).To(Equal(expectedMap))

		Expect(w.Keys()).To(Equal(expectedKeys))

		generation3 := w.CurrentGeneration()
		w.SetObject("obj1", expectedKeys)
		Expect(w.GetObject("obj1")).To(Equal(expectedKeys))
		Expect(w.CurrentGeneration()).To(Equal(generation3))

		exported := w.ExportAsFlatMap()
		Expect(exported).To(Equal(expectedData))
	})
})
