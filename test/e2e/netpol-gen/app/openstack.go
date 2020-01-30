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

package app

import (
	np "github.com/gardener/gardener-extensions/test/e2e/framework/networkpolicies"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
)

// openStackNetworkPolicy holds OpenStack-specific network policy settings.
type openStackNetworkPolicy struct {
	np.Agnostic
	// metadata points to AWS metadata service.
	metadata *np.Host
	// cloudControllerManagerNotSecured points to OpenStack specific cloud-controller-manager running on HTTP port.
	cloudControllerManagerNotSecured *np.SourcePod
	// cloudControllerManagerSecured points to OpenStack specific cloud-controller-manager running on HTTPS port.
	cloudControllerManagerSecured *np.SourcePod
}

// NewCloudAware returns OpenStack-specific.
func NewCloudAware() np.CloudAware {
	return &openStackNetworkPolicy{

		metadata: &np.Host{
			Description: "Metadata service",
			HostName:    "169.254.169.254",
			Port:        80,
		},
		cloudControllerManagerNotSecured: &np.SourcePod{
			Ports: np.NewSinglePort(10253),
			Pod: np.NewPod("cloud-controller-manager-http", labels.Set{
				"app":                     "kubernetes",
				"garden.sapcloud.io/role": "controlplane",
				"role":                    "cloud-controller-manager",
			}, "< 1.13"),
			ExpectedPolicies: sets.NewString(
				"allow-from-prometheus",
				"allow-to-dns",
				"allow-to-private-networks",
				"allow-to-public-networks",
				"allow-to-shoot-apiserver",
				"deny-all",
			),
		},
		cloudControllerManagerSecured: &np.SourcePod{
			Ports: np.NewSinglePort(10258),
			Pod: np.NewPod("cloud-controller-manager-https", labels.Set{
				"app":                     "kubernetes",
				"garden.sapcloud.io/role": "controlplane",
				"role":                    "cloud-controller-manager",
			}, ">= 1.13"),
			ExpectedPolicies: sets.NewString(
				"allow-from-prometheus",
				"allow-to-dns",
				"allow-to-private-networks",
				"allow-to-public-networks",
				"allow-to-shoot-apiserver",
				"deny-all",
			),
		},
	}
}

// Sources returns list of all OpenStack-specific sources and targets.
func (a *openStackNetworkPolicy) Rules() []np.Rule {
	ag := a.Agnostic
	return []np.Rule{
		a.newSource(ag.KubeAPIServer()).AllowPod(ag.EtcdMain(), ag.EtcdEvents()).AllowHost(ag.SeedKubeAPIServer(), ag.External()).Build(),
		a.newSource(ag.EtcdMain()).AllowHost(ag.External()).Build(),
		a.newSource(ag.EtcdEvents()).AllowHost(ag.External()).Build(),
		a.newSource(a.cloudControllerManagerNotSecured).AllowPod(ag.KubeAPIServer()).AllowHost(ag.External()).Build(),
		a.newSource(a.cloudControllerManagerSecured).AllowPod(ag.KubeAPIServer()).AllowHost(ag.External()).Build(),
		a.newSource(ag.DependencyWatchdog()).AllowHost(ag.SeedKubeAPIServer(), ag.External()).Build(),
		a.newSource(ag.ElasticSearch()).Build(),
		a.newSource(ag.Grafana()).AllowPod(ag.Prometheus()).Build(),
		a.newSource(ag.Kibana()).AllowTargetPod(ag.ElasticSearch().FromPort("http")).Build(),
		a.newSource(ag.AddonManager()).AllowPod(ag.KubeAPIServer()).AllowHost(ag.SeedKubeAPIServer(), ag.External()).Build(),
		a.newSource(ag.KubeControllerManagerNotSecured()).AllowPod(ag.KubeAPIServer()).AllowHost(a.metadata, ag.External()).Build(),
		a.newSource(ag.KubeControllerManagerSecured()).AllowPod(ag.KubeAPIServer()).AllowHost(a.metadata, ag.External()).Build(),
		a.newSource(ag.KubeSchedulerNotSecured()).AllowPod(ag.KubeAPIServer()).Build(),
		a.newSource(ag.KubeSchedulerSecured()).AllowPod(ag.KubeAPIServer()).Build(),
		a.newSource(ag.KubeStateMetricsShoot()).AllowPod(ag.KubeAPIServer()).Build(),
		a.newSource(ag.KubeStateMetricsSeed()).AllowHost(ag.SeedKubeAPIServer(), ag.External()).Build(),
		a.newSource(ag.MachineControllerManager()).AllowPod(ag.KubeAPIServer()).AllowHost(ag.SeedKubeAPIServer(), ag.External()).Build(),
		a.newSource(ag.Prometheus()).AllowPod(
			a.cloudControllerManagerNotSecured,
			a.cloudControllerManagerSecured,
			ag.EtcdEvents(),
			ag.EtcdMain(),
			ag.KubeAPIServer(),
			ag.KubeControllerManagerNotSecured(),
			ag.KubeControllerManagerSecured(),
			ag.KubeSchedulerNotSecured(),
			ag.KubeSchedulerSecured(),
			ag.KubeStateMetricsSeed(),
			ag.KubeStateMetricsShoot(),
			ag.MachineControllerManager(),
		).AllowTargetPod(ag.ElasticSearch().FromPort("metrics")).AllowHost(ag.SeedKubeAPIServer(), ag.External(), ag.GardenPrometheus()).Build(),
	}
}

// EgressFromOtherNamespaces returns list of all OpenStack-specific sources and targets.
func (a *openStackNetworkPolicy) EgressFromOtherNamespaces(sourcePod *np.SourcePod) np.Rule {
	return np.NewSource(sourcePod).DenyPod(a.Sources()...).AllowPod(a.Agnostic.KubeAPIServer()).Build()
}

func (a *openStackNetworkPolicy) newSource(sourcePod *np.SourcePod) *np.RuleBuilder {
	return np.NewSource(sourcePod).DenyPod(a.Sources()...).DenyHost(a.metadata, a.Agnostic.External(), a.Agnostic.GardenPrometheus())
}

// Sources returns a list of SourcePods of OpenStack.
func (a *openStackNetworkPolicy) Sources() []*np.SourcePod {
	ag := a.Agnostic
	return []*np.SourcePod{
		ag.AddonManager(),
		ag.DependencyWatchdog(),
		a.cloudControllerManagerNotSecured,
		a.cloudControllerManagerSecured,
		ag.ElasticSearch(),
		ag.EtcdEvents(),
		ag.EtcdMain(),
		ag.Grafana(),
		ag.Kibana(),
		ag.KubeAPIServer(),
		ag.KubeControllerManagerNotSecured(),
		ag.KubeControllerManagerSecured(),
		ag.KubeSchedulerNotSecured(),
		ag.KubeSchedulerSecured(),
		ag.KubeStateMetricsSeed(),
		ag.KubeStateMetricsShoot(),
		ag.MachineControllerManager(),
		ag.Prometheus(),
	}
}

// Provider returns OpenStack cloud provider.
func (a *openStackNetworkPolicy) Provider() string {
	return "openstack"
}
