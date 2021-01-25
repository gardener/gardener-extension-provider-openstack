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
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/pkg/api/extensions"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardenv1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

type reconciler struct {
	logger              logr.Logger
	actuator            HealthCheckActuator
	client              client.Client
	recorder            record.EventRecorder
	registeredExtension RegisteredExtension
	syncPeriod          metav1.Duration
}

const (
	// ReasonUnsuccessful is the reason phrase for the health check condition if one or more of its tests failed.
	ReasonUnsuccessful = "HealthCheckUnsuccessful"
	// ReasonProgressing is the reason phrase for the health check condition if one or more of its tests are progressing.
	ReasonProgressing = "HealthCheckProgressing"
	// ReasonSuccessful is the reason phrase for the health check condition if all tests are successful.
	ReasonSuccessful = "HealthCheckSuccessful"
)

// NewReconciler creates a new performHealthCheck.Reconciler that reconciles
// the registered extension resources (Gardener's `extensions.gardener.cloud` API group).
func NewReconciler(mgr manager.Manager, actuator HealthCheckActuator, registeredExtension RegisteredExtension, syncPeriod metav1.Duration) reconcile.Reconciler {
	return &reconciler{
		logger:              log.Log.WithName(ControllerName),
		actuator:            actuator,
		recorder:            mgr.GetEventRecorderFor(ControllerName),
		registeredExtension: registeredExtension,
		syncPeriod:          syncPeriod,
	}
}

func (r *reconciler) InjectFunc(f inject.Func) error {
	return f(r.actuator)
}

func (r *reconciler) InjectClient(client client.Client) error {
	r.client = client
	return nil
}

func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	extension := r.registeredExtension.getExtensionObjFunc()

	if err := r.client.Get(ctx, request.NamespacedName, extension); err != nil {
		if errors.IsNotFound(err) {
			return r.resultWithRequeue(), nil
		}
		return reconcile.Result{}, err
	}

	acc, err := extensions.Accessor(extension.DeepCopyObject())
	if err != nil {
		return reconcile.Result{}, err
	}

	if acc.GetDeletionTimestamp() != nil {
		r.logger.V(6).Info("Do not perform HealthCheck for extension resource. Extension is being deleted.", "name", acc.GetName(), "namespace", acc.GetNamespace())
		return reconcile.Result{}, nil
	}

	if isInMigration(acc) {
		r.logger.Info("Do not perform HealthCheck for extension resource. Extension is being migrated.", "name", acc.GetName(), "namespace", acc.GetNamespace())
		return reconcile.Result{}, nil
	}

	cluster, err := extensionscontroller.GetCluster(ctx, r.client, acc.GetNamespace())
	if err != nil {
		return reconcile.Result{}, err
	}

	if extensionscontroller.IsHibernated(cluster) {
		var conditions []condition
		for _, healthConditionType := range r.registeredExtension.healthConditionTypes {
			conditionBuilder, err := gardencorev1beta1helper.NewConditionBuilder(gardencorev1beta1.ConditionType(healthConditionType))
			if err != nil {
				return reconcile.Result{}, err
			}

			conditions = append(conditions, extensionConditionHibernated(conditionBuilder, healthConditionType))
		}
		if err := r.updateExtensionConditions(ctx, extension, conditions...); err != nil {
			return reconcile.Result{}, err
		}

		r.logger.V(6).Info("Do not perform HealthCheck for extension resource. Shoot is hibernated.", "name", acc.GetName(), "namespace", acc.GetNamespace(), "kind", acc.GetObjectKind().GroupVersionKind().Kind)
		return reconcile.Result{}, nil
	}

	r.logger.V(6).Info("Performing health check", "name", acc.GetName(), "namespace", acc.GetNamespace(), "kind", acc.GetObjectKind().GroupVersionKind().Kind)
	return r.performHealthCheck(ctx, request, extension)
}

