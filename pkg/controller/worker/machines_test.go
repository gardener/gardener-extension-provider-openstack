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

package worker_test

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	apiv1alpha1 "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"
	. "github.com/gardener/gardener-extension-provider-openstack/pkg/controller/worker"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/common"
	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	genericworkeractuator "github.com/gardener/gardener/extensions/pkg/controller/worker/genericactuator"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	mockkubernetes "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
)

var _ = Describe("Machines", func() {
	var (
		ctrl         *gomock.Controller
		c            *mockclient.MockClient
		statusWriter *mockclient.MockStatusWriter
		chartApplier *mockkubernetes.MockChartApplier
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		c = mockclient.NewMockClient(ctrl)
		statusWriter = mockclient.NewMockStatusWriter(ctrl)
		chartApplier = mockkubernetes.NewMockChartApplier(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("workerDelegate", func() {
		workerDelegate, _ := NewWorkerDelegate(common.NewClientContext(nil, nil, nil), nil, "", nil, nil, nil, nil)

		Describe("#MachineClassKind", func() {
			It("should return the correct kind of the machine class", func() {
				Expect(workerDelegate.MachineClassKind()).To(Equal("MachineClass"))
			})
		})

		Describe("#MachineClassList", func() {
			It("should return the correct type for the machine class list", func() {
				Expect(workerDelegate.MachineClassList()).To(Equal(&machinev1alpha1.MachineClassList{}))
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

				keyName           string
				machineType       string
				userData          []byte
				networkID         string
				podCIDR           string
				subnetID          string
				securityGroupName string

				namePool1           string
				minPool1            int32
				maxPool1            int32
				maxSurgePool1       intstr.IntOrString
				maxUnavailablePool1 intstr.IntOrString

				namePool2           string
				minPool2            int32
				maxPool2            int32
				maxSurgePool2       intstr.IntOrString
				maxUnavailablePool2 intstr.IntOrString

				zone1 string
				zone2 string

				nodeCapacity         corev1.ResourceList
				nodeTemplateZone1    machinev1alpha1.NodeTemplate
				nodeTemplateZone2    machinev1alpha1.NodeTemplate
				machineConfiguration *machinev1alpha1.MachineConfiguration

				workerPoolHash1 string
				workerPoolHash2 string

				shootVersionMajorMinor string
				shootVersion           string
				scheme                 *runtime.Scheme
				decoder                runtime.Decoder
				openstackSecretRef     *corev1.SecretReference
				cloudProfileConfig     *api.CloudProfileConfig
				cloudProfileConfigJSON []byte
				clusterWithoutImages   *extensionscontroller.Cluster
				cluster                *extensionscontroller.Cluster
				w                      *extensionsv1alpha1.Worker
			)

			BeforeEach(func() {
				namespace = "shoot--foobar--openstack"
				cloudProfileName = "openstack"

				region = "eu-de-1"
				regionWithImages = "eu-de-2"

				openstackAuthURL = "auth-url"

				machineImageName = "my-os"
				machineImageVersion = "123"
				machineImage = "my-image-in-glance"
				machineImageID = "my-image-id"

				keyName = "key-name"
				machineType = "large"
				userData = []byte("some-user-data")
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
				maxSurgePool2 = intstr.FromInt(10)
				maxUnavailablePool2 = intstr.FromInt(15)

				zone1 = region + "a"
				zone2 = region + "b"

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
				}

				nodeTemplateZone2 = machinev1alpha1.NodeTemplate{
					Capacity:     nodeCapacity,
					InstanceType: machineType,
					Region:       region,
					Zone:         zone2,
				}

				machineConfiguration = &machinev1alpha1.MachineConfiguration{}

				shootVersionMajorMinor = "1.2"
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
							Networking: gardencorev1beta1.Networking{
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
										Name: regionWithImages,
										ID:   machineImageID,
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
								MachineImage: extensionsv1alpha1.MachineImage{
									Name:    machineImageName,
									Version: machineImageVersion,
								},
								NodeTemplate: &extensionsv1alpha1.NodeTemplate{
									Capacity: nodeCapacity,
								},
								UserData: userData,
								Zones: []string{
									zone1,
									zone2,
								},
							},
							{
								Name:           namePool2,
								Minimum:        minPool2,
								Maximum:        maxPool2,
								MaxSurge:       maxSurgePool2,
								MaxUnavailable: maxUnavailablePool2,
								MachineType:    machineType,
								MachineImage: extensionsv1alpha1.MachineImage{
									Name:    machineImageName,
									Version: machineImageVersion,
								},
								NodeTemplate: &extensionsv1alpha1.NodeTemplate{
									Capacity: nodeCapacity,
								},
								UserData: userData,
								Zones: []string{
									zone1,
									zone2,
								},
							},
						},
					},
				}

				scheme = runtime.NewScheme()
				_ = api.AddToScheme(scheme)
				_ = apiv1alpha1.AddToScheme(scheme)
				decoder = serializer.NewCodecFactory(scheme, serializer.EnableStrict).UniversalDecoder()

				workerPoolHash1, _ = worker.WorkerPoolHash(w.Spec.Pools[0], cluster)
				workerPoolHash2, _ = worker.WorkerPoolHash(w.Spec.Pools[1], cluster)

				openstackSecretRef = &corev1.SecretReference{
					Name:      "secret",
					Namespace: namespace,
				}

				workerDelegate, _ = NewWorkerDelegate(common.NewClientContext(c, scheme, decoder), chartApplier, "", w, clusterWithoutImages, nil, openstackSecretRef)
			})

			Describe("machine images", func() {
				var (
					defaultMachineClass map[string]interface{}
					machineDeployments  worker.MachineDeployments
					machineClasses      map[string]interface{}
					workerWithRegion    *extensionsv1alpha1.Worker
					clusterWithRegion   *extensionscontroller.Cluster
				)

				setup := func(region, name, imageID string) {
					workerWithRegion = w.DeepCopy()
					workerWithRegion.Spec.Region = region

					clusterWithRegion = &extensionscontroller.Cluster{
						CloudProfile: cluster.CloudProfile,
						Shoot:        cluster.Shoot.DeepCopy(),
						Seed:         cluster.Seed,
					}
					clusterWithRegion.Shoot.Spec.Region = region

					defaultMachineClass = map[string]interface{}{
						"region":         region,
						"machineType":    machineType,
						"keyName":        keyName,
						"networkID":      networkID,
						"subnetID":       subnetID,
						"podNetworkCidr": podCIDR,
						"securityGroups": []string{securityGroupName},
						"tags": map[string]string{
							fmt.Sprintf("kubernetes.io-cluster-%s", namespace): "1",
							"kubernetes.io-role-node":                          "1",
						},
						"secret": map[string]interface{}{
							"cloudConfig": string(userData),
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
					}

					newNodeTemplateZone2 := machinev1alpha1.NodeTemplate{
						Capacity:     nodeCapacity,
						InstanceType: machineType,
						Region:       region,
						Zone:         zone2,
					}

					var (
						machineClassPool1Zone1 = useDefaultMachineClass(defaultMachineClass, "availabilityZone", zone1)
						machineClassPool1Zone2 = useDefaultMachineClass(defaultMachineClass, "availabilityZone", zone2)
						machineClassPool2Zone1 = useDefaultMachineClass(defaultMachineClass, "availabilityZone", zone1)
						machineClassPool2Zone2 = useDefaultMachineClass(defaultMachineClass, "availabilityZone", zone2)

						machineClassNamePool1Zone1 = fmt.Sprintf("%s-%s-z1", namespace, namePool1)
						machineClassNamePool1Zone2 = fmt.Sprintf("%s-%s-z2", namespace, namePool1)
						machineClassNamePool2Zone1 = fmt.Sprintf("%s-%s-z1", namespace, namePool2)
						machineClassNamePool2Zone2 = fmt.Sprintf("%s-%s-z2", namespace, namePool2)

						machineClassWithHashPool1Zone1 = fmt.Sprintf("%s-%s", machineClassNamePool1Zone1, workerPoolHash1)
						machineClassWithHashPool1Zone2 = fmt.Sprintf("%s-%s", machineClassNamePool1Zone2, workerPoolHash1)
						machineClassWithHashPool2Zone1 = fmt.Sprintf("%s-%s", machineClassNamePool2Zone1, workerPoolHash2)
						machineClassWithHashPool2Zone2 = fmt.Sprintf("%s-%s", machineClassNamePool2Zone2, workerPoolHash2)
					)

					addNameAndSecretToMachineClass(machineClassPool1Zone1, machineClassWithHashPool1Zone1, openstackSecretRef)
					addNameAndSecretToMachineClass(machineClassPool1Zone2, machineClassWithHashPool1Zone2, openstackSecretRef)
					addNameAndSecretToMachineClass(machineClassPool2Zone1, machineClassWithHashPool2Zone1, openstackSecretRef)
					addNameAndSecretToMachineClass(machineClassPool2Zone2, machineClassWithHashPool2Zone2, openstackSecretRef)

					addNodeTemplateToMachineClass(machineClassPool1Zone1, newNodeTemplateZone1)
					addNodeTemplateToMachineClass(machineClassPool1Zone2, newNodeTemplateZone2)
					addNodeTemplateToMachineClass(machineClassPool2Zone1, newNodeTemplateZone1)
					addNodeTemplateToMachineClass(machineClassPool2Zone2, newNodeTemplateZone2)

					machineClasses = map[string]interface{}{"machineClasses": []map[string]interface{}{
						machineClassPool1Zone1,
						machineClassPool1Zone2,
						machineClassPool2Zone1,
						machineClassPool2Zone2,
					}}

					machineDeployments = worker.MachineDeployments{
						{
							Name:                 machineClassNamePool1Zone1,
							ClassName:            machineClassWithHashPool1Zone1,
							SecretName:           machineClassWithHashPool1Zone1,
							Minimum:              worker.DistributeOverZones(0, minPool1, 2),
							Maximum:              worker.DistributeOverZones(0, maxPool1, 2),
							MaxSurge:             worker.DistributePositiveIntOrPercent(0, maxSurgePool1, 2, maxPool1),
							MaxUnavailable:       worker.DistributePositiveIntOrPercent(0, maxUnavailablePool1, 2, minPool1),
							MachineConfiguration: machineConfiguration,
						},
						{
							Name:                 machineClassNamePool1Zone2,
							ClassName:            machineClassWithHashPool1Zone2,
							SecretName:           machineClassWithHashPool1Zone2,
							Minimum:              worker.DistributeOverZones(1, minPool1, 2),
							Maximum:              worker.DistributeOverZones(1, maxPool1, 2),
							MaxSurge:             worker.DistributePositiveIntOrPercent(1, maxSurgePool1, 2, maxPool1),
							MaxUnavailable:       worker.DistributePositiveIntOrPercent(1, maxUnavailablePool1, 2, minPool1),
							MachineConfiguration: machineConfiguration,
						},
						{
							Name:                 machineClassNamePool2Zone1,
							ClassName:            machineClassWithHashPool2Zone1,
							SecretName:           machineClassWithHashPool2Zone1,
							Minimum:              worker.DistributeOverZones(0, minPool2, 2),
							Maximum:              worker.DistributeOverZones(0, maxPool2, 2),
							MaxSurge:             worker.DistributePositiveIntOrPercent(0, maxSurgePool2, 2, maxPool2),
							MaxUnavailable:       worker.DistributePositiveIntOrPercent(0, maxUnavailablePool2, 2, minPool2),
							MachineConfiguration: machineConfiguration,
						},
						{
							Name:                 machineClassNamePool2Zone2,
							ClassName:            machineClassWithHashPool2Zone2,
							SecretName:           machineClassWithHashPool2Zone2,
							Minimum:              worker.DistributeOverZones(1, minPool2, 2),
							Maximum:              worker.DistributeOverZones(1, maxPool2, 2),
							MaxSurge:             worker.DistributePositiveIntOrPercent(1, maxSurgePool2, 2, maxPool2),
							MaxUnavailable:       worker.DistributePositiveIntOrPercent(1, maxUnavailablePool2, 2, minPool2),
							MachineConfiguration: machineConfiguration,
						},
					}
				}

				It("should return the expected machine deployments for profile image types", func() {
					setup(region, machineImage, "")
					workerDelegate, _ := NewWorkerDelegate(common.NewClientContext(c, scheme, decoder), chartApplier, "", w, cluster, nil, openstackSecretRef)

					// Test workerDelegate.DeployMachineClasses()
					chartApplier.
						EXPECT().
						Apply(
							context.TODO(),
							filepath.Join(openstack.InternalChartsPath, "machineclass"),
							namespace,
							"machineclass",
							kubernetes.Values(machineClasses),
						).
						Return(nil)

					err := workerDelegate.DeployMachineClasses(context.TODO())
					Expect(err).NotTo(HaveOccurred())

					// Test workerDelegate.UpdateMachineDeployments()

					expectedImages := &apiv1alpha1.WorkerStatus{
						TypeMeta: metav1.TypeMeta{
							APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
							Kind:       "WorkerStatus",
						},
						MachineImages: []apiv1alpha1.MachineImage{
							{
								Name:    machineImageName,
								Version: machineImageVersion,
								Image:   machineImage,
							},
						},
					}

					workerWithExpectedImages := w.DeepCopy()
					workerWithExpectedImages.Status.ProviderStatus = &runtime.RawExtension{
						Object: expectedImages,
					}

					ctx := context.TODO()
					c.EXPECT().Status().Return(statusWriter)
					statusWriter.EXPECT().Patch(ctx, workerWithExpectedImages, gomock.Any()).Return(nil)

					err = workerDelegate.UpdateMachineImagesStatus(ctx)
					Expect(err).NotTo(HaveOccurred())

					// Test workerDelegate.GenerateMachineDeployments()

					result, err := workerDelegate.GenerateMachineDeployments(context.TODO())
					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(Equal(machineDeployments))
				})

				It("should return the expected machine deployments for profile image types with id", func() {
					setup(regionWithImages, "", machineImageID)
					workerDelegate, _ := NewWorkerDelegate(common.NewClientContext(c, scheme, decoder), chartApplier, "", workerWithRegion, clusterWithRegion, nil, openstackSecretRef)
					clusterWithRegion.Shoot.Spec.Hibernation = &gardencorev1beta1.Hibernation{Enabled: pointer.BoolPtr(true)}

					// Test workerDelegate.DeployMachineClasses()

					chartApplier.
						EXPECT().
						Apply(
							context.TODO(),
							filepath.Join(openstack.InternalChartsPath, "machineclass"),
							namespace,
							"machineclass",
							kubernetes.Values(machineClasses),
						).
						Return(nil)

					err := workerDelegate.DeployMachineClasses(context.TODO())
					Expect(err).NotTo(HaveOccurred())

					// Test workerDelegate.GetMachineImages()
					expectedImages := &apiv1alpha1.WorkerStatus{
						TypeMeta: metav1.TypeMeta{
							APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
							Kind:       "WorkerStatus",
						},
						MachineImages: []apiv1alpha1.MachineImage{
							{
								Name:    machineImageName,
								Version: machineImageVersion,
								ID:      machineImageID,
							},
						},
					}

					workerWithExpectedImages := workerWithRegion.DeepCopy()
					workerWithExpectedImages.Status.ProviderStatus = &runtime.RawExtension{
						Object: expectedImages,
					}

					ctx := context.TODO()
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
							serverGroupID1   = "id1"
							serverGroupID2   = "id2"
						)

						setup(region, machineImage, "")

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
								},
							},
						}

						workerDelegate, _ := NewWorkerDelegate(common.NewClientContext(c, scheme, decoder), chartApplier, "", workerWithServerGroup, cluster, nil, openstackSecretRef)

						// Test workerDelegate.DeployMachineClasses()
						workerPoolHash1, _ := worker.WorkerPoolHash(w.Spec.Pools[0], cluster, serverGroupID1)
						workerPoolHash2, _ := worker.WorkerPoolHash(w.Spec.Pools[1], cluster, serverGroupID2)
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
						machineClassNamePool1Zone1 := fmt.Sprintf("%s-%s-z1", namespace, namePool1)
						machineClassNamePool1Zone2 := fmt.Sprintf("%s-%s-z2", namespace, namePool1)
						machineClassNamePool2Zone1 := fmt.Sprintf("%s-%s-z1", namespace, namePool2)
						machineClassNamePool2Zone2 := fmt.Sprintf("%s-%s-z2", namespace, namePool2)
						machineClassWithHashPool1Zone1 := fmt.Sprintf("%s-%s", machineClassNamePool1Zone1, workerPoolHash1)
						machineClassWithHashPool1Zone2 := fmt.Sprintf("%s-%s", machineClassNamePool1Zone2, workerPoolHash1)
						machineClassWithHashPool2Zone1 := fmt.Sprintf("%s-%s", machineClassNamePool2Zone1, workerPoolHash2)
						machineClassWithHashPool2Zone2 := fmt.Sprintf("%s-%s", machineClassNamePool2Zone2, workerPoolHash2)
						addNameAndSecretToMachineClass(machineClassPool1Zone1, machineClassWithHashPool1Zone1, openstackSecretRef)
						addNameAndSecretToMachineClass(machineClassPool1Zone2, machineClassWithHashPool1Zone2, openstackSecretRef)
						addNameAndSecretToMachineClass(machineClassPool2Zone1, machineClassWithHashPool2Zone1, openstackSecretRef)
						addNameAndSecretToMachineClass(machineClassPool2Zone2, machineClassWithHashPool2Zone2, openstackSecretRef)
						addNodeTemplateToMachineClass(machineClassPool1Zone1, nodeTemplateZone1)
						addNodeTemplateToMachineClass(machineClassPool1Zone2, nodeTemplateZone2)
						addNodeTemplateToMachineClass(machineClassPool2Zone1, nodeTemplateZone1)
						addNodeTemplateToMachineClass(machineClassPool2Zone2, nodeTemplateZone2)

						machineClasses := map[string]interface{}{"machineClasses": []map[string]interface{}{
							machineClassPool1Zone1,
							machineClassPool1Zone2,
							machineClassPool2Zone1,
							machineClassPool2Zone2,
						}}

						chartApplier.
							EXPECT().
							Apply(
								context.TODO(),
								filepath.Join(openstack.InternalChartsPath, "machineclass"),
								namespace,
								"machineclass",
								kubernetes.Values(machineClasses),
							).
							Return(nil)

						err := workerDelegate.DeployMachineClasses(context.TODO())
						Expect(err).NotTo(HaveOccurred())
					})
				})

				It("should fail if the server group dependencies do not exist", func() {
					setup(region, machineImage, "")

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

					workerDelegate, _ := NewWorkerDelegate(common.NewClientContext(c, scheme, decoder), chartApplier, "", workerWithServerGroup, cluster, nil, openstackSecretRef)
					err := workerDelegate.DeployMachineClasses(context.TODO())
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal(`server group is required for pool "pool-1", but no server group dependency found`))
				})
			})

			It("should fail because the version is invalid", func() {
				clusterWithoutImages.Shoot.Spec.Kubernetes.Version = "invalid"
				workerDelegate, _ = NewWorkerDelegate(common.NewClientContext(c, scheme, decoder), chartApplier, "", w, cluster, nil, openstackSecretRef)

				result, err := workerDelegate.GenerateMachineDeployments(context.TODO())
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should fail because the infrastructure status cannot be decoded", func() {
				w.Spec.InfrastructureProviderStatus = &runtime.RawExtension{}

				workerDelegate, _ = NewWorkerDelegate(common.NewClientContext(c, scheme, decoder), chartApplier, "", w, cluster, nil, openstackSecretRef)

				result, err := workerDelegate.GenerateMachineDeployments(context.TODO())
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should fail because the security group cannot be found", func() {
				w.Spec.InfrastructureProviderStatus = &runtime.RawExtension{
					Raw: encode(&api.InfrastructureStatus{}),
				}

				workerDelegate, _ = NewWorkerDelegate(common.NewClientContext(c, scheme, decoder), chartApplier, "", w, cluster, nil, openstackSecretRef)

				result, err := workerDelegate.GenerateMachineDeployments(context.TODO())
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should fail because the machine image for this cloud profile cannot be found", func() {
				clusterWithoutImages.CloudProfile.Name = "another-cloud-profile"

				workerDelegate, _ = NewWorkerDelegate(common.NewClientContext(c, scheme, decoder), chartApplier, "", w, clusterWithoutImages, nil, openstackSecretRef)

				result, err := workerDelegate.GenerateMachineDeployments(context.TODO())
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

				workerDelegate, _ = NewWorkerDelegate(common.NewClientContext(c, scheme, decoder), chartApplier, "", w, cluster, nil, openstackSecretRef)

				result, err := workerDelegate.GenerateMachineDeployments(context.TODO())
				resultSettings := result[0].MachineConfiguration
				resultNodeConditions := strings.Join(testNodeConditions, ",")

				Expect(err).NotTo(HaveOccurred())
				Expect(resultSettings.MachineDrainTimeout).To(Equal(&testDrainTimeout))
				Expect(resultSettings.MachineCreationTimeout).To(Equal(&testCreationTimeout))
				Expect(resultSettings.MachineHealthTimeout).To(Equal(&testHealthTimeout))
				Expect(resultSettings.MaxEvictRetries).To(Equal(&testMaxEvictRetries))
				Expect(resultSettings.NodeConditions).To(Equal(&resultNodeConditions))
			})
		})
	})
})

func encode(obj runtime.Object) []byte {
	data, _ := json.Marshal(obj)
	return data
}

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

func addNameAndSecretToMachineClass(class map[string]interface{}, name string, credentialsSecretRef *corev1.SecretReference) {
	class["name"] = name
	class["labels"] = map[string]string{
		v1beta1constants.GardenerPurpose: genericworkeractuator.GardenPurposeMachineClass,
	}
	class["credentialsSecretRef"] = map[string]interface{}{
		"name":      credentialsSecretRef.Name,
		"namespace": credentialsSecretRef.Namespace,
	}
}
