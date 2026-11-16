// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
)

// Migrate removes legacy Terraformer artifacts so that they cannot block deletion of the source shoot namespace.
func (a *actuator) Migrate(ctx context.Context, _ logr.Logger, infra *extensionsv1alpha1.Infrastructure, _ *controller.Cluster) error {
	return util.DetermineError(a.cleanupTerraformerResources(ctx, infra), helper.KnownCodes)
}
