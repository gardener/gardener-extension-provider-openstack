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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Backend string

const (
	Bird  Backend = "bird"
	None  Backend = "none"
	VXLan Backend = "vxlan"
)

type IPv4PoolMode string

const (
	Always      IPv4PoolMode = "Always"
	Never       IPv4PoolMode = "Never"
	CrossSubnet IPv4PoolMode = "CrossSubnet"
	Off         IPv4PoolMode = "Off"
)

type CIDR string

type IPv4Pool string

const (
	PoolIPIP  IPv4Pool = "ipip"
	PoolVXLan IPv4Pool = "vxlan"
)

// IPv4 contains configuration for calico ipv4 specific settings
type IPv4 struct {
	// Pool configures the type of ip pool for the tunnel interface
	// https://docs.projectcalico.org/v3.8/reference/node/configuration#environment-variables
	// +optional
	Pool *IPv4Pool `json:"pool,omitempty"`
	// Mode is the mode for the IPv4 Pool (e.g. Always, Never, CrossSubnet)
	// ipip pools accept all pool mode values values
	// vxlan pools accept only Always and Never (unchecked)
	// +optional
	Mode *IPv4PoolMode `json:"mode,omitempty"`
	// AutoDetectionMethod is the method to use to autodetect the IPv4 address for this host. This is only used when the IPv4 address is being autodetected.
	// https://docs.projectcalico.org/v3.8/reference/node/configuration#ip-autodetection-methods
	// +optional
	AutoDetectionMethod *string `json:"autoDetectionMethod,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NetworkConfig configuration for the calico networking plugin
type NetworkConfig struct {
	metav1.TypeMeta `json:",inline"`
	// Backend defines whether a backend should be used or not (e.g., bird or none)
	// +optional
	Backend *Backend `json:"backend"`
	// IPAM to use for the Calico Plugin (e.g., host-local or Calico)
	// +optional
	IPAM *IPAM `json:"ipam,omitempty"`
	// IPv4 contains configuration for calico ipv4 specific settings
	// +optional
	IPv4 *IPv4 `json:"ipv4,omitempty"`
	// Typha settings to use for calico-typha component
	// +optional
	Typha *Typha `json:"typha,omitempty"`
	// VethMTU settings used to configure calico port mtu
	// +optional
	VethMTU *string `json:"vethMTU,omitempty"`
	// EbpfDataplane enables the eBPF dataplane mode.
	// +optional
	EbpfDataplane *EbpfDataplane `json:"ebpfDataplane,omitempty"`
	// Overlay enables the network overlay
	// +optional
	Overlay *Overlay `json:"overlay,omitempty"`

	// DEPRECATED.
	// IPIP is the IPIP Mode for the IPv4 Pool (e.g. Always, Never, CrossSubnet)
	// It was moved into the IPv4 struct, kept for backwards compatibility.
	// Will be removed in a future Gardener release.
	// +optional
	IPIP *IPv4PoolMode `json:"ipip,omitempty"`
	// DEPRECATED.
	// IPAutoDetectionMethod is the method to use to autodetect the IPv4 address for this host. This is only used when the IPv4 address is being autodetected.
	// It was moved into the IPv4 struct, kept for backwards compatibility.
	// Will be removed in a future Gardener release.
	// +optional
	IPAutoDetectionMethod *string `json:"ipAutodetectionMethod,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NetworkStatus contains information about created Network resources.
type NetworkStatus struct {
	metav1.TypeMeta `json:",inline"`
}

// IPAM defines the block that configuration for the ip assignment plugin to be used
type IPAM struct {
	// Type defines the IPAM plugin type
	Type string `json:"type"`
	// CIDR defines the CIDR block to be used
	// +optional
	CIDR *CIDR `json:"cidr,omitempty"`
}

// Typha defines the block with configurations for calico typha
type Typha struct {
	// Enabled is used to define whether calico-typha is required or not.
	// Note, typha is used to offload kubernetes API server,
	// thus consider not to disable it for large clusters in terms of node count.
	// More info can be found here https://docs.projectcalico.org/v3.9/reference/typha/
	Enabled bool `json:"enabled"`
}

type EbpfDataplane struct {
	// Enabled enables the eBPF dataplane mode.
	Enabled bool `json:"enabled"`
}

type Overlay struct {
	// Enabled enables the network overlay.
	Enabled bool `json:"enabled"`
}
