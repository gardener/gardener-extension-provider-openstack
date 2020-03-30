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

	extensionscontroller "github.com/gardener/gardener-extensions/pkg/controller"
	mockclient "github.com/gardener/gardener-extensions/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener-extensions/pkg/util"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
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
)

var dhcpDomain = util.StringPtr("dhcp-domain")
var requestTimeout = util.StringPtr("2s")

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
		ctrl *gomock.Controller

		// Build scheme
		scheme = runtime.NewScheme()
		_      = api.AddToScheme(scheme)

		cp = defaultControlPlane()

		cidr       = "10.250.0.0/19"
		useOctavia = true

		cloudProfileConfig = &api.CloudProfileConfig{
			KeyStoneURL:    authURL,
			DHCPDomain:     dhcpDomain,
			RequestTimeout: requestTimeout,
			UseOctavia:     pointer.BoolPtr(useOctavia),
		}
		cloudProfileConfigJSON, _ = json.Marshal(cloudProfileConfig)

		cluster = &extensionscontroller.Cluster{
			CloudProfile: &gardencorev1beta1.CloudProfile{
				Spec: gardencorev1beta1.CloudProfileSpec{
					ProviderConfig: &gardencorev1beta1.ProviderConfig{
						RawExtension: runtime.RawExtension{
							Raw: cloudProfileConfigJSON,
						},
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

		checksums = map[string]string{
			v1beta1constants.SecretNameCloudProvider:                "8bafb35ff1ac60275d62e1cbd495aceb511fb354f74a20f7d06ecb48b3a68432",
			"cloud-controller-manager":                              "3d791b164a808638da9a8df03924be2a41e34cd664e42231c00fe369e3588272",
			"cloud-controller-manager-server":                       "6dff2a2e6f14444b66d8e4a351c049f7e89ee24ba3eaab95dbec40ba6bdebb52",
			openstack.CloudProviderConfigCloudControllerManagerName: "08a7bc7fe8f59b055f173145e211760a83f02cf89635cef26ebb351378635606",
		}

		configChartValues = map[string]interface{}{
			"kubernetesVersion": "1.13.4",
			"domainName":        "domain-name",
			"tenantName":        "tenant-name",
			"username":          "username",
			"password":          "password",
			"subnetID":          "subnet-acbd1234",
			"lbProvider":        "load-balancer-provider",
			"floatingNetworkID": "floating-network-id",
			"authUrl":           authURL,
			"dhcpDomain":        dhcpDomain,
			"requestTimeout":    requestTimeout,
			"useOctavia":        useOctavia,
		}

		ccmChartValues = map[string]interface{}{
			"replicas":          1,
			"kubernetesVersion": "1.13.4",
			"clusterName":       namespace,
			"podNetwork":        cidr,
			"podAnnotations": map[string]interface{}{
				"checksum/secret-cloud-controller-manager":                          "3d791b164a808638da9a8df03924be2a41e34cd664e42231c00fe369e3588272",
				"checksum/secret-cloud-controller-manager-server":                   "6dff2a2e6f14444b66d8e4a351c049f7e89ee24ba3eaab95dbec40ba6bdebb52",
				"checksum/secret-cloudprovider":                                     "8bafb35ff1ac60275d62e1cbd495aceb511fb354f74a20f7d06ecb48b3a68432",
				"checksum/configmap-cloud-provider-config-cloud-controller-manager": "08a7bc7fe8f59b055f173145e211760a83f02cf89635cef26ebb351378635606",
			},
			"podLabels": map[string]interface{}{
				"maintenance.gardener.cloud/restart": "true",
			},
			"featureGates": map[string]bool{
				"CustomResourceValidation": true,
			},
		}

		logger = log.Log.WithName("test")
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#GetConfigChartValues", func() {
		It("should return correct config chart values", func() {
			// Create mock client
			client := mockclient.NewMockClient(ctrl)
			client.EXPECT().Get(context.TODO(), cpSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpSecret))

			// Create valuesProvider
			vp := NewValuesProvider(logger)
			err := vp.(inject.Scheme).InjectScheme(scheme)
			Expect(err).NotTo(HaveOccurred())
			err = vp.(inject.Client).InjectClient(client)
			Expect(err).NotTo(HaveOccurred())

			// Call GetConfigChartValues method and check the result
			values, err := vp.GetConfigChartValues(context.TODO(), cp, cluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(configChartValues))
		})
	})

	Describe("#GetControlPlaneChartValues", func() {
		It("should return correct control plane chart values", func() {
			// Create valuesProvider
			vp := NewValuesProvider(logger)
			err := vp.(inject.Scheme).InjectScheme(scheme)
			Expect(err).NotTo(HaveOccurred())

			// Call GetControlPlaneChartValues method and check the result
			values, err := vp.GetControlPlaneChartValues(context.TODO(), cp, cluster, checksums, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(ccmChartValues))
		})
	})

	Describe("#GetConfigChartValues with Classes", func() {
		It("should return correct config chart values", func() {
			// Create mock client
			client := mockclient.NewMockClient(ctrl)
			client.EXPECT().Get(context.TODO(), cpSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpSecret))

			// Create valuesProvider
			vp := NewValuesProvider(logger)
			err := vp.(inject.Scheme).InjectScheme(scheme)
			Expect(err).NotTo(HaveOccurred())
			err = vp.(inject.Client).InjectClient(client)
			Expect(err).NotTo(HaveOccurred())

			floatingNetworkID := "4711"
			floatingNetworkID2 := "pub"
			fsid := "0815"
			floatingSubnetID2 := "pub0815"
			subnetID := "priv"
			floatingSubnetID := "default-floating-subnet-id"
			cp := controlPlane(
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
							Name:              "public",
							FloatingSubnetID:  &floatingSubnetID2,
							FloatingNetworkID: &floatingNetworkID2,
							SubnetID:          nil,
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

			configChartValues = map[string]interface{}{
				"kubernetesVersion": "1.13.4",
				"domainName":        "domain-name",
				"tenantName":        "tenant-name",
				"username":          "username",
				"password":          "password",
				"subnetID":          "subnet-acbd1234",
				"lbProvider":        "load-balancer-provider",
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
						"floatingNetworkID": floatingNetworkID2,
						"floatingSubnetID":  floatingSubnetID2,
					},
					{
						"name":     "other",
						"subnetID": subnetID,
					},
				},
				"authUrl":        authURL,
				"dhcpDomain":     dhcpDomain,
				"requestTimeout": requestTimeout,
				"useOctavia":     useOctavia,
			}
			// Call GetConfigChartValues method and check the result
			values, err := vp.GetConfigChartValues(context.TODO(), cp, cluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(configChartValues))
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
