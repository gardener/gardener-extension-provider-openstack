// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package mutator_test

import (
	"context"
	"encoding/json"
	"time"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	mockmanager "github.com/gardener/gardener/pkg/mock/controller-runtime/manager"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/admission/mutator"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
)

var _ = Describe("Shoot mutator", func() {
	Describe("#Mutate", func() {
		const namespace = "garden-dev"

		var (
			ctrl         *gomock.Controller
			mgr          *mockmanager.MockManager
			shootMutator extensionswebhook.Mutator
			shoot        *gardencorev1beta1.Shoot
			oldShoot     *gardencorev1beta1.Shoot
			ctx          = context.TODO()
			now          = metav1.Now()
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())

			scheme := runtime.NewScheme()
			Expect(gardencorev1beta1.AddToScheme(scheme)).To(Succeed())

			mgr = mockmanager.NewMockManager(ctrl)
			mgr.EXPECT().GetScheme().Return(scheme)

			shootMutator = mutator.NewShootMutator(mgr)

			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: namespace,
				},
				Spec: gardencorev1beta1.ShootSpec{
					SeedName: pointer.String("openstack"),
					Provider: gardencorev1beta1.Provider{
						Type: openstack.Type,
						Workers: []gardencorev1beta1.Worker{
							{
								Name: "worker",
							},
						},
					},
					Region: "eu-fr-1",
					Networking: &gardencorev1beta1.Networking{
						Nodes: pointer.String("10.250.0.0/16"),
						Type:  pointer.String("calico"),
					},
				},
			}

			oldShoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: namespace,
				},
				Spec: gardencorev1beta1.ShootSpec{
					SeedName: pointer.String("openstack"),
					Provider: gardencorev1beta1.Provider{
						Type: openstack.Type,
					},
					Region: "eu-fr-1",
					Networking: &gardencorev1beta1.Networking{
						Nodes: pointer.String("10.250.0.0/16"),
						Type:  pointer.String("calico"),
					},
				},
			}
		})

		Context("Mutate shoot networking providerconfig for type calico", func() {
			It("should return without mutation when shoot is in scheduled to new seed phase", func() {
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Description:    "test",
					LastUpdateTime: metav1.Time{Time: metav1.Now().Add(time.Second * -1000)},
					Progress:       0,
					Type:           gardencorev1beta1.LastOperationTypeReconcile,
					State:          gardencorev1beta1.LastOperationStateProcessing,
				}
				shoot.Status.SeedName = pointer.String("aws")
				shootExpected := shoot.DeepCopy()
				err := shootMutator.Mutate(ctx, shoot, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(shoot).To(DeepEqual(shootExpected))
			})

			It("should return without mutation when shoot is in migration or restore phase", func() {
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Description:    "test",
					LastUpdateTime: metav1.Time{Time: metav1.Now().Add(time.Second * -1000)},
					Progress:       0,
					Type:           gardencorev1beta1.LastOperationTypeMigrate,
					State:          gardencorev1beta1.LastOperationStateProcessing,
				}
				shoot.Status.SeedName = pointer.String("openstack")
				shootExpected := shoot.DeepCopy()
				err := shootMutator.Mutate(ctx, shoot, shoot)
				Expect(err).NotTo(HaveOccurred())
				Expect(shoot).To(DeepEqual(shootExpected))
			})

			It("should return without mutation when shoot is in deletion phase", func() {
				shoot.DeletionTimestamp = &now
				shootExpected := shoot.DeepCopy()
				err := shootMutator.Mutate(ctx, shoot, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(shoot).To(DeepEqual(shootExpected))
			})

			It("should return nil when shoot specs have not changed", func() {
				shootWithAnnotations := shoot.DeepCopy()
				shootWithAnnotations.Annotations = map[string]string{"foo": "bar"}
				shootExpected := shootWithAnnotations.DeepCopy()

				err := shootMutator.Mutate(ctx, shootWithAnnotations, shoot)
				Expect(err).To(BeNil())
				Expect(shootWithAnnotations).To(DeepEqual(shootExpected))
			})

			It("should return nil when shoot specs have not changed", func() {
				shootWithAnnotations := shoot.DeepCopy()
				shootWithAnnotations.Annotations = map[string]string{"foo": "bar"}
				shootExpected := shootWithAnnotations.DeepCopy()

				err := shootMutator.Mutate(ctx, shootWithAnnotations, shoot)
				Expect(err).To(BeNil())
				Expect(shootWithAnnotations).To(DeepEqual(shootExpected))
			})

			It("should disable overlay for a new shoot", func() {
				err := shootMutator.Mutate(ctx, shoot, nil)
				Expect(err).NotTo(HaveOccurred())
				var networkConfig, expectedConfig map[string]interface{}
				err = json.Unmarshal(shoot.Spec.Networking.ProviderConfig.Raw, &networkConfig)
				Expect(err).NotTo(HaveOccurred())
				err = json.Unmarshal([]byte(`{"overlay": {"enabled": false, "createPodRoutes": true}}`), &expectedConfig)
				Expect(err).NotTo(HaveOccurred())
				Expect(networkConfig).To(DeepEqual(expectedConfig))
			})

			It("should disable overlay for a new shoot non empty network config", func() {
				shoot.Spec.Networking.ProviderConfig = &runtime.RawExtension{
					Raw: []byte(`{"foo":{"enabled":true}}`),
				}
				err := shootMutator.Mutate(ctx, shoot, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Spec.Networking.ProviderConfig).To(Equal(&runtime.RawExtension{
					Raw: []byte(`{"foo":{"enabled":true},"overlay":{"createPodRoutes":true,"enabled":false}}`),
				}))
			})

			It("should take overlay field value from old shoot when unspecified in new shoot", func() {
				oldShoot.Spec.Networking.ProviderConfig = &runtime.RawExtension{
					Raw: []byte(`{"overlay":{"enabled":true}}`),
				}
				err := shootMutator.Mutate(ctx, shoot, oldShoot)
				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Spec.Networking.ProviderConfig).To(Equal(&runtime.RawExtension{
					Raw: []byte(`{"overlay":{"enabled":true}}`),
				}))
			})
		})

		It("should disable overlay for a new shoot when unspecified in new and old shoot", func() {
			err := shootMutator.Mutate(ctx, shoot, oldShoot)
			Expect(err).NotTo(HaveOccurred())
			Expect(shoot.Spec.Networking.ProviderConfig).To(Equal(&runtime.RawExtension{
				Raw: []byte(`{"overlay":{"createPodRoutes":true,"enabled":false}}`),
			}))
		})

		Context("Mutate shoot networking providerconfig for type cilium", func() {
			BeforeEach(func() {
				shoot.Spec.Networking.Type = pointer.String("cilium")
				oldShoot.Spec.Networking.Type = pointer.String("cilium")
			})

			It("should return without mutation when shoot is in scheduled to new seed phase", func() {
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Description:    "test",
					LastUpdateTime: metav1.Time{Time: metav1.Now().Add(time.Second * -1000)},
					Progress:       0,
					Type:           gardencorev1beta1.LastOperationTypeReconcile,
					State:          gardencorev1beta1.LastOperationStateProcessing,
				}
				shoot.Status.SeedName = pointer.String("aws")
				shootExpected := shoot.DeepCopy()
				err := shootMutator.Mutate(ctx, shoot, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(shoot).To(DeepEqual(shootExpected))
			})

			It("should return without mutation when shoot is in migration or restore phase", func() {
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Description:    "test",
					LastUpdateTime: metav1.Time{Time: metav1.Now().Add(time.Second * -1000)},
					Progress:       0,
					Type:           gardencorev1beta1.LastOperationTypeMigrate,
					State:          gardencorev1beta1.LastOperationStateProcessing,
				}
				shoot.Status.SeedName = pointer.String("openstack")
				shootExpected := shoot.DeepCopy()
				err := shootMutator.Mutate(ctx, shoot, shoot)
				Expect(err).NotTo(HaveOccurred())
				Expect(shoot).To(DeepEqual(shootExpected))
			})

			It("should return without mutation when shoot is in deletion phase", func() {
				shoot.DeletionTimestamp = &now
				shootExpected := shoot.DeepCopy()
				err := shootMutator.Mutate(ctx, shoot, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(shoot).To(DeepEqual(shootExpected))
			})

			It("should disable overlay for a new shoot", func() {
				err := shootMutator.Mutate(ctx, shoot, nil)
				Expect(err).NotTo(HaveOccurred())
				var networkConfig, expectedConfig interface{}
				err = json.Unmarshal(shoot.Spec.Networking.ProviderConfig.Raw, &networkConfig)
				Expect(err).NotTo(HaveOccurred())
				err = json.Unmarshal([]byte(`{"overlay": {"enabled": false, "createPodRoutes": true}}`), &expectedConfig)
				Expect(err).NotTo(HaveOccurred())
				Expect(networkConfig).To(DeepEqual(expectedConfig))
			})

			It("should take overlay field value from old shoot when unspecified in new shoot", func() {
				oldShoot.Spec.Networking.ProviderConfig = &runtime.RawExtension{
					Raw: []byte(`{"overlay":{"enabled":true}}`),
				}
				err := shootMutator.Mutate(ctx, shoot, oldShoot)
				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Spec.Networking.ProviderConfig).To(Equal(&runtime.RawExtension{
					Raw: []byte(`{"overlay":{"enabled":true}}`),
				}))
			})
		})

		Context("Workerless Shoot", func() {
			BeforeEach(func() {
				shoot.Spec.Provider.Workers = nil
			})

			It("should return without mutation", func() {
				shootExpected := shoot.DeepCopy()
				err := shootMutator.Mutate(ctx, shoot, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(shoot).To(DeepEqual(shootExpected))
			})
		})
	})
})
