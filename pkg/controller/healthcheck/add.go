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

package healthcheck

import (
	"context"
	"time"

	healthcheckconfig "github.com/gardener/gardener/extensions/pkg/apis/config"
	"github.com/gardener/gardener/extensions/pkg/controller/healthcheck"
	"github.com/gardener/gardener/extensions/pkg/controller/healthcheck/general"
	"github.com/gardener/gardener/extensions/pkg/controller/healthcheck/worker"
	extensionspredicate "github.com/gardener/gardener/extensions/pkg/predicate"
	"github.com/gardener/gardener/extensions/pkg/util"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
)

var (
	defaultSyncPeriod = time.Second * 30
	// DefaultAddOptions are the default DefaultAddArgs for AddToManager.
	DefaultAddOptions = healthcheck.DefaultAddArgs{
		HealthCheckConfig: healthcheckconfig.HealthCheckConfig{
			SyncPeriod: metav1.Duration{Duration: defaultSyncPeriod},
			ShootRESTOptions: &healthcheckconfig.RESTOptions{
				QPS:   pointer.Float32(100),
				Burst: pointer.Int(130),
			},
		},
	}
	// GardenletManagesMCM specifies whether the machine-controller-manager is managed by gardenlet.
	GardenletManagesMCM bool
)

// RegisterHealthChecks registers health checks for each extension resource
// HealthChecks are grouped by extension (e.g worker), extension.type (e.g aws) and  Health Check Type (e.g ShootControlPlaneHealthy)
func RegisterHealthChecks(ctx context.Context, mgr manager.Manager, opts healthcheck.DefaultAddArgs) error {
	if err := healthcheck.DefaultRegistration(
		ctx,
		openstack.Type,
		extensionsv1alpha1.SchemeGroupVersion.WithKind(extensionsv1alpha1.ControlPlaneResource),
		func() client.ObjectList { return &extensionsv1alpha1.ControlPlaneList{} },
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.ControlPlane{} },
		mgr,
		opts,
		[]predicate.Predicate{extensionspredicate.HasPurpose(extensionsv1alpha1.Normal)},
		[]healthcheck.ConditionTypeToHealthCheck{
			{
				ConditionType: string(gardencorev1beta1.ShootControlPlaneHealthy),
				HealthCheck:   general.NewSeedDeploymentHealthChecker(openstack.CloudControllerManagerName),
			},
			{
				ConditionType: string(gardencorev1beta1.ShootControlPlaneHealthy),
				HealthCheck:   general.NewSeedDeploymentHealthChecker(openstack.CSIControllerName),
			},
			{
				ConditionType: string(gardencorev1beta1.ShootControlPlaneHealthy),
				HealthCheck:   general.NewSeedDeploymentHealthChecker(openstack.CSISnapshotControllerName),
			},
			{
				ConditionType: string(gardencorev1beta1.ShootControlPlaneHealthy),
				HealthCheck:   general.NewSeedDeploymentHealthChecker(openstack.CSISnapshotValidationName),
			},
		},
		sets.New[gardencorev1beta1.ConditionType](),
	); err != nil {
		return err
	}

	var (
		workerHealthChecks = []healthcheck.ConditionTypeToHealthCheck{{
			ConditionType: string(gardencorev1beta1.ShootEveryNodeReady),
			HealthCheck:   worker.NewNodesChecker(),
			ErrorCodeCheckFunc: func(err error) []gardencorev1beta1.ErrorCode {
				return util.DetermineErrorCodes(err, helper.KnownCodes)
			},
		}}
		workerConditionTypesToRemove = sets.New(gardencorev1beta1.ShootControlPlaneHealthy)
	)

	if !GardenletManagesMCM {
		workerHealthChecks = append(workerHealthChecks, healthcheck.ConditionTypeToHealthCheck{
			ConditionType: string(gardencorev1beta1.ShootControlPlaneHealthy),
			HealthCheck:   general.NewSeedDeploymentHealthChecker(openstack.MachineControllerManagerName),
		})
		workerConditionTypesToRemove = workerConditionTypesToRemove.Delete(gardencorev1beta1.ShootControlPlaneHealthy)
	}

	return healthcheck.DefaultRegistration(
		ctx,
		openstack.Type,
		extensionsv1alpha1.SchemeGroupVersion.WithKind(extensionsv1alpha1.WorkerResource),
		func() client.ObjectList { return &extensionsv1alpha1.WorkerList{} },
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.Worker{} },
		mgr,
		opts,
		nil,
		workerHealthChecks,
		workerConditionTypesToRemove,
	)
}

// AddToManager adds a controller with the default Options.
func AddToManager(ctx context.Context, mgr manager.Manager) error {
	return RegisterHealthChecks(ctx, mgr, DefaultAddOptions)
}
