// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dnsrecord_test

import (
	"context"

	"github.com/gardener/gardener/extensions/pkg/controller/dnsrecord"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
	mockmanager "github.com/gardener/gardener/third_party/mock/controller-runtime/manager"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	. "github.com/gardener/gardener-extension-provider-openstack/pkg/controller/dnsrecord"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	mockopenstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client/mocks"
)

const (
	name        = "openstack-external"
	namespace   = "shoot--foobar--az"
	shootDomain = "shoot.example.com"
	dnsName     = "api.openstack.foobar." + shootDomain
	zone        = "zone"
	address     = "1.2.3.4"

	domainName = "domainName"
	tenantName = "tenantName"
	userName   = "userName"
	password   = "password"
	authURL    = "authURL"
)

var _ = Describe("Actuator", func() {
	var (
		ctrl                          *gomock.Controller
		mgr                           *mockmanager.MockManager
		c                             *mockclient.MockClient
		sw                            *mockclient.MockStatusWriter
		openstackClientFactoryFactory *mockopenstackclient.MockFactoryFactory
		openstackClientFactory        *mockopenstackclient.MockFactory
		dnsClient                     *mockopenstackclient.MockDNS
		ctx                           context.Context
		logger                        logr.Logger
		a                             dnsrecord.Actuator
		dns                           *extensionsv1alpha1.DNSRecord
		secret                        *corev1.Secret
		credentials                   *openstack.Credentials
		zones                         map[string]string
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		c = mockclient.NewMockClient(ctrl)
		sw = mockclient.NewMockStatusWriter(ctrl)
		openstackClientFactoryFactory = mockopenstackclient.NewMockFactoryFactory(ctrl)
		openstackClientFactory = mockopenstackclient.NewMockFactory(ctrl)
		dnsClient = mockopenstackclient.NewMockDNS(ctrl)

		c.EXPECT().Status().Return(sw).AnyTimes()

		ctx = context.TODO()
		logger = log.Log.WithName("test")

		mgr = mockmanager.NewMockManager(ctrl)
		mgr.EXPECT().GetClient().Return(c)

		a = NewActuator(mgr, openstackClientFactoryFactory)

		dns = &extensionsv1alpha1.DNSRecord{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: extensionsv1alpha1.DNSRecordSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type: openstack.DNSType,
				},
				SecretRef: corev1.SecretReference{
					Name:      name,
					Namespace: namespace,
				},
				Name:       dnsName,
				RecordType: extensionsv1alpha1.DNSRecordTypeA,
				Values:     []string{address},
			},
		}
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				openstack.DNSDomainName: []byte(domainName),
				openstack.DNSTenantName: []byte(tenantName),
				openstack.DNSUserName:   []byte(userName),
				openstack.DNSPassword:   []byte(password),
				openstack.DNSAuthURL:    []byte(authURL),
			},
		}
		credentials = &openstack.Credentials{
			DomainName: domainName,
			TenantName: tenantName,
			Username:   userName,
			Password:   password,
			AuthURL:    authURL,
		}

		zones = map[string]string{
			shootDomain:   zone,
			"example.com": "zone2",
			"other.com":   "zone3",
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Reconcile", func() {
		It("should reconcile the DNSRecord", func() {
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret, _ ...client.GetOption) error {
					*obj = *secret
					return nil
				},
			)
			openstackClientFactoryFactory.EXPECT().NewFactory(credentials).Return(openstackClientFactory, nil)
			openstackClientFactory.EXPECT().DNS().Return(dnsClient, nil)
			dnsClient.EXPECT().GetZones(ctx).Return(zones, nil)
			dnsClient.EXPECT().CreateOrUpdateRecordSet(ctx, zone, dnsName, string(extensionsv1alpha1.DNSRecordTypeA), []string{address}, 120).Return(nil)
			sw.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&extensionsv1alpha1.DNSRecord{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, obj *extensionsv1alpha1.DNSRecord, _ client.Patch, _ ...client.PatchOption) error {
					Expect(obj.Status).To(Equal(extensionsv1alpha1.DNSRecordStatus{
						Zone: ptr.To(zone),
					}))
					return nil
				},
			)

			err := a.Reconcile(ctx, logger, dns, nil)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#Delete", func() {
		It("should delete the DNSRecord", func() {
			dns.Status.Zone = ptr.To(zone)

			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret, _ ...client.GetOption) error {
					*obj = *secret
					return nil
				},
			)
			openstackClientFactoryFactory.EXPECT().NewFactory(credentials).Return(openstackClientFactory, nil)
			openstackClientFactory.EXPECT().DNS().Return(dnsClient, nil)
			dnsClient.EXPECT().DeleteRecordSet(ctx, zone, dnsName, string(extensionsv1alpha1.DNSRecordTypeA)).Return(nil)

			err := a.Delete(ctx, logger, dns, nil)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
