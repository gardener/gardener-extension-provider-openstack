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

package controlplane

import (
	"fmt"
	"hash/crc32"
	"path"
	"strings"

	extensionscontroller "github.com/gardener/gardener-extensions/pkg/controller"
	extensionswebhook "github.com/gardener/gardener-extensions/pkg/webhook"

	"github.com/gardener/gardener/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	// EtcdMainVolumeClaimTemplateName is the name of the volume claim template in the etcd-main StatefulSet. It uses a
	// different naming scheme because Gardener was using HDD-based volumes for etcd in the past and did migrate to fast
	// SSD volumes recently. Due to the migration of the data of the old volume to the new one the PVC name is now different.
	EtcdMainVolumeClaimTemplateName = "main-etcd"
	// BackupRestoreContainerName is the name of the backup-restore sidecar injected in etcd StatefulSet.
	BackupRestoreContainerName = "backup-restore"
)

// GetBackupRestoreContainer returns an etcd backup-restore container with the given name, schedule, provider, image,
// and additional provider-specific command line args and env variables.
func GetBackupRestoreContainer(
	name, volumeClaimTemplateName, schedule, provider, prefix, image string,
	args map[string]string,
	env []corev1.EnvVar,
	volumeMounts []corev1.VolumeMount,
) *corev1.Container {
	c := &corev1.Container{
		Name: BackupRestoreContainerName,
		Command: []string{
			"etcdbrctl",
			"server",
			fmt.Sprintf("--schedule=%s", schedule),
			"--data-dir=/var/etcd/data/new.etcd",
			fmt.Sprintf("--storage-provider=%s", provider),
			fmt.Sprintf("--store-prefix=%s", path.Join(prefix, name)),
			"--cert=/var/etcd/ssl/client/tls.crt",
			"--key=/var/etcd/ssl/client/tls.key",
			"--cacert=/var/etcd/ssl/ca/ca.crt",
			"--insecure-transport=false",
			"--insecure-skip-tls-verify=false",
			fmt.Sprintf("--endpoints=https://%s-0:2379", name),
			"--etcd-connection-timeout=300",
			"--delta-snapshot-period-seconds=300",
			"--delta-snapshot-memory-limit=104857600", // 100MB
			"--garbage-collection-period-seconds=43200",
			"--snapstore-temp-directory=/var/etcd/data/temp",
		},
		Env:             []corev1.EnvVar{},
		Image:           image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Ports: []corev1.ContainerPort{
			{
				Name:          "server",
				ContainerPort: 8080,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("23m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("10G"),
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      volumeClaimTemplateName,
				MountPath: "/var/etcd/data",
			},
			{
				Name:      "ca-etcd",
				MountPath: "/var/etcd/ssl/ca",
			},
			{
				Name:      "etcd-client-tls",
				MountPath: "/var/etcd/ssl/client",
			},
		},
	}

	// Ensure additional command line args
	for k, v := range args {
		c.Command = extensionswebhook.EnsureStringWithPrefix(c.Command, fmt.Sprintf("--%s=", k), v)
	}

	// Ensure additional env variables
	for _, envVar := range env {
		c.Env = extensionswebhook.EnsureEnvVarWithName(c.Env, envVar)
	}

	// Ensure additional volume mounts
	for _, volumeMount := range volumeMounts {
		c.VolumeMounts = extensionswebhook.EnsureVolumeMountWithName(c.VolumeMounts, volumeMount)
	}

	return c
}

// GetETCDVolumeClaimTemplate returns an etcd backup-restore container with the given name, storageClass and storageCapacity.
func GetETCDVolumeClaimTemplate(name string, storageClassName *string, storageCapacity *resource.Quantity) *corev1.PersistentVolumeClaim {
	// Determine the storage capacity
	// A non-default storage capacity is used only if it's configured
	capacity := resource.MustParse("10Gi")
	if storageCapacity != nil {
		capacity = *storageCapacity
	}

	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: storageClassName,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: capacity,
				},
			},
		},
	}
}

// DetermineBackupSchedule determines the backup schedule based on the shoot creation and maintenance time window.
func DetermineBackupSchedule(c *corev1.Container, cluster *extensionscontroller.Cluster) (string, error) {
	schedule := ParseExistingBackupSchedule(c)
	if len(schedule) != 0 {
		return schedule, nil
	}

	var (
		begin, end string
		shootUID   types.UID
	)

	if cluster.Shoot.Spec.Maintenance != nil && cluster.Shoot.Spec.Maintenance.TimeWindow != nil {
		begin = cluster.Shoot.Spec.Maintenance.TimeWindow.Begin
		end = cluster.Shoot.Spec.Maintenance.TimeWindow.End
		shootUID = cluster.Shoot.Status.UID
	}

	if len(begin) != 0 && len(end) != 0 {
		maintenanceTimeWindow, err := utils.ParseMaintenanceTimeWindow(begin, end)
		if err != nil {
			return "", err
		}

		if maintenanceTimeWindow != utils.AlwaysTimeWindow {
			// Randomize the snapshot timing daily but within last hour.
			// The 15 minutes buffer is set to snapshot upload time before actual maintenance window start.
			snapshotWindowBegin := maintenanceTimeWindow.Begin().Add(-1, -15, 0)
			randomMinutes := int(crc32.ChecksumIEEE([]byte(shootUID)) % 60)
			snapshotTime := snapshotWindowBegin.Add(0, randomMinutes, 0)
			return fmt.Sprintf("%d %d * * *", snapshotTime.Minute(), snapshotTime.Hour()), nil
		}
	}

	creationMinute := cluster.Shoot.CreationTimestamp.Minute()
	creationHour := cluster.Shoot.CreationTimestamp.Hour()
	return fmt.Sprintf("%d %d * * *", creationMinute, creationHour), nil
}

// ParseExistingBackupSchedule parse the backup container to get already configured schedule.
func ParseExistingBackupSchedule(c *corev1.Container) string {
	if c != nil {
		for _, c := range c.Command {
			if strings.HasPrefix(c, "--schedule=") {
				return strings.TrimPrefix(c, "--schedule=")
			}
		}
	}
	return ""
}
