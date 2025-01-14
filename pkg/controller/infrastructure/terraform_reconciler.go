// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0
//

package infrastructure

import (
	"context"
	"fmt"
	"time"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/terraformer"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/internal"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/internal/infrastructure"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
)

// TerraformReconciler can manage infrastructure resources using Terraformer.
type TerraformReconciler struct {
	client                     k8sClient.Client
	restConfig                 *rest.Config
	log                        logr.Logger
	disableProjectedTokenMount bool
}

// NewTerraformReconciler returns a new instance of TerraformReconciler.
func NewTerraformReconciler(client k8sClient.Client, restConfig *rest.Config, log logr.Logger, disableProjectedTokenMount bool) *TerraformReconciler {
	return &TerraformReconciler{
		client:                     client,
		restConfig:                 restConfig,
		log:                        log,
		disableProjectedTokenMount: disableProjectedTokenMount,
	}
}

// Restore restores the infrastructure after a control plane migration. Effectively it performs a recovery of data from the infrastructure.status.state and
// proceeds to reconcile.
func (t *TerraformReconciler) Restore(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	var initializer terraformer.StateConfigMapInitializer

	terraformState, err := terraformer.UnmarshalRawState(infra.Status.State)
	if err != nil {
		return err
	}

	initializer = terraformer.CreateOrUpdateState{State: &terraformState.Data}
	return t.reconcile(ctx, infra, cluster, initializer)
}

// Reconcile reconciles infrastructure using Terraformer.
func (t *TerraformReconciler) Reconcile(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	err := t.reconcile(ctx, infra, cluster, terraformer.StateConfigMapInitializerFunc(terraformer.CreateState))
	return util.DetermineError(err, helper.KnownCodes)
}

func (t *TerraformReconciler) reconcile(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster, initializer terraformer.StateConfigMapInitializer) error {
	log := t.log

	log.Info("reconcile infrastructure using terraform reconciler")
	config, err := helper.InfrastructureConfigFromInfrastructure(infra)
	if err != nil {
		return err
	}

	terraformFiles, err := infrastructure.RenderTerraformerTemplate(infra, config, cluster)
	if err != nil {
		return err
	}

	// need to know if application credentials are used
	credentials, err := openstack.GetCredentials(ctx, t.client, infra.Spec.SecretRef, false)
	if err != nil {
		return err
	}

	tf, err := internal.NewTerraformerWithAuth(log, t.restConfig, infrastructure.TerraformerPurpose, infra, credentials, t.disableProjectedTokenMount)
	if err != nil {
		return err
	}

	if err := tf.
		InitializeWith(ctx, terraformer.DefaultInitializer(t.client, terraformFiles.Main, terraformFiles.Variables, terraformFiles.TFVars, initializer)).
		Apply(ctx); err != nil {
		return fmt.Errorf("failed to apply the terraform config: %w", err)
	}

	status, state, err := t.computeTerraformStatusState(ctx, tf, config)
	if err != nil {
		return err
	}

	return infrastructure.PatchProviderStatusAndState(ctx, t.client, infra, status, state)
}

// Delete deletes the infrastructure using Terraformer.
func (t *TerraformReconciler) Delete(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, c *extensions.Cluster) error {
	return util.DetermineError(t.delete(ctx, infra, c), helper.KnownCodes)
}

