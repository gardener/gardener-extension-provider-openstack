// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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
	"github.com/gardener/gardener-extension-provider-openstack/pkg/internal"
	infrainternal "github.com/gardener/gardener-extension-provider-openstack/pkg/internal/infrastructure"
)

// Migrate deletes the k8s infrastructure resources without deleting the corresponding resources in the IaaS provider.
func (a *actuator) Migrate(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure, _ *controller.Cluster) error {
	tf, err := internal.NewTerraformer(log, a.restConfig, infrainternal.TerraformerPurpose, infra, a.disableProjectedTokenMount)
	if err != nil {
		return err
	}
	return util.DetermineError(CleanupTerraformerResources(ctx, tf), helper.KnownCodes)
}
