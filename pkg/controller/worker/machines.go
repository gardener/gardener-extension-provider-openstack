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

package worker

import (
	"context"
	"fmt"
	"path/filepath"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	genericworkeractuator "github.com/gardener/gardener/extensions/pkg/controller/worker/genericactuator"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MachineClassKind yields the name of the OpenStack machine class.
func (w *workerDelegate) MachineClassKind() string {
	return "OpenStackMachineClass"
}

// MachineClass yields a newly initialized machine class object.
func (w *workerDelegate) MachineClass() client.Object {
	return &machinev1alpha1.OpenStackMachineClass{}
}

// MachineClassList yields a newly initialized OpenStackMachineClassList object.
func (w *workerDelegate) MachineClassList() client.ObjectList {
	return &machinev1alpha1.OpenStackMachineClassList{}
}

// DeployMachineClasses generates and creates the OpenStack specific machine classes.
func (w *workerDelegate) DeployMachineClasses(ctx context.Context) error {
	if w.machineClasses == nil {
		if err := w.generateMachineConfig(ctx); err != nil {
			return err
		}
	}
	return w.seedChartApplier.Apply(ctx, filepath.Join(openstack.InternalChartsPath, "machineclass"), w.worker.Namespace, "machineclass", kubernetes.Values(map[string]interface{}{"machineClasses": w.machineClasses}))
}

// GenerateMachineDeployments generates the configuration for the desired machine deployments.
func (w *workerDelegate) GenerateMachineDeployments(ctx context.Context) (worker.MachineDeployments, error) {
	if w.machineDeployments == nil {
		if err := w.generateMachineConfig(ctx); err != nil {
			return nil, err
		}
	}
	return w.machineDeployments, nil
}

func (w *workerDelegate) generateMachineClassSecretData(ctx context.Context) (map[string][]byte, error) {
	credentials, err := openstack.GetCredentials(ctx, w.Client(), w.worker.Spec.SecretRef)
	if err != nil {
		return nil, err
	}

	cloudProfileConfig, err := helper.CloudProfileConfigFromCluster(w.cluster)
	if err != nil {
		return nil, err
	}

	keyStoneURL, err := helper.FindKeyStoneURL(cloudProfileConfig.KeyStoneURLs, cloudProfileConfig.KeyStoneURL, w.worker.Spec.Region)
	if err != nil {
		return nil, err
	}

	return map[string][]byte{
		machinev1alpha1.OpenStackAuthURL:    []byte(keyStoneURL),
		machinev1alpha1.OpenStackInsecure:   []byte("true"),
		machinev1alpha1.OpenStackDomainName: []byte(credentials.DomainName),
		machinev1alpha1.OpenStackTenantName: []byte(credentials.TenantName),
		machinev1alpha1.OpenStackUsername:   []byte(credentials.Username),
		machinev1alpha1.OpenStackPassword:   []byte(credentials.Password),
	}, nil
}

func (w *workerDelegate) generateMachineConfig(ctx context.Context) error {
	var (
		machineDeployments = worker.MachineDeployments{}
		machineClasses     []map[string]interface{}
		machineImages      []api.MachineImage
	)

	machineClassSecretData, err := w.generateMachineClassSecretData(ctx)
	if err != nil {
		return err
	}

	infrastructureStatus := &api.InfrastructureStatus{}
	if _, _, err := w.Decoder().Decode(w.worker.Spec.InfrastructureProviderStatus.Raw, nil, infrastructureStatus); err != nil {
		return err
	}

	workerStatus, err := w.decodeWorkerProviderStatus()
	if err != nil {
		return err
	}

	serverGroupDepSet := newServerGroupDependencySet(workerStatus.ServerGroupDependencies)

	nodesSecurityGroup, err := helper.FindSecurityGroupByPurpose(infrastructureStatus.SecurityGroups, api.PurposeNodes)
	if err != nil {
		return err
	}

	for _, pool := range w.worker.Spec.Pools {
		zoneLen := int32(len(pool.Zones))

		machineImage, err := w.findMachineImage(pool.MachineImage.Name, pool.MachineImage.Version)
		if err != nil {
			return err
		}
		machineImages = appendMachineImage(machineImages, *machineImage)

		var volumeSize int
		if pool.Volume != nil {
			volumeSize, err = worker.DiskSize(pool.Volume.Size)
			if err != nil {
				return err
			}
		}

		serverGroupDep := serverGroupDepSet.getByPoolName(pool.Name)
		workerPoolHash, err := w.generateWorkerPoolHash(pool, serverGroupDep)
		if err != nil {
			return err
		}

		for zoneIndex, zone := range pool.Zones {
			zoneIdx := int32(zoneIndex)
			machineClassSpec := map[string]interface{}{
				"region":           w.worker.Spec.Region,
				"availabilityZone": zone,
				"machineType":      pool.MachineType,
				"keyName":          infrastructureStatus.Node.KeyName,
				"networkID":        infrastructureStatus.Networks.ID,
				"podNetworkCidr":   extensionscontroller.GetPodNetwork(w.cluster),
				"securityGroups":   []string{nodesSecurityGroup.Name},
				"tags": map[string]string{
					fmt.Sprintf("kubernetes.io-cluster-%s", w.worker.Namespace): "1",
					"kubernetes.io-role-node":                                   "1",
				},
				"secret": map[string]interface{}{
					"cloudConfig": string(pool.UserData),
				},
			}

			if volumeSize > 0 {
				machineClassSpec["rootDiskSize"] = volumeSize
			}

			if machineImage.ID != "" {
				machineClassSpec["imageID"] = machineImage.ID
			} else {
				machineClassSpec["imageName"] = machineImage.Image
			}

			if serverGroupDep != nil {
				machineClassSpec["serverGroupID"] = serverGroupDep.ID
			}

			var (
				deploymentName = fmt.Sprintf("%s-%s-z%d", w.worker.Namespace, pool.Name, zoneIndex+1)
				className      = fmt.Sprintf("%s-%s", deploymentName, workerPoolHash)
			)

			machineDeployments = append(machineDeployments, worker.MachineDeployment{
				Name:                 deploymentName,
				ClassName:            className,
				SecretName:           className,
				Minimum:              worker.DistributeOverZones(zoneIdx, pool.Minimum, zoneLen),
				Maximum:              worker.DistributeOverZones(zoneIdx, pool.Maximum, zoneLen),
				MaxSurge:             worker.DistributePositiveIntOrPercent(zoneIdx, pool.MaxSurge, zoneLen, pool.Maximum),
				MaxUnavailable:       worker.DistributePositiveIntOrPercent(zoneIdx, pool.MaxUnavailable, zoneLen, pool.Minimum),
				Labels:               pool.Labels,
				Annotations:          pool.Annotations,
				Taints:               pool.Taints,
				MachineConfiguration: genericworkeractuator.ReadMachineConfiguration(pool),
			})

			machineClassSpec["name"] = className
			machineClassSpec["labels"] = map[string]string{
				v1beta1constants.GardenerPurpose: genericworkeractuator.GardenPurposeMachineClass,
			}
			machineClassSpec["secret"].(map[string]interface{})[openstack.AuthURL] = string(machineClassSecretData[machinev1alpha1.OpenStackAuthURL])
			machineClassSpec["secret"].(map[string]interface{})[openstack.DomainName] = string(machineClassSecretData[machinev1alpha1.OpenStackDomainName])
			machineClassSpec["secret"].(map[string]interface{})[openstack.TenantName] = string(machineClassSecretData[machinev1alpha1.OpenStackTenantName])
			machineClassSpec["secret"].(map[string]interface{})[openstack.UserName] = string(machineClassSecretData[machinev1alpha1.OpenStackUsername])
			machineClassSpec["secret"].(map[string]interface{})[openstack.Password] = string(machineClassSecretData[machinev1alpha1.OpenStackPassword])

			machineClasses = append(machineClasses, machineClassSpec)
		}
	}

	w.machineDeployments = machineDeployments
	w.machineClasses = machineClasses
	w.machineImages = machineImages

	return nil
}

func (w *workerDelegate) generateWorkerPoolHash(pool extensionsv1alpha1.WorkerPool, serverGroupDependency *api.ServerGroupDependency) (string, error) {
	var additionalHashData = []string{}

	// Include the given worker pool dependencies into the hash.
	if serverGroupDependency != nil {
		additionalHashData = append(additionalHashData, serverGroupDependency.ID)
	}

	// Generate the worker pool hash.
	return worker.WorkerPoolHash(pool, w.cluster, additionalHashData...)
}
