// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	healthcheckconfigv1alpha1 "github.com/gardener/gardener/extensions/pkg/apis/config/v1alpha1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ControllerConfiguration defines the configuration for the OpenStack provider.
type ControllerConfiguration struct {
	metav1.TypeMeta

	// ClientConnection specifies the kubeconfig file and client connection
	// settings for the proxy server to use when communicating with the apiserver.
	ClientConnection *componentbaseconfigv1alpha1.ClientConnectionConfiguration
	// ETCD is the etcd configuration.
	ETCD ETCD
	// HealthCheckConfig is the config for the health check controller
	HealthCheckConfig *healthcheckconfigv1alpha1.HealthCheckConfig
	// BastionConfig is the config for the Bastion
	// Deprecated: Configuring the bastion will be done via CloudProfile in future
	BastionConfig *BastionConfig
}

// ETCD is an etcd configuration.
type ETCD struct {
	// ETCDStorage is the etcd storage configuration.
	Main   ETCDStorage
	Events ETCDStorage
	// ETCDBackup is the etcd backup configuration.
	Backup ETCDBackup
}

// ETCDStorage is an etcd storage configuration.
type ETCDStorage struct {
	// ClassName is the name of the storage class used in etcd-main volume claims.
	ClassName *string
	// Capacity is the storage capacity used in etcd-main volume claims.
	Capacity *resource.Quantity
}

// ETCDBackup is an etcd backup configuration.
type ETCDBackup struct {
	// Schedule is the etcd backup schedule.
	Schedule *string
}

// BastionConfig is the config for the Bastion
// Deprecated: Configuring the bastion will be done via CloudProfile in future
type BastionConfig struct {
	// ImageRef is the openstack image reference (name or id)
	ImageRef string
	// FlavorRef is the openstack flavorRef reference
	FlavorRef string
}
