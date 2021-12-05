// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	"github.com/gardener/gardener/extensions/pkg/controller/healthcheck"
	"github.com/gardener/gardener/extensions/test/integration/operation"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/test/framework"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ControlPlaneHealthCheckWithManagedResource is a convenience function to tests that an unhealthy condition in a given ManagedResource leads to an unhealthy health check condition in the given ControlPlane CRD.
func ControlPlaneHealthCheckWithManagedResource(ctx context.Context, timeout time.Duration, f *framework.ShootFramework, managedResourceName string, healthConditionType gardencorev1beta1.ConditionType) error {
	return TestHealthCheckWithManagedResource(
		ctx,
		timeout,
		f,
		extensionsv1alpha1.SchemeGroupVersion.WithKind(extensionsv1alpha1.ControlPlaneResource),
		f.Shoot.GetName(),
		managedResourceName,
		healthConditionType)
}

// WorkerHealthCheckWithManagedResource is a convenience function to tests that an unhealthy condition in a given ManagedResource leads to an unhealthy health check condition in the given Worker CRD.
func WorkerHealthCheckWithManagedResource(ctx context.Context, timeout time.Duration, f *framework.ShootFramework, managedResourceName string, healthConditionType gardencorev1beta1.ConditionType) error {
	return TestHealthCheckWithManagedResource(
		ctx,
		timeout,
		f,
		extensionsv1alpha1.SchemeGroupVersion.WithKind(extensionsv1alpha1.WorkerResource),
		f.Shoot.GetName(),
		managedResourceName,
		healthConditionType)
}

// NetworkHealthCheckWithManagedResource is a convenience function to tests that an unhealthy condition in a given ManagedResource leads to an unhealthy health check condition in the given Network CRD.
func NetworkHealthCheckWithManagedResource(ctx context.Context, timeout time.Duration, f *framework.ShootFramework, managedResourceName string, healthConditionType gardencorev1beta1.ConditionType) error {
	return TestHealthCheckWithManagedResource(
		ctx,
		timeout,
		f,
		extensionsv1alpha1.SchemeGroupVersion.WithKind(extensionsv1alpha1.NetworkResource),
		f.Shoot.GetName(),
		managedResourceName,
		healthConditionType)
}

// ExtensionHealthCheckWithManagedResource is a convenience function to tests that an unhealthy condition in a given ManagedResource leads to an unhealthy health check condition in the given Extension CRD.
func ExtensionHealthCheckWithManagedResource(ctx context.Context, timeout time.Duration, f *framework.ShootFramework, extensionName string, managedResourceName string, healthConditionType gardencorev1beta1.ConditionType) error {
	return TestHealthCheckWithManagedResource(
		ctx,
		timeout,
		f,
		extensionsv1alpha1.SchemeGroupVersion.WithKind(extensionsv1alpha1.ExtensionResource),
		extensionName,
		managedResourceName,
		healthConditionType)
}

// ContainerRuntimeHealthCheckWithManagedResource is a convenience function to tests that an unhealthy condition in a given ManagedResource leads to an unhealthy health check condition in the given ContainerRuntime CRD.
func ContainerRuntimeHealthCheckWithManagedResource(ctx context.Context, timeout time.Duration, f *framework.ShootFramework, extensionName string, managedResourceName string, healthConditionType gardencorev1beta1.ConditionType) error {
	return TestHealthCheckWithManagedResource(
		ctx,
		timeout,
		f,
		extensionsv1alpha1.SchemeGroupVersion.WithKind(extensionsv1alpha1.ContainerRuntimeResource),
		extensionName,
		managedResourceName,
		healthConditionType)
}

// TestHealthCheckWithManagedResource tests that an unhealthy condition in a given ManagedResource leads to an unhealthy health check condition in the given CRD.
// To be able to manipulate the ManagedResource with an unhealthy condition, the function needs to scale down the Gardener Resource Manager.
// After the unhealthy condition is observed in the Extension CRD, the function scales up the Gardener Resource Manager again and waits for the ManagedResource to be healthy.
// This function is used by integration tests of Gardener extensions to check their health checks on ManagedResources.
func TestHealthCheckWithManagedResource(ctx context.Context, timeout time.Duration, f *framework.ShootFramework, extensionKind schema.GroupVersionKind, extensionName string, managedResourceName string, healthConditionType gardencorev1beta1.ConditionType) error {
	var (
		err                                              error
		resourceManagerDeploymentReplicasBeforeScaledown *int32

		cancel context.CancelFunc
	)
	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()

	resourceManagerDeploymentReplicasBeforeScaledown, err = operation.ScaleGardenerResourceManager(ctx, f.ShootSeedNamespace(), f.SeedClient.Client(), pointer.Int32(0))
	if err != nil {
		return err
	}

	defer func() {
		f.Logger.Infof("Cleanup: scaling resource manager to %d replicas again", int(*resourceManagerDeploymentReplicasBeforeScaledown))
		// scale up again
		_, err = operation.ScaleGardenerResourceManager(ctx, f.ShootSeedNamespace(), f.SeedClient.Client(), resourceManagerDeploymentReplicasBeforeScaledown)
		framework.ExpectNoError(err)

		// wait until healthy again
		f.Logger.Info("Cleanup: wait until health check is successful again")
		err = operation.WaitForExtensionCondition(ctx, f.Logger, f.SeedClient.Client(), extensionKind, types.NamespacedName{
			Namespace: f.ShootSeedNamespace(),
			Name:      extensionName,
		}, healthConditionType, gardencorev1beta1.ConditionTrue, healthcheck.ReasonSuccessful)
		framework.ExpectNoError(err)
	}()
	managedResource := resourcesv1alpha1.ManagedResource{}
	if err = f.SeedClient.Client().Get(ctx, kutil.Key(f.ShootSeedNamespace(), managedResourceName), &managedResource); err != nil {
		return err
	}
	// overwrite Condition with type ResourcesHealthy on the managed resource to make the health check in the provider fail
	managedResourceCondition := gardencorev1beta1.Condition{
		Type:   resourcesv1alpha1.ResourcesHealthy,
		Status: gardencorev1beta1.ConditionFalse,
		Reason: "dummyFailureReason",
	}
	if err = controllerutils.TryUpdateStatus(ctx, retry.DefaultBackoff, f.SeedClient.Client(), &managedResource, func() error {
		newConditions := v1beta1helper.MergeConditions(managedResource.Status.Conditions, managedResourceCondition)
		managedResource.Status.Conditions = newConditions
		return nil
	}); err != nil {
		return err
	}

	// wait until the health check reports the unhealthy managed resource
	return operation.WaitForExtensionCondition(ctx, f.Logger, f.SeedClient.Client(), extensionKind, types.NamespacedName{
		Namespace: f.ShootSeedNamespace(),
		Name:      extensionName,
	}, healthConditionType, gardencorev1beta1.ConditionFalse, healthcheck.ReasonUnsuccessful)
}

