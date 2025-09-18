// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"context"
	"fmt"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"

	openstackvalidation "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/validation"
)

// backupBucketValidator validates create and update operations on BackupBucket resources,
type backupBucketValidator struct{}

// NewBackupBucketValidator returns a new instance of backupBucket validator.
func NewBackupBucketValidator() extensionswebhook.Validator {
	return &backupBucketValidator{}
}

// Validate validates the BackupBucket resource during create or update operations.
func (s *backupBucketValidator) Validate(_ context.Context, newObj, _ client.Object) error {
	backupBucket, ok := newObj.(*gardencore.BackupBucket)
	if !ok {
		return fmt.Errorf("wrong object type %T for object", newObj)
	}

	return s.validateBackupBucket(backupBucket).ToAggregate()
}

// validateBackupBucket validates the BackupBucket object.
func (b *backupBucketValidator) validateBackupBucket(backupBucket *gardencore.BackupBucket) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, openstackvalidation.ValidateBackupBucketCredentialsRef(backupBucket.Spec.CredentialsRef, field.NewPath("spec", "credentialsRef"))...)

	return allErrs
}
