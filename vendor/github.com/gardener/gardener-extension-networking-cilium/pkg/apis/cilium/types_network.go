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

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IdentityAllocationMode selects how identities are shared between cilium
// nodes by setting how they are stored. The options are "crd" or "kvstore".
type IdentityAllocationMode string

const (
	// CRD defines the crd IdentityAllocationMode type.
	CRD IdentityAllocationMode = "crd"
	// KVStore defines the kvstore IdentityAllocationMode type.
	KVStore IdentityAllocationMode = "kvstore"
)

// TunnelMode defines what tunnel mode to use for Cilium.
type TunnelMode string

const (
	// VXLan defines the vxlan tunnel mode
	VXLan TunnelMode = "vxlan"
	// Geneve defines the geneve tunnel mode.
	Geneve TunnelMode = "geneve"
	// Disabled defines the disabled tunnel mode.
	Disabled TunnelMode = "disabled"
)

// LoadBalancingMode defines what load balancing mode to use for Cilium.
type LoadBalancingMode string

const (
	// SNAT defines the snat load balancing mode
	SNAT LoadBalancingMode = "snat"
	// DSR defines the dsr load balancing mode
	DSR LoadBalancingMode = "dsr"
	// Hybrid defines the hybrid load balancing mode
	Hybrid LoadBalancingMode = "hybrid"
)

// KubeProxyReplacementMode defines which mode should kube-proxy run in.
// More infromation here: https://docs.cilium.io/en/v1.7/gettingstarted/kubeproxy-free/
type KubeProxyReplacementMode string

const (
	// Strict defines the strict kube-proxy replacement mode
	Strict KubeProxyReplacementMode = "strict"
	// Probe defines the probe kube-proxy replacement mode
	Probe KubeProxyReplacementMode = "probe"
	// Partial defines the partial kube-proxy replacement mode
	Partial KubeProxyReplacementMode = "partial"
	// Disabled defines the disabled kube-proxy replacement mode
	KubeProxyReplacementDisabled KubeProxyReplacementMode = "disabled"
	// KubeProxyReplacementTrue defines the true kube-proxy replacement mode
	KubeProxyReplacementTrue KubeProxyReplacementMode = "true"
	// KubeProxyReplacementFalse defines the false kube-proxy replacement mode
	KubeProxyReplacementFalse KubeProxyReplacementMode = "false"
)

// NodePortMode defines how NodePort services are enabled.
type NodePortMode string

const (
	Hybird NodePortMode = "hybrid"
)

// Store defines the kubernetes storage backend
type Store string

const (
	// Kubernetes defines the kubernetes CRD store type
	Kubernetes Store = "kubernetes"
)

// InstallIPTableRules configuration for cilium
type InstallIPTableRules struct {
	Enabled bool
}

// ExternalIPs configuration for cilium
type ExternalIP struct {
	// ExternalIPenabled is used to define whether ExternalIP address is required or not.
	Enabled bool
}

// Hubble enablement for cilium
type Hubble struct {
	// Enabled defines whether hubble will be enabled for the cluster.
	Enabled bool
}

// IPv6 enablement for cilium
type IPv6 struct {
	// Enabled indicates whether IPv6 is enabled or not.
	Enabled bool
}

// BPFSocketLBHostnsOnly enablement for cilium
type BPFSocketLBHostnsOnly struct {
	Enabled bool
}

// CNI configuration for cilium
type CNI struct {
	Exclusive bool
}

// EgressGateway enablement for cilium
type EgressGateway struct {
	Enabled bool
}

// Nodeport enablement for cilium
type Nodeport struct {
	// Enabled is used to define whether Nodeport is required or not.
	Enabled bool
	// Mode is the mode of NodePort feature
	Mode NodePortMode
}

// KubeProxy configuration for cilium
type KubeProxy struct {
	// ServiceHost specify the controlplane node IP Address.
	ServiceHost *string
	// ServicePort specify the kube-apiserver port number.
	ServicePort *int32
}

// Overlay configuration for cilium
type Overlay struct {
	// Enabled enables the network overlay.
	Enabled bool
	// CreatePodRoutes installs routes to pods on all cluster nodes.
	// This will only work if the cluster nodes share a single L2 network.
	CreatePodRoutes *bool
}

// SnatToUpstreamDNS  enables the masquerading of packets to the upstream dns server
type SnatToUpstreamDNS struct {
	Enabled bool
}

// SnatOutOfCluster enables the masquerading of packets outside of the cluster
type SnatOutOfCluster struct {
	Enabled bool
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NetworkConfig is a struct representing the configmap for the cilium
// networking plugin
type NetworkConfig struct {
	metav1.TypeMeta
	// Debug configuration to be enabled or not
	Debug *bool
	// PSPEnabled configuration
	PSPEnabled *bool
	// KubeProxy configuration to be enabled or not
	KubeProxy *KubeProxy
	// Hubble configuration to be enabled or not
	Hubble *Hubble
	// TunnelMode configuration, it should be 'vxlan', 'geneve' or 'disabled'
	TunnelMode *TunnelMode
	// Store can be either Kubernetes or etcd.
	Store *Store
	// Enable IPv6
	IPv6 *IPv6
	// BPFSocketLBHostnsOnly flag to be enabled or not
	BPFSocketLBHostnsOnly *BPFSocketLBHostnsOnly
	// CNI configuration
	CNI *CNI
	// EgressGateway flag to be enabled or not
	EgressGateway *EgressGateway
	// MTU overwrites the auto-detected MTU of the underlying network
	MTU *int
	// Devices is the list of devices facing cluster/external network
	Devices []string
	// LoadBalancingMode configuration, it should be 'snat', 'dsr' or 'hybrid'
	LoadBalancingMode *LoadBalancingMode
	// IPv4NativeRoutingCIDRMode will set the ipv4 native routing cidr from the network configs node's cidr if enabled.
	IPv4NativeRoutingCIDREnabled *bool
	// Overlay enables the network overlay
	Overlay *Overlay
	// SnatToUpstreamDNS enables the masquerading of packets to the upstream dns server
	SnatToUpstreamDNS *SnatToUpstreamDNS
	// SnatOutOfCluster enables the masquerading of packets outside of the cluster
	SnatOutOfCluster *SnatOutOfCluster
}
