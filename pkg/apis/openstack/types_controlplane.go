// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package openstack

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ControlPlaneConfig contains configuration settings for the control plane.
type ControlPlaneConfig struct {
	metav1.TypeMeta

	// CloudControllerManager contains configuration settings for the cloud-controller-manager.
	CloudControllerManager *CloudControllerManagerConfig
	// LoadBalancerClasses available for a dedicated Shoot.
	LoadBalancerClasses []LoadBalancerClass
	// LoadBalancerProvider is the name of the load balancer provider in the OpenStack environment.
	LoadBalancerProvider string
	// Zone is the OpenStack zone.
	// Deprecated: Don't use anymore. Will be removed in a future version.
	Zone *string
	// Storage contains configuration for storage in the cluster.
	Storage *Storage
}

const (
	// DefaultLoadBalancerClass defines the default load balancer class.
	DefaultLoadBalancerClass = "default"
	// PrivateLoadBalancerClass defines the load balancer class used to default the private load balancers.
	PrivateLoadBalancerClass = "private"
	// VPNLoadBalancerClass defines the floating pool class used by the VPN service.
	VPNLoadBalancerClass = "vpn"
)

// CloudControllerManagerConfig contains configuration settings for the cloud-controller-manager.
type CloudControllerManagerConfig struct {
	// FeatureGates contains information about enabled feature gates.
	FeatureGates map[string]bool
}

// Storage contains configuration for storage in the cluster.
type Storage struct {
	// CSIManila contains configuration for CSI Manila driver (support for NFS volumes)
	CSIManila *CSIManila
}

// CSIManila contains configuration for CSI Manila driver (support for NFS volumes)
type CSIManila struct {
	// Enabled is the switch to enable the CSI Manila driver support
	Enabled bool
}
