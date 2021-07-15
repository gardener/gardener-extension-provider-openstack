// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dnsrecord_test

import (
	"context"

	. "github.com/gardener/gardener-extension-provider-openstack/pkg/controller/dnsrecord"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	mockopenstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client/mocks"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"

	"github.com/gardener/gardener/extensions/pkg/controller/dnsrecord"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
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

		a = NewActuator(openstackClientFactoryFactory, logger)

		err := a.(inject.Client).InjectClient(c)
		Expect(err).NotTo(HaveOccurred())

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
			c.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret) error {
					*obj = *secret
					return nil
				},
			)
			openstackClientFactoryFactory.EXPECT().NewFactory(credentials).Return(openstackClientFactory, nil)
			openstackClientFactory.EXPECT().DNS().Return(dnsClient, nil)
			dnsClient.EXPECT().GetZones(ctx).Return(zones, nil)
			dnsClient.EXPECT().CreateOrUpdateRecordSet(ctx, zone, dnsName, string(extensionsv1alpha1.DNSRecordTypeA), []string{address}, 120).Return(nil)
			dnsClient.EXPECT().DeleteRecordSet(ctx, zone, "comment-"+dnsName, "TXT").Return(nil)
			c.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&extensionsv1alpha1.DNSRecord{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, obj *extensionsv1alpha1.DNSRecord) error {
					*obj = *dns
					return nil
				},
			)
			sw.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&extensionsv1alpha1.DNSRecord{})).DoAndReturn(
				func(_ context.Context, obj *extensionsv1alpha1.DNSRecord, opts ...client.UpdateOption) error {
					Expect(obj.Status).To(Equal(extensionsv1alpha1.DNSRecordStatus{
						Zone: pointer.String(zone),
					}))
					return nil
				},
			)

			err := a.Reconcile(ctx, dns, nil)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#Delete", func() {
		It("should delete the DNSRecord", func() {
			dns.Status.Zone = pointer.String(zone)

			c.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret) error {
					*obj = *secret
					return nil
				},
			)
			openstackClientFactoryFactory.EXPECT().NewFactory(credentials).Return(openstackClientFactory, nil)
			openstackClientFactory.EXPECT().DNS().Return(dnsClient, nil)
			dnsClient.EXPECT().DeleteRecordSet(ctx, zone, dnsName, string(extensionsv1alpha1.DNSRecordTypeA)).Return(nil)

			err := a.Delete(ctx, dns, nil)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
