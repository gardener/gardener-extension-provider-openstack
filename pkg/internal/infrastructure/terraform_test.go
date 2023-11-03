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

package infrastructure

import (
	"encoding/json"
	"strconv"

	"github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	apiv1alpha1 "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"
)

var _ = Describe("Terraform", func() {
	var (
		infra                  *extensionsv1alpha1.Infrastructure
		cloudProfileConfig     *api.CloudProfileConfig
		cloudProfileConfigJSON []byte
		config                 *api.InfrastructureConfig
		cluster                *controller.Cluster

		keystoneURL = "foo-bar.com"
		dnsServers  = []string{"a", "b"}
	)

	BeforeEach(func() {
		config = &api.InfrastructureConfig{
			Networks: api.Networks{
				Router: &api.Router{
					ID: "1",
				},
				Workers: "10.1.0.0/16",
			},
		}

		infra = &extensionsv1alpha1.Infrastructure{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar",
			},

			Spec: extensionsv1alpha1.InfrastructureSpec{
				Region: "de_1_1",
				SecretRef: corev1.SecretReference{
					Namespace: "foo",
					Name:      "openstack-credentials",
				},
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					ProviderConfig: &runtime.RawExtension{
						Object: config,
					},
				},
			},
		}

		podsCIDR := "11.0.0.0/16"
		servicesCIDR := "12.0.0.0/16"

		cloudProfileConfig = &api.CloudProfileConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: api.SchemeGroupVersion.String(),
				Kind:       "CloudProfileConfig",
			},
			DNSServers:  dnsServers,
			KeyStoneURL: keystoneURL,
		}
		cloudProfileConfigJSON, _ = json.Marshal(cloudProfileConfig)
		cluster = &controller.Cluster{
			CloudProfile: &gardencorev1beta1.CloudProfile{
				Spec: gardencorev1beta1.CloudProfileSpec{
					ProviderConfig: &runtime.RawExtension{
						Raw: cloudProfileConfigJSON,
					},
				},
			},
			Shoot: &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Networking: &gardencorev1beta1.Networking{
						Pods:     &podsCIDR,
						Services: &servicesCIDR,
					},
				},
			},
		}
	})

	Describe("#ComputeTerraformerTemplateValues", func() {
		var (
			expectedOpenStackValues  map[string]interface{}
			expectedCreateValues     map[string]interface{}
			expectedRouterValues     map[string]interface{}
			expectedNetworkValues    map[string]interface{}
			expectedOutputKeysValues map[string]interface{}
		)

		BeforeEach(func() {
			expectedOpenStackValues = map[string]interface{}{
				"authURL":           keystoneURL,
				"region":            infra.Spec.Region,
				"floatingPoolName":  config.FloatingPoolName,
				"maxApiCallRetries": MaxApiCallRetries,
				"insecure":          false,
				"useCACert":         false,
			}
			expectedCreateValues = map[string]interface{}{
				"router":       false,
				"subnet":       true,
				"network":      true,
				"shareNetwork": false,
			}
			expectedRouterValues = map[string]interface{}{
				"id": strconv.Quote("1"),
			}
			expectedNetworkValues = map[string]interface{}{
				"workers": config.Networks.Workers,
			}
			expectedOutputKeysValues = map[string]interface{}{
				"routerID":          TerraformOutputKeyRouterID,
				"routerIP":          TerraformOutputKeyRouterIP,
				"networkID":         TerraformOutputKeyNetworkID,
				"networkName":       TerraformOutputKeyNetworkName,
				"keyName":           TerraformOutputKeySSHKeyName,
				"securityGroupID":   TerraformOutputKeySecurityGroupID,
				"securityGroupName": TerraformOutputKeySecurityGroupName,
				"floatingNetworkID": TerraformOutputKeyFloatingNetworkID,
				"subnetID":          TerraformOutputKeySubnetID,
			}
		})

		It("should correctly compute the terraformer chart values", func() {
			values, err := ComputeTerraformerTemplateValues(infra, config, cluster)
			Expect(err).To(BeNil())

			Expect(values).To(Equal(map[string]interface{}{
				"openstack":    expectedOpenStackValues,
				"create":       expectedCreateValues,
				"dnsServers":   dnsServers,
				"sshPublicKey": string(infra.Spec.SSHPublicKey),
				"router":       expectedRouterValues,
				"clusterName":  infra.Namespace,
				"networks":     expectedNetworkValues,
				"outputKeys":   expectedOutputKeysValues,
			}))
		})

		It("should correctly compute the terraformer chart values with vpc creation", func() {
			cloudProfileConfig.UseSNAT = pointer.Bool(true)
			cloudProfileConfigJSON, _ = json.Marshal(cloudProfileConfig)
			cluster.CloudProfile.Spec.ProviderConfig.Raw = cloudProfileConfigJSON

			config.Networks.Router = nil
			expectedCreateValues["router"] = true
			expectedRouterValues["id"] = DefaultRouterID
			expectedRouterValues["enableSNAT"] = true

			values, err := ComputeTerraformerTemplateValues(infra, config, cluster)
			Expect(err).To(BeNil())
			Expect(values).To(Equal(map[string]interface{}{
				"openstack":    expectedOpenStackValues,
				"create":       expectedCreateValues,
				"dnsServers":   dnsServers,
				"sshPublicKey": string(infra.Spec.SSHPublicKey),
				"router":       expectedRouterValues,
				"clusterName":  infra.Namespace,
				"networks":     expectedNetworkValues,
				"outputKeys":   expectedOutputKeysValues,
			}))
		})

		It("should correctly compute the terraformer chart values when reusing vpc", func() {
			networkID := "networkID"

			config.Networks.ID = &networkID
			expectedCreateValues["network"] = false
			expectedNetworkValues["id"] = networkID

			values, err := ComputeTerraformerTemplateValues(infra, config, cluster)
			Expect(err).To(BeNil())
			Expect(values).To(Equal(map[string]interface{}{
				"openstack":    expectedOpenStackValues,
				"create":       expectedCreateValues,
				"dnsServers":   dnsServers,
				"sshPublicKey": string(infra.Spec.SSHPublicKey),
				"router":       expectedRouterValues,
				"clusterName":  infra.Namespace,
				"networks":     expectedNetworkValues,
				"outputKeys":   expectedOutputKeysValues,
			}))
		})

		It("should correctly compute the terraformer chart values when reusing vpc and subnet", func() {
			networkID := "networkID"
			subnetID := "subneID"

			config.Networks.ID = &networkID
			config.Networks.SubnetID = &subnetID
			config.Networks.Router = nil
			expectedCreateValues["network"] = false
			expectedNetworkValues["id"] = networkID
			expectedCreateValues["subnet"] = false
			expectedNetworkValues["subnet"] = subnetID
			expectedCreateValues["router"] = true
			expectedRouterValues["id"] = "openstack_networking_router_v2.router.id"

			values, err := ComputeTerraformerTemplateValues(infra, config, cluster)
			Expect(err).To(BeNil())
			Expect(values).To(Equal(map[string]interface{}{
				"openstack":    expectedOpenStackValues,
				"create":       expectedCreateValues,
				"dnsServers":   dnsServers,
				"sshPublicKey": string(infra.Spec.SSHPublicKey),
				"router":       expectedRouterValues,
				"clusterName":  infra.Namespace,
				"networks":     expectedNetworkValues,
				"outputKeys":   expectedOutputKeysValues,
			}))
		})

		It("should correctly compute the terraformer chart values when reusing vpc and subnet and router", func() {
			networkID := "networkID"
			subnetID := "subneID"
			routerID := "routerID"

			config.Networks.ID = &networkID
			config.Networks.SubnetID = &subnetID
			config.Networks.Router.ID = routerID
			expectedCreateValues["network"] = false
			expectedNetworkValues["id"] = networkID
			expectedCreateValues["subnet"] = false
			expectedNetworkValues["subnet"] = subnetID
			expectedCreateValues["router"] = false
			expectedRouterValues["id"] = strconv.Quote(routerID)

			values, err := ComputeTerraformerTemplateValues(infra, config, cluster)
			Expect(err).To(BeNil())
			Expect(values).To(Equal(map[string]interface{}{
				"openstack":    expectedOpenStackValues,
				"create":       expectedCreateValues,
				"dnsServers":   dnsServers,
				"sshPublicKey": string(infra.Spec.SSHPublicKey),
				"router":       expectedRouterValues,
				"clusterName":  infra.Namespace,
				"networks":     expectedNetworkValues,
				"outputKeys":   expectedOutputKeysValues,
			}))
		})

		It("should correctly compute the terraformer chart values with floating pool subnet", func() {
			fipSubnetID := "sample-fip-subnet-id"

			config.Networks.Router = nil
			config.FloatingPoolSubnetName = &fipSubnetID

			expectedCreateValues["router"] = true
			expectedRouterValues["id"] = DefaultRouterID
			expectedRouterValues["floatingPoolSubnet"] = fipSubnetID

			values, err := ComputeTerraformerTemplateValues(infra, config, cluster)
			Expect(err).To(BeNil())
			Expect(values).To(Equal(map[string]interface{}{
				"openstack":    expectedOpenStackValues,
				"create":       expectedCreateValues,
				"dnsServers":   dnsServers,
				"sshPublicKey": string(infra.Spec.SSHPublicKey),
				"router":       expectedRouterValues,
				"clusterName":  infra.Namespace,
				"networks":     expectedNetworkValues,
				"outputKeys":   expectedOutputKeysValues,
			}))
		})

		It("should correctly compute the terraformer chart values for share network creation", func() {
			config.Networks.ShareNetwork = &api.ShareNetwork{Enabled: true}
			expectedCreateValues["shareNetwork"] = true
			expectedOutputKeysValues["shareNetworkID"] = TerraformOutputKeyShareNetworkID
			expectedOutputKeysValues["shareNetworkName"] = TerraformOutputKeyShareNetworkName

			values, err := ComputeTerraformerTemplateValues(infra, config, cluster)
			Expect(err).To(BeNil())

			Expect(values).To(Equal(map[string]interface{}{
				"openstack":    expectedOpenStackValues,
				"create":       expectedCreateValues,
				"dnsServers":   dnsServers,
				"sshPublicKey": string(infra.Spec.SSHPublicKey),
				"router":       expectedRouterValues,
				"clusterName":  infra.Namespace,
				"networks":     expectedNetworkValues,
				"outputKeys":   expectedOutputKeysValues,
			}))
		})

	})

	Describe("#StatusFromTerraformState", func() {
		var (
			SSHKeyName        string
			RouterID          string
			RouterIP          string
			NetworkID         string
			SubnetID          string
			FloatingNetworkID string
			SecurityGroupID   string
			SecurityGroupName string

			state               TerraformState
			expectedInfraStatus apiv1alpha1.InfrastructureStatus
		)

		BeforeEach(func() {
			SSHKeyName = "my-key"
			RouterID = "111"
			RouterIP = "1.1.1.1"
			NetworkID = "222"
			SubnetID = "333"
			FloatingNetworkID = "444"
			SecurityGroupID = "555"
			SecurityGroupName = "my-sec-group"

			state = TerraformState{
				SSHKeyName:        SSHKeyName,
				RouterID:          RouterID,
				RouterIP:          RouterIP,
				NetworkID:         NetworkID,
				SubnetID:          SubnetID,
				FloatingNetworkID: FloatingNetworkID,
				SecurityGroupID:   SecurityGroupID,
				SecurityGroupName: SecurityGroupName,
			}

			expectedInfraStatus = apiv1alpha1.InfrastructureStatus{
				TypeMeta: metav1.TypeMeta{
					APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
					Kind:       "InfrastructureStatus",
				},
				Networks: apiv1alpha1.NetworkStatus{
					ID: state.NetworkID,
					Router: apiv1alpha1.RouterStatus{
						ID: state.RouterID,
						IP: state.RouterIP,
					},
					FloatingPool: apiv1alpha1.FloatingPoolStatus{
						ID: FloatingNetworkID,
					},
					Subnets: []apiv1alpha1.Subnet{
						{
							Purpose: apiv1alpha1.PurposeNodes,
							ID:      state.SubnetID,
						},
					},
				},
				SecurityGroups: []apiv1alpha1.SecurityGroup{
					{
						Purpose: apiv1alpha1.PurposeNodes,
						ID:      state.SecurityGroupID,
						Name:    state.SecurityGroupName,
					},
				},
				Node: apiv1alpha1.NodeStatus{
					KeyName: state.SSHKeyName,
				},
			}
		})

		It("should correctly compute the status", func() {
			status := StatusFromTerraformState(&state)

			Expect(status).To(Equal(&expectedInfraStatus))
		})
	})
})