func (t *TerraformReconciler) delete(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *extensions.Cluster) error {
	tf, err := internal.NewTerraformer(t.log, t.restConfig, infrastructure.TerraformerPurpose, infra, t.disableProjectedTokenMount)
	if err != nil {
		return fmt.Errorf("could not create the Terraformer: %+v", err)
	}

	// terraform pod from previous reconciliation might still be running, ensure they are gone before doing any operations
	if err := tf.EnsureCleanedUp(ctx); err != nil {
		return err
	}

	// If the Terraform state is empty then we can exit early as we didn't create anything. Though, we clean up potentially
	// created configmaps/secrets related to the Terraformer.
	stateIsEmpty := tf.IsStateEmpty(ctx)
	if stateIsEmpty {
		t.log.Info("exiting early as infrastructure state is empty - nothing to do")
		return tf.CleanupConfiguration(ctx)
	}

	config, err := helper.InfrastructureConfigFromInfrastructure(infra)
	if err != nil {
		return err
	}

	terraformFiles, err := infrastructure.RenderTerraformerTemplate(infra, config, cluster)
	if err != nil {
		return err
	}

	// need to know if application credentials are used
	credentials, err := openstack.GetCredentials(ctx, t.client, infra.Spec.SecretRef, false)
	if err != nil {
		return err
	}

	openstackClient, err := openstackclient.NewOpenstackClientFromCredentials(ctx, credentials)
	if err != nil {
		return err
	}

	networkingClient, err := openstackClient.Networking()
	if err != nil {
		return err
	}
	loadbalancerClient, err := openstackClient.Loadbalancing()
	if err != nil {
		return err
	}

	stateInitializer := terraformer.StateConfigMapInitializerFunc(terraformer.CreateState)
	tf = tf.InitializeWith(ctx, terraformer.DefaultInitializer(t.client, terraformFiles.Main, terraformFiles.Variables, terraformFiles.TFVars, stateInitializer)).SetEnvVars(internal.TerraformerEnvVars(infra.Spec.SecretRef, credentials)...)

	configExists, err := tf.ConfigExists(ctx)
	if err != nil {
		return err
	}

	vars, err := tf.GetStateOutputVariables(ctx, infrastructure.TerraformOutputKeyRouterID)
	if err != nil && !terraformer.IsVariablesNotFoundError(err) {
		return err
	}

	var (
		g                              = flow.NewGraph("Openstack infrastructure destruction")
		destroyKubernetesLoadbalancers = g.Add(flow.Task{
			Name: "Destroying Kubernetes loadbalancers entries",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return t.cleanupKubernetesLoadbalancers(ctx, loadbalancerClient, vars[infrastructure.TerraformOutputKeySubnetID], infra.Namespace)
			}).RetryUntilTimeout(10*time.Second, 5*time.Minute),
			SkipIf: !configExists,
		})
		destroyKubernetesRoutes = g.Add(flow.Task{
			Name: "Destroying Kubernetes route entries",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return t.cleanupKubernetesRoutes(ctx, config, networkingClient, vars[infrastructure.TerraformOutputKeyRouterID])
			}).RetryUntilTimeout(10*time.Second, 5*time.Minute),
			SkipIf: !configExists,
		})

		_ = g.Add(flow.Task{
			Name:         "Destroying Shoot infrastructure",
			Fn:           tf.Destroy,
			Dependencies: flow.NewTaskIDs(destroyKubernetesRoutes, destroyKubernetesLoadbalancers),
		})

		f = g.Compile()
	)

	return f.Run(ctx, flow.Opts{Log: t.log})
}

func (t *TerraformReconciler) computeTerraformStatusState(
	ctx context.Context,
	tf terraformer.Terraformer,
	config *api.InfrastructureConfig,
) (*v1alpha1.InfrastructureStatus, *runtime.RawExtension, error) {
	status, err := infrastructure.ComputeStatus(ctx, tf, config)
	if err != nil {
		return nil, nil, err
	}

	state, err := tf.GetRawState(ctx)
	if err != nil {
		return nil, nil, err
	}

	stateByte, err := state.Marshal()
	if err != nil {
		return nil, nil, err
	}

	return status, &runtime.RawExtension{Raw: stateByte}, nil
}

func (t *TerraformReconciler) cleanupKubernetesRoutes(
	ctx context.Context,
	config *api.InfrastructureConfig,
	client openstackclient.Networking,
	routerID string,
) error {
	if routerID == "" {
		return nil
	}
	workesCIDR := infrastructure.WorkersCIDR(config)
	if workesCIDR == "" {
		return nil
	}
	return infrastructure.CleanupKubernetesRoutes(ctx, client, routerID, workesCIDR)
}

func (t *TerraformReconciler) cleanupKubernetesLoadbalancers(
	ctx context.Context,
	client openstackclient.Loadbalancing,
	subnetID string,
	clusterName string,
) error {
	return infrastructure.CleanupKubernetesLoadbalancers(ctx, t.log, client, subnetID, clusterName)
}
