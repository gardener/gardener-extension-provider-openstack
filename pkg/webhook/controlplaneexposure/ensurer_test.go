// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controlplaneexposure

import (
	"context"
	"testing"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	gcontext "github.com/gardener/gardener/extensions/pkg/webhook/context"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/config"
)

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controlplane Exposure Webhook Suite")
}

var _ = Describe("Ensurer", func() {
	var (
		etcdStorage = &config.ETCDStorage{
			ClassName: pointer.String("gardener.cloud-fast"),
			Capacity:  utils.QuantityPtr(resource.MustParse("25Gi")),
		}

		ctrl *gomock.Controller

		dummyContext = gcontext.NewGardenContext(nil, nil)
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#EnsureETCD", func() {
		It("should add or modify elements to etcd-main statefulset", func() {
			var (
				etcd = &druidv1alpha1.Etcd{
					ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.ETCDMain},
				}
			)

			// Create ensurer
			ensurer := NewEnsurer(etcdStorage, logger)

			// Call EnsureETCDStatefulSet method and check the result
			err := ensurer.EnsureETCD(context.TODO(), dummyContext, etcd, nil)
			Expect(err).To(Not(HaveOccurred()))
			checkETCDMain(etcd)
		})

		It("should modify existing elements of etcd-main statefulset", func() {
			var (
				r    = resource.MustParse("10Gi")
				etcd = &druidv1alpha1.Etcd{
					ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.ETCDMain},
					Spec: druidv1alpha1.EtcdSpec{
						StorageCapacity: &r,
					},
				}
			)

			// Create ensurer
			ensurer := NewEnsurer(etcdStorage, logger)

			// Call EnsureETCDStatefulSet method and check the result
			err := ensurer.EnsureETCD(context.TODO(), dummyContext, etcd, nil)
			Expect(err).To(Not(HaveOccurred()))
			checkETCDMain(etcd)
		})

		It("should add or modify elements to etcd-events statefulset", func() {
			var (
				etcd = &druidv1alpha1.Etcd{
					ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.ETCDEvents},
				}
			)

			// Create ensurer
			ensurer := NewEnsurer(etcdStorage, logger)

			// Call EnsureETCDStatefulSet method and check the result
			err := ensurer.EnsureETCD(context.TODO(), dummyContext, etcd, nil)
			Expect(err).To(Not(HaveOccurred()))
			checkETCDEvents(etcd)
		})

		It("should modify existing elements of etcd-events statefulset", func() {
			var (
				r    = resource.MustParse("20Gi")
				etcd = &druidv1alpha1.Etcd{
					ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.ETCDEvents},
					Spec: druidv1alpha1.EtcdSpec{
						StorageCapacity: &r,
					},
				}
			)

			// Create ensurer
			ensurer := NewEnsurer(etcdStorage, logger)

			// Call EnsureETCDStatefulSet method and check the result
			err := ensurer.EnsureETCD(context.TODO(), dummyContext, etcd, nil)
			Expect(err).To(Not(HaveOccurred()))
			checkETCDEvents(etcd)
		})
	})
})

func checkETCDMain(etcd *druidv1alpha1.Etcd) {
	Expect(*etcd.Spec.StorageClass).To(Equal("gardener.cloud-fast"))
	Expect(*etcd.Spec.StorageCapacity).To(Equal(resource.MustParse("25Gi")))
}

func checkETCDEvents(etcd *druidv1alpha1.Etcd) {
	Expect(*etcd.Spec.StorageClass).To(Equal(""))
	Expect(*etcd.Spec.StorageCapacity).To(Equal(resource.MustParse("10Gi")))
}
