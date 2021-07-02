// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package config

import (
	healthcheckconfig "github.com/gardener/gardener/extensions/pkg/controller/healthcheck/config"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfig "k8s.io/component-base/config"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ControllerConfiguration defines the configuration for the OpenStack provider.
type ControllerConfiguration struct {
	metav1.TypeMeta

	// ClientConnection specifies the kubeconfig file and client connection
	// settings for the proxy server to use when communicating with the apiserver.
	ClientConnection *componentbaseconfig.ClientConnectionConfiguration
	// ETCD is the etcd configuration.
	ETCD ETCD
	// HealthCheckConfig is the config for the health check controller
	HealthCheckConfig *healthcheckconfig.HealthCheckConfig
	// BastionConfig is the config for the Bastion
	BastionConfig *BastionConfig
	// CSI is the config for the csi components
	CSI *CSI
}

// ETCD is an etcd configuration.
type ETCD struct {
	// ETCDStorage is the etcd storage configuration.
	Storage ETCDStorage
	// ETCDBackup is the etcd backup configuration.
	Backup ETCDBackup
}

type CSI struct {
	// CSIAttacher is the configuration for the csi-attacher
	CSIAttacher *CSIAttacher
	// CSIDriverCinder is the configuration for the csi-driver-cinder
	CSIDriverCinder *CSIDriverCinder
	// CSIProvisioner is the configuration for the csi-provisioner
	CSIProvisioner *CSIProvisioner
	// CSIResizer is the configuration for the csi-resizer
	CSIResizer *CSIResizer
	// CSISnapshotController is the configuration for the csi-shapshot-controller
	CSISnapshotController *CSISnapshotController
	// CSISnapshotter is the configuration for the csi-snapshotter
	CSISnapshotter *CSISnapshotter
	// CSILivenessProbe is the configuration for the csi-liveness-probe
	CSILivenessProbe *CSILivenessProbe
}

type CSIAttacher struct {
	// RetryIntervalStart The exponential backoff for failures.
	RetryIntervalStart *string
	// RetryIntervalMax The exponential backoff maximum value.
	RetryIntervalMax *string
	// ReconcileSync Resync frequency of the attached volumes with the driver.
	ReconcileSync *string
	// Timeout Timeout of all calls to the container storage interface driver.
	Timeout *string
	// Verbose The verbosity level.
	Verbose *string
}

type CSIDriverCinder struct {
	// Timeout Timeout of all calls to the container storage interface driver.
	Timeout *string
	// Verbose The verbosity level.
	Verbose *string
}

type CSILivenessProbe struct {
	// Timeout Timeout of all calls to the container storage interface driver.
	Timeout *string
	// Verbose The verbosity level.
	Verbose *string
}

type CSIProvisioner struct {
	// Timeout Timeout of all calls to the container storage interface driver.
	Timeout *string
	// Verbose The verbosity level.
	Verbose *string
}

type CSISnapshotter struct {
	// Timeout Timeout of all calls to the container storage interface driver.
	Timeout *string
	// Verbose The verbosity level.
	Verbose *string
}

type CSIResizer struct {
	// CSITimeout Timeout of all calls to the container storage interface driver.
	CSITimeout *string
	// Verbose The verbosity level.
	Verbose *string
}

type CSISnapshotController struct {
	// Timeout Timeout of all calls to the container storage interface driver.
	Timeout *string
	// Verbose The verbosity level.
	Verbose *string
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
type BastionConfig struct {
	// ImageRef is the openstack image reference
	ImageRef string
	// FlavorRef is the openstack flavorRef reference
	FlavorRef string
}
