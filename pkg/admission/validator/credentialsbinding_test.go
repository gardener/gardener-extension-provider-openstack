// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator_test

import (
	"context"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/security"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

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

			scheme *runtime.Scheme
			ctx    = context.TODO()

			credentialsBinding *security.CredentialsBinding
		)

		BeforeEach(func() {
			scheme = runtime.NewScheme()
			Expect(corev1.AddToScheme(scheme)).To(Succeed())
			Expect(gardencorev1beta1.AddToScheme(scheme)).To(Succeed())

			apiReader := fakeclient.NewClientBuilder().WithScheme(scheme).Build()
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
			// Secret not pre-populated → fake returns NotFound, which the validator propagates as an error
			err := credentialsBindingValidator.Validate(ctx, credentialsBinding, nil)
			Expect(err).To(MatchError(ContainSubstring("not found")))
		})

		It("should return err when the corresponding Secret is not valid", func() {
			apiReader := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
				Data:       map[string][]byte{"foo": []byte("bar")},
			}).Build()
			v := validator.NewCredentialsBindingValidator(test.FakeManager{APIReader: apiReader})

			err := v.Validate(ctx, credentialsBinding, nil)
			Expect(err).To(HaveOccurred())
		})

		It("should succeed when the corresponding Secret is valid", func() {
			apiReader := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
				Data: map[string][]byte{
					openstack.DomainName: []byte("domain"),
					openstack.TenantName: []byte("tenant"),
					openstack.UserName:   []byte("user"),
					openstack.Password:   []byte("password"),
				},
			}).Build()
			v := validator.NewCredentialsBindingValidator(test.FakeManager{APIReader: apiReader})

			Expect(v.Validate(ctx, credentialsBinding, nil)).To(Succeed())
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
				// InternalSecret not pre-populated → fake returns NotFound
				err := credentialsBindingValidator.Validate(ctx, credentialsBinding, nil)
				Expect(err).To(MatchError(ContainSubstring("not found")))
			})

			It("should return err when the corresponding InternalSecret is not valid", func() {
				apiReader := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(&gardencorev1beta1.InternalSecret{
					ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
					Data:       map[string][]byte{"foo": []byte("bar")},
				}).Build()
				v := validator.NewCredentialsBindingValidator(test.FakeManager{APIReader: apiReader})

				err := v.Validate(ctx, credentialsBinding, nil)
				Expect(err).To(HaveOccurred())
			})

			It("should succeed when the corresponding InternalSecret is valid", func() {
				apiReader := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(&gardencorev1beta1.InternalSecret{
					ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
					Data: map[string][]byte{
						openstack.DomainName: []byte("domain"),
						openstack.TenantName: []byte("tenant"),
						openstack.UserName:   []byte("user"),
						openstack.Password:   []byte("password"),
					},
				}).Build()
				v := validator.NewCredentialsBindingValidator(test.FakeManager{APIReader: apiReader})

				Expect(v.Validate(ctx, credentialsBinding, nil)).To(Succeed())
			})
		})
	})
})
