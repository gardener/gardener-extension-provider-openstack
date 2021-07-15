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

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/internal"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/internal/infrastructure"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/terraformer"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/pkg/errors"
)

func (a *actuator) Reconcile(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster) error {
	logger := a.logger.WithValues("infrastructure", client.ObjectKeyFromObject(infra), "operation", "reconcile")
	return a.reconcile(ctx, logger, infra, cluster, terraformer.StateConfigMapInitializerFunc(terraformer.CreateState))
}

func (a *actuator) reconcile(ctx context.Context, logger logr.Logger, infra *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster, stateInitializer terraformer.StateConfigMapInitializer) error {
	config, err := helper.InfrastructureConfigFromInfrastructure(infra)
	if err != nil {
		return err
	}

	terraformFiles, err := infrastructure.RenderTerraformerTemplate(infra, config, cluster)
	if err != nil {
		return err
	}

	// need to known if application credentials are used
	credentials, err := openstack.GetCredentials(ctx, a.Client(), infra.Spec.SecretRef, false)
	if err != nil {
		return err
	}

	tf, err := internal.NewTerraformerWithAuth(logger, a.RESTConfig(), infrastructure.TerraformerPurpose, infra, credentials)
	if err != nil {
		return err
	}

	if err := tf.
		InitializeWith(ctx, terraformer.DefaultInitializer(a.Client(), terraformFiles.Main, terraformFiles.Variables, terraformFiles.TFVars, stateInitializer)).
		Apply(ctx); err != nil {

		return errors.Wrap(err, "failed to apply the terraform config")
	}

	return a.updateProviderStatus(ctx, tf, infra, config)
}
