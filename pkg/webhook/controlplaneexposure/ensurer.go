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

package controlplaneexposure

import (
	"context"
	"fmt"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	"github.com/gardener/gardener/extensions/pkg/controller"
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gcontext "github.com/gardener/gardener/extensions/pkg/webhook/context"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/genericmutator"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/config"
)

// NewEnsurer creates a new controlplaneexposure ensurer.
func NewEnsurer(etcdStorage *config.ETCDStorage, logger logr.Logger) genericmutator.Ensurer {
	return &ensurer{
		etcdStorage: etcdStorage,
		logger:      logger.WithName("openstack-controlplaneexposure-ensurer"),
	}
}

type ensurer struct {
	genericmutator.NoopEnsurer
	etcdStorage *config.ETCDStorage
	client      client.Client
	logger      logr.Logger
}

// InjectClient injects the given client into the ensurer.
func (e *ensurer) InjectClient(client client.Client) error {
	e.client = client
	return nil
}

// EnsureKubeAPIServerDeployment ensures that the kube-apiserver deployment conforms to the provider requirements.
func (e *ensurer) EnsureKubeAPIServerDeployment(ctx context.Context, _ gcontext.GardenContext, newObj, _ *appsv1.Deployment) error {
	if v1beta1helper.IsAPIServerExposureManaged(newObj) {
		return nil
	}

	cluster, err := controller.GetCluster(ctx, e.client, newObj.Namespace)
	if err != nil {
		return err
	}

	if controller.IsHibernated(cluster) {
		return nil
	}

	// Get load balancer address of the kube-apiserver service
	address, err := kutil.GetLoadBalancerIngress(ctx, e.client, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: newObj.Namespace, Name: v1beta1constants.DeploymentNameKubeAPIServer}})
	if err != nil {
		return fmt.Errorf("could not get kube-apiserver service load balancer address: %w", err)
	}

	if c := extensionswebhook.ContainerWithName(newObj.Spec.Template.Spec.Containers, "kube-apiserver"); c != nil {
		c.Command = extensionswebhook.EnsureStringWithPrefix(c.Command, "--advertise-address=", address)
	}
	return nil
}

// EnsureETCD ensures that the etcd conform to the provider requirements.
func (e *ensurer) EnsureETCD(_ context.Context, _ gcontext.GardenContext, newObj, _ *druidv1alpha1.Etcd) error {
	capacity := resource.MustParse("10Gi")
	class := ""

	if newObj.Name == v1beta1constants.ETCDMain && e.etcdStorage != nil {
		if e.etcdStorage.Capacity != nil {
			capacity = *e.etcdStorage.Capacity
		}
		if e.etcdStorage.ClassName != nil {
			class = *e.etcdStorage.ClassName
		}
	}

	newObj.Spec.StorageClass = &class
	newObj.Spec.StorageCapacity = &capacity

	return nil
}
