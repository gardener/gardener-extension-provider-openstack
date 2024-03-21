// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-openstack/charts"
	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
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
		if err := w.generateMachineConfig(); err != nil {
			return err
		}
	}

	return w.seedChartApplier.ApplyFromEmbeddedFS(ctx, charts.InternalChart, filepath.Join(charts.InternalChartsPath, "machineclass"), w.worker.Namespace, "machineclass", kubernetes.Values(map[string]interface{}{"machineClasses": w.machineClasses}))
}

// GenerateMachineDeployments generates the configuration for the desired machine deployments.
func (w *workerDelegate) GenerateMachineDeployments(_ context.Context) (worker.MachineDeployments, error) {
	if w.machineDeployments == nil {
		if err := w.generateMachineConfig(); err != nil {
			return nil, err
		}
	}
	return w.machineDeployments, nil
}

func (w *workerDelegate) generateMachineConfig() error {
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
		zoneLen := int32(len(pool.Zones))

		architecture := pointer.StringDeref(pool.Architecture, v1beta1constants.ArchitectureAMD64)
		machineImage, err := w.findMachineImage(pool.MachineImage.Name, pool.MachineImage.Version, architecture)
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
				"tags": utils.MergeStringMaps(
					NormalizeLabelsForMachineClass(pool.Labels),
					NormalizeLabelsForMachineClass(machineLabels),
					map[string]string{
						fmt.Sprintf("kubernetes.io-cluster-%s", w.worker.Namespace): "1",
						"kubernetes.io-role-node":                                   "1",
					},
				),
				"credentialsSecretRef": map[string]interface{}{
					"name":      w.worker.Spec.SecretRef.Name,
					"namespace": w.worker.Spec.SecretRef.Namespace,
				},
				"secret": map[string]interface{}{
					"cloudConfig": string(pool.UserData),
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
				}
			} else if pool.NodeTemplate != nil {
				machineClassSpec["nodeTemplate"] = machinev1alpha1.NodeTemplate{
					Capacity:     pool.NodeTemplate.Capacity,
					InstanceType: pool.MachineType,
					Region:       w.worker.Spec.Region,
					Zone:         zone,
				}
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
				Labels:               addTopologyLabel(pool.Labels, zone),
				Annotations:          pool.Annotations,
				Taints:               pool.Taints,
				MachineConfiguration: genericworkeractuator.ReadMachineConfiguration(pool),
			})

			machineClassSpec["name"] = className
			machineClassSpec["labels"] = map[string]string{
				v1beta1constants.GardenerPurpose: v1beta1constants.GardenPurposeMachineClass,
			}

			if pool.MachineImage.Name != "" && pool.MachineImage.Version != "" {
				machineClassSpec["operatingSystem"] = map[string]interface{}{
					"operatingSystemName":    pool.MachineImage.Name,
					"operatingSystemVersion": pool.MachineImage.Version,
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

	// Currently the raw providerConfig is used to generate the hash which has unintended consequences like causing machine
	// rollouts. Instead the provider-extension should be capable of providing information
	if !w.hasPreserveAnnotation() {
		pool.ProviderConfig = nil
	}

	// Generate the worker pool hash.
	return worker.WorkerPoolHash(pool, w.cluster, additionalHashData...)
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

func (w *workerDelegate) hasPreserveAnnotation() bool {
	if v, ok := w.cluster.Shoot.Annotations[openstack.PreserveWorkerHashAnnotation]; ok && strings.EqualFold(v, "true") {
		return true
	}
	return false
}

func addTopologyLabel(labels map[string]string, zone string) map[string]string {
	return utils.MergeStringMaps(labels, map[string]string{
		openstack.CSIDiskDriverTopologyKey:   zone,
		openstack.CSIManilaDriverTopologyKey: zone,
	})
}
