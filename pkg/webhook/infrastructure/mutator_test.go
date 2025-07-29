// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"
	"testing"

	"github.com/gardener/gardener/extensions/pkg/controller"
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
	mockmanager "github.com/gardener/gardener/third_party/mock/controller-runtime/manager"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
)

const (
	shootNamespace = "shoot--foo--bar"
)

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Infrastructure Webhook Suite")
}

var _ = Describe("Mutate", func() {
	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient
		mgr  *mockmanager.MockManager
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
		mgr = mockmanager.NewMockManager(ctrl)
		mgr.EXPECT().GetClient().Return(c)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#UseFlowAnnotation", func() {
		var (
			mutator extensionswebhook.Mutator
			cluster *controller.Cluster
			ctx     context.Context
		)

		BeforeEach(func() {
			mutator = New(mgr, logger)
			ctx = context.TODO()

			c.EXPECT().Get(ctx, client.ObjectKey{Name: shootNamespace}, gomock.AssignableToTypeOf(&extensionsv1alpha1.Cluster{})).
				DoAndReturn(
					func(_ context.Context, _ types.NamespacedName, obj *extensionsv1alpha1.Cluster, _ ...client.GetOption) error {
						seedJSON, err := json.Marshal(cluster.Seed)
						Expect(err).NotTo(HaveOccurred())
						shootJSON, err := json.Marshal(cluster.Shoot)
						Expect(err).NotTo(HaveOccurred())
						*obj = extensionsv1alpha1.Cluster{
							ObjectMeta: cluster.ObjectMeta,
							Spec: extensionsv1alpha1.ClusterSpec{
								Seed:  runtime.RawExtension{Raw: seedJSON},
								Shoot: runtime.RawExtension{Raw: shootJSON},
							},
						}
						return nil
					}).AnyTimes()

			cluster = &controller.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: shootNamespace,
				},
				Seed: &gardencorev1beta1.Seed{
					ObjectMeta: metav1.ObjectMeta{
						Name:        shootNamespace,
						Annotations: map[string]string{},
					},
				},
				Shoot: &gardencorev1beta1.Shoot{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{},
					},
				},
			}
		})

		Context("infrastructure creation", func() {
			It("should add global use-flow annotation if shoot contains it", func() {
				cluster.Shoot.Annotations[openstack.GlobalAnnotationKeyUseFlow] = "foo"
				newInfra := &extensionsv1alpha1.Infrastructure{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dummy",
						Namespace: shootNamespace,
					},
				}

				err := mutator.Mutate(ctx, newInfra, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(newInfra.Annotations[openstack.AnnotationKeyUseFlow]).To(Equal("foo"))
			})

			It("should add use-flow annotation if shoot contains it", func() {
				cluster.Shoot.Annotations[openstack.AnnotationKeyUseFlow] = "foo"
				newInfra := &extensionsv1alpha1.Infrastructure{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dummy",
						Namespace: shootNamespace,
					},
				}

				err := mutator.Mutate(ctx, newInfra, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(newInfra.Annotations[openstack.AnnotationKeyUseFlow]).To(Equal("foo"))
			})

			It("should add use-flow annotation if seed label is set to new", func() {
				cluster.Seed.Annotations[openstack.SeedAnnotationKeyUseFlow] = openstack.SeedAnnotationUseFlowValueNew
				newInfra := &extensionsv1alpha1.Infrastructure{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dummy",
						Namespace: shootNamespace,
					},
				}

				err := mutator.Mutate(ctx, newInfra, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(newInfra.Annotations[openstack.AnnotationKeyUseFlow]).To(Equal("true"))
			})

			It("should add use-flow annotation if seed necessitates it", func() {
				cluster.Seed.Annotations[openstack.SeedAnnotationKeyUseFlow] = "true"
				newInfra := &extensionsv1alpha1.Infrastructure{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dummy",
						Namespace: shootNamespace,
					},
				}
				err := mutator.Mutate(ctx, newInfra, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(newInfra.Annotations[openstack.AnnotationKeyUseFlow]).To(Equal("true"))
			})
		})

		Context("update", func() {
			It("should not mutate the seed if seed annotation is set to false", func() {
				cluster.Seed.Annotations[openstack.SeedAnnotationKeyUseFlow] = "false"
				newInfra := &extensionsv1alpha1.Infrastructure{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dummy",
						Namespace: shootNamespace,
					},
				}
				err := mutator.Mutate(ctx, newInfra, newInfra)
				Expect(err).ToNot(HaveOccurred())
				Expect(newInfra.Annotations[openstack.AnnotationKeyUseFlow]).To(Equal(""))
			})
			It("should mutate the seed if seed annotation is not set to false", func() {
				newInfra := &extensionsv1alpha1.Infrastructure{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dummy",
						Namespace: shootNamespace,
					},
				}
				err := mutator.Mutate(ctx, newInfra, newInfra)
				Expect(err).ToNot(HaveOccurred())
				Expect(newInfra.Annotations[openstack.AnnotationKeyUseFlow]).To(Equal("true"))
			})

			It("should mutate if seed annotation is set to all shoots", func() {
				cluster.Seed.Annotations[openstack.SeedAnnotationKeyUseFlow] = "true"
				newInfra := &extensionsv1alpha1.Infrastructure{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dummy",
						Namespace: shootNamespace,
					},
				}
				err := mutator.Mutate(ctx, newInfra, newInfra)
				Expect(err).ToNot(HaveOccurred())
				Expect(newInfra.Annotations[openstack.AnnotationKeyUseFlow]).To(Equal("true"))
			})

			It("should mutate if shoot annotation is set", func() {
				cluster.Shoot.Annotations[openstack.GlobalAnnotationKeyUseFlow] = "foo"
				newInfra := &extensionsv1alpha1.Infrastructure{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dummy",
						Namespace: shootNamespace,
					},
				}
				err := mutator.Mutate(ctx, newInfra, newInfra)
				Expect(err).ToNot(HaveOccurred())
				Expect(newInfra.Annotations[openstack.AnnotationKeyUseFlow]).To(Equal("foo"))
			})
		})

		Context("infrastructure deletion", func() {
			It("should do nothing if infra is deleted", func() {
				cluster.Shoot.Annotations[openstack.GlobalAnnotationKeyUseFlow] = "foo"

				newInfra := &extensionsv1alpha1.Infrastructure{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "dummy",
						Namespace:         shootNamespace,
						DeletionTimestamp: ptr.To(metav1.Now()),
					},
				}

				err := mutator.Mutate(ctx, newInfra, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(newInfra.Annotations[openstack.AnnotationKeyUseFlow]).To(Equal(""))
			})

			It("should do nothing if shoot is deleted", func() {
				cluster.Shoot.Annotations[openstack.GlobalAnnotationKeyUseFlow] = "foo"
				cluster.Shoot.DeletionTimestamp = ptr.To(metav1.Now())

				newInfra := &extensionsv1alpha1.Infrastructure{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dummy",
						Namespace: shootNamespace,
					},
				}

				err := mutator.Mutate(ctx, newInfra, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(newInfra.Annotations[openstack.AnnotationKeyUseFlow]).To(Equal(""))
			})
		})
	})
})
