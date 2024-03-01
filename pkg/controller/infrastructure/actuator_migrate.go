// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"
	"fmt"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/internal"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/internal/infrastructure"
)

func (a *actuator) Migrate(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster) error {
	flowState, err := a.getStateFromInfraStatus(ctx, infra)
	if err != nil {
		return err
	}
	if flowState != nil {
		return nil // nothing to do if already using new flow without Terraformer
	}
	return a.migrateWithTerraformer(ctx, log, infra, cluster)
}

func (a *actuator) migrateWithTerraformer(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure, _ *extensionscontroller.Cluster) error {
	tf, err := internal.NewTerraformer(log, a.restConfig, infrastructure.TerraformerPurpose, infra, a.disableProjectedTokenMount)
	if err != nil {
		return util.DetermineError(fmt.Errorf("could not create the Terraformer: %+v", err), helper.KnownCodes)
	}

	if err := tf.CleanupConfiguration(ctx); err != nil {
		return err
	}
	return tf.RemoveTerraformerFinalizerFromConfig(ctx)
}