// ControlPlaneHealthCheckDeleteSeedDeployment is a convenience function to delete the given deployment and check the control plane resource condition.
func ControlPlaneHealthCheckDeleteSeedDeployment(ctx context.Context, f *framework.ShootFramework, controlPlaneName, deploymentName string, healthConditionType gardencorev1beta1.ConditionType) error {
	return deleteSeedDeploymentCheck(ctx, f, extensionsv1alpha1.SchemeGroupVersion.WithKind(extensionsv1alpha1.ControlPlaneResource), controlPlaneName, deploymentName, healthConditionType)
}

// WorkerHealthCheckDeleteSeedDeployment is a convenience function to delete the given deployment and check the worker resource condition.
func WorkerHealthCheckDeleteSeedDeployment(ctx context.Context, f *framework.ShootFramework, controlPlaneName, deploymentName string, healthConditionType gardencorev1beta1.ConditionType) error {
	return deleteSeedDeploymentCheck(ctx, f, extensionsv1alpha1.SchemeGroupVersion.WithKind(extensionsv1alpha1.WorkerResource), controlPlaneName, deploymentName, healthConditionType)
}

func deleteSeedDeploymentCheck(ctx context.Context, f *framework.ShootFramework, extensionKind schema.GroupVersionKind, controlPlaneName, deploymentName string, healthConditionType gardencorev1beta1.ConditionType) error {
	var err error

	cloudControllerDeployment := appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: deploymentName, Namespace: f.ShootSeedNamespace()}}
	if err := f.SeedClient.Client().Delete(ctx, &cloudControllerDeployment); err != nil {
		return err
	}
	defer func() {
		err = f.GardenerFramework.UpdateShoot(ctx, f.Shoot, func(shoot *gardencorev1beta1.Shoot) error {
			shoot.Annotations[v1beta1constants.GardenerOperation] = v1beta1constants.GardenerOperationReconcile
			return nil
		})
		framework.ExpectNoError(err)

		// then make sure the condition is fine again after reconciliation
		err = operation.WaitForExtensionCondition(ctx, f.Logger, f.SeedClient.Client(), extensionKind, types.NamespacedName{
			Namespace: f.ShootSeedNamespace(),
			Name:      controlPlaneName,
		}, healthConditionType, gardencorev1beta1.ConditionTrue, healthcheck.ReasonSuccessful)
		framework.ExpectNoError(err)
	}()
	return operation.WaitForExtensionCondition(ctx, f.Logger, f.SeedClient.Client(), extensionKind, types.NamespacedName{
		Namespace: f.ShootSeedNamespace(),
		Name:      controlPlaneName,
	}, healthConditionType, gardencorev1beta1.ConditionUnknown, gardencorev1beta1.ConditionCheckError)
}

// MachineDeletionHealthCheck is a convenience function to delete the first machine and check the worker resource condition.
func MachineDeletionHealthCheck(ctx context.Context, f *framework.ShootFramework) error {
	var err error
	machineList := machinev1alpha1.MachineList{}
	if err := f.SeedClient.Client().List(ctx, &machineList, client.InNamespace(f.ShootSeedNamespace())); err != nil {
		return err
	}

	if len(machineList.Items) == 0 {
		return fmt.Errorf("trying to delete machine as part of health check test from a cluster with no nodes (seed: %s, namespace: %s)", f.Seed.Name, f.ShootSeedNamespace())
	}

	machine := machineList.Items[0]
	err = f.SeedClient.Client().Delete(ctx, &machine)
	if err != nil {
		return err
	}

	if err := operation.WaitForExtensionCondition(ctx, f.Logger, f.SeedClient.Client(), extensionsv1alpha1.SchemeGroupVersion.WithKind(extensionsv1alpha1.WorkerResource), types.NamespacedName{
		Namespace: f.ShootSeedNamespace(),
		Name:      f.Shoot.GetName(),
	}, gardencorev1beta1.ShootEveryNodeReady, gardencorev1beta1.ConditionFalse, healthcheck.ReasonUnsuccessful); err != nil {
		return err
	}

	// then make sure the condition is fine again after reconciliation
	return operation.WaitForExtensionCondition(ctx, f.Logger, f.SeedClient.Client(), extensionsv1alpha1.SchemeGroupVersion.WithKind(extensionsv1alpha1.WorkerResource), types.NamespacedName{
		Namespace: f.ShootSeedNamespace(),
		Name:      f.Shoot.GetName(),
	}, gardencorev1beta1.ShootEveryNodeReady, gardencorev1beta1.ConditionTrue, healthcheck.ReasonSuccessful)
}
