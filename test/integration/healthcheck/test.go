// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

/**
	Overview
		- Tests the health checks of the extension: provider-openstack.
		- Manipulates health check relevant resources and expects the extension-provider to properly report the results as conditions in the respective CRD (ControlPlane(Type Normal) & Worker CRD).

	Prerequisites
		- A Shoot exists.

	Test-cases:
		1) ControlPlane
			1.1) HealthCondition Type: Shoot ControlPlaneHealthy
				- delete the deployment 'cloud-controller-manager' and verify health check conditions in the ControlPlane status.
			1.2) HealthCondition Type: Shoot SystemComponentsHealthy
				- update the ManagedResource 'extension-controlplane-shoot' with an unhealthy condition and verify health check conditions in the ControlPlane status.
		2) Worker
			2.1) HealthCondition Type: Shoot EveryNodeReady
				- delete a machine of the shoot cluster and verify the health check conditions in the Worker status report a missing node.
 **/

package healthcheck

import (
	"context"
	"fmt"
	"time"

	genericcontrolplaneactuator "github.com/gardener/gardener/extensions/pkg/controller/controlplane/genericactuator"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/test/framework"
	healthcheckoperation "github.com/gardener/gardener/test/testmachinery/extensions/healthcheck"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/onsi/ginkgo/v2"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
)

const (
	timeout               = 10 * time.Minute
	nodeRecreationTimeout = 20 * time.Minute
	setupContextTimeout   = 2 * time.Minute
)

var _ = ginkgo.Describe("Provider-openstack integration test: health checks", func() {
	f := createShootFramework()

	ginkgo.Context("ControlPlane", func() {
		ginkgo.Context("Condition type: ShootControlPlaneHealthy", func() {
			f.Serial().Release().CIt(fmt.Sprintf("ControlPlane CRD should contain unhealthy condition because the deployment '%s' cannot be found in the shoot namespace in the seed", openstack.CloudControllerManagerName), func(ctx context.Context) {
				err := healthcheckoperation.ControlPlaneHealthCheckDeleteSeedDeployment(ctx, f, f.Shoot.GetName(), openstack.CloudControllerManagerName, gardencorev1beta1.ShootControlPlaneHealthy)
				framework.ExpectNoError(err)
			}, timeout)
		})

		ginkgo.Context("Condition type: ShootSystemComponentsHealthy", func() {
			f.Serial().Release().CIt(fmt.Sprintf("ControlPlane CRD should contain unhealthy condition due to ManagedResource ('%s') unhealthy", genericcontrolplaneactuator.ControlPlaneShootChartResourceName), func(ctx context.Context) {
				err := healthcheckoperation.ControlPlaneHealthCheckWithManagedResource(ctx, setupContextTimeout, f, genericcontrolplaneactuator.ControlPlaneShootChartResourceName, gardencorev1beta1.ShootSystemComponentsHealthy)
				framework.ExpectNoError(err)
			}, timeout)
		})
	})

	ginkgo.Context("Worker", func() {
		ginkgo.Context("Condition type: ShootEveryNodeReady", func() {
			f.Serial().Release().CIt("Worker CRD should contain unhealthy condition because not enough machines are available", func(ctx context.Context) {
				err := healthcheckoperation.MachineDeletionHealthCheck(ctx, f)
				framework.ExpectNoError(err)
			}, nodeRecreationTimeout)
		})
	})
})

func createShootFramework() *framework.ShootFramework {
	extensionSeedScheme := kubernetes.SeedScheme
	seedSchemeBuilder := runtime.NewSchemeBuilder(
		machinev1alpha1.AddToScheme,
	)
	utilruntime.Must(seedSchemeBuilder.AddToScheme(extensionSeedScheme))
	return framework.NewShootFramework(&framework.ShootConfig{
		SeedScheme: nil,
	})
}
