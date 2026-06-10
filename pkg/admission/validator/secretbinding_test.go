// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator_test

import (
	"context"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core"
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

var _ = Describe("SecretBinding validator", func() {

	Describe("#Validate", func() {
		const (
			namespace = "garden-dev"
			name      = "my-provider-account"
		)

		var (
			secretBindingValidator extensionswebhook.Validator

			scheme        *runtime.Scheme
			ctx           = context.TODO()
			secretBinding = &core.SecretBinding{
				SecretRef: corev1.SecretReference{
					Name:      name,
					Namespace: namespace,
				},
			}
		)

		BeforeEach(func() {
			scheme = runtime.NewScheme()
			Expect(corev1.AddToScheme(scheme)).To(Succeed())

			apiReader := fakeclient.NewClientBuilder().WithScheme(scheme).Build()
			mgr := test.FakeManager{APIReader: apiReader}
			secretBindingValidator = validator.NewSecretBindingValidator(mgr)
		})

		It("should return err when obj is not a SecretBinding", func() {
			err := secretBindingValidator.Validate(ctx, &corev1.Secret{}, nil)
			Expect(err).To(MatchError("wrong object type *v1.Secret"))
		})

		It("should return err when oldObj is not a SecretBinding", func() {
			err := secretBindingValidator.Validate(ctx, &core.SecretBinding{}, &corev1.Secret{})
			Expect(err).To(MatchError("wrong object type *v1.Secret for old object"))
		})

		It("should return err if it fails to get the corresponding Secret", func() {
			// Secret not pre-populated → fake returns NotFound, which the validator propagates as an error
			err := secretBindingValidator.Validate(ctx, secretBinding, nil)
			Expect(err).To(MatchError(ContainSubstring("not found")))
		})

		It("should return err when the corresponding Secret is not valid", func() {
			apiReader := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
				Data:       map[string][]byte{"foo": []byte("bar")},
			}).Build()
			v := validator.NewSecretBindingValidator(test.FakeManager{APIReader: apiReader})

			err := v.Validate(ctx, secretBinding, nil)
			Expect(err).To(HaveOccurred())
		})

		It("should return nil when the corresponding Secret is valid", func() {
			apiReader := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
				Data: map[string][]byte{
					openstack.DomainName: []byte("domain"),
					openstack.TenantName: []byte("tenant"),
					openstack.UserName:   []byte("user"),
					openstack.Password:   []byte("password"),
				},
			}).Build()
			v := validator.NewSecretBindingValidator(test.FakeManager{APIReader: apiReader})

			err := v.Validate(ctx, secretBinding, nil)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
