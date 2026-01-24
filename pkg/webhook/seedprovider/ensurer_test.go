// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seedprovider

import (
	"context"
	"testing"

	druidcorev1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	gcontext "github.com/gardener/gardener/extensions/pkg/webhook/context"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/config"
)

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Seedprovider Webhook Suite")
}

var _ = Describe("Ensurer", func() {
	var (
		etcdStorage = &config.ETCDStorage{
			ClassName: ptr.To("gardener.cloud-fast"),
			Capacity:  ptr.To(resource.MustParse("25Gi")),
		}

		eventsStorage = &config.ETCDStorage{
			ClassName: ptr.To("gardener.cloud-fast"),
			Capacity:  ptr.To(resource.MustParse("25Gi")),
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
				etcd = &druidcorev1alpha1.Etcd{
					ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.ETCDMain},
				}
			)

			// Create ensurer
			ensurer := NewEnsurer(etcdStorage, nil, logger)

			// Call EnsureETCDStatefulSet method and check the result
			err := ensurer.EnsureETCD(context.TODO(), dummyContext, etcd, nil)
			Expect(err).To(Not(HaveOccurred()))
			checkETCDMain(etcd)
		})

		It("should modify existing elements of etcd-main statefulset", func() {
			var (
				r    = resource.MustParse("10Gi")
				etcd = &druidcorev1alpha1.Etcd{
					ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.ETCDMain},
					Spec: druidcorev1alpha1.EtcdSpec{
						StorageCapacity: &r,
					},
				}
			)

			// Create ensurer
			ensurer := NewEnsurer(etcdStorage, nil, logger)

			// Call EnsureETCDStatefulSet method and check the result
			err := ensurer.EnsureETCD(context.TODO(), dummyContext, etcd, nil)
			Expect(err).To(Not(HaveOccurred()))
			checkETCDMain(etcd)
		})

		It("should keep default behavior for etcd-events when events config is not provided", func() {
			var (
				etcd = &druidcorev1alpha1.Etcd{
					ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.ETCDEvents},
				}
			)

			// Create ensurer (events storage NOT configured)
			ensurer := NewEnsurer(etcdStorage, nil, logger)

			// Call EnsureETCDStatefulSet method and check the result
			err := ensurer.EnsureETCD(context.TODO(), dummyContext, etcd, nil)
			Expect(err).To(Not(HaveOccurred()))
			checkETCDEventsDefault(etcd)
		})

		It("should keep default behavior for etcd-events even if it already has different capacity when events config is not provided", func() {
			var (
				r    = resource.MustParse("20Gi")
				etcd = &druidcorev1alpha1.Etcd{
					ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.ETCDEvents},
					Spec: druidcorev1alpha1.EtcdSpec{
						StorageCapacity: &r,
					},
				}
			)

			// Create ensurer (events storage NOT configured)
			ensurer := NewEnsurer(etcdStorage, nil, logger)

			// Call EnsureETCDStatefulSet method and check the result
			err := ensurer.EnsureETCD(context.TODO(), dummyContext, etcd, nil)
			Expect(err).To(Not(HaveOccurred()))
			checkETCDEventsDefault(etcd)
		})

		It("should add or modify elements to etcd-events statefulset when events config is provided", func() {
			var (
				etcd = &druidcorev1alpha1.Etcd{
					ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.ETCDEvents},
				}
			)

			// Create ensurer (events storage configured)
			ensurer := NewEnsurer(etcdStorage, eventsStorage, logger)

			// Call EnsureETCDStatefulSet method and check the result
			err := ensurer.EnsureETCD(context.TODO(), dummyContext, etcd, nil)
			Expect(err).To(Not(HaveOccurred()))
			checkETCDEventsConfigured(etcd)
		})

		It("should modify existing elements of etcd-events statefulset when events config is provided", func() {
			var (
				r    = resource.MustParse("20Gi")
				etcd = &druidcorev1alpha1.Etcd{
					ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.ETCDEvents},
					Spec: druidcorev1alpha1.EtcdSpec{
						StorageCapacity: &r,
					},
				}
			)

			// Create ensurer (events storage configured)
			ensurer := NewEnsurer(etcdStorage, eventsStorage, logger)

			// Call EnsureETCDStatefulSet method and check the result
			err := ensurer.EnsureETCD(context.TODO(), dummyContext, etcd, nil)
			Expect(err).To(Not(HaveOccurred()))
			checkETCDEventsConfigured(etcd)
		})

		It("should return an error for an unknown ETCD name", func() {
			etcd := &druidcorev1alpha1.Etcd{
				ObjectMeta: metav1.ObjectMeta{Name: "some-unknown-etcd"},
			}

			// Create ensurer (events storage not configured)
			ensurer := NewEnsurer(etcdStorage, nil, logger)

			// Call EnsureETCDStatefulSet method and check the result
			err := ensurer.EnsureETCD(context.TODO(), dummyContext, etcd, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unknown ETCD name"))
		})
	})
})

func checkETCDMain(etcd *druidcorev1alpha1.Etcd) {
	Expect(etcd.Spec.StorageClass).ToNot(BeNil())
	Expect(*etcd.Spec.StorageClass).To(Equal("gardener.cloud-fast"))

	Expect(etcd.Spec.StorageCapacity).ToNot(BeNil())
	Expect(*etcd.Spec.StorageCapacity).To(Equal(resource.MustParse("25Gi")))
}

func checkETCDEventsDefault(etcd *druidcorev1alpha1.Etcd) {
	Expect(etcd.Spec.StorageClass).ToNot(BeNil())
	Expect(*etcd.Spec.StorageClass).To(Equal(""))

	Expect(etcd.Spec.StorageCapacity).ToNot(BeNil())
	Expect(*etcd.Spec.StorageCapacity).To(Equal(resource.MustParse("10Gi")))
}

func checkETCDEventsConfigured(etcd *druidcorev1alpha1.Etcd) {
	Expect(etcd.Spec.StorageClass).ToNot(BeNil())
	Expect(*etcd.Spec.StorageClass).To(Equal("gardener.cloud-fast"))

	Expect(etcd.Spec.StorageCapacity).ToNot(BeNil())
	Expect(*etcd.Spec.StorageCapacity).To(Equal(resource.MustParse("25Gi")))
}
