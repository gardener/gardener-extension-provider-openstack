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
	"github.com/gophercloud/gophercloud/openstack/sharedfilesystems/v2/shares"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/internal"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/internal/infrastructure"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
)

func (a *actuator) Delete(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster) error {
	config, err := helper.InfrastructureConfigFromInfrastructure(infra)
	if err != nil {
		return err
	}
	// need to known if application credentials are used
	credentials, err := openstack.GetCredentials(ctx, a.Client(), infra.Spec.SecretRef, false)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	openstackClient, err := openstackclient.NewOpenstackClientFromCredentials(credentials)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	if config.Networks.ShareNetwork != nil && config.Networks.ShareNetwork.Enabled {
		// as the controller runs on the shoot cluster, shares may not have been cleaned up (e.g. on deletion of hibernated cluster)
		if err := a.cleanupOrphanedShares(ctx, log, infra, openstackClient); err != nil {
			return err
		}
	}

	tf, err := internal.NewTerraformer(log, a.RESTConfig(), infrastructure.TerraformerPurpose, infra, a.disableProjectedTokenMount)
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

	terraformFiles, err := infrastructure.RenderTerraformerTemplate(infra, config, cluster)
	if err != nil {
		return err
	}

	networkingClient, err := openstackClient.Networking()
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	stateInitializer := terraformer.StateConfigMapInitializerFunc(terraformer.CreateState)
	tf = tf.InitializeWith(ctx, terraformer.DefaultInitializer(a.Client(), terraformFiles.Main, terraformFiles.Variables, terraformFiles.TFVars, stateInitializer)).SetEnvVars(internal.TerraformerEnvVars(infra.Spec.SecretRef, credentials)...)

	configExists, err := tf.ConfigExists(ctx)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	vars, err := tf.GetStateOutputVariables(ctx, infrastructure.TerraformOutputKeyRouterID)
	if err != nil && !terraformer.IsVariablesNotFoundError(err) {
		return util.DetermineError(err, helper.KnownCodes)
	}

	var (
		g = flow.NewGraph("Openstack infrastructure destruction")

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
			Dependencies: flow.NewTaskIDs(destroyKubernetesRoutes),
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

func (a *actuator) cleanupOrphanedShares(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure, openstackClient openstackclient.Factory) error {
	status, err := helper.InfrastructureStatusFromRaw(infra.Status.ProviderStatus)
	if err != nil {
		return err
	}
	if status.Networks.ShareNetwork == nil {
		return nil
	}

	sharedFSClient, err := openstackClient.SharedFileSystem()
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	shares, err := sharedFSClient.ListShares(shares.ListOpts{
		ShareNetworkID: status.Networks.ShareNetwork.ID,
	})
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	for _, share := range shares {
		log.Info("deleting orphaned share", "shareID", share.ID)
		if err = sharedFSClient.DeleteShare(share.ID); err != nil {
			return util.DetermineError(err, helper.KnownCodes)
		}
	}
	return nil
}
