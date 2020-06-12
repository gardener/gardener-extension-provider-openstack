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

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	apiv1alpha1 "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	"github.com/gardener/gardener/extensions/pkg/controller"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("Terraform", func() {
	var (
		infra                  *extensionsv1alpha1.Infrastructure
		cloudProfileConfig     *api.CloudProfileConfig
		cloudProfileConfigJSON []byte
		config                 *api.InfrastructureConfig
		cluster                *controller.Cluster
		credentials            *openstack.Credentials

		keystoneURL = "foo-bar.com"
		dnsServers  = []string{"a", "b"}
	)

	BeforeEach(func() {
		config = &api.InfrastructureConfig{
			Networks: api.Networks{
				Router: &api.Router{
					ID: sp("1"),
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
						Pods:     &podsCIDR,
						Services: &servicesCIDR,
					},
				},
			},
		}

		credentials = &openstack.Credentials{Username: "user", Password: "secret"}
	})

	Describe("#ComputeTerraformerChartValues", func() {
		It("should correctly compute the terraformer chart values", func() {
			values, err := ComputeTerraformerChartValues(infra, credentials, config, cluster)
			Expect(err).To(BeNil())

			Expect(values).To(Equal(map[string]interface{}{
				"openstack": map[string]interface{}{
					"authURL":          keystoneURL,
					"domainName":       credentials.DomainName,
					"tenantName":       credentials.TenantName,
					"region":           infra.Spec.Region,
					"floatingPoolName": config.FloatingPoolName,
				},
				"create": map[string]interface{}{
					"router": false,
				},
				"dnsServers":   dnsServers,
				"sshPublicKey": string(infra.Spec.SSHPublicKey),
				"router": map[string]interface{}{
					"id": "1",
				},
				"clusterName": infra.Namespace,
				"networks": map[string]interface{}{
					"workers": config.Networks.Workers,
				},
				"outputKeys": map[string]interface{}{
					"routerID":          TerraformOutputKeyRouterID,
					"networkID":         TerraformOutputKeyNetworkID,
					"keyName":           TerraformOutputKeySSHKeyName,
					"securityGroupID":   TerraformOutputKeySecurityGroupID,
					"securityGroupName": TerraformOutputKeySecurityGroupName,
					"floatingNetworkID": TerraformOutputKeyFloatingNetworkID,
					"subnetID":          TerraformOutputKeySubnetID,
				},
			}))
		})

		It("should correctly compute the terraformer chart values with vpc creation", func() {
			config.Networks.Router = nil

			values, err := ComputeTerraformerChartValues(infra, credentials, config, cluster)
			Expect(err).To(BeNil())

			Expect(values).To(Equal(map[string]interface{}{
				"openstack": map[string]interface{}{
					"authURL":          keystoneURL,
					"domainName":       credentials.DomainName,
					"tenantName":       credentials.TenantName,
					"region":           infra.Spec.Region,
					"floatingPoolName": config.FloatingPoolName,
				},
				"create": map[string]interface{}{
					"router": true,
				},
				"dnsServers":   dnsServers,
				"sshPublicKey": string(infra.Spec.SSHPublicKey),
				"router": map[string]interface{}{
					"id": DefaultRouterID,
				},
				"clusterName": infra.Namespace,
				"networks": map[string]interface{}{
					"workers": config.Networks.Workers,
				},
				"outputKeys": map[string]interface{}{
					"routerID":          TerraformOutputKeyRouterID,
					"networkID":         TerraformOutputKeyNetworkID,
					"keyName":           TerraformOutputKeySSHKeyName,
					"securityGroupID":   TerraformOutputKeySecurityGroupID,
					"securityGroupName": TerraformOutputKeySecurityGroupName,
					"floatingNetworkID": TerraformOutputKeyFloatingNetworkID,
					"subnetID":          TerraformOutputKeySubnetID,
				},
			}))
		})
	})

	Describe("#StatusFromTerraformState", func() {
		var (
			SSHKeyName        string
			RouterID          string
			NetworkID         string
			SubnetID          string
			FloatingNetworkID string
			SecurityGroupID   string
			SecurityGroupName string

			state *TerraformState
		)

		BeforeEach(func() {
			SSHKeyName = "my-key"
			RouterID = "111"
			NetworkID = "222"
			SubnetID = "333"
			FloatingNetworkID = "444"
			SecurityGroupID = "555"
			SecurityGroupName = "my-sec-group"

			state = &TerraformState{
				SSHKeyName:        SSHKeyName,
				RouterID:          RouterID,
				NetworkID:         NetworkID,
				SubnetID:          SubnetID,
				FloatingNetworkID: FloatingNetworkID,
				SecurityGroupID:   SecurityGroupID,
				SecurityGroupName: SecurityGroupName,
			}
		})

		It("should correctly compute the status", func() {
			status := StatusFromTerraformState(state)

			Expect(status).To(Equal(&apiv1alpha1.InfrastructureStatus{
				TypeMeta: metav1.TypeMeta{
					APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
					Kind:       "InfrastructureStatus",
				},
				Networks: apiv1alpha1.NetworkStatus{
					ID: state.NetworkID,
					Router: apiv1alpha1.RouterStatus{
						ID: state.RouterID,
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
			}))
		})
	})
})

func sp(s string) *string {
	return &s
}
