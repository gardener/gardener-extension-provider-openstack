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
	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	"github.com/gardener/gardener/extensions/pkg/controller/worker/genericactuator"
	"github.com/gardener/gardener/extensions/pkg/util"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	gardener "github.com/gardener/gardener/pkg/client/kubernetes"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
)

type delegateFactory struct {
	seedClient   client.Client
	restConfig   *rest.Config
	scheme       *runtime.Scheme
	gardenReader client.Reader
}

// NewActuator creates a new Actuator that updates the status of the handled WorkerPoolConfigs.
func NewActuator(mgr manager.Manager, gardenCluster cluster.Cluster) worker.Actuator {
	var (
		workerDelegate = &delegateFactory{
			seedClient: mgr.GetClient(),
			restConfig: mgr.GetConfig(),
			scheme:     mgr.GetScheme(),
		}
	)

	return genericactuator.NewActuator(
		mgr,
		gardenCluster,
		workerDelegate,
		func(err error) []gardencorev1beta1.ErrorCode {
			return util.DetermineErrorCodes(err, helper.KnownCodes)
		},
	)
}

func (d *delegateFactory) WorkerDelegate(ctx context.Context, worker *extensionsv1alpha1.Worker, cluster *extensionscontroller.Cluster) (genericactuator.WorkerDelegate, error) {
	clientset, err := kubernetes.NewForConfig(d.restConfig)
	if err != nil {
		return nil, err
	}

	serverVersion, err := clientset.Discovery().ServerVersion()
	if err != nil {
		return nil, err
	}

	seedChartApplier, err := gardener.NewChartApplierForConfig(d.restConfig)
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

	openstackClient, err := openstackclient.NewOpenStackClientFromSecretRef(ctx, d.seedClient, worker.Spec.SecretRef, &keyStoneURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create openstack seedClient: %w", err)
	}

	return NewWorkerDelegate(
		d.seedClient,
		d.scheme,

		seedChartApplier,
		serverVersion.GitVersion,

		worker,
		cluster,
		openstackClient,
	)
}

type workerDelegate struct {
	seedClient client.Client
	scheme     *runtime.Scheme
	decoder    runtime.Decoder

	seedChartApplier gardener.ChartApplier
	serverVersion    string

	cloudProfileConfig *api.CloudProfileConfig
	cluster            *extensionscontroller.Cluster
	worker             *extensionsv1alpha1.Worker

	machineClasses     []map[string]interface{}
	machineDeployments worker.MachineDeployments
	machineImages      []api.MachineImage

	openstackClient openstackclient.Factory
}

// NewWorkerDelegate creates a new context for a worker reconciliation.
func NewWorkerDelegate(
	seedClient client.Client,
	scheme *runtime.Scheme,

	seedChartApplier gardener.ChartApplier,
	serverVersion string,

	worker *extensionsv1alpha1.Worker,
	cluster *extensionscontroller.Cluster,
	openstackClient openstackclient.Factory,
) (genericactuator.WorkerDelegate, error) {
	config, err := helper.CloudProfileConfigFromCluster(cluster)
	if err != nil {
		return nil, err
	}

	return &workerDelegate{
		seedClient: seedClient,
		scheme:     scheme,
		decoder:    serializer.NewCodecFactory(scheme, serializer.EnableStrict).UniversalDecoder(),

		seedChartApplier: seedChartApplier,
		serverVersion:    serverVersion,

		cloudProfileConfig: config,
		cluster:            cluster,
		worker:             worker,
		openstackClient:    openstackClient,
	}, nil
}