func (r *reconciler) performHealthCheck(ctx context.Context, request reconcile.Request, extension extensionsv1alpha1.Object) (reconcile.Result, error) {
	healthCheckResults, err := r.actuator.ExecuteHealthCheckFunctions(ctx, types.NamespacedName{Namespace: request.Namespace, Name: request.Name})
	if err != nil {
		var conditions []condition
		r.logger.Info("Failed to execute healthChecks. Updating each HealthCheckCondition for the extension resource to ConditionCheckError.", "kind", r.registeredExtension.groupVersionKind.Kind, "health condition types", r.registeredExtension.healthConditionTypes, "name", request.Name, "namespace", request.Namespace, "error", err.Error())
		for _, healthConditionType := range r.registeredExtension.healthConditionTypes {
			conditionBuilder, buildErr := gardencorev1beta1helper.NewConditionBuilder(gardencorev1beta1.ConditionType(healthConditionType))
			if buildErr != nil {
				return reconcile.Result{}, buildErr
			}

			conditions = append(conditions, extensionConditionFailedToExecute(conditionBuilder, healthConditionType, r.registeredExtension.groupVersionKind.Kind, err))
		}
		if updateErr := r.updateExtensionConditions(ctx, extension, conditions...); updateErr != nil {
			return reconcile.Result{}, updateErr
		}
		return r.resultWithRequeue(), nil
	}

	conditions := make([]condition, 0, len(*healthCheckResults))
	for _, healthCheckResult := range *healthCheckResults {
		conditionBuilder, err := gardencorev1beta1helper.NewConditionBuilder(gardencorev1beta1.ConditionType(healthCheckResult.HealthConditionType))
		if err != nil {
			return reconcile.Result{}, err
		}

		var logger logr.Logger
		if healthCheckResult.Status == gardencorev1beta1.ConditionTrue || healthCheckResult.Status == gardencorev1beta1.ConditionProgressing {
			logger = r.logger.V(6)
		} else {
			logger = r.logger
		}

		if healthCheckResult.Status == gardencorev1beta1.ConditionProgressing || healthCheckResult.Status == gardencorev1beta1.ConditionFalse {
			if healthCheckResult.FailedChecks > 0 {
				r.logger.Info("Updating HealthCheckCondition for extension resource to ConditionCheckError.", "kind", r.registeredExtension.groupVersionKind.Kind, "health condition type", healthCheckResult.HealthConditionType, "name", request.Name, "namespace", request.Namespace)
				conditions = append(conditions, extensionConditionCheckError(conditionBuilder, healthCheckResult.HealthConditionType, r.registeredExtension.groupVersionKind.Kind, healthCheckResult))
				continue
			}

			logger.Info("Health check for extension resource progressing or unsuccessful.", "kind", fmt.Sprintf("%s.%s.%s", r.registeredExtension.groupVersionKind.Kind, r.registeredExtension.groupVersionKind.Group, r.registeredExtension.groupVersionKind.Version), "name", request.Name, "namespace", request.Namespace, "failed", healthCheckResult.FailedChecks, "progressing", healthCheckResult.ProgressingChecks, "successful", healthCheckResult.SuccessfulChecks, "details", healthCheckResult.GetDetails())
			conditions = append(conditions, extensionConditionUnsuccessful(conditionBuilder, healthCheckResult.HealthConditionType, extension, healthCheckResult))
			continue
		}

		logger.Info("Health check for extension resource successful.", "kind", r.registeredExtension.groupVersionKind.Kind, "health condition type", healthCheckResult.HealthConditionType, "name", request.Name, "namespace", request.Namespace)
		conditions = append(conditions, extensionConditionSuccessful(conditionBuilder, healthCheckResult.HealthConditionType, healthCheckResult))
	}

	if err := r.updateExtensionConditions(ctx, extension, conditions...); err != nil {
		return reconcile.Result{}, err
	}

	return r.resultWithRequeue(), nil
}

func extensionConditionFailedToExecute(conditionBuilder gardencorev1beta1helper.ConditionBuilder, healthConditionType string, kind string, executionError error) condition {
	conditionBuilder.
		WithStatus(gardencorev1beta1.ConditionUnknown).
		WithReason(gardencorev1beta1.ConditionCheckError).
		WithMessage(fmt.Sprintf("failed to execute health checks for '%s': %v", kind, executionError.Error()))
	return condition{
		builder:             conditionBuilder,
		healthConditionType: healthConditionType,
	}
}

func extensionConditionCheckError(conditionBuilder gardencorev1beta1helper.ConditionBuilder, healthConditionType string, kind string, healthCheckResult Result) condition {
	conditionBuilder.
		WithStatus(gardencorev1beta1.ConditionUnknown).
		WithReason(gardencorev1beta1.ConditionCheckError).
		WithMessage(fmt.Sprintf("failed to execute %d/%d health checks for '%s': %v", healthCheckResult.FailedChecks, healthCheckResult.SuccessfulChecks+healthCheckResult.UnsuccessfulChecks+healthCheckResult.FailedChecks, kind, healthCheckResult.GetDetails()))
	return condition{
		builder:             conditionBuilder,
		healthConditionType: healthConditionType,
	}
}

