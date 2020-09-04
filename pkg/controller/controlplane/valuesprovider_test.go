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
	"context"
	"encoding/json"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/controlplane/genericactuator"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/utils"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

const (
	namespace = "test"
	authURL   = "someurl"
	region    = "europe"
)

var (
	dhcpDomain     = pointer.StringPtr("dhcp-domain")
	requestTimeout = pointer.StringPtr("2s")
)

func defaultControlPlane() *extensionsv1alpha1.ControlPlane {
	return controlPlane(
		"floating-network-id",
		&api.ControlPlaneConfig{
			LoadBalancerProvider: "load-balancer-provider",
			CloudControllerManager: &api.CloudControllerManagerConfig{
				FeatureGates: map[string]bool{
					"CustomResourceValidation": true,
				},
			},
		})
}

func controlPlane(floatingPoolID string, cfg *api.ControlPlaneConfig) *extensionsv1alpha1.ControlPlane {
	return &extensionsv1alpha1.ControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "control-plane",
			Namespace: namespace,
		},
		Spec: extensionsv1alpha1.ControlPlaneSpec{
			SecretRef: corev1.SecretReference{
				Name:      v1beta1constants.SecretNameCloudProvider,
				Namespace: namespace,
			},
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				ProviderConfig: &runtime.RawExtension{
					Raw: encode(cfg),
				},
			},
			Region: region,
			InfrastructureProviderStatus: &runtime.RawExtension{
				Raw: encode(&api.InfrastructureStatus{
					Networks: api.NetworkStatus{
						FloatingPool: api.FloatingPoolStatus{
							ID: floatingPoolID,
						},
						Subnets: []api.Subnet{
							{
								ID:      "subnet-acbd1234",
								Purpose: api.PurposeNodes,
							},
						},
					},
				}),
			},
		},
	}
}

