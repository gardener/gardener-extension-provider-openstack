// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package infrastructure

import (
	"context"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/terraformer"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
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
			return a.addErrorCodes(err)
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
