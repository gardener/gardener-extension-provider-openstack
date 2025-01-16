// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure_test

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/gardener/gardener/extensions/pkg/controller/infrastructure"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
	mockmanager "github.com/gardener/gardener/third_party/mock/controller-runtime/manager"
	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/layer3/routers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/ports"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/subnets"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
		c                             *mockclient.MockClient
		mgr                           *mockmanager.MockManager
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

		c = mockclient.NewMockClient(ctrl)
		openstackClientFactoryFactory = mockopenstackclient.NewMockFactoryFactory(ctrl)
		openstackClientFactory = mockopenstackclient.NewMockFactory(ctrl)
		networkingClient = mockopenstackclient.NewMockNetworking(ctrl)

		ctx = context.TODO()
		logger = log.Log.WithName("test")

		mgr = mockmanager.NewMockManager(ctrl)
		mgr.EXPECT().GetClient().Return(c)

		cv = NewConfigValidator(mgr, openstackClientFactoryFactory, logger)

		infra = &extensionsv1alpha1.Infrastructure{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: extensionsv1alpha1.InfrastructureSpec{
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
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret, _ ...client.GetOption) error {
					*obj = *secret
					return nil
				},
			)
			openstackClientFactoryFactory.EXPECT().NewFactory(credentials).Return(openstackClientFactory, nil)
			openstackClientFactory.EXPECT().Networking().Return(networkingClient, nil)
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

		Context("failing path", func() {

			var (
				config *apisopenstack.InfrastructureConfig
				id     string
			)

			BeforeEach(func() {
				networkingClient.EXPECT().GetExternalNetworkNames(ctx).Return([]string{"test1", "test2", "test3"}, nil)

				id = uuid.NewString()
				config = &apisopenstack.InfrastructureConfig{
					FloatingPoolName: floatingPoolName,
					Networks:         apisopenstack.Networks{},
				}
			})

			It("should fail with InternalError if getting network failed", func() {
				config.Networks.ID = ptr.To(id)
				infra.Spec.ProviderConfig.Raw = encode(config)

				networkingClient.EXPECT().ListNetwork(gomock.Any()).Return(nil, errors.New("test"))

				errorList := cv.Validate(ctx, infra)
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":  Equal(field.ErrorTypeInternal),
					"Field": Equal("networks.id"),
				}))
			})

			It("should fail with NotFound if no network found", func() {
				config.Networks.ID = ptr.To("nonexisting")
				infra.Spec.ProviderConfig.Raw = encode(config)

				networkingClient.EXPECT().ListNetwork(gomock.Any()).Return([]networks.Network{}, nil)

				errorList := cv.Validate(ctx, infra)
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":  Equal(field.ErrorTypeNotFound),
					"Field": Equal("networks.id"),
				}))
			})

			It("should fail with InternalError if getting subnet failed", func() {
				config.Networks.ID = ptr.To(id)
				config.Networks.SubnetID = ptr.To(id)
				infra.Spec.ProviderConfig.Raw = encode(config)

				networkingClient.EXPECT().ListNetwork(gomock.Any()).Return([]networks.Network{{ID: "id"}}, nil)
				networkingClient.EXPECT().ListSubnets(gomock.Any()).Return(nil, errors.New("test"))

				errorList := cv.Validate(ctx, infra)
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":  Equal(field.ErrorTypeInternal),
					"Field": Equal("networks.subnetId"),
				}))
			})

			It("should fail with NotFound if no subnet found", func() {
				config.Networks.ID = ptr.To(id)
				config.Networks.SubnetID = ptr.To("nonexisting")
				infra.Spec.ProviderConfig.Raw = encode(config)

				networkingClient.EXPECT().ListNetwork(gomock.Any()).Return([]networks.Network{{ID: "id"}}, nil)
				networkingClient.EXPECT().ListSubnets(gomock.Any()).Return([]subnets.Subnet{}, nil)

				errorList := cv.Validate(ctx, infra)
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":  Equal(field.ErrorTypeNotFound),
					"Field": Equal("networks.subnetId"),
				}))
			})

			It("should fail with Invalid if subnet isn't child of specified network", func() {
				config.Networks.ID = ptr.To(id)
				config.Networks.SubnetID = ptr.To("subnetID")
				infra.Spec.ProviderConfig.Raw = encode(config)

				networkingClient.EXPECT().ListNetwork(gomock.Any()).Return([]networks.Network{{ID: "id"}}, nil)
				networkingClient.EXPECT().ListSubnets(gomock.Any()).Return([]subnets.Subnet{{NetworkID: "wrong-parent"}}, nil)

				errorList := cv.Validate(ctx, infra)
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("networks.subnetId"),
				}))
			})

			It("should fail with InternalError if getting router failed", func() {
				config.Networks.Router = &apisopenstack.Router{ID: id}
				infra.Spec.ProviderConfig.Raw = encode(config)

				networkingClient.EXPECT().ListRouters(gomock.Any()).Return(nil, errors.New("test"))

				errorList := cv.Validate(ctx, infra)
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":  Equal(field.ErrorTypeInternal),
					"Field": Equal("networks.router.id"),
				}))
			})

			It("should fail with NotFound if no router found", func() {
				config.Networks.Router = &apisopenstack.Router{ID: "nonexisting"}
				infra.Spec.ProviderConfig.Raw = encode(config)

				networkingClient.EXPECT().ListRouters(gomock.Any()).Return([]routers.Router{}, nil)

				errorList := cv.Validate(ctx, infra)
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":  Equal(field.ErrorTypeNotFound),
					"Field": Equal("networks.router.id"),
				}))
			})

			It("should fail with NotFound if no router interface for the subnet is found", func() {
				subnetID := "subnetID"
				routerID := "routerID"
				config.Networks.ID = ptr.To(id)
				config.Networks.SubnetID = ptr.To(subnetID)
				config.Networks.Router = &apisopenstack.Router{ID: routerID}
				infra.Spec.ProviderConfig.Raw = encode(config)

				networkingClient.EXPECT().ListNetwork(gomock.Any()).Return([]networks.Network{{ID: "id"}}, nil)
				networkingClient.EXPECT().ListSubnets(gomock.Any()).Return([]subnets.Subnet{{ID: subnetID, NetworkID: id}}, nil)
				networkingClient.EXPECT().ListRouters(gomock.Any()).Return([]routers.Router{{ID: routerID}}, nil)
				networkingClient.EXPECT().GetRouterInterfacePort(routerID, subnetID).Return(nil, nil)

				errorList := cv.Validate(ctx, infra)
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("networks.router"),
				}))
			})
		})

		Context("happy path", func() {
			It("doesn't fail", func() {
				networkingClient.EXPECT().GetExternalNetworkNames(ctx).Return([]string{"test1", "test2", "test3"}, nil)

				id := uuid.NewString()
				config := &apisopenstack.InfrastructureConfig{
					FloatingPoolName: floatingPoolName,
					Networks: apisopenstack.Networks{
						ID:       &id,
						SubnetID: &id,
						Router: &apisopenstack.Router{
							ID: id,
						},
					},
				}
				infra.Spec.ProviderConfig.Raw = encode(config)

				networkingClient.EXPECT().ListNetwork(gomock.Any()).Return([]networks.Network{{ID: id}}, nil)
				networkingClient.EXPECT().ListSubnets(gomock.Any()).Return([]subnets.Subnet{{NetworkID: id}}, nil)
				networkingClient.EXPECT().ListRouters(gomock.Any()).Return([]routers.Router{{ID: id}}, nil)
				networkingClient.EXPECT().GetRouterInterfacePort(id, id).Return(&ports.Port{ID: "portID"}, nil)

				errorList := cv.Validate(ctx, infra)
				Expect(errorList).To(BeEmpty())
			})
		})

	})
})

func encode(obj runtime.Object) []byte {
	data, _ := json.Marshal(obj)
	return data
}
