// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	genericworkeractuator "github.com/gardener/gardener/extensions/pkg/controller/worker/genericactuator"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-openstack/charts"
	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	openstackapi "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
)

// MachineClassKind yields the name of the machine class kind used by OpenStack provider.
func (w *workerDelegate) MachineClassKind() string {
	return "MachineClass"
}

// MachineClass yields a newly initialized machine class object.
func (w *workerDelegate) MachineClass() client.Object {
	return &machinev1alpha1.MachineClass{}
}

// MachineClassList yields a newly initialized MachineClassList object.
func (w *workerDelegate) MachineClassList() client.ObjectList {
	return &machinev1alpha1.MachineClassList{}
}

// DeployMachineClasses generates and creates the OpenStack specific machine classes.
func (w *workerDelegate) DeployMachineClasses(ctx context.Context) error {
	if w.machineClasses == nil {
		if err := w.generateMachineConfig(ctx); err != nil {
			return err
		}
	}

	return w.seedChartApplier.ApplyFromEmbeddedFS(ctx, charts.InternalChart, filepath.Join(charts.InternalChartsPath, "machineclass"), w.worker.Namespace, "machineclass", kubernetes.Values(map[string]interface{}{"machineClasses": w.machineClasses}))
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

func (w *workerDelegate) generateMachineConfig(ctx context.Context) error {
	var (
		machineDeployments = worker.MachineDeployments{}
		machineClasses     []map[string]interface{}
		machineImages      []api.MachineImage
	)

	infrastructureStatus := &api.InfrastructureStatus{}
	if _, _, err := w.decoder.Decode(w.worker.Spec.InfrastructureProviderStatus.Raw, nil, infrastructureStatus); err != nil {
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

	subnet, err := helper.FindSubnetByPurpose(infrastructureStatus.Networks.Subnets, api.PurposeNodes)
	if err != nil {
		return err
	}

	for _, pool := range w.worker.Spec.Pools {
		zoneLen := int32(len(pool.Zones)) // #nosec: G115 - We validate if num pool zones exceeds max_int32.

		//TODO: this returns a capability with arch amd64, but worker pool defined architecture arm64.
		machineTypeFromCloudProfile := gardencorev1beta1helper.FindMachineTypeByName(w.cluster.CloudProfile.Spec.MachineTypes, pool.MachineType)
		if machineTypeFromCloudProfile == nil {
			return fmt.Errorf("machine type %q not found in cloud profile %q", pool.MachineType, w.cluster.CloudProfile.Name)
		}

		architecture := ptr.Deref(pool.Architecture, v1beta1constants.ArchitectureAMD64)
		machineImage, err := w.selectMachineImageForWorkerPool(pool.MachineImage.Name, pool.MachineImage.Version, w.worker.Spec.Region, &architecture, machineTypeFromCloudProfile.Capabilities)
		if err != nil {
			return err
		}

		machineImages = EnsureUniformMachineImages(machineImages, w.cluster.CloudProfile.Spec.MachineCapabilities)
		machineImages = appendMachineImage(machineImages, *machineImage, w.cluster.CloudProfile.Spec.MachineCapabilities)

		var volumeSize int
		if pool.Volume != nil {
			volumeSize, err = worker.DiskSize(pool.Volume.Size)
			if err != nil {
				return err
			}
		}

		workerConfig, err := helper.WorkerConfigFromRawExtension(pool.ProviderConfig)
		if err != nil {
			return err
		}

		var serverGroupDep *api.ServerGroupDependency
		if isServerGroupRequired(workerConfig) {
			serverGroupDep = serverGroupDepSet.getByPoolName(pool.Name)
			if serverGroupDep == nil {
				return fmt.Errorf("server group is required for pool %q, but no server group dependency found", pool.Name)
			}
		}

		workerPoolHash, err := w.generateWorkerPoolHash(pool, serverGroupDep, workerConfig)
		if err != nil {
			return err
		}

		machineLabels := map[string]string{}
		for _, pair := range workerConfig.MachineLabels {
			machineLabels[pair.Name] = pair.Value
		}

		userData, err := worker.FetchUserData(ctx, w.seedClient, w.worker.Namespace, pool)
		if err != nil {
			return err
		}

		for zoneIndex, zone := range pool.Zones {
			zoneIdx := int32(zoneIndex) // #nosec: G115 - We validate if num pool zones exceeds max_int32.
			machineClassSpec := map[string]interface{}{
				"region":           w.worker.Spec.Region,
				"availabilityZone": zone,
				"machineType":      pool.MachineType,
				"keyName":          infrastructureStatus.Node.KeyName,
				"networkID":        infrastructureStatus.Networks.ID,
				"podNetworkCIDRs":  extensionscontroller.GetPodNetwork(w.cluster),
				"securityGroups":   []string{nodesSecurityGroup.Name},
				"tags": utils.MergeStringMaps(
					NormalizeLabelsForMachineClass(pool.Labels),
					NormalizeLabelsForMachineClass(machineLabels),
					map[string]string{
						fmt.Sprintf("kubernetes.io-cluster-%s", w.cluster.Shoot.Status.TechnicalID): "1",
						"kubernetes.io-role-node": "1",
					},
				),
				"credentialsSecretRef": map[string]interface{}{
					"name":      w.worker.Spec.SecretRef.Name,
					"namespace": w.worker.Spec.SecretRef.Namespace,
				},
				"secret": map[string]interface{}{
					"cloudConfig": string(userData),
				},
			}

			machineClassSpec["subnetID"] = subnet.ID

			if volumeSize > 0 {
				machineClassSpec["rootDiskSize"] = volumeSize
			}

			// specifying the volume type requires a custom volume size to be specified too.
			if pool.Volume != nil && pool.Volume.Type != nil {
				machineClassSpec["rootDiskType"] = *pool.Volume.Type
			}

			if machineImage.ID != "" {
				machineClassSpec["imageID"] = machineImage.ID
			} else {
				machineClassSpec["imageName"] = machineImage.Image
			}

			if serverGroupDep != nil {
				machineClassSpec["serverGroupID"] = serverGroupDep.ID
			}

			if workerConfig.NodeTemplate != nil {
				machineClassSpec["nodeTemplate"] = machinev1alpha1.NodeTemplate{
					Capacity:     workerConfig.NodeTemplate.Capacity,
					InstanceType: pool.MachineType,
					Region:       w.worker.Spec.Region,
					Zone:         zone,
					Architecture: ptr.To(architecture),
				}
			} else if pool.NodeTemplate != nil {
				machineClassSpec["nodeTemplate"] = machinev1alpha1.NodeTemplate{
					Capacity:     pool.NodeTemplate.Capacity,
					InstanceType: pool.MachineType,
					Region:       w.worker.Spec.Region,
					Zone:         zone,
					Architecture: ptr.To(architecture),
				}
			}

			var (
				deploymentName = fmt.Sprintf("%s-%s-z%d", w.cluster.Shoot.Status.TechnicalID, pool.Name, zoneIndex+1)
				className      = fmt.Sprintf("%s-%s", deploymentName, workerPoolHash)
			)

			updateConfiguration := machinev1alpha1.UpdateConfiguration{
				MaxUnavailable: ptr.To(worker.DistributePositiveIntOrPercent(zoneIdx, pool.MaxUnavailable, zoneLen, pool.Minimum)),
				MaxSurge:       ptr.To(worker.DistributePositiveIntOrPercent(zoneIdx, pool.MaxSurge, zoneLen, pool.Maximum)),
			}

			machineDeploymentStrategy := machinev1alpha1.MachineDeploymentStrategy{
				Type: machinev1alpha1.RollingUpdateMachineDeploymentStrategyType,
				RollingUpdate: &machinev1alpha1.RollingUpdateMachineDeployment{
					UpdateConfiguration: updateConfiguration,
				},
			}

			if gardencorev1beta1helper.IsUpdateStrategyInPlace(pool.UpdateStrategy) {
				machineDeploymentStrategy = machinev1alpha1.MachineDeploymentStrategy{
					Type: machinev1alpha1.InPlaceUpdateMachineDeploymentStrategyType,
					InPlaceUpdate: &machinev1alpha1.InPlaceUpdateMachineDeployment{
						UpdateConfiguration: updateConfiguration,
						OrchestrationType:   machinev1alpha1.OrchestrationTypeAuto,
					},
				}

				if gardencorev1beta1helper.IsUpdateStrategyManualInPlace(pool.UpdateStrategy) {
					machineDeploymentStrategy.InPlaceUpdate.OrchestrationType = machinev1alpha1.OrchestrationTypeManual
				}
			}

			machineDeployments = append(machineDeployments, worker.MachineDeployment{
				Name:                         deploymentName,
				ClassName:                    className,
				SecretName:                   className,
				PoolName:                     pool.Name,
				Minimum:                      worker.DistributeOverZones(zoneIdx, pool.Minimum, zoneLen),
				Maximum:                      worker.DistributeOverZones(zoneIdx, pool.Maximum, zoneLen),
				Strategy:                     machineDeploymentStrategy,
				Priority:                     pool.Priority,
				Labels:                       addTopologyLabel(pool.Labels, zone),
				Annotations:                  pool.Annotations,
				Taints:                       pool.Taints,
				MachineConfiguration:         genericworkeractuator.ReadMachineConfiguration(pool),
				ClusterAutoscalerAnnotations: extensionsv1alpha1helper.GetMachineDeploymentClusterAutoscalerAnnotations(pool.ClusterAutoscaler),
			})

			machineClassSpec["name"] = className
			machineClassSpec["labels"] = map[string]string{
				v1beta1constants.GardenerPurpose: v1beta1constants.GardenPurposeMachineClass,
			}

			if pool.MachineImage.Name != "" && pool.MachineImage.Version != "" {
				machineClassSpec["operatingSystem"] = map[string]interface{}{
					"operatingSystemName":    pool.MachineImage.Name,
					"operatingSystemVersion": strings.ReplaceAll(pool.MachineImage.Version, "+", "_"),
				}
			}

			machineClasses = append(machineClasses, machineClassSpec)
		}
	}

	w.machineDeployments = machineDeployments
	w.machineClasses = machineClasses
	w.machineImages = machineImages

	return nil
}

func (w *workerDelegate) generateWorkerPoolHash(pool extensionsv1alpha1.WorkerPool, serverGroupDependency *api.ServerGroupDependency, workerConfig *api.WorkerConfig) (string, error) {
	var additionalHashData []string

	// Include the given worker pool dependencies into the hash.
	if serverGroupDependency != nil {
		additionalHashData = append(additionalHashData, serverGroupDependency.ID)
	}

	var pairs []string
	for _, pair := range workerConfig.MachineLabels {
		if pair.TriggerRollingOnUpdate {
			pairs = append(pairs, pair.Name+"="+pair.Value)
		}
	}

	if len(pairs) > 0 {
		// include machine labels marked for rolling
		sort.Strings(pairs)
		additionalHashData = append(additionalHashData, pairs...)
	}

	// hash v1 would otherwise hash the ProviderConfig
	pool.ProviderConfig = nil

	// Generate the worker pool hash.
	return worker.WorkerPoolHash(pool, w.cluster, additionalHashData, additionalHashData, nil)
}

// NormalizeLabelsForMachineClass because metadata in OpenStack resources do not allow for certain characters that present in k8s labels e.g. "/",
// normalize the label by replacing illegal characters with "-"
func NormalizeLabelsForMachineClass(in map[string]string) map[string]string {
	notAllowedChars := regexp.MustCompile(`[^a-zA-Z0-9-_:. ]`)
	res := make(map[string]string)
	for k, v := range in {
		newKey := notAllowedChars.ReplaceAllLiteralString(k, "-")
		res[newKey] = v
	}
	return res
}

func addTopologyLabel(labels map[string]string, zone string) map[string]string {
	return utils.MergeStringMaps(labels, map[string]string{
		openstack.CSIDiskDriverTopologyKey:   zone,
		openstack.CSIManilaDriverTopologyKey: zone,
	})
}

// EnsureUniformMachineImages ensures that all machine images are in the same format, either with or without Capabilities.
func EnsureUniformMachineImages(images []openstackapi.MachineImage, definitions []gardencorev1beta1.CapabilityDefinition) []openstackapi.MachineImage {
	var uniformMachineImages []openstackapi.MachineImage

	if len(definitions) == 0 {
		// transform images that were added with Capabilities to the legacy format without Capabilities
		for _, img := range images {
			if len(img.Capabilities) == 0 {
				// image is already legacy format
				uniformMachineImages = appendMachineImage(uniformMachineImages, img, definitions)
				continue
			}
			// transform to legacy format by using the Architecture capability if it exists
			var architecture *string
			if len(img.Capabilities[v1beta1constants.ArchitectureName]) > 0 {
				architecture = &img.Capabilities[v1beta1constants.ArchitectureName][0]
			}
			// TODO: make uniform for imageID is not set BUT the global image name is set
			uniformMachineImages = appendMachineImage(uniformMachineImages, openstackapi.MachineImage{
				Name:         img.Name,
				Version:      img.Version,
				ID:           img.ID,
				Architecture: architecture,
			}, definitions)
		}
		return uniformMachineImages
	}

	// transform images that were added without Capabilities to contain a MachineImageFlavor with defaulted Architecture
	for _, img := range images {
		if len(img.Capabilities) > 0 {
			// image is already in the new format with Capabilities
			uniformMachineImages = appendMachineImage(uniformMachineImages, img, definitions)
		} else {
			// add image as a capability set with defaulted Architecture
			architecture := ptr.Deref(img.Architecture, v1beta1constants.ArchitectureAMD64)
			uniformMachineImages = appendMachineImage(uniformMachineImages, openstackapi.MachineImage{
				Name:         img.Name,
				Version:      img.Version,
				ID:           img.ID,
				Capabilities: gardencorev1beta1.Capabilities{v1beta1constants.ArchitectureName: []string{architecture}},
			}, definitions)
		}
	}
	return uniformMachineImages
}
