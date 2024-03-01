// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cloudprovider

import (
	"context"
	"testing"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/webhook/cloudprovider"
	gcontext "github.com/gardener/gardener/extensions/pkg/webhook/context"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	mockmanager "github.com/gardener/gardener/pkg/mock/controller-runtime/manager"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/install"
	openstackv1alpha1 "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"
	types "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
)

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CloudProvider Webhook Suite")
}

var _ = Describe("Ensurer", func() {
	var (
		ctx     = context.TODO()
		ectx    gcontext.GardenContext
		ensurer cloudprovider.Ensurer
		scheme  *runtime.Scheme
		cluster *extensionscontroller.Cluster

		ctrl *gomock.Controller
		mgr  *mockmanager.MockManager

		authUrl = "foo://bar"
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		install.Install(scheme)

		ctrl = gomock.NewController(GinkgoT())
		mgr = mockmanager.NewMockManager(ctrl)
		mgr.EXPECT().GetScheme().Return(scheme)

		cluster = &extensionscontroller.Cluster{
			CloudProfile: &gardencorev1beta1.CloudProfile{
				TypeMeta: metav1.TypeMeta{
					APIVersion: gardencorev1beta1.GroupName,
					Kind:       "CloudProfile",
				},
				Spec: gardencorev1beta1.CloudProfileSpec{
					ProviderConfig: &runtime.RawExtension{
						Object: &openstackv1alpha1.CloudProfileConfig{
							TypeMeta: metav1.TypeMeta{
								Kind:       "CloudProfileConfig",
								APIVersion: openstackv1alpha1.SchemeGroupVersion.String(),
							},
							KeyStoneURL: authUrl,
						},
					},
				},
			},
			Shoot: &gardencorev1beta1.Shoot{},
		}

		ectx = gcontext.NewInternalGardenContext(cluster)

		ensurer = NewEnsurer(mgr, logger)
	})

	It("Should ensure auth_url if present in cluster object", func() {
		newSecret := &corev1.Secret{}
		err := ensurer.EnsureCloudProviderSecret(ctx, ectx, newSecret, nil)

		Expect(err).NotTo(HaveOccurred())
		Expect(string(newSecret.Data[types.AuthURL])).To(Equal(authUrl))
	})

	It("Should return an error if no authURL is found", func() {
		newSecret := &corev1.Secret{}
		cluster.CloudProfile.Spec.ProviderConfig = encodeCloudProfileConfig(&openstackv1alpha1.CloudProfileConfig{})

		err := ensurer.EnsureCloudProviderSecret(ctx, ectx, newSecret, nil)
		Expect(err).To(HaveOccurred())
	})

	It("Should ensure that insecure is not set if not enforced by CloudProfile", func() {
		newSecret := &corev1.Secret{
			Data: map[string][]byte{
				types.Insecure: []byte("true"),
			},
		}

		err := ensurer.EnsureCloudProviderSecret(ctx, ectx, newSecret, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(newSecret.Data[types.Insecure])).NotTo(Equal("true"))
	})

	It("Should ensure that insecure is set if enabled in CloudProfile", func() {
		newSecret := &corev1.Secret{
			Data: map[string][]byte{
				types.Insecure: []byte("true"),
			},
		}

		cluster.CloudProfile.Spec.ProviderConfig = encodeCloudProfileConfig(&openstackv1alpha1.CloudProfileConfig{
			KeyStoneURL:           authUrl,
			KeyStoneForceInsecure: true,
		})

		err := ensurer.EnsureCloudProviderSecret(ctx, ectx, newSecret, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(newSecret.Data[types.Insecure]).To(Equal([]byte("true")))
	})

	It("Should ensure that CACert is populated if specified in CloudProfile", func() {
		newSecret := &corev1.Secret{
			Data: map[string][]byte{
				types.Insecure: []byte("true"),
			},
		}
		cluster.CloudProfile.Spec.ProviderConfig = encodeCloudProfileConfig(&openstackv1alpha1.CloudProfileConfig{
			KeyStoneCACert: pointer.String("cert"),
			KeyStoneURL:    "url",
		})

		err := ensurer.EnsureCloudProviderSecret(ctx, ectx, newSecret, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(newSecret.Data[types.CACert]).To(Equal([]byte("cert")))
	})
})

func encodeCloudProfileConfig(config *openstackv1alpha1.CloudProfileConfig) *runtime.RawExtension {
	config.TypeMeta = metav1.TypeMeta{
		Kind:       "CloudProfileConfig",
		APIVersion: openstackv1alpha1.SchemeGroupVersion.String(),
	}
	return &runtime.RawExtension{
		Object: config,
	}
}
