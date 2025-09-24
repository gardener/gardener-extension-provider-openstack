// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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

// Migrate deletes the k8s infrastructure resources without deleting the corresponding resources in the IaaS provider.
func (a *actuator) Migrate(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure, _ *controller.Cluster) error {
	tf, err := newTerraformer(log, a.restConfig, terraformerPurpose, infra, a.disableProjectedTokenMount)
	if err != nil {
		return err
	}
	return util.DetermineError(cleanupTerraformerResources(ctx, tf), helper.KnownCodes)
}
