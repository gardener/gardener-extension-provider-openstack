// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package components

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConfigurableKubeletCLIFlags is the set of configurable kubelet command line parameters.
type ConfigurableKubeletCLIFlags struct {
	ImagePullProgressDeadline *metav1.Duration
}

// KubeletCLIFlagsFromCoreV1beta1KubeletConfig computes the ConfigurableKubeletCLIFlags based on the provided
// gardencorev1beta1.KubeletConfig.
func KubeletCLIFlagsFromCoreV1beta1KubeletConfig(kubeletConfig *gardencorev1beta1.KubeletConfig) ConfigurableKubeletCLIFlags {
	var out ConfigurableKubeletCLIFlags

	if kubeletConfig != nil {
		out.ImagePullProgressDeadline = kubeletConfig.ImagePullProgressDeadline
	}

	return out
}

// ConfigurableKubeletConfigParameters is the set of configurable kubelet config parameters.
type ConfigurableKubeletConfigParameters struct {
	CpuCFSQuota                      *bool
	CpuManagerPolicy                 *string
	EvictionHard                     map[string]string
	EvictionMinimumReclaim           map[string]string
	EvictionSoft                     map[string]string
	EvictionSoftGracePeriod          map[string]string
	EvictionPressureTransitionPeriod *metav1.Duration
	EvictionMaxPodGracePeriod        *int32
	FailSwapOn                       *bool
	FeatureGates                     map[string]bool
	ImageGCHighThresholdPercent      *int32
	ImageGCLowThresholdPercent       *int32
	SerializeImagePulls              *bool
	KubeReserved                     map[string]string
	MaxPods                          *int32
	PodPidsLimit                     *int64
	SystemReserved                   map[string]string
}

const (
	// MemoryAvailable is a constant for the 'memory.available' eviction setting.
	MemoryAvailable = "memory.available"
	// ImageFSAvailable is a constant for the 'imagefs.available' eviction setting.
	ImageFSAvailable = "imagefs.available"
	// ImageFSInodesFree is a constant for the 'imagefs.inodesFree' eviction setting.
	ImageFSInodesFree = "imagefs.inodesFree"
	// NodeFSAvailable is a constant for the 'nodefs.available' eviction setting.
	NodeFSAvailable = "nodefs.available"
	// NodeFSInodesFree is a constant for the 'nodefs.inodesFree' eviction setting.
	NodeFSInodesFree = "nodefs.inodesFree"
)

