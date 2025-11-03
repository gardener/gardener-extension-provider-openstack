// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator_test

import (
	"context"
	"fmt"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/admission/validator"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/install"
)

var _ = DescribeTableSubtree("NamespacedCloudProfile Validator", func(isCapabilitiesCloudProfile bool) {
	var (
		fakeClient  client.Client
		fakeManager manager.Manager
		namespace   string
		ctx         = context.Background()

		namespacedCloudProfileValidator extensionswebhook.Validator
		namespacedCloudProfile          *core.NamespacedCloudProfile
		cloudProfile                    *v1beta1.CloudProfile
		capabilityDefinitions           []v1beta1.CapabilityDefinition
	)

	BeforeEach(func() {
		if isCapabilitiesCloudProfile {
			capabilityDefinitions = []v1beta1.CapabilityDefinition{
				{Name: v1beta1constants.ArchitectureName, Values: []string{"amd64"}},
			}
		}
		scheme := runtime.NewScheme()
		utilruntime.Must(install.AddToScheme(scheme))
		utilruntime.Must(v1beta1.AddToScheme(scheme))
		fakeClient = fakeclient.NewClientBuilder().WithScheme(scheme).Build()
		fakeManager = &test.FakeManager{
			Client: fakeClient,
			Scheme: scheme,
		}
		namespace = "garden-dev"

		namespacedCloudProfileValidator = validator.NewNamespacedCloudProfileValidator(fakeManager)
		namespacedCloudProfile = &core.NamespacedCloudProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "profile-1",
				Namespace: namespace,
			},
			Spec: core.NamespacedCloudProfileSpec{
				Parent: core.CloudProfileReference{
					Name: "cloud-profile",
					Kind: "CloudProfile",
				},
			},
		}
		cloudProfile = &v1beta1.CloudProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cloud-profile",
			},
			Spec: v1beta1.CloudProfileSpec{
				MachineCapabilities: capabilityDefinitions,
			},
		}
	})

	Describe("#Validate", func() {
		It("should succeed for NamespacedCloudProfile without provider config", func() {
			Expect(fakeClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(namespacedCloudProfileValidator.Validate(ctx, namespacedCloudProfile, nil)).To(Succeed())
		})

		It("should succeed if NamespacedCloudProfile is in deletion phase", func() {
			namespacedCloudProfile.DeletionTimestamp = ptr.To(metav1.Now())

			Expect(namespacedCloudProfileValidator.Validate(ctx, namespacedCloudProfile, nil)).To(Succeed())
		})

		FIt("should succeed if the NamespacedCloudProfile correctly defines new machine images and types", func() {

			imageIDMappings := `"regions":[{"name":"image-region-1","id":"id-img-reg-1"}]`
			namespacedImageIDMappings := `{"name":"image-1","versions":[{"version":"1.1","regions":[{"name":"image-region-1","id":"id-img-reg-1"}]}]},
  {"name":"image-2","versions":[{"version":"2.0","regions":[{"name":"image-region-1","id":"id-img-reg-1"}]}]}`
			if isCapabilitiesCloudProfile {
				imageIDMappings = `"capabilityFlavors":[{"regions":[{"name":"image-region-1","id":"id-img-reg-1"}]}]`
				namespacedImageIDMappings = `{"name":"image-1","versions":[{"version":"1.1","capabilityFlavors":[{"regions":[{"name":"image-region-1","id":"id-img-reg-1"}]}]}]},
  {"name":"image-2","versions":[{"version":"2.0","capabilityFlavors":[{"regions":[{"name":"image-region-1","id":"id-img-reg-1"}]}]}]}`
			}
			cloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(fmt.Sprintf(`{
"apiVersion":"openstack.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[{"name":"image-1","versions":[{"version":"1.0",%s}]}]
}`, imageIDMappings))}

			namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(fmt.Sprintf(`{
"apiVersion":"openstack.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[%s]
}`, namespacedImageIDMappings))}
			namespacedCloudProfile.Spec.MachineImages = []core.MachineImage{
				{
					Name:     "image-1",
					Versions: []core.MachineImageVersion{{ExpirableVersion: core.ExpirableVersion{Version: "1.1"}, Architectures: []string{"amd64"}}},
				},
				{
					Name:     "image-2",
					Versions: []core.MachineImageVersion{{ExpirableVersion: core.ExpirableVersion{Version: "2.0"}, Architectures: []string{"amd64"}}},
				},
			}
			namespacedCloudProfile.Spec.MachineTypes = []core.MachineType{
				{Name: "type-2"},
			}
			Expect(fakeClient.Create(ctx, cloudProfile)).To(Succeed())

			Expect(namespacedCloudProfileValidator.Validate(ctx, namespacedCloudProfile, nil)).To(Succeed())
		})

		It("should fail for NamespacedCloudProfile with invalid parent kind", func() {
			namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{
"apiVersion":"openstack.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig"
}`)}
			namespacedCloudProfile.Spec.Parent = core.CloudProfileReference{
				Name: "cloud-profile",
				Kind: "NamespacedCloudProfile",
			}

			Expect(namespacedCloudProfileValidator.Validate(ctx, namespacedCloudProfile, nil)).To(MatchError(ContainSubstring("parent reference must be of kind CloudProfile")))
		})

		It("should fail for NamespacedCloudProfile trying to override an already existing machine image version", func() {
			cloudProfile.Spec.MachineImages = []v1beta1.MachineImage{
				{Name: "image-1", Versions: []v1beta1.MachineImageVersion{{ExpirableVersion: v1beta1.ExpirableVersion{Version: "1.0"}}}},
			}
			cloudProfile.Spec.MachineTypes = []v1beta1.MachineType{{Name: "type-1"}}

			namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{
"apiVersion":"openstack.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[
  {"name":"image-1","versions":[{"version":"1.0","image":"image-name-1","regions":[{"name":"image-region-1","id":"id-img-reg-1"}]}]}
]
}`)}
			namespacedCloudProfile.Spec.MachineImages = []core.MachineImage{
				{
					Name: "image-1",
					Versions: []core.MachineImageVersion{
						{ExpirableVersion: core.ExpirableVersion{Version: "1.0"}, Architectures: []string{"amd64"}},
					},
				},
			}

			Expect(fakeClient.Create(ctx, cloudProfile)).To(Succeed())

			err := namespacedCloudProfileValidator.Validate(ctx, namespacedCloudProfile, nil)
			Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal("spec.providerConfig.machineImages[0].versions[0]"),
				"Detail": Equal("machine image version image-1@1.0 is already defined in the parent CloudProfile"),
			}))))
		})

		It("should fail for NamespacedCloudProfile specifying provider config without the according version in the spec.machineImages", func() {
			namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{
"apiVersion":"openstack.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[
  {"name":"image-1","versions":[{"version":"1.1","image":"image-name-1","regions":[{"name":"image-region-1","id":"id-img-reg-1"}]}]}
]
}`)}
			namespacedCloudProfile.Spec.MachineImages = []core.MachineImage{
				{
					Name: "image-1",
					Versions: []core.MachineImageVersion{
						{ExpirableVersion: core.ExpirableVersion{Version: "1.2"}},
					},
				},
			}

			Expect(fakeClient.Create(ctx, cloudProfile)).To(Succeed())

			err := namespacedCloudProfileValidator.Validate(ctx, namespacedCloudProfile, nil)
			Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeRequired),
				"Field":  Equal("spec.providerConfig.machineImages"),
				"Detail": Equal("machine image version image-1@1.2 is not defined in the NamespacedCloudProfile providerConfig"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":     Equal(field.ErrorTypeInvalid),
				"Field":    Equal("spec.providerConfig.machineImages[0].versions[0]"),
				"BadValue": Equal("image-1@1.1"),
				"Detail":   Equal("machine image version is not defined in the NamespacedCloudProfile"),
			}))))
		})

		It("should fail for NamespacedCloudProfile specifying new spec.machineImages without the according version in the provider config", func() {
			namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{
"apiVersion":"openstack.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig"
}`)}
			namespacedCloudProfile.Spec.MachineImages = []core.MachineImage{
				{
					Name: "image-3",
					Versions: []core.MachineImageVersion{
						{ExpirableVersion: core.ExpirableVersion{Version: "3.0"}},
					},
				},
			}

			Expect(fakeClient.Create(ctx, cloudProfile)).To(Succeed())

			err := namespacedCloudProfileValidator.Validate(ctx, namespacedCloudProfile, nil)
			Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeRequired),
				"Field":  Equal("spec.providerConfig.machineImages"),
				"Detail": Equal("machine image image-3 is not defined in the NamespacedCloudProfile providerConfig"),
			}))))
		})

		It("should fail for NamespacedCloudProfile specifying new spec.machineImages without the according version and architecture entries in the provider config", func() {
			namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{
"apiVersion":"openstack.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[
  {"name":"image-1","versions":[
	{"version":"1.1-regions","image":"image-regions-1","regions":[
      {"name":"image-region-1","id":"id-img-reg-1","architecture":"arm64"},
      {"name":"image-region-2","id":"id-img-reg-2","architecture":"amd64"}
    ]},
    {"version":"1.1-fallback","image":"image-fallback-1","regions":[
      {"name":"image-region-2","id":"id-img-reg-2"}
    ]},
    {"version":"1.1-missing","image":"image-missing-1","regions":[]}
  ]}
]
}`)}
			namespacedCloudProfile.Spec.MachineImages = []core.MachineImage{
				{
					Name: "image-1",
					Versions: []core.MachineImageVersion{
						{ExpirableVersion: core.ExpirableVersion{Version: "1.1-regions"}, Architectures: []string{"amd64", "arm64"}},
						{ExpirableVersion: core.ExpirableVersion{Version: "1.1-fallback"}, Architectures: []string{"arm64"}},
						{ExpirableVersion: core.ExpirableVersion{Version: "1.1-missing"}, Architectures: []string{"arm64"}},
					},
				},
			}

			Expect(fakeClient.Create(ctx, cloudProfile)).To(Succeed())

			err := namespacedCloudProfileValidator.Validate(ctx, namespacedCloudProfile, nil)
			Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeRequired),
				"Field":  Equal("spec.providerConfig.machineImages"),
				"Detail": Equal("machine image version image-1@1.1-regions for region \"image-region-1\" with architecture \"amd64\" is not defined in the NamespacedCloudProfile providerConfig"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeRequired),
				"Field":  Equal("spec.providerConfig.machineImages"),
				"Detail": Equal("machine image version image-1@1.1-regions for region \"image-region-2\" with architecture \"arm64\" is not defined in the NamespacedCloudProfile providerConfig"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal("spec.providerConfig.machineImages"),
				"Detail": Equal("machine image version image-1@1.1-fallback in region \"image-region-2\" has an excess entry for architecture \"amd64\", which is not defined in the machineImages spec"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeRequired),
				"Field":  Equal("spec.providerConfig.machineImages"),
				"Detail": Equal("machine image version image-1@1.1-fallback for region \"image-region-2\" with architecture \"arm64\" is not defined in the NamespacedCloudProfile providerConfig"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeRequired),
				"Field":  Equal("spec.providerConfig.machineImages"),
				"Detail": Equal("machine image version image-1@1.1-missing with architecture \"arm64\" is not defined in the NamespacedCloudProfile providerConfig"),
			}))))
		})

		It("should fail for NamespacedCloudProfile specifying an invalid field in the provider config", func() {
			namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{
"apiVersion":"openstack.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"keystoneURL":"http://url-to-keystone/v3"
}`)}

			Expect(fakeClient.Create(ctx, cloudProfile)).To(Succeed())

			err := namespacedCloudProfileValidator.Validate(ctx, namespacedCloudProfile, nil)
			Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal("spec.providerConfig"),
				"Detail": Equal("must only set machineImages"),
			}))))
		})
	})
},
	Entry("CloudProfile uses regions only", false),
	Entry("CloudProfile uses capabilities", true),
)
