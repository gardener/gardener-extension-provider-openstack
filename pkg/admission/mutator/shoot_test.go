// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package mutator_test

import (
	"context"
	"encoding/json"
	"time"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/admission/mutator"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
)

var _ = Describe("Shoot mutator", func() {
	Describe("#Mutate", func() {
		const namespace = "garden-dev"

		var (
			shootMutator extensionswebhook.Mutator
			shoot        *gardencorev1beta1.Shoot
			oldShoot     *gardencorev1beta1.Shoot
			ctx          = context.TODO()
			now          = metav1.Now()
		)

		BeforeEach(func() {
			shootMutator = mutator.NewShootMutator()

			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: namespace,
				},
				Spec: gardencorev1beta1.ShootSpec{
					SeedName: ptr.To("openstack"),
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
						Nodes: ptr.To("10.250.0.0/16"),
						Type:  ptr.To("calico"),
					},
				},
			}

			oldShoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: namespace,
				},
				Spec: gardencorev1beta1.ShootSpec{
					SeedName: ptr.To("openstack"),
					Provider: gardencorev1beta1.Provider{
						Type: openstack.Type,
					},
					Region: "eu-fr-1",
					Networking: &gardencorev1beta1.Networking{
						Nodes: ptr.To("10.250.0.0/16"),
						Type:  ptr.To("calico"),
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
				shoot.Status.SeedName = ptr.To("aws")
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
				shoot.Status.SeedName = ptr.To("openstack")
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
				Expect(err).ToNot(HaveOccurred())
				Expect(shootWithAnnotations).To(DeepEqual(shootExpected))
			})

			It("should return nil when shoot specs have not changed", func() {
				shootWithAnnotations := shoot.DeepCopy()
				shootWithAnnotations.Annotations = map[string]string{"foo": "bar"}
				shootExpected := shootWithAnnotations.DeepCopy()

				err := shootMutator.Mutate(ctx, shootWithAnnotations, shoot)
				Expect(err).ToNot(HaveOccurred())
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
				shoot.Spec.Networking.Type = ptr.To("cilium")
				oldShoot.Spec.Networking.Type = ptr.To("cilium")
			})

			It("should return without mutation when shoot is in scheduled to new seed phase", func() {
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Description:    "test",
					LastUpdateTime: metav1.Time{Time: metav1.Now().Add(time.Second * -1000)},
					Progress:       0,
					Type:           gardencorev1beta1.LastOperationTypeReconcile,
					State:          gardencorev1beta1.LastOperationStateProcessing,
				}
				shoot.Status.SeedName = ptr.To("aws")
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
				shoot.Status.SeedName = ptr.To("openstack")
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

		Context("Mutate shoot NodeLocalDNS default for ForceTCPToUpstreamDNS property", func() {
			BeforeEach(func() {
				shoot.Spec.SystemComponents = &gardencorev1beta1.SystemComponents{
					NodeLocalDNS: &gardencorev1beta1.NodeLocalDNS{
						Enabled: true,
					},
				}
			})

			It("should not touch the ForceTCPToUpstreamDNS property if NodeLocalDNS is disabled", func() {
				shoot.Spec.SystemComponents.NodeLocalDNS.Enabled = false
				err := shootMutator.Mutate(ctx, shoot, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Spec.SystemComponents.NodeLocalDNS.ForceTCPToUpstreamDNS).To(BeNil())
			})

			It("should not touch the ForceTCPToUpstreamDNS property if it is already set", func() {
				shoot.Spec.SystemComponents.NodeLocalDNS.ForceTCPToUpstreamDNS = ptr.To(true)
				err := shootMutator.Mutate(ctx, shoot, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Spec.SystemComponents.NodeLocalDNS.ForceTCPToUpstreamDNS).ToNot(BeNil())
				Expect(*shoot.Spec.SystemComponents.NodeLocalDNS.ForceTCPToUpstreamDNS).To(BeTrue())
			})

			It("should set the ForceTCPToUpstreamDNS property to false by default", func() {
				err := shootMutator.Mutate(ctx, shoot, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Spec.SystemComponents.NodeLocalDNS.ForceTCPToUpstreamDNS).ToNot(BeNil())
				Expect(*shoot.Spec.SystemComponents.NodeLocalDNS.ForceTCPToUpstreamDNS).To(BeFalse())
			})
		})
	})
})
