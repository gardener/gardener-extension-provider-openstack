// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package mutator_test

import (
	"context"

	"github.com/gardener/gardener/extensions/pkg/util"
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/admission/mutator"
	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/install"
)

var _ = Describe("NamespacedCloudProfile Mutator", func() {
	var (
		fakeClient  client.Client
		fakeManager manager.Manager
		namespace   string
		ctx         = context.Background()
		decoder     runtime.Decoder

		namespacedCloudProfileMutator extensionswebhook.Mutator
		namespacedCloudProfile        *v1beta1.NamespacedCloudProfile
	)

	BeforeEach(func() {
		scheme := runtime.NewScheme()
		utilruntime.Must(install.AddToScheme(scheme))
		utilruntime.Must(v1beta1.AddToScheme(scheme))
		fakeClient = fakeclient.NewClientBuilder().WithScheme(scheme).Build()
		fakeManager = &test.FakeManager{
			Client: fakeClient,
			Scheme: scheme,
		}
		namespace = "garden-dev"
		decoder = serializer.NewCodecFactory(fakeManager.GetScheme(), serializer.EnableStrict).UniversalDecoder()

		namespacedCloudProfileMutator = mutator.NewNamespacedCloudProfileMutator(fakeManager)
		namespacedCloudProfile = &v1beta1.NamespacedCloudProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "profile-1",
				Namespace: namespace,
			},
			Spec: v1beta1.NamespacedCloudProfileSpec{
				Parent: v1beta1.CloudProfileReference{
					Kind: "CloudProfile",
					Name: "parent-profile",
				},
			},
		}
	})

	Describe("#Mutate", func() {
		It("should succeed for NamespacedCloudProfile without provider config", func() {
			Expect(namespacedCloudProfileMutator.Mutate(ctx, namespacedCloudProfile, nil)).To(Succeed())
		})

		It("should skip if NamespacedCloudProfile is in deletion phase", func() {
			namespacedCloudProfile.DeletionTimestamp = ptr.To(metav1.Now())
			expectedProfile := namespacedCloudProfile.DeepCopy()

			Expect(namespacedCloudProfileMutator.Mutate(ctx, namespacedCloudProfile, nil)).To(Succeed())

			Expect(namespacedCloudProfile).To(DeepEqual(expectedProfile))
		})

		Describe("populate capabilityFlavors on spec.machineImages", func() {
			It("should skip if parent has no machineCapabilities", func() {
				Expect(fakeClient.Create(ctx, &v1beta1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{Name: "parent-profile"},
					Spec: v1beta1.CloudProfileSpec{
						MachineCapabilities: nil,
					},
				})).To(Succeed())

				namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{
"apiVersion":"openstack.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[
  {"name":"image-1","versions":[{"version":"1.0","capabilityFlavors":[
{"capabilities":{"architecture":["amd64"]},"regions":[{"name":"eu1","id":"id-1"}]}
]}]}
]}`)}
				namespacedCloudProfile.Spec.MachineImages = []v1beta1.MachineImage{
					{Name: "image-1", Versions: []v1beta1.MachineImageVersion{
						{ExpirableVersion: v1beta1.ExpirableVersion{Version: "1.0"}},
					}},
				}

				expectedImages := namespacedCloudProfile.Spec.MachineImages

				Expect(namespacedCloudProfileMutator.Mutate(ctx, namespacedCloudProfile, nil)).To(Succeed())
				Expect(namespacedCloudProfile.Spec.MachineImages).To(Equal(expectedImages))
			})

			It("should populate capabilityFlavors from new-format provider config", func() {
				Expect(fakeClient.Create(ctx, &v1beta1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{Name: "parent-profile"},
					Spec: v1beta1.CloudProfileSpec{
						MachineCapabilities: []v1beta1.CapabilityDefinition{{
							Name:   "architecture",
							Values: []string{"amd64", "arm64"},
						}},
					},
				})).To(Succeed())

				namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{
"apiVersion":"openstack.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[
  {"name":"image-1","versions":[{"version":"1.0","capabilityFlavors":[
{"capabilities":{"architecture":["amd64"]},"regions":[{"name":"eu1","id":"id-amd64"}]},
{"capabilities":{"architecture":["arm64"]},"regions":[{"name":"eu1","id":"id-arm64"}]}
]}]}
]}`)}
				namespacedCloudProfile.Spec.MachineImages = []v1beta1.MachineImage{
					{Name: "image-1", Versions: []v1beta1.MachineImageVersion{
						{ExpirableVersion: v1beta1.ExpirableVersion{Version: "1.0"}},
					}},
				}

				Expect(namespacedCloudProfileMutator.Mutate(ctx, namespacedCloudProfile, nil)).To(Succeed())
				Expect(namespacedCloudProfile.Spec.MachineImages).To(ConsistOf(
					MatchFields(IgnoreExtras, Fields{
						"Name": Equal("image-1"),
						"Versions": ConsistOf(
							MatchFields(IgnoreExtras, Fields{
								"ExpirableVersion": MatchFields(IgnoreExtras, Fields{"Version": Equal("1.0")}),
								"CapabilityFlavors": ConsistOf(
									MatchFields(IgnoreExtras, Fields{
										"Capabilities": Equal(v1beta1.Capabilities{"architecture": []string{"amd64"}}),
									}),
									MatchFields(IgnoreExtras, Fields{
										"Capabilities": Equal(v1beta1.Capabilities{"architecture": []string{"arm64"}}),
									}),
								),
							}),
						),
					}),
				))
			})

			It("should populate capabilityFlavors from old-format regions provider config", func() {
				Expect(fakeClient.Create(ctx, &v1beta1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{Name: "parent-profile"},
					Spec: v1beta1.CloudProfileSpec{
						MachineCapabilities: []v1beta1.CapabilityDefinition{{
							Name:   "architecture",
							Values: []string{"amd64", "arm64"},
						}},
					},
				})).To(Succeed())

				namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{
"apiVersion":"openstack.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[
  {"name":"image-1","versions":[{"version":"1.0","regions":[
{"name":"eu1","id":"id-amd64","architecture":"amd64"},
{"name":"eu1","id":"id-arm64","architecture":"arm64"}
]}]}
]}`)}
				namespacedCloudProfile.Spec.MachineImages = []v1beta1.MachineImage{
					{Name: "image-1", Versions: []v1beta1.MachineImageVersion{
						{ExpirableVersion: v1beta1.ExpirableVersion{Version: "1.0"}},
					}},
				}

				Expect(namespacedCloudProfileMutator.Mutate(ctx, namespacedCloudProfile, nil)).To(Succeed())
				Expect(namespacedCloudProfile.Spec.MachineImages).To(ConsistOf(
					MatchFields(IgnoreExtras, Fields{
						"Name": Equal("image-1"),
						"Versions": ConsistOf(
							MatchFields(IgnoreExtras, Fields{
								"ExpirableVersion": MatchFields(IgnoreExtras, Fields{"Version": Equal("1.0")}),
								"CapabilityFlavors": ConsistOf(
									MatchFields(IgnoreExtras, Fields{
										"Capabilities": Equal(v1beta1.Capabilities{"architecture": []string{"amd64"}}),
									}),
									MatchFields(IgnoreExtras, Fields{
										"Capabilities": Equal(v1beta1.Capabilities{"architecture": []string{"arm64"}}),
									}),
								),
							}),
						),
					}),
				))
			})
		})

		Describe("merge the provider configurations from a NamespacedCloudProfile and the parent CloudProfile", func() {
			BeforeEach(func() {
				// Create a parent profile without machineCapabilities for status merge tests
				Expect(fakeClient.Create(ctx, &v1beta1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{Name: "parent-profile"},
				})).To(Succeed())
			})

			It("should correctly merge extended machineImages", func() {
				namespacedCloudProfile.Status.CloudProfileSpec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{
"apiVersion":"openstack.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[
  {"name":"image-1","versions":[{"version":"1.0","image":"image-name-1","regions":[{"name":"image-region-1","id":"id-img-reg-1"}]}]}
]}`)}
				namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{
"apiVersion":"openstack.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[
  {"name":"image-1","versions":[{"version":"1.1","image":"image-name-2","regions":[{"name":"image-region-2","id":"id-img-reg-2","architecture":"armhf"}]}]},
  {"name":"image-2","versions":[{"version":"2.0","image":"image-name-3","regions":[{"name":"image-region-3","id":"id-img-reg-3"}]}]}
]}`)}

				Expect(namespacedCloudProfileMutator.Mutate(ctx, namespacedCloudProfile, nil)).To(Succeed())

				mergedConfig, err := decodeCloudProfileConfig(decoder, namespacedCloudProfile.Status.CloudProfileSpec.ProviderConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(mergedConfig.MachineImages).To(ConsistOf(
					MatchFields(IgnoreExtras, Fields{
						"Name": Equal("image-1"),
						"Versions": ContainElements(
							api.MachineImageVersion{Version: "1.0", Image: "image-name-1", Regions: []api.RegionIDMapping{{Name: "image-region-1", ID: "id-img-reg-1"}}},
							api.MachineImageVersion{Version: "1.1", Image: "image-name-2", Regions: []api.RegionIDMapping{{Name: "image-region-2", ID: "id-img-reg-2", Architecture: ptr.To("armhf")}}},
						),
					}),
					MatchFields(IgnoreExtras, Fields{
						"Name":     Equal("image-2"),
						"Versions": ContainElements(api.MachineImageVersion{Version: "2.0", Image: "image-name-3", Regions: []api.RegionIDMapping{{Name: "image-region-3", ID: "id-img-reg-3"}}}),
					}),
				))
			})
			It("should correctly merge extended machineImages using capabilities ", func() {
				Expect(fakeClient.Delete(ctx, &v1beta1.CloudProfile{ObjectMeta: metav1.ObjectMeta{Name: "parent-profile"}})).To(Succeed())
				Expect(fakeClient.Create(ctx, &v1beta1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{Name: "parent-profile"},
					Spec: v1beta1.CloudProfileSpec{
						MachineCapabilities: []v1beta1.CapabilityDefinition{{
							Name:   "architecture",
							Values: []string{"amd64", "arm64"},
						}},
					},
				})).To(Succeed())

				namespacedCloudProfile.Status.CloudProfileSpec.MachineCapabilities = []v1beta1.CapabilityDefinition{{
					Name:   "architecture",
					Values: []string{"amd64", "arm64"},
				}}
				namespacedCloudProfile.Status.CloudProfileSpec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{
"apiVersion":"openstack.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[
  {"name":"image-1","versions":[{"version":"1.0","capabilityFlavors":[
{"capabilities":{"architecture":["amd64"]},"regions":[{"name":"eu1","id":"id-img-reg-1"}]}
]}]}
]}`)}
				namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{
"apiVersion":"openstack.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[
  {"name":"image-1","versions":[{"version":"1.1","capabilityFlavors":[
{"capabilities":{"architecture":["arm64"]},"regions":[{"name":"eu2","id":"id-img-reg-2"}]}
]}]},
  {"name":"image-2","versions":[{"version":"2.0","capabilityFlavors":[
{"capabilities":{"architecture":["amd64"]},"regions":[{"name":"eu3","id":"id-img-reg-3"}]}
]}]}
]}`)}

				Expect(namespacedCloudProfileMutator.Mutate(ctx, namespacedCloudProfile, nil)).To(Succeed())

				mergedConfig, err := decodeCloudProfileConfig(decoder, namespacedCloudProfile.Status.CloudProfileSpec.ProviderConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(mergedConfig.MachineImages).To(ConsistOf(
					MatchFields(IgnoreExtras, Fields{
						"Name": Equal("image-1"),
						"Versions": ContainElements(
							api.MachineImageVersion{Version: "1.0",
								CapabilityFlavors: []api.MachineImageFlavor{{
									Capabilities: v1beta1.Capabilities{"architecture": []string{"amd64"}},
									Regions:      []api.RegionIDMapping{{Name: "eu1", ID: "id-img-reg-1"}},
								}},
							},
							api.MachineImageVersion{Version: "1.1",
								CapabilityFlavors: []api.MachineImageFlavor{{
									Capabilities: v1beta1.Capabilities{"architecture": []string{"arm64"}},
									Regions:      []api.RegionIDMapping{{Name: "eu2", ID: "id-img-reg-2"}},
								}},
							},
						),
					}),
					MatchFields(IgnoreExtras, Fields{
						"Name": Equal("image-2"),
						"Versions": ContainElements(
							api.MachineImageVersion{Version: "2.0",
								CapabilityFlavors: []api.MachineImageFlavor{{
									Capabilities: v1beta1.Capabilities{"architecture": []string{"amd64"}},
									Regions:      []api.RegionIDMapping{{Name: "eu3", ID: "id-img-reg-3"}},
								}},
							}),
					}),
				))
			})

			It("should correctly merge mixed format machineImages preserving both old and new format", func() {
				Expect(fakeClient.Delete(ctx, &v1beta1.CloudProfile{ObjectMeta: metav1.ObjectMeta{Name: "parent-profile"}})).To(Succeed())
				Expect(fakeClient.Create(ctx, &v1beta1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{Name: "parent-profile"},
					Spec: v1beta1.CloudProfileSpec{
						MachineCapabilities: []v1beta1.CapabilityDefinition{{
							Name:   "architecture",
							Values: []string{"amd64", "arm64"},
						}},
					},
				})).To(Succeed())

				namespacedCloudProfile.Status.CloudProfileSpec.MachineCapabilities = []v1beta1.CapabilityDefinition{{
					Name:   "architecture",
					Values: []string{"amd64", "arm64"},
				}}
				// Parent status has new-format capabilityFlavors
				namespacedCloudProfile.Status.CloudProfileSpec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{
"apiVersion":"openstack.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[
  {"name":"image-1","versions":[{"version":"1.0","capabilityFlavors":[
{"capabilities":{"architecture":["amd64"]},"regions":[{"name":"eu1","id":"id-cap-amd64"}]}
]}]}
]}`)}
				// Spec has mixed: one version old-format regions, one version new-format capabilityFlavors
				namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{
"apiVersion":"openstack.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[
  {"name":"image-1","versions":[
    {"version":"1.1","regions":[
      {"name":"eu1","id":"id-old-amd64","architecture":"amd64"},
      {"name":"eu1","id":"id-old-arm64","architecture":"arm64"}
    ]}
  ]},
  {"name":"image-2","versions":[
    {"version":"2.0","capabilityFlavors":[
      {"capabilities":{"architecture":["amd64"]},"regions":[{"name":"eu1","id":"id-new-amd64"}]},
      {"capabilities":{"architecture":["arm64"]},"regions":[{"name":"eu1","id":"id-new-arm64"}]}
    ]}
  ]}
]}`)}

				Expect(namespacedCloudProfileMutator.Mutate(ctx, namespacedCloudProfile, nil)).To(Succeed())

				mergedConfig, err := decodeCloudProfileConfig(decoder, namespacedCloudProfile.Status.CloudProfileSpec.ProviderConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(mergedConfig.MachineImages).To(ConsistOf(
					MatchFields(IgnoreExtras, Fields{
						"Name": Equal("image-1"),
						"Versions": ContainElements(
							// Parent version preserved as-is (new format)
							api.MachineImageVersion{Version: "1.0",
								CapabilityFlavors: []api.MachineImageFlavor{{
									Capabilities: v1beta1.Capabilities{"architecture": []string{"amd64"}},
									Regions:      []api.RegionIDMapping{{Name: "eu1", ID: "id-cap-amd64"}},
								}},
							},
							// Spec version preserved as-is (old format with regions)
							api.MachineImageVersion{Version: "1.1",
								Regions: []api.RegionIDMapping{
									{Name: "eu1", ID: "id-old-amd64", Architecture: ptr.To("amd64")},
									{Name: "eu1", ID: "id-old-arm64", Architecture: ptr.To("arm64")},
								},
							},
						),
					}),
					MatchFields(IgnoreExtras, Fields{
						"Name": Equal("image-2"),
						"Versions": ContainElements(
							// Spec version preserved as-is (new format)
							api.MachineImageVersion{Version: "2.0",
								CapabilityFlavors: []api.MachineImageFlavor{
									{
										Capabilities: v1beta1.Capabilities{"architecture": []string{"amd64"}},
										Regions:      []api.RegionIDMapping{{Name: "eu1", ID: "id-new-amd64"}},
									},
									{
										Capabilities: v1beta1.Capabilities{"architecture": []string{"arm64"}},
										Regions:      []api.RegionIDMapping{{Name: "eu1", ID: "id-new-arm64"}},
									},
								},
							}),
					}),
				))
			})
		})
	})
})

func decodeCloudProfileConfig(decoder runtime.Decoder, config *runtime.RawExtension) (*api.CloudProfileConfig, error) {
	cloudProfileConfig := &api.CloudProfileConfig{}
	if err := util.Decode(decoder, config.Raw, cloudProfileConfig); err != nil {
		return nil, err
	}
	return cloudProfileConfig, nil
}
