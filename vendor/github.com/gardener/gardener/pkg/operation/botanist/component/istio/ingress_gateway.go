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

package istio

import (
	"context"
	"path/filepath"

	v1alpha1constants "github.com/gardener/gardener/pkg/apis/core/v1alpha1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	networkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type ingress struct {
	values    *IngressValues
	namespace string
	kubernetes.ChartApplier
	chartPath string
	client    crclient.Client
}

// IngressValues holds values for the istio-ingress chart.
// The only opened port is 15021.
type IngressValues struct {
	TrustDomain     string            `json:"trustDomain,omitempty"`
	Image           string            `json:"image,omitempty"`
	Annotations     map[string]string `json:"annotations,omitempty"`
	IstiodNamespace string            `json:"istiodNamespace,omitempty"`
	LoadBalancerIP  *string           `json:"loadBalancerIP,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	// Ports is a list of all Ports the istio-ingress gateways is listening on.
	// Port 15021 and 15000 cannot be used.
	Ports []corev1.ServicePort `json:"ports,omitempty"`
}

// NewIngressGateway creates a new DeployWaiter for istio ingress gateway in
// "istio-ingress" namespace.
// It only supports Deploy. Destroy does nothing.
func NewIngressGateway(
	values *IngressValues,
	namespace string,
	applier kubernetes.ChartApplier,
	chartsRootPath string,
	client crclient.Client,
) component.DeployWaiter {
	return &ingress{
		values:       values,
		namespace:    namespace,
		ChartApplier: applier,
		chartPath:    filepath.Join(chartsRootPath, istioReleaseName, "istio-ingress"),
		client:       client,
	}
}

func (i *ingress) Deploy(ctx context.Context) error {
	// TODO(mvladev): Rotate this on on every istio version upgrade.
	for _, filterName := range []string{"tcp-metadata-exchange-1.8", "tcp-stats-filter-1.8"} {
		if err := crclient.IgnoreNotFound(i.client.Delete(ctx, &networkingv1alpha3.EnvoyFilter{
			ObjectMeta: metav1.ObjectMeta{Name: filterName, Namespace: i.namespace},
		})); err != nil {
			return err
		}
	}

	if err := i.client.Create(
		ctx,
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:   i.namespace,
				Labels: getIngressGatewayNamespaceLabels(i.values.Labels),
			},
		},
	); kutil.IgnoreAlreadyExists(err) != nil {
		return err
	}

	applierOptions := kubernetes.CopyApplierOptions(kubernetes.DefaultMergeFuncs)
	applierOptions[appsv1.SchemeGroupVersion.WithKind("Deployment").GroupKind()] = kubernetes.DeploymentKeepReplicasMergeFunc

	return i.Apply(ctx, i.chartPath, i.namespace, istioReleaseName, kubernetes.Values(i.values), applierOptions)
}

func (i *ingress) Destroy(ctx context.Context) error {
	// istio cannot be safely removed
	return nil
}

func (i *ingress) Wait(ctx context.Context) error {
	return nil
}

func (i *ingress) WaitCleanup(ctx context.Context) error {
	return nil
}

func getIngressGatewayNamespaceLabels(labels map[string]string) map[string]string {
	var namespaceLabels = map[string]string{
		"istio-operator-managed": "Reconcile",
		"istio-injection":        "disabled",
	}

	if value, ok := labels[v1alpha1constants.GardenRole]; ok && value == v1alpha1constants.GardenRoleExposureClassHandler {
		namespaceLabels[v1alpha1constants.GardenRole] = v1alpha1constants.GardenRoleExposureClassHandler
	}
	if value, ok := labels[v1alpha1constants.LabelExposureClassHandlerName]; ok {
		namespaceLabels[v1alpha1constants.LabelExposureClassHandlerName] = value
	}

	return namespaceLabels
}
