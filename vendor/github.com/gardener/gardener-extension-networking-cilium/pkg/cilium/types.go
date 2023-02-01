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

package cilium

import "path/filepath"

const (
	// Name defines the extension name.
	Name = "networking-cilium"

	// CiliumAgentImageName defines the agent name.
	CiliumAgentImageName = "cilium-agent"
	// CiliumOperatorImageName defines cilium operator's image name.
	CiliumOperatorImageName = "cilium-operator"
	// CiliumETCDOperatorImageName defines cilium etcd operator image name.
	CiliumETCDOperatorImageName = "cilium-etcd-operator"
	// CiliumNodeInitImageName defines cilium's node init image name.
	CiliumNodeInitImageName = "cilium-nodeinit"
	// CiliumPreflightImageName defines the cilium preflight image name.
	CiliumPreflightImageName = "cilium-preflight"

	// HubbleRelayImageName defines the Hubble image name.
	HubbleRelayImageName = "hubble-relay"
	// HubbleUIImageName defines the UI image name.
	HubbleUIImageName = "hubble-ui"
	// HubbleUIBackendImageName defines the UI Backend image name.
	HubbleUIBackendImageName = "hubble-ui-backend"

	// CertGenImageName defines certificate generation image name.
	CertGenImageName = "certgen"

	// KubeProxyImageName defines the kube-proxy image name.
	KubeProxyImageName = "kube-proxy"

	// PortmapCopierImageName defines the portmap-copier image name.
	PortmapCopierImageName = "portmap-copier"

	// MonitoringChartName
	MonitoringName = "cilium-monitoring-config"

	// ReleaseName is the name of the Cilium Release.
	ReleaseName = "cilium"
)

var (
	// ChartsPath is the path to the charts
	ChartsPath = filepath.Join("charts")
	// InternalChartsPath is the path to the internal charts
	InternalChartsPath = filepath.Join(ChartsPath, "internal")

	// ChartPath path for internal Cilium Chart
	ChartPath = filepath.Join(InternalChartsPath, "cilium")

	// CiliumMonitoringChartPath  path for internal Cilium monitoring chart
	CiliumMonitoringChartPath = filepath.Join(InternalChartsPath, "cilium-monitoring")
)
