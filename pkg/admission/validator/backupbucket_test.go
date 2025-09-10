// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator_test

import (
	"context"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/admission/validator"
)

var _ = Describe("BackupBucket Validator", func() {
	Describe("#Validate", func() {
		var (
			ctx            context.Context
			credentialsRef *corev1.ObjectReference

			backupBucketValidator extensionswebhook.Validator
		)

		BeforeEach(func() {
			ctx = context.TODO()
			credentialsRef = &corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Name:       "backup-credentials",
				Namespace:  "garden",
			}

			backupBucketValidator = validator.NewBackupBucketValidator()
		})

		It("should return err when obj is not a core.gardener.cloud/.BackupBucket", func() {
			err := backupBucketValidator.Validate(ctx, &corev1.Secret{}, nil)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("wrong object type *v1.Secret for object"))
		})

		It("should succeed when BackupBucket is created with valid spec", func() {
			backupBucket := &gardencore.BackupBucket{
				Spec: gardencore.BackupBucketSpec{
					CredentialsRef: credentialsRef,
					ProviderConfig: &runtime.RawExtension{
						Raw: []byte(`{"apiVersion": "openstack.provider.extensions.gardener.cloud/v1alpha1", "kind": "BackupBucketConfig"}`),
					},
				},
			}

			Expect(backupBucketValidator.Validate(ctx, backupBucket, nil)).To(Succeed())
		})
	})
})
