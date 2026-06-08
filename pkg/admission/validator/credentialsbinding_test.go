// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator_test

import (
	"context"
	"fmt"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/security"
	"github.com/gardener/gardener/pkg/utils/test"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/admission/validator"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
)

var _ = Describe("CredentialsBinding validator", func() {
	Describe("#Validate", func() {
		const (
			namespace = "garden-dev"
			name      = "my-provider-account"
		)

		var (
			credentialsBindingValidator extensionswebhook.Validator

			ctrl      *gomock.Controller
			apiReader *mockclient.MockReader

			ctx                = context.TODO()
			credentialsBinding *security.CredentialsBinding

			fakeErr = fmt.Errorf("fake err")
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())

			apiReader = mockclient.NewMockReader(ctrl)

			mgr := test.FakeManager{APIReader: apiReader}
			credentialsBindingValidator = validator.NewCredentialsBindingValidator(mgr)

			credentialsBinding = &security.CredentialsBinding{
				CredentialsRef: corev1.ObjectReference{
					Name:       name,
					Namespace:  namespace,
					Kind:       "Secret",
					APIVersion: "v1",
				},
			}
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("should return err when obj is not a CredentialsBinding", func() {
			err := credentialsBindingValidator.Validate(ctx, &corev1.Secret{}, nil)
			Expect(err).To(MatchError("wrong object type *v1.Secret"))
		})

		It("should return err when oldObj is not a CredentialsBinding", func() {
			err := credentialsBindingValidator.Validate(ctx, &security.CredentialsBinding{}, &corev1.Secret{})
			Expect(err).To(MatchError("wrong object type *v1.Secret for old object"))
		})

		It("should return err if the CredentialsBinding references unknown credentials type", func() {
			credentialsBinding.CredentialsRef.APIVersion = "unknown"
			err := credentialsBindingValidator.Validate(ctx, credentialsBinding, nil)
			Expect(err).To(MatchError(ContainSubstring("unsupported credentials reference")))
		})

		It("should return err if it fails to get the corresponding Secret", func() {
			apiReader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(fakeErr)

			err := credentialsBindingValidator.Validate(ctx, credentialsBinding, nil)
			Expect(err).To(MatchError(fakeErr))
		})

		It("should return err when the corresponding Secret is not valid", func() {
			apiReader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret, _ ...client.GetOption) error {
					secret := &corev1.Secret{Data: map[string][]byte{
						"foo": []byte("bar"),
					}}
					*obj = *secret
					return nil
				})

			err := credentialsBindingValidator.Validate(ctx, credentialsBinding, nil)
			Expect(err).To(HaveOccurred())
		})

		It("should succeed when the corresponding Secret is valid", func() {
			apiReader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret, _ ...client.GetOption) error {
					secret := &corev1.Secret{Data: map[string][]byte{
						openstack.DomainName: []byte("domain"),
						openstack.TenantName: []byte("tenant"),
						openstack.UserName:   []byte("user"),
						openstack.Password:   []byte("password"),
					}}
					*obj = *secret
					return nil
				})

			Expect(credentialsBindingValidator.Validate(ctx, credentialsBinding, nil)).To(Succeed())
		})

		It("should return nil when the CredentialsBinding did not change", func() {
			old := credentialsBinding.DeepCopy()

			Expect(credentialsBindingValidator.Validate(ctx, credentialsBinding, old)).To(Succeed())
		})

		Context("InternalSecret", func() {
			BeforeEach(func() {
				credentialsBinding.CredentialsRef = corev1.ObjectReference{
					Name:       name,
					Namespace:  namespace,
					Kind:       "InternalSecret",
					APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				}
			})

			It("should return err if it fails to get the corresponding InternalSecret", func() {
				apiReader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&gardencorev1beta1.InternalSecret{})).Return(fakeErr)

				err := credentialsBindingValidator.Validate(ctx, credentialsBinding, nil)
				Expect(err).To(MatchError(fakeErr))
			})

			It("should return err when the corresponding InternalSecret is not valid", func() {
				apiReader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&gardencorev1beta1.InternalSecret{})).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.InternalSecret, _ ...client.GetOption) error {
						obj.Data = map[string][]byte{"foo": []byte("bar")}
						return nil
					})

				err := credentialsBindingValidator.Validate(ctx, credentialsBinding, nil)
				Expect(err).To(HaveOccurred())
			})

			It("should succeed when the corresponding InternalSecret is valid", func() {
				apiReader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&gardencorev1beta1.InternalSecret{})).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.InternalSecret, _ ...client.GetOption) error {
						obj.Data = map[string][]byte{
							openstack.DomainName: []byte("domain"),
							openstack.TenantName: []byte("tenant"),
							openstack.UserName:   []byte("user"),
							openstack.Password:   []byte("password"),
						}
						return nil
					})

				Expect(credentialsBindingValidator.Validate(ctx, credentialsBinding, nil)).To(Succeed())
			})
		})
	})
})
