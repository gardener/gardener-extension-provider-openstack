// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker_test

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	genericworkeractuator "github.com/gardener/gardener/extensions/pkg/controller/worker/genericactuator"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	mockkubernetes "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	"github.com/gardener/gardener/pkg/utils"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-openstack/charts"
	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	apiv1alpha1 "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"
	. "github.com/gardener/gardener-extension-provider-openstack/pkg/controller/worker"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
)

var _ = Describe("Machines", func() {
	var (
		ctx = context.Background()

		ctrl         *gomock.Controller
		c            *mockclient.MockClient
		statusWriter *mockclient.MockStatusWriter
		chartApplier *mockkubernetes.MockChartApplier

		workerDelegate genericworkeractuator.WorkerDelegate
		scheme         *runtime.Scheme
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		c = mockclient.NewMockClient(ctrl)
		statusWriter = mockclient.NewMockStatusWriter(ctrl)
		chartApplier = mockkubernetes.NewMockChartApplier(ctrl)

		scheme = runtime.NewScheme()
		_ = api.AddToScheme(scheme)
		_ = apiv1alpha1.AddToScheme(scheme)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("workerDelegate", func() {
		BeforeEach(func() {
			workerDelegate, _ = NewWorkerDelegate(nil, scheme, nil, "", nil, nil, nil)
		})

		Describe("#TestLabelNormalization", func() {
			It("should return the correct list of labels", func() {
				input := map[string]string{
					"a/b/c":     "value",
					"test/node": "value",
					"node-role": "value",
				}

				output := NormalizeLabelsForMachineClass(input)
				expected := map[string]string{
					"a-b-c":     "value",
					"test-node": "value",
					"node-role": "value",
				}
				Expect(output).To(Equal(expected))
			})
		})

		Describe("#GenerateMachineDeployments, #DeployMachineClasses", func() {
			var (
				namespace        string
				cloudProfileName string

				openstackAuthURL string
				region           string
				regionWithImages string

				machineImageName    string
				machineImageVersion string
				machineImage        string
				machineImageID      string

				archAMD string
				archARM string

				keyName               string
				machineType           string
				userData              []byte
				userDataSecretName    string
				userDataSecretDataKey string
				networkID             string
				podCIDR               string
				subnetID              string
				securityGroupName     string

				namePool1           string
				minPool1            int32
				maxPool1            int32
				maxSurgePool1       intstr.IntOrString
				maxUnavailablePool1 intstr.IntOrString

				namePool2           string
				minPool2            int32
				maxPool2            int32
				priorityPool2       int32
				maxSurgePool2       intstr.IntOrString
				maxUnavailablePool2 intstr.IntOrString

				namePool3 string

				zone1 string
				zone2 string

				nodeCapacity         corev1.ResourceList
				nodeTemplateZone1    machinev1alpha1.NodeTemplate
				nodeTemplateZone2    machinev1alpha1.NodeTemplate
				machineConfiguration *machinev1alpha1.MachineConfiguration

				workerPoolHash1 string
				workerPoolHash2 string
				workerPoolHash3 string

				shootVersionMajorMinor string
				shootVersion           string
				cloudProfileConfig     *api.CloudProfileConfig
				cloudProfileConfigJSON []byte
				clusterWithoutImages   *extensionscontroller.Cluster
				cluster                *extensionscontroller.Cluster
				w                      *extensionsv1alpha1.Worker

				emptyClusterAutoscalerAnnotations map[string]string
			)

			BeforeEach(func() {
				namespace = "shoot--foobar--openstack"
				cloudProfileName = "openstack"

				region = "eu-de-1"
				regionWithImages = "eu-de-2"

				openstackAuthURL = "auth-url"

				machineImageName = "my-os"
				machineImageVersion = "123.4.5-foo+bar123"
				machineImage = "my-image-in-glance"
				machineImageID = "my-image-id"

				archAMD = "amd64"
				archARM = "arm64"

				keyName = "key-name"
				machineType = "large"
				userData = []byte("some-user-data")
				userDataSecretName = "userdata-secret-name"
				userDataSecretDataKey = "userdata-secret-key"
				networkID = "network-id"
				podCIDR = "1.2.3.4/5"
				subnetID = "subnetID"
				securityGroupName = "nodes-sec-group"

				namePool1 = "pool-1"
				minPool1 = 5
				maxPool1 = 10
				maxSurgePool1 = intstr.FromInt(3)
				maxUnavailablePool1 = intstr.FromInt(2)

				namePool2 = "pool-2"
				minPool2 = 30
				maxPool2 = 45
				priorityPool2 = 100
				maxSurgePool2 = intstr.FromInt(10)
				maxUnavailablePool2 = intstr.FromInt(15)

				namePool3 = "pool-3"

				zone1 = region + "a"
				zone2 = region + "b"

				emptyClusterAutoscalerAnnotations = map[string]string{
					"autoscaler.gardener.cloud/max-node-provision-time":              "",
					"autoscaler.gardener.cloud/scale-down-gpu-utilization-threshold": "",
					"autoscaler.gardener.cloud/scale-down-unneeded-time":             "",
					"autoscaler.gardener.cloud/scale-down-unready-time":              "",
					"autoscaler.gardener.cloud/scale-down-utilization-threshold":     "",
				}

				nodeCapacity = corev1.ResourceList{
					"cpu":    resource.MustParse("8"),
					"gpu":    resource.MustParse("1"),
					"memory": resource.MustParse("128Gi"),
				}
				nodeTemplateZone1 = machinev1alpha1.NodeTemplate{
					Capacity:     nodeCapacity,
					InstanceType: machineType,
					Region:       region,
					Zone:         zone1,
					Architecture: &archAMD,
				}

				nodeTemplateZone2 = machinev1alpha1.NodeTemplate{
					Capacity:     nodeCapacity,
					InstanceType: machineType,
					Region:       region,
					Zone:         zone2,
					Architecture: &archAMD,
				}

				machineConfiguration = &machinev1alpha1.MachineConfiguration{}

				shootVersionMajorMinor = "1.29"
				shootVersion = shootVersionMajorMinor + ".3"

				cloudProfileConfig = &api.CloudProfileConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: api.SchemeGroupVersion.String(),
						Kind:       "CloudProfileConfig",
					},
					KeyStoneURL: openstackAuthURL,
				}
				cloudProfileConfigJSON, _ = json.Marshal(cloudProfileConfig)

				clusterWithoutImages = &extensionscontroller.Cluster{
					CloudProfile: &gardencorev1beta1.CloudProfile{
						ObjectMeta: metav1.ObjectMeta{
							Name: cloudProfileName,
						},
						Spec: gardencorev1beta1.CloudProfileSpec{
							ProviderConfig: &runtime.RawExtension{
								Raw: cloudProfileConfigJSON,
							},
						},
					},
					Shoot: &gardencorev1beta1.Shoot{
						Spec: gardencorev1beta1.ShootSpec{
							Networking: &gardencorev1beta1.Networking{
								Pods: &podCIDR,
							},
							Kubernetes: gardencorev1beta1.Kubernetes{
								Version: shootVersion,
							},
						},
					},
				}

				cloudProfileConfig.MachineImages = []api.MachineImages{
					{
						Name: machineImageName,
						Versions: []api.MachineImageVersion{
							{
								Version: machineImageVersion,
								Image:   machineImage,
								Regions: []api.RegionIDMapping{
									{
										Name:         regionWithImages,
										ID:           machineImageID,
										Architecture: &archARM,
									},
									{
										Name:         regionWithImages,
										ID:           machineImageID,
										Architecture: &archAMD,
									},
								},
							},
						},
					},
				}
				cloudProfileConfigJSON, _ = json.Marshal(cloudProfileConfig)
				cluster = &extensionscontroller.Cluster{
					CloudProfile: &gardencorev1beta1.CloudProfile{
						ObjectMeta: metav1.ObjectMeta{
							Name: cloudProfileName,
						},
						Spec: gardencorev1beta1.CloudProfileSpec{
							ProviderConfig: &runtime.RawExtension{
								Raw: cloudProfileConfigJSON,
							},
						},
					},
					Shoot: clusterWithoutImages.Shoot,
				}

				w = &extensionsv1alpha1.Worker{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
					},
					Spec: extensionsv1alpha1.WorkerSpec{
						SecretRef: corev1.SecretReference{
							Name:      "secret",
							Namespace: namespace,
						},
						Region: region,
						InfrastructureProviderStatus: &runtime.RawExtension{
							Raw: encode(&api.InfrastructureStatus{
								SecurityGroups: []api.SecurityGroup{
									{
										Purpose: api.PurposeNodes,
										Name:    securityGroupName,
									},
								},
								Node: api.NodeStatus{
									KeyName: keyName,
								},
								Networks: api.NetworkStatus{
									ID: networkID,
									Subnets: []api.Subnet{
										{
											Purpose: api.PurposeNodes,
											ID:      subnetID,
										},
									},
								},
							}),
						},
						Pools: []extensionsv1alpha1.WorkerPool{
							{
								Name:           namePool1,
								Minimum:        minPool1,
								Maximum:        maxPool1,
								MaxSurge:       maxSurgePool1,
								MaxUnavailable: maxUnavailablePool1,
								MachineType:    machineType,
								Architecture:   &archAMD,
								MachineImage: extensionsv1alpha1.MachineImage{
									Name:    machineImageName,
									Version: machineImageVersion,
								},
								NodeTemplate: &extensionsv1alpha1.NodeTemplate{
									Capacity: nodeCapacity,
								},
								UserDataSecretRef: corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: userDataSecretName},
									Key:                  userDataSecretDataKey,
								},
								Zones: []string{
									zone1,
									zone2,
								},
							},
							{
								Name:           namePool2,
								Minimum:        minPool2,
								Maximum:        maxPool2,
								Priority:       priorityPool2,
								MaxSurge:       maxSurgePool2,
								Architecture:   &archAMD,
								MaxUnavailable: maxUnavailablePool2,
								MachineType:    machineType,
								MachineImage: extensionsv1alpha1.MachineImage{
									Name:    machineImageName,
									Version: machineImageVersion,
								},
								NodeTemplate: &extensionsv1alpha1.NodeTemplate{
									Capacity: nodeCapacity,
								},
								UserDataSecretRef: corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: userDataSecretName},
									Key:                  userDataSecretDataKey,
								},
								Zones: []string{
									zone1,
									zone2,
								},
								UpdateStrategy:    ptr.To(gardencorev1beta1.AutoInPlaceUpdate),
								KubernetesVersion: ptr.To(shootVersion),
							},
							{
								Name:           namePool3,
								Minimum:        minPool2,
								Maximum:        maxPool2,
								Priority:       priorityPool2,
								MaxSurge:       maxSurgePool2,
								Architecture:   &archAMD,
								MaxUnavailable: maxUnavailablePool2,
								MachineType:    machineType,
								MachineImage: extensionsv1alpha1.MachineImage{
									Name:    machineImageName,
									Version: machineImageVersion,
								},
								NodeTemplate: &extensionsv1alpha1.NodeTemplate{
									Capacity: nodeCapacity,
								},
								UserDataSecretRef: corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: userDataSecretName},
									Key:                  userDataSecretDataKey,
								},
								Zones: []string{
									zone1,
									zone2,
								},
								UpdateStrategy:    ptr.To(gardencorev1beta1.ManualInPlaceUpdate),
								KubernetesVersion: ptr.To(shootVersion),
							},
						},
					},
				}

				workerPoolHash1, _ = worker.WorkerPoolHash(w.Spec.Pools[0], cluster, nil, nil, nil)
				workerPoolHash2, _ = worker.WorkerPoolHash(w.Spec.Pools[1], cluster, nil, nil, nil)
				workerPoolHash3, _ = worker.WorkerPoolHash(w.Spec.Pools[2], cluster, nil, nil, nil)

				workerDelegate, _ = NewWorkerDelegate(c, scheme, chartApplier, "", w, clusterWithoutImages, nil)
			})

			expectedUserDataSecretRefRead := func() {
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: userDataSecretName}, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
					func(_ context.Context, _ client.ObjectKey, secret *corev1.Secret, _ ...client.GetOption) error {
						secret.Data = map[string][]byte{userDataSecretDataKey: userData}
						return nil
					},
				).AnyTimes()
			}

			Describe("machine images", func() {
				var (
					defaultMachineClass map[string]interface{}
					machineDeployments  worker.MachineDeployments
					machineClasses      map[string]interface{}
					workerWithRegion    *extensionsv1alpha1.Worker
					clusterWithRegion   *extensionscontroller.Cluster
				)

				setup := func(region, name, imageID, architecture string) {
					workerWithRegion = w.DeepCopy()
					zone1 = region + "a"
					zone2 = region + "b"
					workerWithRegion.Spec.Region = region
					workerWithRegion.Spec.Pools[0].Architecture = &architecture
					workerWithRegion.Spec.Pools[1].Architecture = &architecture
					workerWithRegion.Spec.Pools[2].Architecture = &architecture

					workerWithRegion.Spec.Pools[0].Zones = []string{zone1, zone2}
					workerWithRegion.Spec.Pools[1].Zones = []string{zone1, zone2}
					workerWithRegion.Spec.Pools[2].Zones = []string{zone1, zone2}

					clusterWithRegion = &extensionscontroller.Cluster{
						CloudProfile: cluster.CloudProfile,
						Shoot:        cluster.Shoot.DeepCopy(),
						Seed:         cluster.Seed,
					}
					clusterWithRegion.Shoot.Spec.Region = region

					defaultMachineClass = map[string]interface{}{
						"region":          region,
						"machineType":     machineType,
						"keyName":         keyName,
						"networkID":       networkID,
						"subnetID":        subnetID,
						"podNetworkCIDRs": []string{podCIDR},
						"securityGroups":  []string{securityGroupName},
						"tags": map[string]string{
							fmt.Sprintf("kubernetes.io-cluster-%s", namespace): "1",
							"kubernetes.io-role-node":                          "1",
						},
						"secret": map[string]interface{}{
							"cloudConfig": string(userData),
						},
						"operatingSystem": map[string]interface{}{
							"operatingSystemName":    machineImageName,
							"operatingSystemVersion": strings.ReplaceAll(machineImageVersion, "+", "_"),
						},
					}
					if imageID == "" {
						defaultMachineClass["imageName"] = name
					} else {
						defaultMachineClass["imageID"] = imageID
					}

					newNodeTemplateZone1 := machinev1alpha1.NodeTemplate{
						Capacity:     nodeCapacity,
						InstanceType: machineType,
						Region:       region,
						Zone:         zone1,
						Architecture: &architecture,
					}

					newNodeTemplateZone2 := machinev1alpha1.NodeTemplate{
						Capacity:     nodeCapacity,
						InstanceType: machineType,
						Region:       region,
						Zone:         zone2,
						Architecture: &architecture,
					}

					var (
						machineClassPool1Zone1 = useDefaultMachineClass(defaultMachineClass, "availabilityZone", zone1)
						machineClassPool1Zone2 = useDefaultMachineClass(defaultMachineClass, "availabilityZone", zone2)
						machineClassPool2Zone1 = useDefaultMachineClass(defaultMachineClass, "availabilityZone", zone1)
						machineClassPool2Zone2 = useDefaultMachineClass(defaultMachineClass, "availabilityZone", zone2)
						machineClassPool3Zone1 = useDefaultMachineClass(defaultMachineClass, "availabilityZone", zone1)
						machineClassPool3Zone2 = useDefaultMachineClass(defaultMachineClass, "availabilityZone", zone2)

						machineClassNamePool1Zone1 = fmt.Sprintf("%s-%s-z1", namespace, namePool1)
						machineClassNamePool1Zone2 = fmt.Sprintf("%s-%s-z2", namespace, namePool1)
						machineClassNamePool2Zone1 = fmt.Sprintf("%s-%s-z1", namespace, namePool2)
						machineClassNamePool2Zone2 = fmt.Sprintf("%s-%s-z2", namespace, namePool2)
						machineClassNamePool3Zone1 = fmt.Sprintf("%s-%s-z1", namespace, namePool3)
						machineClassNamePool3Zone2 = fmt.Sprintf("%s-%s-z2", namespace, namePool3)

						machineClassWithHashPool1Zone1 = fmt.Sprintf("%s-%s", machineClassNamePool1Zone1, workerPoolHash1)
						machineClassWithHashPool1Zone2 = fmt.Sprintf("%s-%s", machineClassNamePool1Zone2, workerPoolHash1)
						machineClassWithHashPool2Zone1 = fmt.Sprintf("%s-%s", machineClassNamePool2Zone1, workerPoolHash2)
						machineClassWithHashPool2Zone2 = fmt.Sprintf("%s-%s", machineClassNamePool2Zone2, workerPoolHash2)
						machineClassWithHashPool3Zone1 = fmt.Sprintf("%s-%s", machineClassNamePool3Zone1, workerPoolHash3)
						machineClassWithHashPool3Zone2 = fmt.Sprintf("%s-%s", machineClassNamePool3Zone2, workerPoolHash3)
					)

					addNameAndSecretToMachineClass(machineClassPool1Zone1, machineClassWithHashPool1Zone1, w.Spec.SecretRef)
					addNameAndSecretToMachineClass(machineClassPool1Zone2, machineClassWithHashPool1Zone2, w.Spec.SecretRef)
					addNameAndSecretToMachineClass(machineClassPool2Zone1, machineClassWithHashPool2Zone1, w.Spec.SecretRef)
					addNameAndSecretToMachineClass(machineClassPool2Zone2, machineClassWithHashPool2Zone2, w.Spec.SecretRef)
					addNameAndSecretToMachineClass(machineClassPool3Zone1, machineClassWithHashPool3Zone1, w.Spec.SecretRef)
					addNameAndSecretToMachineClass(machineClassPool3Zone2, machineClassWithHashPool3Zone2, w.Spec.SecretRef)

					addNodeTemplateToMachineClass(machineClassPool1Zone1, newNodeTemplateZone1)
					addNodeTemplateToMachineClass(machineClassPool1Zone2, newNodeTemplateZone2)
					addNodeTemplateToMachineClass(machineClassPool2Zone1, newNodeTemplateZone1)
					addNodeTemplateToMachineClass(machineClassPool2Zone2, newNodeTemplateZone2)
					addNodeTemplateToMachineClass(machineClassPool3Zone1, newNodeTemplateZone1)
					addNodeTemplateToMachineClass(machineClassPool3Zone2, newNodeTemplateZone2)

					machineClasses = map[string]interface{}{"machineClasses": []map[string]interface{}{
						machineClassPool1Zone1,
						machineClassPool1Zone2,
						machineClassPool2Zone1,
						machineClassPool2Zone2,
						machineClassPool3Zone1,
						machineClassPool3Zone2,
					}}

					labelsZone1 := map[string]string{openstack.CSIDiskDriverTopologyKey: zone1, openstack.CSIManilaDriverTopologyKey: zone1}
					labelsZone2 := map[string]string{openstack.CSIDiskDriverTopologyKey: zone2, openstack.CSIManilaDriverTopologyKey: zone2}
					machineDeployments = worker.MachineDeployments{
						{
							Name:       machineClassNamePool1Zone1,
							ClassName:  machineClassWithHashPool1Zone1,
							SecretName: machineClassWithHashPool1Zone1,
							Minimum:    worker.DistributeOverZones(0, minPool1, 2),
							Maximum:    worker.DistributeOverZones(0, maxPool1, 2),
							PoolName:   namePool1,
							Strategy: machinev1alpha1.MachineDeploymentStrategy{
								Type: machinev1alpha1.RollingUpdateMachineDeploymentStrategyType,
								RollingUpdate: &machinev1alpha1.RollingUpdateMachineDeployment{
									UpdateConfiguration: machinev1alpha1.UpdateConfiguration{
										MaxUnavailable: ptr.To(worker.DistributePositiveIntOrPercent(0, maxUnavailablePool1, 2, minPool1)),
										MaxSurge:       ptr.To(worker.DistributePositiveIntOrPercent(0, maxSurgePool1, 2, maxPool1)),
									},
								},
							},
							Labels:                       labelsZone1,
							MachineConfiguration:         machineConfiguration,
							ClusterAutoscalerAnnotations: emptyClusterAutoscalerAnnotations,
						},
						{
							Name:       machineClassNamePool1Zone2,
							ClassName:  machineClassWithHashPool1Zone2,
							SecretName: machineClassWithHashPool1Zone2,
							Minimum:    worker.DistributeOverZones(1, minPool1, 2),
							Maximum:    worker.DistributeOverZones(1, maxPool1, 2),
							PoolName:   namePool1,
							Strategy: machinev1alpha1.MachineDeploymentStrategy{
								Type: machinev1alpha1.RollingUpdateMachineDeploymentStrategyType,
								RollingUpdate: &machinev1alpha1.RollingUpdateMachineDeployment{
									UpdateConfiguration: machinev1alpha1.UpdateConfiguration{
										MaxUnavailable: ptr.To(worker.DistributePositiveIntOrPercent(1, maxUnavailablePool1, 2, minPool1)),
										MaxSurge:       ptr.To(worker.DistributePositiveIntOrPercent(1, maxSurgePool1, 2, maxPool1)),
									},
								},
							},
							Labels:                       labelsZone2,
							MachineConfiguration:         machineConfiguration,
							ClusterAutoscalerAnnotations: emptyClusterAutoscalerAnnotations,
						},
						{
							Name:       machineClassNamePool2Zone1,
							ClassName:  machineClassWithHashPool2Zone1,
							SecretName: machineClassWithHashPool2Zone1,
							Minimum:    worker.DistributeOverZones(0, minPool2, 2),
							Maximum:    worker.DistributeOverZones(0, maxPool2, 2),
							Priority:   priorityPool2,
							PoolName:   namePool2,
							Strategy: machinev1alpha1.MachineDeploymentStrategy{
								Type: machinev1alpha1.InPlaceUpdateMachineDeploymentStrategyType,
								InPlaceUpdate: &machinev1alpha1.InPlaceUpdateMachineDeployment{
									OrchestrationType: machinev1alpha1.OrchestrationTypeAuto,
									UpdateConfiguration: machinev1alpha1.UpdateConfiguration{
										MaxUnavailable: ptr.To(worker.DistributePositiveIntOrPercent(0, maxUnavailablePool2, 2, minPool2)),
										MaxSurge:       ptr.To(worker.DistributePositiveIntOrPercent(0, maxSurgePool2, 2, maxPool2)),
									},
								},
							},
							Labels:                       labelsZone1,
							MachineConfiguration:         machineConfiguration,
							ClusterAutoscalerAnnotations: emptyClusterAutoscalerAnnotations,
						},
						{
							Name:       machineClassNamePool2Zone2,
							ClassName:  machineClassWithHashPool2Zone2,
							SecretName: machineClassWithHashPool2Zone2,
							Minimum:    worker.DistributeOverZones(1, minPool2, 2),
							Maximum:    worker.DistributeOverZones(1, maxPool2, 2),
							Priority:   priorityPool2,
							PoolName:   namePool2,
							Strategy: machinev1alpha1.MachineDeploymentStrategy{
								Type: machinev1alpha1.InPlaceUpdateMachineDeploymentStrategyType,
								InPlaceUpdate: &machinev1alpha1.InPlaceUpdateMachineDeployment{
									OrchestrationType: machinev1alpha1.OrchestrationTypeAuto,
									UpdateConfiguration: machinev1alpha1.UpdateConfiguration{
										MaxUnavailable: ptr.To(worker.DistributePositiveIntOrPercent(1, maxUnavailablePool2, 2, minPool2)),
										MaxSurge:       ptr.To(worker.DistributePositiveIntOrPercent(1, maxSurgePool2, 2, maxPool2)),
									},
								},
							},
							Labels:                       labelsZone2,
							MachineConfiguration:         machineConfiguration,
							ClusterAutoscalerAnnotations: emptyClusterAutoscalerAnnotations,
						},
						{
							Name:       machineClassNamePool3Zone1,
							ClassName:  machineClassWithHashPool3Zone1,
							SecretName: machineClassWithHashPool3Zone1,
							Minimum:    worker.DistributeOverZones(0, minPool2, 2),
							Maximum:    worker.DistributeOverZones(0, maxPool2, 2),
							Priority:   priorityPool2,
							PoolName:   namePool3,
							Strategy: machinev1alpha1.MachineDeploymentStrategy{
								Type: machinev1alpha1.InPlaceUpdateMachineDeploymentStrategyType,
								InPlaceUpdate: &machinev1alpha1.InPlaceUpdateMachineDeployment{
									OrchestrationType: machinev1alpha1.OrchestrationTypeManual,
									UpdateConfiguration: machinev1alpha1.UpdateConfiguration{
										MaxUnavailable: ptr.To(worker.DistributePositiveIntOrPercent(0, maxUnavailablePool2, 2, minPool2)),
										MaxSurge:       ptr.To(worker.DistributePositiveIntOrPercent(0, maxSurgePool2, 2, maxPool2)),
									},
								},
							},
							Labels:                       labelsZone1,
							MachineConfiguration:         machineConfiguration,
							ClusterAutoscalerAnnotations: emptyClusterAutoscalerAnnotations,
						},
						{
							Name:       machineClassNamePool3Zone2,
							ClassName:  machineClassWithHashPool3Zone2,
							SecretName: machineClassWithHashPool3Zone2,
							Minimum:    worker.DistributeOverZones(1, minPool2, 2),
							Maximum:    worker.DistributeOverZones(1, maxPool2, 2),
							Priority:   priorityPool2,
							PoolName:   namePool3,
							Strategy: machinev1alpha1.MachineDeploymentStrategy{
								Type: machinev1alpha1.InPlaceUpdateMachineDeploymentStrategyType,
								InPlaceUpdate: &machinev1alpha1.InPlaceUpdateMachineDeployment{
									OrchestrationType: machinev1alpha1.OrchestrationTypeManual,
									UpdateConfiguration: machinev1alpha1.UpdateConfiguration{
										MaxUnavailable: ptr.To(worker.DistributePositiveIntOrPercent(1, maxUnavailablePool2, 2, minPool2)),
										MaxSurge:       ptr.To(worker.DistributePositiveIntOrPercent(1, maxSurgePool2, 2, maxPool2)),
									},
								},
							},
							Labels:                       labelsZone2,
							MachineConfiguration:         machineConfiguration,
							ClusterAutoscalerAnnotations: emptyClusterAutoscalerAnnotations,
						},
					}
				}

				It("should return the expected machine deployments for profile image types", func() {
					setup(region, machineImage, "", archAMD)
					workerDelegate, _ := NewWorkerDelegate(c, scheme, chartApplier, "", w, cluster, nil)

					// Test workerDelegate.DeployMachineClasses()
					expectedUserDataSecretRefRead()

					chartApplier.
						EXPECT().
						ApplyFromEmbeddedFS(
							ctx,
							charts.InternalChart,
							filepath.Join("internal", "machineclass"),
							namespace,
							"machineclass",
							kubernetes.Values(machineClasses),
						).
						Return(nil)

					err := workerDelegate.DeployMachineClasses(ctx)
					Expect(err).NotTo(HaveOccurred())

					// Test workerDelegate.UpdateMachineDeployments()

					expectedImages := &apiv1alpha1.WorkerStatus{
						TypeMeta: metav1.TypeMeta{
							APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
							Kind:       "WorkerStatus",
						},
						MachineImages: []apiv1alpha1.MachineImage{
							{
								Name:         machineImageName,
								Version:      machineImageVersion,
								Image:        machineImage,
								Architecture: ptr.To(v1beta1constants.ArchitectureAMD64),
							},
						},
					}

					workerWithExpectedImages := w.DeepCopy()
					workerWithExpectedImages.Status.ProviderStatus = &runtime.RawExtension{
						Object: expectedImages,
					}

					c.EXPECT().Status().Return(statusWriter)
					statusWriter.EXPECT().Patch(ctx, workerWithExpectedImages, gomock.Any()).Return(nil)

					err = workerDelegate.UpdateMachineImagesStatus(ctx)
					Expect(err).NotTo(HaveOccurred())

					// Test workerDelegate.GenerateMachineDeployments()

					result, err := workerDelegate.GenerateMachineDeployments(ctx)
					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(Equal(machineDeployments))
				})

				It("should return the expected machine deployments for profile image types with id", func() {
					setup(regionWithImages, "", machineImageID, archARM)
					workerDelegate, _ := NewWorkerDelegate(c, scheme, chartApplier, "", workerWithRegion, clusterWithRegion, nil)
					clusterWithRegion.Shoot.Spec.Hibernation = &gardencorev1beta1.Hibernation{Enabled: ptr.To(true)}

					// Test workerDelegate.DeployMachineClasses()
					expectedUserDataSecretRefRead()

					chartApplier.
						EXPECT().
						ApplyFromEmbeddedFS(
							ctx,
							charts.InternalChart,
							filepath.Join("internal", "machineclass"),
							namespace,
							"machineclass",
							kubernetes.Values(machineClasses),
						).
						Return(nil)

					err := workerDelegate.DeployMachineClasses(ctx)
					Expect(err).NotTo(HaveOccurred())

					// Test workerDelegate.GetMachineImages()
					expectedImages := &apiv1alpha1.WorkerStatus{
						TypeMeta: metav1.TypeMeta{
							APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
							Kind:       "WorkerStatus",
						},
						MachineImages: []apiv1alpha1.MachineImage{
							{
								Name:         machineImageName,
								Version:      machineImageVersion,
								ID:           machineImageID,
								Architecture: ptr.To(v1beta1constants.ArchitectureARM64),
							},
						},
					}

					workerWithExpectedImages := workerWithRegion.DeepCopy()
					workerWithExpectedImages.Status.ProviderStatus = &runtime.RawExtension{
						Object: expectedImages,
					}

					ctx := ctx
					c.EXPECT().Status().Return(statusWriter)
					statusWriter.EXPECT().Patch(ctx, workerWithExpectedImages, gomock.Any()).Return(nil)

					err = workerDelegate.UpdateMachineImagesStatus(ctx)
					Expect(err).NotTo(HaveOccurred())

					// Test workerDelegate.GenerateMachineDeployments()

					result, err := workerDelegate.GenerateMachineDeployments(ctx)
					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(Equal(machineDeployments))
				})

				Context("Server Groups", func() {
					It("should create the expected machine classes with server group configurations", func() {
						var (
							serverGroupName1 = "servergroup1"
							serverGroupName2 = "servergroup2"
							serverGroupName3 = "servergroup3"
							serverGroupID1   = "id1"
							serverGroupID2   = "id2"
							serverGroupID3   = "id3"
						)

						setup(region, machineImage, "", archAMD)

						workerWithServerGroup := w.DeepCopy()
						workerWithServerGroup.Spec.Pools[0].ProviderConfig = &runtime.RawExtension{
							Object: &apiv1alpha1.WorkerConfig{
								TypeMeta: metav1.TypeMeta{
									Kind:       "WorkerConfig",
									APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
								},
								ServerGroup: &apiv1alpha1.ServerGroup{
									Policy: "policy",
								},
							},
						}
						workerWithServerGroup.Spec.Pools[1].ProviderConfig = &runtime.RawExtension{
							Object: &apiv1alpha1.WorkerConfig{
								TypeMeta: metav1.TypeMeta{
									Kind:       "WorkerConfig",
									APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
								},
								ServerGroup: &apiv1alpha1.ServerGroup{
									Policy: "policy",
								},
							},
						}
						workerWithServerGroup.Spec.Pools[2].ProviderConfig = &runtime.RawExtension{
							Object: &apiv1alpha1.WorkerConfig{
								TypeMeta: metav1.TypeMeta{
									Kind:       "WorkerConfig",
									APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
								},
								ServerGroup: &apiv1alpha1.ServerGroup{
									Policy: "policy",
								},
							},
						}
						workerWithServerGroup.Status.ProviderStatus = &runtime.RawExtension{
							Object: &apiv1alpha1.WorkerStatus{
								TypeMeta: metav1.TypeMeta{
									Kind:       "WorkerStatus",
									APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
								},
								ServerGroupDependencies: []apiv1alpha1.ServerGroupDependency{
									{
										PoolName: namePool1,
										Name:     serverGroupName1,
										ID:       serverGroupID1,
									},
									{
										PoolName: namePool2,
										Name:     serverGroupName2,
										ID:       serverGroupID2,
									},
									{
										PoolName: namePool3,
										Name:     serverGroupName3,
										ID:       serverGroupID3,
									},
								},
							},
						}

						workerDelegate, _ := NewWorkerDelegate(c, scheme, chartApplier, "", workerWithServerGroup, cluster, nil)

						// Test workerDelegate.DeployMachineClasses()
						workerPoolHash1, _ := worker.WorkerPoolHash(w.Spec.Pools[0], cluster, []string{serverGroupID1}, []string{serverGroupID1}, nil)
						workerPoolHash2, _ := worker.WorkerPoolHash(w.Spec.Pools[1], cluster, nil, nil, nil)
						workerPoolHash3, _ := worker.WorkerPoolHash(w.Spec.Pools[2], cluster, nil, nil, nil)
						machineClassPool1Zone1 := useDefaultMachineClassWith(defaultMachineClass, map[string]interface{}{
							"availabilityZone": zone1,
							"serverGroupID":    serverGroupID1,
						})
						machineClassPool1Zone2 := useDefaultMachineClassWith(defaultMachineClass, map[string]interface{}{
							"availabilityZone": zone2,
							"serverGroupID":    serverGroupID1,
						})
						machineClassPool2Zone1 := useDefaultMachineClassWith(defaultMachineClass, map[string]interface{}{
							"availabilityZone": zone1,
							"serverGroupID":    serverGroupID2,
						})
						machineClassPool2Zone2 := useDefaultMachineClassWith(defaultMachineClass, map[string]interface{}{
							"availabilityZone": zone2,
							"serverGroupID":    serverGroupID2,
						})
						machineClassPool3Zone1 := useDefaultMachineClassWith(defaultMachineClass, map[string]interface{}{
							"availabilityZone": zone1,
							"serverGroupID":    serverGroupID3,
						})
						machineClassPool3Zone2 := useDefaultMachineClassWith(defaultMachineClass, map[string]interface{}{
							"availabilityZone": zone2,
							"serverGroupID":    serverGroupID3,
						})
						machineClassNamePool1Zone1 := fmt.Sprintf("%s-%s-z1", namespace, namePool1)
						machineClassNamePool1Zone2 := fmt.Sprintf("%s-%s-z2", namespace, namePool1)
						machineClassNamePool2Zone1 := fmt.Sprintf("%s-%s-z1", namespace, namePool2)
						machineClassNamePool2Zone2 := fmt.Sprintf("%s-%s-z2", namespace, namePool2)
						machineClassNamePool3Zone1 := fmt.Sprintf("%s-%s-z1", namespace, namePool3)
						machineClassNamePool3Zone2 := fmt.Sprintf("%s-%s-z2", namespace, namePool3)
						machineClassWithHashPool1Zone1 := fmt.Sprintf("%s-%s", machineClassNamePool1Zone1, workerPoolHash1)
						machineClassWithHashPool1Zone2 := fmt.Sprintf("%s-%s", machineClassNamePool1Zone2, workerPoolHash1)
						machineClassWithHashPool2Zone1 := fmt.Sprintf("%s-%s", machineClassNamePool2Zone1, workerPoolHash2)
						machineClassWithHashPool2Zone2 := fmt.Sprintf("%s-%s", machineClassNamePool2Zone2, workerPoolHash2)
						machineClassWithHashPool3Zone1 := fmt.Sprintf("%s-%s", machineClassNamePool3Zone1, workerPoolHash3)
						machineClassWithHashPool3Zone2 := fmt.Sprintf("%s-%s", machineClassNamePool3Zone2, workerPoolHash3)
						addNameAndSecretToMachineClass(machineClassPool1Zone1, machineClassWithHashPool1Zone1, w.Spec.SecretRef)
						addNameAndSecretToMachineClass(machineClassPool1Zone2, machineClassWithHashPool1Zone2, w.Spec.SecretRef)
						addNameAndSecretToMachineClass(machineClassPool2Zone1, machineClassWithHashPool2Zone1, w.Spec.SecretRef)
						addNameAndSecretToMachineClass(machineClassPool2Zone2, machineClassWithHashPool2Zone2, w.Spec.SecretRef)
						addNameAndSecretToMachineClass(machineClassPool3Zone1, machineClassWithHashPool3Zone1, w.Spec.SecretRef)
						addNameAndSecretToMachineClass(machineClassPool3Zone2, machineClassWithHashPool3Zone2, w.Spec.SecretRef)
						addNodeTemplateToMachineClass(machineClassPool1Zone1, nodeTemplateZone1)
						addNodeTemplateToMachineClass(machineClassPool1Zone2, nodeTemplateZone2)
						addNodeTemplateToMachineClass(machineClassPool2Zone1, nodeTemplateZone1)
						addNodeTemplateToMachineClass(machineClassPool2Zone2, nodeTemplateZone2)
						addNodeTemplateToMachineClass(machineClassPool3Zone1, nodeTemplateZone1)
						addNodeTemplateToMachineClass(machineClassPool3Zone2, nodeTemplateZone2)
						machineClasses := map[string]interface{}{"machineClasses": []map[string]interface{}{
							machineClassPool1Zone1,
							machineClassPool1Zone2,
							machineClassPool2Zone1,
							machineClassPool2Zone2,
							machineClassPool3Zone1,
							machineClassPool3Zone2,
						}}

						expectedUserDataSecretRefRead()

						chartApplier.
							EXPECT().
							ApplyFromEmbeddedFS(
								ctx,
								charts.InternalChart,
								filepath.Join("internal", "machineclass"),
								namespace,
								"machineclass",
								kubernetes.Values(machineClasses),
							).
							Return(nil)

						err := workerDelegate.DeployMachineClasses(ctx)
						Expect(err).NotTo(HaveOccurred())
					})

					It("should fail if the server group dependencies do not exist", func() {
						setup(region, machineImage, "", archAMD)

						workerWithServerGroup := w.DeepCopy()
						workerWithServerGroup.Spec.Pools[0].ProviderConfig = &runtime.RawExtension{
							Object: &apiv1alpha1.WorkerConfig{
								TypeMeta: metav1.TypeMeta{
									Kind:       "WorkerConfig",
									APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
								},
								ServerGroup: &apiv1alpha1.ServerGroup{
									Policy: "policy",
								},
							},
						}

						workerDelegate, _ := NewWorkerDelegate(c, scheme, chartApplier, "", workerWithServerGroup, cluster, nil)
						err := workerDelegate.DeployMachineClasses(ctx)
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(Equal(`server group is required for pool "pool-1", but no server group dependency found`))
					})
				})

				Context("Machine Labels", func() {
					It("should consider rolling machine labels for the worker pool hash", func() {
						setup(region, machineImage, "", archAMD)

						applyLabelsAndPolicy := func(labels []apiv1alpha1.MachineLabel, policy *string) string {
							w.Spec.Pools[0].Labels = utils.MergeStringMaps(w.Spec.Pools[0].Labels, map[string]string{"k1": "v1"})
							workerConfig := &apiv1alpha1.WorkerConfig{
								TypeMeta: metav1.TypeMeta{
									Kind:       "WorkerConfig",
									APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
								},
								MachineLabels: labels,
							}
							if policy != nil {
								workerConfig.ServerGroup = &apiv1alpha1.ServerGroup{Policy: *policy}
								w.Status.ProviderStatus = &runtime.RawExtension{
									Object: &apiv1alpha1.WorkerStatus{
										TypeMeta: metav1.TypeMeta{
											Kind:       "WorkerStatus",
											APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
										},
										ServerGroupDependencies: []apiv1alpha1.ServerGroupDependency{
											{
												PoolName: namePool1,
												Name:     "servergroup1",
												ID:       *policy,
											},
										},
									},
								}
							}
							w.Spec.Pools[0].ProviderConfig = &runtime.RawExtension{
								Raw: encode(workerConfig),
							}
							workerDelegate, _ := NewWorkerDelegate(c, scheme, chartApplier, "", w, cluster, nil)

							expectedUserDataSecretRefRead()

							result, err := workerDelegate.GenerateMachineDeployments(ctx)
							Expect(err).NotTo(HaveOccurred())
							Expect(result[0].Labels).To(HaveKeyWithValue("k1", "v1"))
							return result[0].ClassName
						}

						className0 := applyLabelsAndPolicy(nil, nil)
						className1 := applyLabelsAndPolicy([]apiv1alpha1.MachineLabel{
							{Name: "foo", Value: "bar"},
						}, nil)
						className1b := applyLabelsAndPolicy([]apiv1alpha1.MachineLabel{
							{Name: "foo", Value: "bar2"},
						}, nil)
						className2 := applyLabelsAndPolicy([]apiv1alpha1.MachineLabel{
							{Name: "foo", Value: "bar"},
							{Name: "vmspec/a", Value: "blabla", TriggerRollingOnUpdate: true},
							{Name: "vmspec/c", Value: "rabarber1", TriggerRollingOnUpdate: true},
						}, nil)
						className2b := applyLabelsAndPolicy([]apiv1alpha1.MachineLabel{
							{Name: "vmspec/c", Value: "rabarber1", TriggerRollingOnUpdate: true},
							{Name: "vmspec/b", Value: "abc"},
							{Name: "vmspec/a", Value: "blabla", TriggerRollingOnUpdate: true},
						}, nil)
						className3 := applyLabelsAndPolicy([]apiv1alpha1.MachineLabel{
							{Name: "foo", Value: "bar"},
							{Name: "vmspec/a", Value: "blabla", TriggerRollingOnUpdate: true},
							{Name: "vmspec/c", Value: "rabarber2", TriggerRollingOnUpdate: true},
						}, nil)
						className4 := applyLabelsAndPolicy([]apiv1alpha1.MachineLabel{
							{Name: "foo", Value: "bar"},
							{Name: "vmspec/a", Value: "blabla", TriggerRollingOnUpdate: true},
							{Name: "vmspec/c", Value: "rabarber2", TriggerRollingOnUpdate: false},
						}, nil)

						Expect(className0).To(Equal(className1))
						Expect(className1).To(Equal(className1b))
						Expect(className0).NotTo(Equal(className2))
						Expect(className2).To(Equal(className2b))
						Expect(className0).NotTo(Equal(className3))
						Expect(className2).NotTo(Equal(className3))
						Expect(className3).NotTo(Equal(className4))

						By("with server group policy")
						policy1 := ptr.To("soft-anti-affinity")
						policy2 := ptr.To("foo")
						classNamePolicy01 := applyLabelsAndPolicy(nil, policy1)
						classNamePolicy02 := applyLabelsAndPolicy(nil, policy2)
						classNamePolicy11 := applyLabelsAndPolicy([]apiv1alpha1.MachineLabel{
							{Name: "foo", Value: "bar"},
						}, policy1)
						classNamePolicy21 := applyLabelsAndPolicy([]apiv1alpha1.MachineLabel{
							{Name: "foo", Value: "bar"},
							{Name: "vmspec/a", Value: "blabla", TriggerRollingOnUpdate: true},
							{Name: "vmspec/c", Value: "rabarber1", TriggerRollingOnUpdate: true},
						}, policy1)
						classNamePolicy22 := applyLabelsAndPolicy([]apiv1alpha1.MachineLabel{
							{Name: "foo", Value: "bar"},
							{Name: "vmspec/a", Value: "blabla", TriggerRollingOnUpdate: true},
							{Name: "vmspec/c", Value: "rabarber1", TriggerRollingOnUpdate: true},
						}, policy2)
						classNamePolicy22b := applyLabelsAndPolicy([]apiv1alpha1.MachineLabel{
							{Name: "vmspec/a", Value: "blabla", TriggerRollingOnUpdate: true},
							{Name: "vmspec/c", Value: "rabarber1", TriggerRollingOnUpdate: true},
						}, policy2)

						Expect(className0).NotTo(Equal(classNamePolicy01))
						Expect(className0).NotTo(Equal(classNamePolicy02))
						Expect(classNamePolicy01).NotTo(Equal(classNamePolicy02))
						Expect(classNamePolicy01).To(Equal(classNamePolicy11))
						Expect(classNamePolicy11).NotTo(Equal(classNamePolicy21))
						Expect(classNamePolicy21).NotTo(Equal(classNamePolicy22))
						Expect(classNamePolicy22).To(Equal(classNamePolicy22b))
					})
				})
			})

			It("should fail because the version is invalid", func() {
				clusterWithoutImages.Shoot.Spec.Kubernetes.Version = "invalid"
				workerDelegate, _ = NewWorkerDelegate(c, scheme, chartApplier, "", w, cluster, nil)

				result, err := workerDelegate.GenerateMachineDeployments(ctx)
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should fail because the infrastructure status cannot be decoded", func() {
				w.Spec.InfrastructureProviderStatus = &runtime.RawExtension{}

				workerDelegate, _ = NewWorkerDelegate(c, scheme, chartApplier, "", w, cluster, nil)

				result, err := workerDelegate.GenerateMachineDeployments(ctx)
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should fail because the security group cannot be found", func() {
				w.Spec.InfrastructureProviderStatus = &runtime.RawExtension{
					Raw: encode(&api.InfrastructureStatus{}),
				}

				workerDelegate, _ = NewWorkerDelegate(c, scheme, chartApplier, "", w, cluster, nil)

				result, err := workerDelegate.GenerateMachineDeployments(ctx)
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should fail because the machine image for this cloud profile cannot be found", func() {
				clusterWithoutImages.CloudProfile.Name = "another-cloud-profile"

				workerDelegate, _ = NewWorkerDelegate(c, scheme, chartApplier, "", w, clusterWithoutImages, nil)

				result, err := workerDelegate.GenerateMachineDeployments(ctx)
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should set expected machineControllerManager settings on machine deployment", func() {
				testDrainTimeout := metav1.Duration{Duration: 10 * time.Minute}
				testHealthTimeout := metav1.Duration{Duration: 20 * time.Minute}
				testCreationTimeout := metav1.Duration{Duration: 30 * time.Minute}
				testMaxEvictRetries := int32(30)
				testNodeConditions := []string{"ReadonlyFilesystem", "KernelDeadlock", "DiskPressure"}
				w.Spec.Pools[0].MachineControllerManagerSettings = &gardencorev1beta1.MachineControllerManagerSettings{
					MachineDrainTimeout:    &testDrainTimeout,
					MachineCreationTimeout: &testCreationTimeout,
					MachineHealthTimeout:   &testHealthTimeout,
					MaxEvictRetries:        &testMaxEvictRetries,
					NodeConditions:         testNodeConditions,
				}

				workerDelegate, _ = NewWorkerDelegate(c, scheme, chartApplier, "", w, cluster, nil)

				expectedUserDataSecretRefRead()

				result, err := workerDelegate.GenerateMachineDeployments(ctx)
				resultSettings := result[0].MachineConfiguration
				resultNodeConditions := strings.Join(testNodeConditions, ",")

				Expect(err).NotTo(HaveOccurred())
				Expect(resultSettings.MachineDrainTimeout).To(Equal(&testDrainTimeout))
				Expect(resultSettings.MachineCreationTimeout).To(Equal(&testCreationTimeout))
				Expect(resultSettings.MachineHealthTimeout).To(Equal(&testHealthTimeout))
				Expect(resultSettings.MaxEvictRetries).To(Equal(&testMaxEvictRetries))
				Expect(resultSettings.NodeConditions).To(Equal(&resultNodeConditions))
			})

			It("should set expected cluster-autoscaler annotations on the machine deployment", func() {
				w.Spec.Pools[0].ClusterAutoscaler = &extensionsv1alpha1.ClusterAutoscalerOptions{
					MaxNodeProvisionTime:             ptr.To(metav1.Duration{Duration: time.Minute}),
					ScaleDownGpuUtilizationThreshold: ptr.To("0.4"),
					ScaleDownUnneededTime:            ptr.To(metav1.Duration{Duration: 2 * time.Minute}),
					ScaleDownUnreadyTime:             ptr.To(metav1.Duration{Duration: 3 * time.Minute}),
					ScaleDownUtilizationThreshold:    ptr.To("0.5"),
				}
				w.Spec.Pools[1].ClusterAutoscaler = nil
				workerDelegate, _ = NewWorkerDelegate(c, scheme, chartApplier, "", w, cluster, nil)

				expectedUserDataSecretRefRead()

				result, err := workerDelegate.GenerateMachineDeployments(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())

				Expect(result[0].ClusterAutoscalerAnnotations).NotTo(BeNil())
				Expect(result[1].ClusterAutoscalerAnnotations).NotTo(BeNil())
				for k, v := range result[2].ClusterAutoscalerAnnotations {
					Expect(v).To(BeEmpty(), "entry for key %v is not empty", k)
				}
				for k, v := range result[3].ClusterAutoscalerAnnotations {
					Expect(v).To(BeEmpty(), "entry for key %v is not empty", k)
				}

				Expect(result[0].ClusterAutoscalerAnnotations[extensionsv1alpha1.MaxNodeProvisionTimeAnnotation]).To(Equal("1m0s"))
				Expect(result[0].ClusterAutoscalerAnnotations[extensionsv1alpha1.ScaleDownGpuUtilizationThresholdAnnotation]).To(Equal("0.4"))
				Expect(result[0].ClusterAutoscalerAnnotations[extensionsv1alpha1.ScaleDownUnneededTimeAnnotation]).To(Equal("2m0s"))
				Expect(result[0].ClusterAutoscalerAnnotations[extensionsv1alpha1.ScaleDownUnreadyTimeAnnotation]).To(Equal("3m0s"))
				Expect(result[0].ClusterAutoscalerAnnotations[extensionsv1alpha1.ScaleDownUtilizationThresholdAnnotation]).To(Equal("0.5"))

				Expect(result[1].ClusterAutoscalerAnnotations[extensionsv1alpha1.MaxNodeProvisionTimeAnnotation]).To(Equal("1m0s"))
				Expect(result[1].ClusterAutoscalerAnnotations[extensionsv1alpha1.ScaleDownGpuUtilizationThresholdAnnotation]).To(Equal("0.4"))
				Expect(result[1].ClusterAutoscalerAnnotations[extensionsv1alpha1.ScaleDownUnneededTimeAnnotation]).To(Equal("2m0s"))
				Expect(result[1].ClusterAutoscalerAnnotations[extensionsv1alpha1.ScaleDownUnreadyTimeAnnotation]).To(Equal("3m0s"))
				Expect(result[1].ClusterAutoscalerAnnotations[extensionsv1alpha1.ScaleDownUtilizationThresholdAnnotation]).To(Equal("0.5"))
			})
		})
	})
})

func encode(obj runtime.Object) []byte {
	data, _ := json.Marshal(obj)
	return data
}

// nolint:unparam
func useDefaultMachineClass(def map[string]interface{}, key string, value interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(def)+1)

	for k, v := range def {
		out[k] = v
	}

	out[key] = value
	return out
}

func useDefaultMachineClassWith(def map[string]interface{}, add map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(add))

	for k, v := range def {
		out[k] = v
	}

	for k, v := range add {
		out[k] = v
	}

	return out
}

func addNodeTemplateToMachineClass(class map[string]interface{}, nodeTemplate machinev1alpha1.NodeTemplate) {
	class["nodeTemplate"] = nodeTemplate
}

func addNameAndSecretToMachineClass(class map[string]interface{}, name string, credentialsSecretRef corev1.SecretReference) {
	class["name"] = name
	class["labels"] = map[string]string{
		v1beta1constants.GardenerPurpose: v1beta1constants.GardenPurposeMachineClass,
	}
	class["credentialsSecretRef"] = map[string]interface{}{
		"name":      credentialsSecretRef.Name,
		"namespace": credentialsSecretRef.Namespace,
	}
}
