// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure_test

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/gardener/gardener/extensions/pkg/controller/infrastructure"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"

	apisopenstack "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	. "github.com/gardener/gardener-extension-provider-openstack/pkg/controller/infrastructure"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	mockopenstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client/mocks"
)

const (
	name             = "infrastructure"
	namespace        = "shoot--foobar--openstack"
	floatingPoolName = "test3"

	domainName = "domainName"
	tenantName = "tenantName"
	userName   = "userName"
	password   = "password"
	authURL    = "authURL"
)

var _ = Describe("ConfigValidator", func() {
	var (
		ctrl                          *gomock.Controller
		openstackClientFactoryFactory *mockopenstackclient.MockFactoryFactory
		openstackClientFactory        *mockopenstackclient.MockFactory
		networkingClient              *mockopenstackclient.MockNetworking
		ctx                           context.Context
		logger                        logr.Logger
		cv                            infrastructure.ConfigValidator
		infra                         *extensionsv1alpha1.Infrastructure
		secret                        *corev1.Secret
		credentials                   *openstack.Credentials
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		openstackClientFactoryFactory = mockopenstackclient.NewMockFactoryFactory(ctrl)
		openstackClientFactory = mockopenstackclient.NewMockFactory(ctrl)
		networkingClient = mockopenstackclient.NewMockNetworking(ctrl)

		ctx = context.TODO()
		logger = log.Log.WithName("test")

		scheme := runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				openstack.DomainName: []byte(domainName),
				openstack.TenantName: []byte(tenantName),
				openstack.UserName:   []byte(userName),
				openstack.Password:   []byte(password),
				openstack.AuthURL:    []byte(authURL),
			},
		}

		c := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()
		mgr := test.FakeManager{Client: c}
		cv = NewConfigValidator(mgr, openstackClientFactoryFactory, logger)

		infra = &extensionsv1alpha1.Infrastructure{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: extensionsv1alpha1.InfrastructureSpec{
				Region: "region",
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type: openstack.Type,
					ProviderConfig: &runtime.RawExtension{
						Raw: encode(&apisopenstack.InfrastructureConfig{
							FloatingPoolName: floatingPoolName,
						}),
					},
				},
				SecretRef: corev1.SecretReference{
					Name:      name,
					Namespace: namespace,
				},
			},
		}
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				openstack.DomainName: []byte(domainName),
				openstack.TenantName: []byte(tenantName),
				openstack.UserName:   []byte(userName),
				openstack.Password:   []byte(password),
				openstack.AuthURL:    []byte(authURL),
			},
		}
		credentials = &openstack.Credentials{
			DomainName: domainName,
			TenantName: tenantName,
			Username:   userName,
			Password:   password,
			AuthURL:    authURL,
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Validate", func() {
		BeforeEach(func() {
			openstackClientFactoryFactory.EXPECT().NewFactory(ctx, credentials).Return(openstackClientFactory, nil)
			openstackClientFactory.EXPECT().Networking(gomock.Any()).Return(networkingClient, nil)
		})

		It("should forbid floating pool name that doesn't exist", func() {
			networkingClient.EXPECT().GetExternalNetworkNames(ctx).Return([]string{"test1", "test2"}, nil)

			errorList := cv.Validate(ctx, infra)
			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":  Equal(field.ErrorTypeNotFound),
				"Field": Equal("floatingPoolName"),
			}))
		})

		It("should allow NAT IP names that exist and are available", func() {
			networkingClient.EXPECT().GetExternalNetworkNames(ctx).Return([]string{"test1", "test2", "test3"}, nil)

			errorList := cv.Validate(ctx, infra)
			Expect(errorList).To(BeEmpty())
		})

		It("should fail with InternalError if getting external network names failed", func() {
			networkingClient.EXPECT().GetExternalNetworkNames(ctx).Return(nil, errors.New("test"))

			errorList := cv.Validate(ctx, infra)
			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":   Equal(field.ErrorTypeInternal),
				"Field":  Equal("floatingPoolName"),
				"Detail": Equal("could not get external network names: test"),
			}))
		})
	})
})

func encode(obj runtime.Object) []byte {
	data, _ := json.Marshal(obj)
	return data
}
