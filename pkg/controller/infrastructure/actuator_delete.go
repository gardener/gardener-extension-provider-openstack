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
	"fmt"
	"time"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/terraformer"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/go-logr/logr"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/controller/infrastructure/infraflow"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/internal"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/internal/infrastructure"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
)

func (a *actuator) Delete(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster) error {
	state, err := a.getStateFromInfraStatus(ctx, infra)
	if err != nil {
		return err
	}
	if state != nil {
		err = a.deleteWithFlow(ctx, log, infra, cluster, state)
	} else {
		err = a.deleteWithTerraformer(ctx, log, infra, cluster)
	}
	return util.DetermineError(err, helper.KnownCodes)
}

func (a *actuator) deleteWithFlow(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure,
	cluster *extensionscontroller.Cluster, oldState *infraflow.PersistentState) error {
	log.Info("deleteWithFlow")

	flowContext, err := a.createFlowContext(ctx, log, infra, cluster, oldState)
	if err != nil {
		return err
	}
	if err = flowContext.Delete(ctx); err != nil {
		_ = flowContext.PersistState(ctx, true)
		return err
	}
	return flowContext.PersistState(ctx, true)
}

func (a *actuator) deleteWithTerraformer(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster) error {
	tf, err := internal.NewTerraformer(log, a.restConfig, infrastructure.TerraformerPurpose, infra, a.disableProjectedTokenMount)
	if err != nil {
		return util.DetermineError(fmt.Errorf("could not create the Terraformer: %+v", err), helper.KnownCodes)
	}

	// terraform pod from previous reconciliation might still be running, ensure they are gone before doing any operations
	if err := tf.EnsureCleanedUp(ctx); err != nil {
		return err
	}

	// If the Terraform state is empty then we can exit early as we didn't create anything. Though, we clean up potentially
	// created configmaps/secrets related to the Terraformer.
	stateIsEmpty := tf.IsStateEmpty(ctx)
	if stateIsEmpty {
		log.Info("exiting early as infrastructure state is empty - nothing to do")
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

	// need to known if application credentials are used
	credentials, err := openstack.GetCredentials(ctx, a.client, infra.Spec.SecretRef, false)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	openstackClient, err := openstackclient.NewOpenstackClientFromCredentials(credentials)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	networkingClient, err := openstackClient.Networking()
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}
	loadbalancerClient, err := openstackClient.Loadbalancing()
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	stateInitializer := terraformer.StateConfigMapInitializerFunc(terraformer.CreateState)
	tf = tf.InitializeWith(ctx, terraformer.DefaultInitializer(a.client, terraformFiles.Main, terraformFiles.Variables, terraformFiles.TFVars, stateInitializer)).SetEnvVars(internal.TerraformerEnvVars(infra.Spec.SecretRef, credentials)...)

	configExists, err := tf.ConfigExists(ctx)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	vars, err := tf.GetStateOutputVariables(ctx, infrastructure.TerraformOutputKeyRouterID)
	if err != nil && !terraformer.IsVariablesNotFoundError(err) {
		return util.DetermineError(err, helper.KnownCodes)
	}

	var (
		g                              = flow.NewGraph("Openstack infrastructure destruction")
		destroyKubernetesLoadbalancers = g.Add(flow.Task{
			Name: "Destroying Kubernetes loadbalancers entries",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return a.cleanupKubernetesLoadbalancers(ctx, log, loadbalancerClient, vars[infrastructure.TerraformOutputKeySubnetID], infra.Namespace)
			}).
				RetryUntilTimeout(10*time.Second, 5*time.Minute).
				DoIf(configExists),
		})
		destroyKubernetesRoutes = g.Add(flow.Task{
			Name: "Destroying Kubernetes route entries",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return a.cleanupKubernetesRoutes(ctx, config, networkingClient, vars[infrastructure.TerraformOutputKeyRouterID])
			}).
				RetryUntilTimeout(10*time.Second, 5*time.Minute).
				DoIf(configExists),
		})

		_ = g.Add(flow.Task{
			Name:         "Destroying Shoot infrastructure",
			Fn:           tf.Destroy,
			Dependencies: flow.NewTaskIDs(destroyKubernetesRoutes, destroyKubernetesLoadbalancers),
		})

		f = g.Compile()
	)

	if err := f.Run(ctx, flow.Opts{}); err != nil {
		return util.DetermineError(flow.Errors(err), helper.KnownCodes)
	}
	return nil
}

func (a *actuator) cleanupKubernetesRoutes(
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

func (a *actuator) cleanupKubernetesLoadbalancers(
	ctx context.Context,
	log logr.Logger,
	client openstackclient.Loadbalancing,
	subnetID string,
	clusterName string,
) error {
	return infrastructure.CleanupKubernetesLoadbalancers(ctx, log, client, subnetID, clusterName)
}