func extensionConditionUnsuccessful(conditionBuilder gardencorev1beta1helper.ConditionBuilder, healthConditionType string, extension extensionsv1alpha1.Object, healthCheckResult Result) condition {
	var (
		numberOfChecks = healthCheckResult.UnsuccessfulChecks + healthCheckResult.ProgressingChecks + healthCheckResult.SuccessfulChecks
		detail         = fmt.Sprintf("Health check summary: %d/%d unsuccessful, %d/%d progressing, %d/%d successful. %v", healthCheckResult.UnsuccessfulChecks, numberOfChecks, healthCheckResult.ProgressingChecks, numberOfChecks, healthCheckResult.SuccessfulChecks, numberOfChecks, healthCheckResult.GetDetails())
		status         = gardencorev1beta1.ConditionFalse
		reason         = ReasonUnsuccessful
	)

	if healthCheckResult.ProgressingChecks > 0 && healthCheckResult.ProgressingThreshold != nil {
		if oldCondition := gardencorev1beta1helper.GetCondition(extension.GetExtensionStatus().GetConditions(), gardencorev1beta1.ConditionType(healthConditionType)); oldCondition == nil {
			status = gardencorev1beta1.ConditionProgressing
			reason = ReasonProgressing
		} else if oldCondition.Status != gardencorev1beta1.ConditionFalse {
			delta := time.Now().UTC().Sub(oldCondition.LastTransitionTime.Time.UTC())
			if oldCondition.Status == gardencorev1beta1.ConditionTrue || delta <= *healthCheckResult.ProgressingThreshold {
				status = gardencorev1beta1.ConditionProgressing
				reason = ReasonProgressing
			}
		}
	}

	conditionBuilder.
		WithStatus(status).
		WithReason(reason).
		WithCodes(healthCheckResult.Codes...).
		WithMessage(detail)
	return condition{
		builder:             conditionBuilder,
		healthConditionType: healthConditionType,
	}
}

func extensionConditionSuccessful(conditionBuilder gardencorev1beta1helper.ConditionBuilder, healthConditionType string, healthCheckResult Result) condition {
	conditionBuilder.
		WithStatus(gardencorev1beta1.ConditionTrue).
		WithReason(ReasonSuccessful).
		WithMessage(fmt.Sprintf("(%d/%d) Health checks successful", healthCheckResult.SuccessfulChecks, healthCheckResult.SuccessfulChecks))
	return condition{
		builder:             conditionBuilder,
		healthConditionType: healthConditionType,
	}
}

func extensionConditionHibernated(conditionBuilder gardencorev1beta1helper.ConditionBuilder, healthConditionType string) condition {
	conditionBuilder.
		WithStatus(gardencorev1beta1.ConditionTrue).
		WithReason(ReasonSuccessful).
		WithMessage("Shoot is hibernated")
	return condition{
		builder:             conditionBuilder,
		healthConditionType: healthConditionType,
	}
}

type condition struct {
	builder             gardencorev1beta1helper.ConditionBuilder
	healthConditionType string
}

func (r *reconciler) updateExtensionConditions(ctx context.Context, extension extensionsv1alpha1.Object, conditions ...condition) error {
	return extensionscontroller.TryPatchStatus(ctx, retry.DefaultBackoff, r.client, extension, func() error {
		for _, cond := range conditions {
			now := metav1.Now()
			if c := gardencorev1beta1helper.GetCondition(extension.GetExtensionStatus().GetConditions(), gardencorev1beta1.ConditionType(cond.healthConditionType)); c != nil {
				cond.builder.WithOldCondition(*c)
			}
			updatedCondition, _ := cond.builder.WithNowFunc(func() metav1.Time { return now }).Build()
			// always update - the Gardenlet expects a recent health check
			updatedCondition.LastUpdateTime = now
			extension.GetExtensionStatus().SetConditions(gardencorev1beta1helper.MergeConditions(extension.GetExtensionStatus().GetConditions(), updatedCondition))
		}
		return nil
	})
}

func (r *reconciler) resultWithRequeue() reconcile.Result {
	return reconcile.Result{RequeueAfter: r.syncPeriod.Duration}
}

func isInMigration(accessor extensionsv1alpha1.Object) bool {
	annotations := accessor.GetAnnotations()
	if annotations != nil &&
		annotations[gardenv1beta1constants.GardenerOperation] == gardenv1beta1constants.GardenerOperationMigrate {
		return true
	}

	status := accessor.GetExtensionStatus()
	if status == nil {
		return false
	}

	lastOperation := status.GetLastOperation()
	return lastOperation != nil && lastOperation.Type == gardencorev1beta1.LastOperationTypeMigrate
}
