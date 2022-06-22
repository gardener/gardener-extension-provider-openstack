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
	healthcheckconfigv1alpha1 "github.com/gardener/gardener/extensions/pkg/controller/healthcheck/config/v1alpha1"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ControllerConfiguration defines the configuration for the OpenStack provider.
type ControllerConfiguration struct {
	metav1.TypeMeta `json:",inline"`

	// ClientConnection specifies the kubeconfig file and client connection
	// settings for the proxy server to use when communicating with the apiserver.
	// +optional
	ClientConnection *componentbaseconfigv1alpha1.ClientConnectionConfiguration `json:"clientConnection,omitempty"`
	// ETCD is the etcd configuration.
	ETCD ETCD `json:"etcd"`
	// HealthCheckConfig is the config for the health check controller
	// +optional
	HealthCheckConfig *healthcheckconfigv1alpha1.HealthCheckConfig `json:"healthCheckConfig,omitempty"`
	// BastionConfig the config for the Bastion
	// +optional
	BastionConfig *BastionConfig `json:"bastionConfig,omitempty"`
	// ApplicationCrednentialConfig defines the configuration for managed application credentials.
	// +optional
	ApplicationCredentialConfig *ApplicationCredentialConfig `json:"managedApplicationCredential,omitempty"`
	// FeatureGates is a map of feature names to bools that enable
	// or disable alpha/experimental features.
	// Default: nil
	// +optional
	FeatureGates map[string]bool `json:"featureGates,omitempty"`
}

// ETCD is an etcd configuration.
type ETCD struct {
	// ETCDStorage is the etcd storage configuration.
	Storage ETCDStorage `json:"storage"`
	// ETCDBackup is the etcd backup configuration.
	Backup ETCDBackup `json:"backup"`
}

// ETCDStorage is an etcd storage configuration.
type ETCDStorage struct {
	// ClassName is the name of the storage class used in etcd-main volume claims.
	// +optional
	ClassName *string `json:"className,omitempty"`
	// Capacity is the storage capacity used in etcd-main volume claims.
	// +optional
	Capacity *resource.Quantity `json:"capacity,omitempty"`
}

// ETCDBackup is an etcd backup configuration.
type ETCDBackup struct {
	// Schedule is the etcd backup schedule.
	// +optional
	Schedule *string `json:"schedule,omitempty"`
}

// BastionConfig is the config for the Bastion
type BastionConfig struct {
	// ImageRef is the openstack image reference
	ImageRef string `json:"imageRef,omitempty"`
	// FlavorRef is the openstack flavorRef reference
	FlavorRef string `json:"flavorRef,omitempty"`
}

// ApplicationCredentialConfig defines the configuration for managed application credentials.
type ApplicationCredentialConfig struct {
	// Lifetime define how long a managed application credentials are valid.
	// Once the creation time + lifetime of an application credential is expired
	// it will be renewed once it is next reconciled.
	// Defaults to 48h.
	// +optional
	Lifetime *metav1.Duration `json:"lifetime,omitempty"`
	// OpenstackExpirationPeriod is a duration to calculate the expiration time
	// of a managed application credential on the Openstack layer.
	// The expiration time will be calculated in the following way:
	//
	// expiration time = creation time + expiration period
	//
	// This is a security measure to ensure that managed appplication credentials
	// get deactivated even if the owning user of the application credential
	// is not available to the openstack-extension anymore and therefore
	// cannot be removed by the openstack-extension on its own.
	// Defaults to 720h = 30d.
	// +optional
	OpenstackExpirationPeriod *metav1.Duration `json:"openstackExpirationPeriod,omitempty"`
	// RenewThreshold defines a threshold before the openstack expiration time.
	// Once the threshold is reached the managed application credential need to be renewed.
	// Defaults to 72h.
	// +optional
	RenewThreshold *metav1.Duration `json:"renewThreshold,omitempty"`
}
