// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var _ = Describe("BackupBucket", func() {
	Describe("ValidateBackupBucketCredentialsRef", func() {
		var fldPath *field.Path

		BeforeEach(func() {
			fldPath = field.NewPath("spec", "credentialsRef")
		})

		It("should forbid nil credentialsRef", func() {
			errs := ValidateBackupBucketCredentialsRef(nil, fldPath)
			Expect(errs).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeRequired),
				"Field":  Equal("spec.credentialsRef"),
				"Detail": Equal("must be set"),
			}))))
		})

		It("should forbid v1.ConfigMap credentials", func() {
			credsRef := &corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "ConfigMap",
				Name:       "my-creds",
				Namespace:  "my-namespace",
			}
			errs := ValidateBackupBucketCredentialsRef(credsRef, fldPath)
			Expect(errs).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeNotSupported),
				"Field":  Equal("spec.credentialsRef"),
				"Detail": Equal("supported values: \"/v1, Kind=Secret\""),
			}))))
		})

		It("should forbid security.gardener.cloud/v1alpha1.WorkloadIdentity credentials", func() {
			credsRef := &corev1.ObjectReference{
				APIVersion: "security.gardener.cloud/v1alpha1",
				Kind:       "WorkloadIdentity",
				Name:       "my-creds",
				Namespace:  "my-namespace",
			}
			errs := ValidateBackupBucketCredentialsRef(credsRef, fldPath)
			Expect(errs).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeNotSupported),
				"Field":  Equal("spec.credentialsRef"),
				"Detail": Equal("supported values: \"/v1, Kind=Secret\""),
			}))))
		})

		It("should allow v1.Secret credentials", func() {
			credsRef := &corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Name:       "my-creds",
				Namespace:  "my-namespace",
			}
			errs := ValidateBackupBucketCredentialsRef(credsRef, fldPath)
			Expect(errs).To(BeEmpty())
		})
	})
})
