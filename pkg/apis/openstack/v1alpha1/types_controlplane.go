// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ControlPlaneConfig contains configuration settings for the control plane.
type ControlPlaneConfig struct {
	metav1.TypeMeta `json:",inline"`

	// CloudControllerManager contains configuration settings for the cloud-controller-manager.
	// +optional
	CloudControllerManager *CloudControllerManagerConfig `json:"cloudControllerManager,omitempty"`
	// LoadBalancerClasses available for a dedicated Shoot.
	// +optional
	LoadBalancerClasses []LoadBalancerClass `json:"loadBalancerClasses,omitempty"`
	// LoadBalancerProvider is the name of the load balancer provider in the OpenStack environment.
	LoadBalancerProvider string `json:"loadBalancerProvider"`
	// Zone is the OpenStack zone.
	// +optional
	// Deprecated: Don't use anymore. Will be removed in a future version.
	Zone *string `json:"zone,omitempty"`
	// Storage contains configuration for storage in the cluster.
	// +optional
	Storage *Storage `json:"storage,omitempty"`
}

// CloudControllerManagerConfig contains configuration settings for the cloud-controller-manager.
type CloudControllerManagerConfig struct {
	// FeatureGates contains information about enabled feature gates.
	// +optional
	FeatureGates map[string]bool `json:"featureGates,omitempty"`
}

// Storage contains configuration for storage in the cluster.
type Storage struct {
	// CSIManila contains configuration for CSI Manila driver (support for NFS volumes)
	// +optional
	CSIManila *CSIManila `json:"csiManila,omitempty"`
}

// CSIManila contains configuration for CSI Manila driver (support for NFS volumes)
type CSIManila struct {
	// Enabled is the switch to enable the CSI Manila driver support
	Enabled bool `json:"enabled"`
}
