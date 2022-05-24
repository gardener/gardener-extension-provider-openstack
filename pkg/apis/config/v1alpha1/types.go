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
	// CSI is the config for the csi components
	// +optional
	CSI *CSI `json:"csi,omitempty"`
}

// ETCD is an etcd configuration.
type ETCD struct {
	// ETCDStorage is the etcd storage configuration.
	Storage ETCDStorage `json:"storage"`
	// ETCDBackup is the etcd backup configuration.
	Backup ETCDBackup `json:"backup"`
}

type CSI struct {
	// CSIAttacher is the configuration for the csi-attacher
	// +optional
	CSIAttacher *CSIAttacher `json:"csiAttacher,omitempty"`
	// CSIDriverCinder is the configuration for the csi-driver-cinder
	// +optional
	CSIDriverCinder *CSIDriverCinder `json:"csiDriverCinder,omitempty"`
	// CSIProvisioner is the configuration for the csi-provisioner
	// +optional
	CSIProvisioner *CSIProvisioner `json:"csiProvisioner,omitempty"`
	// CSIResizer is the configuration for the csi-resizer
	// +optional
	CSIResizer *CSIResizer `json:"csiResizer,omitempty"`
	// CSISnapshotController is the configuration for the csi-shapshot-controller
	// +optional
	CSISnapshotController *CSISnapshotController `json:"csiSnapshotController,omitempty"`
	// CSISnapshotter is the configuration for the csi-snapshotter
	// +optional
	CSISnapshotter *CSISnapshotter `json:"csiSnapshotter,omitempty"`
	// CSILivenessProbe is the configuration for the csi-liveness-probe
	// +optional
	CSILivenessProbe *CSILivenessProbe `json:"csiLivenessProbe,omitempty"`
}

type CSIAttacher struct {
	// CSIBaseArgs Base arguments like verbose or timeout
	// +optional
	CSIBaseArgs
	// RetryIntervalStart The exponential backoff for failures.
	// +optional
	RetryIntervalStart *string `json:"retryIntervalStart,omitempty"`
	// RetryIntervalMax The exponential backoff maximum value.
	// +optional
	RetryIntervalMax *string `json:"retryIntervalMax,omitempty"`
	// ReconcileSync Resync frequency of the attached volumes with the driver.
	// +optional
	ReconcileSync *string `json:"reconcileSync,omitempty"`
}

type CSIDriverCinder struct {
	// CSIBaseArgs Base arguments like verbose or timeout
	// +optional
	CSIBaseArgs
}

type CSILivenessProbe struct {
	// CSIBaseArgs Base arguments like verbose or timeout
	// +optional
	CSIBaseArgs
}

type CSIProvisioner struct {
	// CSIBaseArgs Base arguments like verbose or timeout
	// +optional
	CSIBaseArgs
}

type CSISnapshotter struct {
	// CSIBaseArgs Base arguments like verbose or timeout
	// +optional
	CSIBaseArgs
}

type CSIResizer struct {
	// CSIBaseArgs Base arguments like verbose or timeout
	// +optional
	CSIBaseArgs
}

type CSISnapshotController struct {
	// CSIBaseArgs Base arguments like verbose or timeout
	// +optional
	CSIBaseArgs
}

type CSIBaseArgs struct {
	// Timeout Timeout of all calls to the container storage interface driver.
	// +optional
	Timeout *string `json:"timeout,omitempty"`
	// Verbose The verbosity level.
	// +optional
	Verbose *string `json:"verbose,omitempty"`
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
