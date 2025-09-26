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

	"github.com/gardener/gardener-extension-provider-openstack/pkg/admission/validator"
)

var _ = Describe("Seed Validator", func() {
	Describe("#Validate", func() {
		var (
			ctx           context.Context
			seedValidator extensionswebhook.Validator
		)

		BeforeEach(func() {
			ctx = context.TODO()

			seedValidator = validator.NewSeedValidator()
		})

		It("should return err when obj is not a gardencore.Seed", func() {
			Expect(seedValidator.Validate(ctx, &corev1.Secret{}, nil)).To(MatchError("wrong object type *v1.Secret for object"))
		})

		It("should succeed to create seed when backup is unset", func() {
			seed := &gardencore.Seed{
				Spec: gardencore.SeedSpec{
					Backup: nil,
				},
			}

			Expect(seedValidator.Validate(ctx, seed, nil)).To(Succeed())
		})
	})
})
