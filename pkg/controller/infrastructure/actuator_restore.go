// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/terraformer"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
)

// Restore implements infrastructure.Actuator.
func (a *actuator) Restore(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	flowState, err := a.getStateFromInfraStatus(ctx, infra)
	if err != nil {
		return err
	}
	if flowState != nil {
		return a.reconcileWithFlow(ctx, log, infra, cluster, flowState)
	}
	if a.shouldUseFlow(infra, cluster) {
		flowState, err = a.migrateFromTerraformerState(ctx, log, infra)
		if err != nil {
			return util.DetermineError(err, helper.KnownCodes)
		}
		return a.reconcileWithFlow(ctx, log, infra, cluster, flowState)
	}
	return a.restoreWithTerraformer(ctx, log, infra, cluster)
}

func (a *actuator) restoreWithTerraformer(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	terraformState, err := terraformer.UnmarshalRawState(infra.Status.State)
	if err != nil {
		return err
	}
	return a.reconcileWithTerraformer(ctx, log, infra, cluster, terraformer.CreateOrUpdateState{State: &terraformState.Data})
}