var _ = Describe("ValuesProvider", func() {
	var (
		ctx = context.TODO()

		ctrl *gomock.Controller

		scheme = runtime.NewScheme()
		_      = api.AddToScheme(scheme)

		vp genericactuator.ValuesProvider
		c  *mockclient.MockClient

		cp = defaultControlPlane()

		cidr                       = "10.250.0.0/19"
		useOctavia                 = true
		rescanBlockStorageOnResize = true

		cloudProfileConfig = &api.CloudProfileConfig{
			KeyStoneURL:                authURL,
			DHCPDomain:                 dhcpDomain,
			RequestTimeout:             requestTimeout,
			UseOctavia:                 pointer.BoolPtr(useOctavia),
			RescanBlockStorageOnResize: pointer.BoolPtr(rescanBlockStorageOnResize),
		}
		cloudProfileConfigJSON, _ = json.Marshal(cloudProfileConfig)

		clusterK8sLessThan119 = &extensionscontroller.Cluster{
			CloudProfile: &gardencorev1beta1.CloudProfile{
				Spec: gardencorev1beta1.CloudProfileSpec{
					ProviderConfig: &runtime.RawExtension{
						Raw: cloudProfileConfigJSON,
					},
				},
			},
			Shoot: &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Networking: gardencorev1beta1.Networking{
						Pods: &cidr,
					},
					Kubernetes: gardencorev1beta1.Kubernetes{
						Version: "1.13.4",
					},
				},
			},
		}
		clusterK8sAtLeast119 = &extensionscontroller.Cluster{
			CloudProfile: &gardencorev1beta1.CloudProfile{
				Spec: gardencorev1beta1.CloudProfileSpec{
					ProviderConfig: &runtime.RawExtension{
						Raw: cloudProfileConfigJSON,
					},
				},
			},
			Shoot: &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Networking: gardencorev1beta1.Networking{
						Pods: &cidr,
					},
					Kubernetes: gardencorev1beta1.Kubernetes{
						Version: "1.19.4",
						VerticalPodAutoscaler: &gardencorev1beta1.VerticalPodAutoscaler{
							Enabled: true,
						},
					},
				},
			},
		}

		cpSecretKey = client.ObjectKey{Namespace: namespace, Name: v1beta1constants.SecretNameCloudProvider}
		cpSecret    = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1beta1constants.SecretNameCloudProvider,
				Namespace: namespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"domainName": []byte(`domain-name`),
				"tenantName": []byte(`tenant-name`),
				"username":   []byte(`username`),
				"password":   []byte(`password`),
			},
		}

		cpConfigKey = client.ObjectKey{Namespace: namespace, Name: openstack.CloudProviderConfigName}
		cpConfig    = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      openstack.CloudProviderConfigName,
				Namespace: namespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				openstack.CloudProviderConfigDataKey: []byte("some data"),
			},
		}

		cloudProviderDiskConfig = []byte("foo")
		cpDiskConfigKey         = client.ObjectKey{Namespace: namespace, Name: openstack.CloudProviderDiskConfigName}
		cpDiskConfig            = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      openstack.CloudProviderDiskConfigName,
				Namespace: namespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				openstack.CloudProviderConfigDataKey: cloudProviderDiskConfig,
			},
		}

		checksums = map[string]string{
			v1beta1constants.SecretNameCloudProvider:         "8bafb35ff1ac60275d62e1cbd495aceb511fb354f74a20f7d06ecb48b3a68432",
			openstack.CloudControllerManagerName:             "3d791b164a808638da9a8df03924be2a41e34cd664e42231c00fe369e3588272",
			openstack.CloudControllerManagerName + "-server": "6dff2a2e6f14444b66d8e4a351c049f7e89ee24ba3eaab95dbec40ba6bdebb52",
			openstack.CloudProviderConfigName:                "bf19236c3ff3be18cf28cb4f58532bda4fd944857dd163baa05d23f952550392",
			openstack.CSIProvisionerName:                     "65b1dac6b50673535cff480564c2e5c71077ed19b1b6e0e2291207225bdf77d4",
			openstack.CSIAttacherName:                        "3f22909841cdbb80e5382d689d920309c0a7d995128e52c79773f9608ed7c289",
			openstack.CSISnapshotterName:                     "6a5bfc847638c499062f7fb44e31a30a9760bf4179e1dbf85e0ff4b4f162cd68",
			openstack.CSIResizerName:                         "a77e663ba1af340fb3dd7f6f8a1be47c7aa9e658198695480641e6b934c0b9ed",
			openstack.CSISnapshotControllerName:              "84cba346d2e2cf96c3811b55b01f57bdd9b9bcaed7065760470942d267984eaf",
		}

		enabledTrue  = map[string]interface{}{"enabled": true}
		enabledFalse = map[string]interface{}{"enabled": false}

		logger = log.Log.WithName("test")
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		c = mockclient.NewMockClient(ctrl)

		vp = NewValuesProvider(logger)
		err := vp.(inject.Scheme).InjectScheme(scheme)
		Expect(err).NotTo(HaveOccurred())
		err = vp.(inject.Client).InjectClient(c)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#GetConfigChartValues", func() {
		configChartValues := map[string]interface{}{
			"kubernetesVersion":          "1.13.4",
			"domainName":                 "domain-name",
			"tenantName":                 "tenant-name",
			"username":                   "username",
			"password":                   "password",
			"region":                     region,
			"subnetID":                   "subnet-acbd1234",
			"lbProvider":                 "load-balancer-provider",
			"floatingNetworkID":          "floating-network-id",
			"authUrl":                    authURL,
			"dhcpDomain":                 dhcpDomain,
			"requestTimeout":             requestTimeout,
			"useOctavia":                 useOctavia,
			"rescanBlockStorageOnResize": rescanBlockStorageOnResize,
		}

		It("should return correct config chart values", func() {
			c.EXPECT().Get(ctx, cpSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpSecret))

			values, err := vp.GetConfigChartValues(ctx, cp, clusterK8sLessThan119)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(configChartValues))
		})

		It("should return correct config chart values with load balancer classes", func() {
			c.EXPECT().Get(ctx, cpSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpSecret))

			var (
				floatingNetworkID = "4711"
				fsid              = "0815"
				floatingSubnetID2 = "pub0815"
				subnetID          = "priv"
				floatingSubnetID  = "default-floating-subnet-id"
				cp                = controlPlane(
					floatingNetworkID,
					&api.ControlPlaneConfig{
						LoadBalancerProvider: "load-balancer-provider",
						LoadBalancerClasses: []api.LoadBalancerClass{
							{
								Name:             "test",
								FloatingSubnetID: &fsid,
								SubnetID:         nil,
							},
							{
								Name:             "default",
								FloatingSubnetID: &floatingSubnetID,
								SubnetID:         nil,
							},
							{
								Name:             "public",
								FloatingSubnetID: &floatingSubnetID2,
								SubnetID:         nil,
							},
							{
								Name:     "other",
								SubnetID: &subnetID,
							},
						},
						CloudControllerManager: &api.CloudControllerManagerConfig{
							FeatureGates: map[string]bool{
								"CustomResourceValidation": true,
							},
						},
					},
				)

				configValues = utils.MergeMaps(configChartValues, map[string]interface{}{
					"floatingNetworkID": floatingNetworkID,
					"floatingSubnetID":  floatingSubnetID,
					"floatingClasses": []map[string]interface{}{
						{
							"name":              "test",
							"floatingNetworkID": floatingNetworkID,
							"floatingSubnetID":  fsid,
						},
						{
							"name":              "default",
							"floatingNetworkID": floatingNetworkID,
							"floatingSubnetID":  floatingSubnetID,
						},
						{
							"name":              "public",
							"floatingNetworkID": floatingNetworkID,
							"floatingSubnetID":  floatingSubnetID2,
						},
						{
							"name":     "other",
							"subnetID": subnetID,
						},
					},
				})
			)

			values, err := vp.GetConfigChartValues(ctx, cp, clusterK8sLessThan119)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(configValues))
		})
	})

	Describe("#GetControlPlaneChartValues", func() {
		ccmChartValues := utils.MergeMaps(enabledTrue, map[string]interface{}{
			"replicas":          1,
			"kubernetesVersion": "1.13.4",
			"clusterName":       namespace,
			"podNetwork":        cidr,
			"podAnnotations": map[string]interface{}{
				"checksum/secret-" + openstack.CloudControllerManagerName:             checksums[openstack.CloudControllerManagerName],
				"checksum/secret-" + openstack.CloudControllerManagerName + "-server": checksums[openstack.CloudControllerManagerName+"-server"],
				"checksum/secret-" + v1beta1constants.SecretNameCloudProvider:         checksums[v1beta1constants.SecretNameCloudProvider],
				"checksum/secret-" + openstack.CloudProviderConfigName:                checksums[openstack.CloudProviderConfigName],
			},
			"podLabels": map[string]interface{}{
				"maintenance.gardener.cloud/restart": "true",
			},
			"featureGates": map[string]bool{
				"CustomResourceValidation": true,
			},
		})

		BeforeEach(func() {
			c.EXPECT().Get(ctx, cpConfigKey, &corev1.Secret{}).DoAndReturn(clientGet(cpConfig))
			c.EXPECT().Get(ctx, cpDiskConfigKey, &corev1.Secret{}).DoAndReturn(clientGet(cpDiskConfig))
			c.EXPECT().Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cloud-provider-config-cloud-controller-manager", Namespace: namespace}})
			c.EXPECT().Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cloud-provider-config-kube-controller-manager", Namespace: namespace}})
		})

		It("should return correct control plane chart values (k8s < 1.19)", func() {
			values, err := vp.GetControlPlaneChartValues(ctx, cp, clusterK8sLessThan119, checksums, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{
				openstack.CloudControllerManagerName: utils.MergeMaps(ccmChartValues, map[string]interface{}{
					"kubernetesVersion": clusterK8sLessThan119.Shoot.Spec.Kubernetes.Version,
				}),
				openstack.CSIControllerName: enabledFalse,
			}))
		})

		It("should return correct control plane chart values (k8s >= 1.19)", func() {
			values, err := vp.GetControlPlaneChartValues(ctx, cp, clusterK8sAtLeast119, checksums, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{
				openstack.CloudControllerManagerName: utils.MergeMaps(ccmChartValues, map[string]interface{}{
					"kubernetesVersion": clusterK8sAtLeast119.Shoot.Spec.Kubernetes.Version,
				}),
				openstack.CSIControllerName: utils.MergeMaps(enabledTrue, map[string]interface{}{
					"replicas": 1,
					"podAnnotations": map[string]interface{}{
						"checksum/secret-" + openstack.CSIProvisionerName:          checksums[openstack.CSIProvisionerName],
						"checksum/secret-" + openstack.CSIAttacherName:             checksums[openstack.CSIAttacherName],
						"checksum/secret-" + openstack.CSISnapshotterName:          checksums[openstack.CSISnapshotterName],
						"checksum/secret-" + openstack.CSIResizerName:              checksums[openstack.CSIResizerName],
						"checksum/secret-" + openstack.CloudProviderDiskConfigName: checksums[openstack.CloudProviderDiskConfigName],
					},
					"csiSnapshotController": map[string]interface{}{
						"replicas": 1,
						"podAnnotations": map[string]interface{}{
							"checksum/secret-" + openstack.CSISnapshotControllerName: checksums[openstack.CSISnapshotControllerName],
						},
					},
				}),
			}))
		})
	})

	Describe("#GetControlPlaneShootChartValues", func() {
		It("should return correct shoot control plane chart values (k8s < 1.19)", func() {
			var b []byte
			values, err := vp.GetControlPlaneShootChartValues(ctx, cp, clusterK8sLessThan119, map[string]string{})
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{
				openstack.CloudControllerManagerName: enabledTrue,
				openstack.CSINodeName: utils.MergeMaps(enabledFalse, map[string]interface{}{
					"vpaEnabled": false,
					"podAnnotations": map[string]interface{}{
						"checksum/secret-" + openstack.CloudProviderDiskConfigName: "",
					},
					"cloudProviderConfig": b,
				}),
			}))
		})

		It("should return correct shoot control plane chart values (k8s >= 1.19)", func() {
			c.EXPECT().Get(ctx, cpDiskConfigKey, &corev1.Secret{}).DoAndReturn(clientGet(cpDiskConfig))

			values, err := vp.GetControlPlaneShootChartValues(ctx, cp, clusterK8sAtLeast119, map[string]string{})
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{
				openstack.CloudControllerManagerName: enabledTrue,
				openstack.CSINodeName: utils.MergeMaps(enabledTrue, map[string]interface{}{
					"vpaEnabled": true,
					"podAnnotations": map[string]interface{}{
						"checksum/secret-" + openstack.CloudProviderDiskConfigName: checksums[openstack.CloudProviderDiskConfigName],
					},
					"cloudProviderConfig": cloudProviderDiskConfig,
				}),
			}))
		})
	})

	Describe("#GetStorageClassesChartValues", func() {
		It("should return correct storage class chart values (k8s < 1.19)", func() {
			values, err := vp.GetStorageClassesChartValues(ctx, cp, clusterK8sLessThan119)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{
				"useLegacyProvisioner": true,
			}))
		})

		It("should return correct storage class chart values (k8s >= 1.19)", func() {
			values, err := vp.GetStorageClassesChartValues(ctx, cp, clusterK8sAtLeast119)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{
				"useLegacyProvisioner": false,
			}))
		})
	})
})

func encode(obj runtime.Object) []byte {
	data, _ := json.Marshal(obj)
	return data
}

func clientGet(result runtime.Object) interface{} {
	return func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
		switch obj.(type) {
		case *corev1.Secret:
			*obj.(*corev1.Secret) = *result.(*corev1.Secret)
		}
		return nil
	}
}