// KubeletConfigParametersFromCoreV1beta1KubeletConfig computes the ConfigurableKubeletConfigParameters based on the provided
// gardencorev1beta1.KubeletConfig.
func KubeletConfigParametersFromCoreV1beta1KubeletConfig(kubeletConfig *gardencorev1beta1.KubeletConfig) ConfigurableKubeletConfigParameters {
	var out ConfigurableKubeletConfigParameters

	if kubeletConfig != nil {
		out.CpuCFSQuota = kubeletConfig.CPUCFSQuota
		out.CpuManagerPolicy = kubeletConfig.CPUManagerPolicy
		out.EvictionMaxPodGracePeriod = kubeletConfig.EvictionMaxPodGracePeriod
		out.EvictionPressureTransitionPeriod = kubeletConfig.EvictionPressureTransitionPeriod
		out.FailSwapOn = kubeletConfig.FailSwapOn
		out.ImageGCHighThresholdPercent = kubeletConfig.ImageGCHighThresholdPercent
		out.ImageGCLowThresholdPercent = kubeletConfig.ImageGCLowThresholdPercent
		out.SerializeImagePulls = kubeletConfig.SerializeImagePulls
		out.FeatureGates = kubeletConfig.FeatureGates
		out.KubeReserved = reservedFromKubeletConfig(kubeletConfig.KubeReserved)
		out.MaxPods = kubeletConfig.MaxPods
		out.PodPidsLimit = kubeletConfig.PodPIDsLimit
		out.SystemReserved = reservedFromKubeletConfig(kubeletConfig.SystemReserved)

		if eviction := kubeletConfig.EvictionHard; eviction != nil {
			if out.EvictionHard == nil {
				out.EvictionHard = make(map[string]string)
			}

			if val := eviction.MemoryAvailable; val != nil {
				out.EvictionHard[MemoryAvailable] = *val
			}
			if val := eviction.ImageFSAvailable; val != nil {
				out.EvictionHard[ImageFSAvailable] = *val
			}
			if val := eviction.ImageFSInodesFree; val != nil {
				out.EvictionHard[ImageFSInodesFree] = *val
			}
			if val := eviction.NodeFSAvailable; val != nil {
				out.EvictionHard[NodeFSAvailable] = *val
			}
			if val := eviction.NodeFSInodesFree; val != nil {
				out.EvictionHard[NodeFSInodesFree] = *val
			}
		}

		if eviction := kubeletConfig.EvictionSoft; eviction != nil {
			if out.EvictionSoft == nil {
				out.EvictionSoft = make(map[string]string)
			}

			if val := eviction.MemoryAvailable; val != nil {
				out.EvictionSoft[MemoryAvailable] = *val
			}
			if val := eviction.ImageFSAvailable; val != nil {
				out.EvictionSoft[ImageFSAvailable] = *val
			}
			if val := eviction.ImageFSInodesFree; val != nil {
				out.EvictionSoft[ImageFSInodesFree] = *val
			}
			if val := eviction.NodeFSAvailable; val != nil {
				out.EvictionSoft[NodeFSAvailable] = *val
			}
			if val := eviction.NodeFSInodesFree; val != nil {
				out.EvictionSoft[NodeFSInodesFree] = *val
			}
		}

		if eviction := kubeletConfig.EvictionMinimumReclaim; eviction != nil {
			if out.EvictionMinimumReclaim == nil {
				out.EvictionMinimumReclaim = make(map[string]string)
			}

			if val := eviction.MemoryAvailable; val != nil {
				out.EvictionMinimumReclaim[MemoryAvailable] = val.String()
			}
			if val := eviction.ImageFSAvailable; val != nil {
				out.EvictionMinimumReclaim[ImageFSAvailable] = val.String()
			}
			if val := eviction.ImageFSInodesFree; val != nil {
				out.EvictionMinimumReclaim[ImageFSInodesFree] = val.String()
			}
			if val := eviction.NodeFSAvailable; val != nil {
				out.EvictionMinimumReclaim[NodeFSAvailable] = val.String()
			}
			if val := eviction.NodeFSInodesFree; val != nil {
				out.EvictionMinimumReclaim[NodeFSInodesFree] = val.String()
			}
		}

		if eviction := kubeletConfig.EvictionSoftGracePeriod; eviction != nil {
			if out.EvictionSoftGracePeriod == nil {
				out.EvictionSoftGracePeriod = make(map[string]string)
			}

			if val := eviction.MemoryAvailable; val != nil {
				out.EvictionSoftGracePeriod[MemoryAvailable] = val.Duration.String()
			}
			if val := eviction.ImageFSAvailable; val != nil {
				out.EvictionSoftGracePeriod[ImageFSAvailable] = val.Duration.String()
			}
			if val := eviction.ImageFSInodesFree; val != nil {
				out.EvictionSoftGracePeriod[ImageFSInodesFree] = val.Duration.String()
			}
			if val := eviction.NodeFSAvailable; val != nil {
				out.EvictionSoftGracePeriod[NodeFSAvailable] = val.Duration.String()
			}
			if val := eviction.NodeFSInodesFree; val != nil {
				out.EvictionSoftGracePeriod[NodeFSInodesFree] = val.Duration.String()
			}
		}
	}

	return out
}

func reservedFromKubeletConfig(reserved *gardencorev1beta1.KubeletConfigReserved) map[string]string {
	if reserved == nil {
		return nil
	}

	out := make(map[string]string)

	if cpu := reserved.CPU; cpu != nil {
		out["cpu"] = cpu.String()
	}
	if memory := reserved.Memory; memory != nil {
		out["memory"] = memory.String()
	}
	if ephemeralStorage := reserved.EphemeralStorage; ephemeralStorage != nil {
		out["ephemeral-storage"] = ephemeralStorage.String()
	}
	if pid := reserved.PID; pid != nil {
		out["pid"] = pid.String()
	}

	return out
}
