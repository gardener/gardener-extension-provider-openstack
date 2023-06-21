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

package worker

import (
	"context"
	"fmt"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/common"
	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	"github.com/gardener/gardener/extensions/pkg/controller/worker/genericactuator"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	gardener "github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/chart"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
	"k8s.io/client-go/kubernetes"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/imagevector"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
)

type delegateFactory struct {
	common.RESTConfigContext
}

// NewActuator creates a new Actuator that updates the status of the handled WorkerPoolConfigs.
func NewActuator(gardenletManagesMCM bool) worker.Actuator {
	var (
		mcmName              string
		mcmChartSeed         *chart.Chart
		mcmChartShoot        *chart.Chart
		imageVector          imagevectorutils.ImageVector
		chartRendererFactory extensionscontroller.ChartRendererFactory
		workerDelegate       = &delegateFactory{}
	)

	if !gardenletManagesMCM {
		mcmName = openstack.MachineControllerManagerName
		mcmChartSeed = mcmChart
		mcmChartShoot = mcmShootChart
		imageVector = imagevector.ImageVector()
		chartRendererFactory = extensionscontroller.ChartRendererFactoryFunc(util.NewChartRendererForShoot)
	}

	return genericactuator.NewActuator(
		workerDelegate,
		mcmName,
		mcmChartSeed,
		mcmChartShoot,
		imageVector,
		chartRendererFactory,
	)
}

func (d *delegateFactory) WorkerDelegate(ctx context.Context, worker *extensionsv1alpha1.Worker, cluster *extensionscontroller.Cluster) (genericactuator.WorkerDelegate, error) {
	clientset, err := kubernetes.NewForConfig(d.RESTConfig())
	if err != nil {
		return nil, err
	}

	serverVersion, err := clientset.Discovery().ServerVersion()
	if err != nil {
		return nil, err
	}

	seedChartApplier, err := gardener.NewChartApplierForConfig(d.RESTConfig())
	if err != nil {
		return nil, err
	}

	cloudProfileConfig, err := helper.CloudProfileConfigFromCluster(cluster)
	if err != nil {
		return nil, err
	}

	keyStoneURL, err := helper.FindKeyStoneURL(cloudProfileConfig.KeyStoneURLs, cloudProfileConfig.KeyStoneURL, worker.Spec.Region)
	if err != nil {
		return nil, err
	}

	openstackClient, err := client.NewOpenStackClientFromSecretRef(ctx, d.Client(), worker.Spec.SecretRef, &keyStoneURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create openstack client: %w", err)
	}

	return NewWorkerDelegate(
		d.ClientContext,

		seedChartApplier,
		serverVersion.GitVersion,

		worker,
		cluster,
		openstackClient,
	)
}

type workerDelegate struct {
	common.ClientContext

	seedChartApplier gardener.ChartApplier
	serverVersion    string

	cloudProfileConfig *api.CloudProfileConfig
	cluster            *extensionscontroller.Cluster
	worker             *extensionsv1alpha1.Worker

	machineClasses     []map[string]interface{}
	machineDeployments worker.MachineDeployments
	machineImages      []api.MachineImage

	openstackClient client.Factory
}

// NewWorkerDelegate creates a new context for a worker reconciliation.
func NewWorkerDelegate(
	clientContext common.ClientContext,

	seedChartApplier gardener.ChartApplier,
	serverVersion string,

	worker *extensionsv1alpha1.Worker,
	cluster *extensionscontroller.Cluster,
	openstackClient client.Factory,
) (genericactuator.WorkerDelegate, error) {
	config, err := helper.CloudProfileConfigFromCluster(cluster)
	if err != nil {
		return nil, err
	}

	return &workerDelegate{
		ClientContext: clientContext,

		seedChartApplier: seedChartApplier,
		serverVersion:    serverVersion,

		cloudProfileConfig: config,
		cluster:            cluster,
		worker:             worker,
		openstackClient:    openstackClient,
	}, nil
}
