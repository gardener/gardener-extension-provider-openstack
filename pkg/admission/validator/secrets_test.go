// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package validator_test

import (
	"context"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/admission/validator"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
)

var _ = Describe("Secret validator", func() {
	Describe("#Validate", func() {
		const (
			namespace             = "garden-dev"
			name                  = "my-secret"
			shootProviderLabelKey = "provider.shoot.gardener.cloud/openstack"
		)

		var (
			secretValidator extensionswebhook.Validator
			ctx             = context.TODO()
			secret          *corev1.Secret
			oldSecret       *corev1.Secret
		)

		BeforeEach(func() {
			secretValidator = validator.NewSecretValidator()
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
				Data: map[string][]byte{
					openstack.DomainName: []byte("domain"),
					openstack.TenantName: []byte("tenant"),
					openstack.UserName:   []byte("user"),
					openstack.Password:   []byte("password"),
				},
			}
			oldSecret = secret.DeepCopy()
		})

		It("should return err when obj is not a Secret", func() {
			err := secretValidator.Validate(ctx, &corev1.ConfigMap{}, nil)
			Expect(err).To(MatchError("wrong object type *v1.ConfigMap"))
		})

		It("should return err when oldObj is not a Secret", func() {
			err := secretValidator.Validate(ctx, secret, &corev1.ConfigMap{})
			Expect(err).To(MatchError("wrong object type *v1.ConfigMap for old object"))
		})

		It("should validate unchanged data when the provider label is introduced", func() {
			secret.Data[openstack.Password] = []byte("bad\nfield: injected")
			oldSecret = secret.DeepCopy()
			secret.Labels = map[string]string{shootProviderLabelKey: "true"}

			Expect(secretValidator.Validate(ctx, secret, oldSecret)).To(HaveOccurred())
		})

		It("should skip validation for unchanged data that was previously validated", func() {
			secret.Labels = map[string]string{shootProviderLabelKey: "true"}
			oldSecret = secret.DeepCopy()

			Expect(secretValidator.Validate(ctx, secret, oldSecret)).To(Succeed())
		})

		It("should validate changed data even when the provider label is present", func() {
			secret.Labels = map[string]string{shootProviderLabelKey: "true"}
			oldSecret = secret.DeepCopy()
			secret.Data[openstack.Password] = []byte("bad\nfield: injected")

			Expect(secretValidator.Validate(ctx, secret, oldSecret)).To(HaveOccurred())
		})

		It("should return nil for a valid new secret", func() {
			Expect(secretValidator.Validate(ctx, secret, nil)).To(Succeed())
		})

		It("should return err for an invalid new secret", func() {
			secret.Data[openstack.Password] = []byte("bad\nfield: injected")
			Expect(secretValidator.Validate(ctx, secret, nil)).To(HaveOccurred())
		})
	})
})
