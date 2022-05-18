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

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/internal"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/internal/infrastructure"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/internal/managedappcredential"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionsinfracontroller "github.com/gardener/gardener/extensions/pkg/controller/infrastructure"
	"github.com/gardener/gardener/extensions/pkg/terraformer"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (a *actuator) Delete(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster) error {
	logger := a.logger.WithValues("infrastructure", client.ObjectKeyFromObject(infra), "operation", "delete")

	// need to known if application credentials are used
	credentials, err := openstack.GetCredentials(ctx, a.Client(), infra.Spec.SecretRef, false)
	if err != nil {
		return err
	}
	userCredentials := credentials

	appCredentialManager := managedappcredential.NewManager(
		openstackclient.FactoryFactoryFunc(openstackclient.NewOpenstackClientFromCredentials),
		a.appCredentialConfig,
		a.Client(),
		infra.Namespace,
		infra.Name,
		extensionsinfracontroller.FinalizerName,
		a.logger,
	)

	if err = appCredentialManager.Ensure(ctx, userCredentials); err != nil {
		return err
	}

	appCredentialCredentials, appCredentialSecretRef, err := managedappcredential.GetCredentials(ctx, a.Client(), infra.Namespace)
	if err != nil {
		return err
	}

	secretRef := &infra.Spec.SecretRef
	if appCredentialCredentials != nil && appCredentialSecretRef != nil {
		credentials = appCredentialCredentials
		secretRef = appCredentialSecretRef
	}

	tf, err := internal.NewTerraformer(logger, a.RESTConfig(), infrastructure.TerraformerPurpose, infra, a.disableProjectedTokenMount)
	if err != nil {
		return fmt.Errorf("could not create the Terraformer: %+v", err)
	}

	// terraform pod from previous reconciliation might still be running, ensure they are gone before doing any operations
	if err := tf.EnsureCleanedUp(ctx); err != nil {
		return err
	}

	// If the Terraform state is empty then we can exit early as we didn't create anything. Though, we clean up potentially
	// created configmaps/secrets related to the Terraformer.
	if tf.IsStateEmpty(ctx) {
		if err := appCredentialManager.Delete(ctx, userCredentials); err != nil {
			return err
		}

		a.logger.Info("exiting early as infrastructure state is empty - nothing to do")
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

	stateInitializer := terraformer.StateConfigMapInitializerFunc(terraformer.CreateState)
	if err := tf.
		InitializeWith(ctx, terraformer.DefaultInitializer(a.Client(), terraformFiles.Main, terraformFiles.Variables, terraformFiles.TFVars, stateInitializer)).
		SetEnvVars(internal.TerraformerEnvVars(*secretRef, credentials)...).
		Destroy(ctx); err != nil {
		return err
	}

	return appCredentialManager.Delete(ctx, userCredentials)
}
