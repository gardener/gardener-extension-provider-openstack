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

// seedValidator validates create and update operations on seed resources,
type seedValidator struct{}

// NewSeedValidator returns a new instance of seed validator.
func NewSeedValidator() extensionswebhook.Validator {
	return &seedValidator{}
}

// Validate validates the seed resource during create or update operations.
func (s *seedValidator) Validate(_ context.Context, newObj, _ client.Object) error {
	seed, ok := newObj.(*gardencore.Seed)
	if !ok {
		return fmt.Errorf("wrong object type %T for object", newObj)
	}

	return s.validateSeed(seed).ToAggregate()
}

// validateSeed validates the seed object.
func (b *seedValidator) validateSeed(seed *gardencore.Seed) field.ErrorList {
	allErrs := field.ErrorList{}
	if seed.Spec.Backup != nil {
		allErrs = append(allErrs, openstackvalidation.ValidateBackupBucketCredentialsRef(seed.Spec.Backup.CredentialsRef, field.NewPath("spec", "backup", "credentialsRef"))...)
	}
	return allErrs
}
