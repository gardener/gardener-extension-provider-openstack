// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator_test

import (
	"context"
	"encoding/json"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
	mockmanager "github.com/gardener/gardener/third_party/mock/controller-runtime/manager"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/admission/validator"
	apisopenstack "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	apisopenstackv1alpha "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"
	openstackv1alpha1 "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"
)

var _ = Describe("Shoot validator", func() {
	Describe("#Validate", func() {
		const namespace = "garden-dev"

		var (
			shootValidator extensionswebhook.Validator

			ctrl      *gomock.Controller
			mgr       *mockmanager.MockManager
			c         *mockclient.MockClient
			apiReader *mockclient.MockReader
			shoot     *core.Shoot

			ctx = context.Background()

			regionName   string
			imageName    string
			imageVersion string
			architecture *string
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())

			scheme := runtime.NewScheme()
			Expect(apisopenstack.AddToScheme(scheme)).To(Succeed())
			Expect(apisopenstackv1alpha.AddToScheme(scheme)).To(Succeed())
			Expect(gardencorev1beta1.AddToScheme(scheme)).To(Succeed())

			c = mockclient.NewMockClient(ctrl)
			apiReader = mockclient.NewMockReader(ctrl)

			mgr = mockmanager.NewMockManager(ctrl)
			mgr.EXPECT().GetScheme().Return(scheme).Times(2)
			mgr.EXPECT().GetClient().Return(c)
			mgr.EXPECT().GetAPIReader().Return(apiReader)
			shootValidator = validator.NewShootValidator(mgr)

			regionName = "eu-de-1"
			imageName = "Foo"
			imageVersion = "1.0.0"
			architecture = ptr.To("analog")

			shoot = &core.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: namespace,
				},
				Spec: core.ShootSpec{
					CloudProfile: &core.CloudProfileReference{
						Kind: "CloudProfile",
						Name: "cloudProfile",
					},
					Provider: core.Provider{
						Type: "openstack",
						Workers: []core.Worker{
							{
								Name: "worker-1",
								Volume: &core.Volume{
									VolumeSize: "50Gi",
									Type:       ptr.To("volumeType"),
								},
								Zones: []string{"zone1"},
								Machine: core.Machine{
									Image: &core.ShootMachineImage{
										Name:    imageName,
										Version: imageVersion,
									},
									Architecture: architecture,
								},
							},
						},
						InfrastructureConfig: &runtime.RawExtension{
							Raw: encode(&openstackv1alpha1.InfrastructureConfig{
								TypeMeta: metav1.TypeMeta{
									APIVersion: openstackv1alpha1.SchemeGroupVersion.String(),
									Kind:       "InfrastructureConfig",
								},
								Networks: openstackv1alpha1.Networks{
									Workers: "10.250.0.0/19",
								},
								FloatingPoolName: "pool-1",
							}),
						},
						ControlPlaneConfig: &runtime.RawExtension{
							Raw: encode(&openstackv1alpha1.ControlPlaneConfig{
								TypeMeta: metav1.TypeMeta{
									APIVersion: openstackv1alpha1.SchemeGroupVersion.String(),
									Kind:       "ControlPlaneConfig",
								},
								LoadBalancerProvider: "haproxy",
							}),
						},
					},
					Region: "eu-de-1",
					Networking: &core.Networking{
						Nodes: ptr.To("10.250.0.0/16"),
					},
				},
			}
		})

		Context("Workerless Shoot", func() {
			BeforeEach(func() {
				shoot.Spec.Provider.Workers = nil
			})

			It("should not validate", func() {
				err := shootValidator.Validate(ctx, shoot, nil)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("Shoot creation", func() {
			var (
				cloudProfileKey           client.ObjectKey
				namespacedCloudProfileKey client.ObjectKey

				cloudProfile           *gardencorev1beta1.CloudProfile
				namespacedCloudProfile *gardencorev1beta1.NamespacedCloudProfile
			)

			BeforeEach(func() {
				cloudProfileKey = client.ObjectKey{Name: "openstack"}
				namespacedCloudProfileKey = client.ObjectKey{Name: "openstack-nscpfl", Namespace: namespace}

				cloudProfile = &gardencorev1beta1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name: "openstack",
					},
					Spec: gardencorev1beta1.CloudProfileSpec{
						Regions: []gardencorev1beta1.Region{
							{
								Name: regionName,
								Zones: []gardencorev1beta1.AvailabilityZone{
									{
										Name: "zone1",
									},
									{
										Name: "zone2",
									},
								},
							},
						},
						ProviderConfig: &runtime.RawExtension{
							Raw: encode(&apisopenstackv1alpha.CloudProfileConfig{
								TypeMeta: metav1.TypeMeta{
									APIVersion: apisopenstackv1alpha.SchemeGroupVersion.String(),
									Kind:       "CloudProfileConfig",
								},
								MachineImages: []apisopenstackv1alpha.MachineImages{
									{
										Name: imageName,
										Versions: []apisopenstackv1alpha.MachineImageVersion{
											{
												Version: imageVersion,
												Regions: []apisopenstackv1alpha.RegionIDMapping{
													{
														Name:         regionName,
														ID:           "Bar",
														Architecture: architecture,
													},
												},
											},
										},
									},
								},
							}),
						},
					},
				}

				namespacedCloudProfile = &gardencorev1beta1.NamespacedCloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name: "openstack-nscpfl",
					},
					Spec: gardencorev1beta1.NamespacedCloudProfileSpec{
						Parent: gardencorev1beta1.CloudProfileReference{
							Kind: "CloudProfile",
							Name: "openstack",
						},
					},
					Status: gardencorev1beta1.NamespacedCloudProfileStatus{
						CloudProfileSpec: cloudProfile.Spec,
					},
				}
			})

			It("should work for CloudProfile referenced from Shoot", func() {
				shoot.Spec.CloudProfile = &core.CloudProfileReference{
					Kind: "CloudProfile",
					Name: "openstack",
				}
				c.EXPECT().Get(ctx, cloudProfileKey, &gardencorev1beta1.CloudProfile{}).SetArg(2, *cloudProfile)

				err := shootValidator.Validate(ctx, shoot, nil)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should work for CloudProfile referenced from cloudProfileName", func() {
				shoot.Spec.CloudProfileName = ptr.To("openstack")
				shoot.Spec.CloudProfile = nil
				c.EXPECT().Get(ctx, cloudProfileKey, &gardencorev1beta1.CloudProfile{}).SetArg(2, *cloudProfile)

				err := shootValidator.Validate(ctx, shoot, nil)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should work for NamespacedCloudProfile referenced from Shoot", func() {
				shoot.Spec.CloudProfile = &core.CloudProfileReference{
					Kind: "NamespacedCloudProfile",
					Name: "openstack-nscpfl",
				}
				c.EXPECT().Get(ctx, namespacedCloudProfileKey, &gardencorev1beta1.NamespacedCloudProfile{}).SetArg(2, *namespacedCloudProfile)

				err := shootValidator.Validate(ctx, shoot, nil)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should fail for a missing cloud profile provider config", func() {
				shoot.Spec.CloudProfile = &core.CloudProfileReference{
					Kind: "NamespacedCloudProfile",
					Name: "openstack-nscpfl",
				}
				namespacedCloudProfile.Status.CloudProfileSpec.ProviderConfig = nil
				c.EXPECT().Get(ctx, namespacedCloudProfileKey, &gardencorev1beta1.NamespacedCloudProfile{}).SetArg(2, *namespacedCloudProfile)

				err := shootValidator.Validate(ctx, shoot, nil)
				Expect(err).To(MatchError(And(
					ContainSubstring("providerConfig is not given for cloud profile"),
					ContainSubstring("NamespacedCloudProfile"),
					ContainSubstring("openstack-nscpfl"),
				)))
			})
		})
	})
})

func encode(obj runtime.Object) []byte {
	data, _ := json.Marshal(obj)
	return data
}
