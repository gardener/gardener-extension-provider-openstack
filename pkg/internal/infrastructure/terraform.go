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
	"context"
	"path/filepath"
	"strconv"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	apiv1alpha1 "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	openstacktypes "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/terraformer"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/chartrenderer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// TerraformerPurpose is a constant for the complete Terraform setup with purpose 'infrastructure'.
	TerraformerPurpose = "infra"
	// TerraformOutputKeySSHKeyName key for accessing SSH key name from outputs in terraform
	TerraformOutputKeySSHKeyName = "key_name"
	// TerraformOutputKeyRouterID is the id the router between provider network and the worker subnet.
	TerraformOutputKeyRouterID = "router_id"
	// TerraformOutputKeyNetworkID is the private worker network.
	TerraformOutputKeyNetworkID = "network_id"
	// TerraformOutputKeySecurityGroupID is the id of worker security group.
	TerraformOutputKeySecurityGroupID = "security_group_id"
	// TerraformOutputKeySecurityGroupName is the name of the worker security group.
	TerraformOutputKeySecurityGroupName = "security_group_name"
	// TerraformOutputKeyFloatingNetworkID is the id of the provider network.
	TerraformOutputKeyFloatingNetworkID = "floating_network_id"
	// TerraformOutputKeyFloatingSubnetID is the id of the floating pool network subnet.
	TerraformOutputKeyFloatingSubnetID = "floating_subnet_id"
	// TerraformOutputKeySubnetID is the id of the worker subnet.
	TerraformOutputKeySubnetID = "subnet_id"
	// DefaultRouterID is the computed router ID as generated by terraform.
	DefaultRouterID = "openstack_networking_router_v2.router.id"
)

// StatusTypeMeta is the TypeMeta of the GCP InfrastructureStatus
var StatusTypeMeta = metav1.TypeMeta{
	APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
	Kind:       "InfrastructureStatus",
}

// ComputeTerraformerChartValues computes the values for the OpenStack Terraformer chart.
func ComputeTerraformerChartValues(
	infra *extensionsv1alpha1.Infrastructure,
	credentials *openstack.Credentials,
	config *api.InfrastructureConfig,
	cluster *controller.Cluster,
) (map[string]interface{}, error) {
	var (
		createRouter  = true
		createNetwork = true
		routerConfig  = map[string]interface{}{
			"id": DefaultRouterID,
		}
		outputKeysConfig = map[string]interface{}{
			"routerID":          TerraformOutputKeyRouterID,
			"networkID":         TerraformOutputKeyNetworkID,
			"keyName":           TerraformOutputKeySSHKeyName,
			"securityGroupID":   TerraformOutputKeySecurityGroupID,
			"securityGroupName": TerraformOutputKeySecurityGroupName,
			"floatingNetworkID": TerraformOutputKeyFloatingNetworkID,
			"subnetID":          TerraformOutputKeySubnetID,
		}
		networkConfig = map[string]interface{}{}
	)

	if config.Networks.Router != nil {
		createRouter = false
		routerConfig["id"] = strconv.Quote(config.Networks.Router.ID)
	}

	if config.Networks.ID != nil {
		createNetwork = false
		networkConfig["id"] = *config.Networks.ID
	}

	if createRouter && config.FloatingPoolSubnetName != nil {
		routerConfig["floatingPoolSubnetName"] = *config.FloatingPoolSubnetName
		outputKeysConfig["floatingSubnetID"] = TerraformOutputKeyFloatingSubnetID
	}

	cloudProfileConfig, err := helper.CloudProfileConfigFromCluster(cluster)
	if err != nil {
		return nil, err
	}

	keyStoneURL, err := helper.FindKeyStoneURL(cloudProfileConfig.KeyStoneURLs, cloudProfileConfig.KeyStoneURL, infra.Spec.Region)
	if err != nil {
		return nil, err
	}

	if cloudProfileConfig.UseSNAT != nil {
		routerConfig["enableSNAT"] = *cloudProfileConfig.UseSNAT
	}

	workersCIDR := config.Networks.Workers
	// Backwards compatibility - remove this code in a future version.
	if workersCIDR == "" {
		workersCIDR = config.Networks.Worker
	}
	networkConfig["workers"] = workersCIDR

	return map[string]interface{}{
		"openstack": map[string]interface{}{
			"authURL":          keyStoneURL,
			"domainName":       credentials.DomainName,
			"tenantName":       credentials.TenantName,
			"region":           infra.Spec.Region,
			"floatingPoolName": config.FloatingPoolName,
		},
		"create": map[string]interface{}{
			"router":  createRouter,
			"network": createNetwork,
		},
		"dnsServers":   cloudProfileConfig.DNSServers,
		"sshPublicKey": string(infra.Spec.SSHPublicKey),
		"router":       routerConfig,
		"clusterName":  infra.Namespace,
		"networks":     networkConfig,
		"outputKeys":   outputKeysConfig,
	}, nil
}

// RenderTerraformerChart renders the openstack-infra chart with the given values.
func RenderTerraformerChart(
	renderer chartrenderer.Interface,
	infra *extensionsv1alpha1.Infrastructure,
	credentials *openstack.Credentials,
	config *api.InfrastructureConfig,
	cluster *controller.Cluster,
) (*TerraformFiles, error) {
	values, err := ComputeTerraformerChartValues(infra, credentials, config, cluster)
	if err != nil {
		return nil, err
	}

	release, err := renderer.Render(filepath.Join(openstacktypes.InternalChartsPath, "openstack-infra"), "openstack-infra", infra.Namespace, values)
	if err != nil {
		return nil, err
	}

	return &TerraformFiles{
		Main:      release.FileContent("main.tf"),
		Variables: release.FileContent("variables.tf"),
		TFVars:    []byte(release.FileContent("terraform.tfvars")),
	}, nil
}

// TerraformFiles are the files that have been rendered from the infrastructure chart.
type TerraformFiles struct {
	Main      string
	Variables string
	TFVars    []byte
}

// TerraformState is the Terraform state for an infrastructure.
type TerraformState struct {
	// SSHKeyName key for accessing SSH key name from outputs in terraform
	SSHKeyName string
	// RouterID is the id the router between provider network and the worker subnet.
	RouterID string
	// NetworkID is the private worker network.
	NetworkID string
	// SubnetID is the id of the worker subnet.
	SubnetID string
	// FloatingNetworkID is the id of the provider network.
	FloatingNetworkID string
	// FloatingPoolSubnetID is the id of the floating pool network subnet.
	FloatingPoolSubnetID string
	// SecurityGroupID is the id of worker security group.
	SecurityGroupID string
	// SecurityGroupName is the name of the worker security group.
	SecurityGroupName string
}

// ExtractTerraformState extracts the TerraformState from the given Terraformer.
func ExtractTerraformState(ctx context.Context, tf terraformer.Terraformer, config *api.InfrastructureConfig) (*TerraformState, error) {
	outputKeys := []string{
		TerraformOutputKeySSHKeyName,
		TerraformOutputKeyRouterID,
		TerraformOutputKeyNetworkID,
		TerraformOutputKeySubnetID,
		TerraformOutputKeyFloatingNetworkID,
		TerraformOutputKeySecurityGroupID,
		TerraformOutputKeySecurityGroupName,
	}

	if config.Networks.Router == nil && config.FloatingPoolSubnetName != nil {
		outputKeys = append(outputKeys, TerraformOutputKeyFloatingSubnetID)
	}

	vars, err := tf.GetStateOutputVariables(ctx, outputKeys...)
	if err != nil {
		return nil, err
	}

	state := &TerraformState{
		SSHKeyName:        vars[TerraformOutputKeySSHKeyName],
		RouterID:          vars[TerraformOutputKeyRouterID],
		NetworkID:         vars[TerraformOutputKeyNetworkID],
		SubnetID:          vars[TerraformOutputKeySubnetID],
		FloatingNetworkID: vars[TerraformOutputKeyFloatingNetworkID],
		SecurityGroupID:   vars[TerraformOutputKeySecurityGroupID],
		SecurityGroupName: vars[TerraformOutputKeySecurityGroupName],
	}

	if config.Networks.Router == nil && config.FloatingPoolSubnetName != nil {
		state.FloatingPoolSubnetID = vars[TerraformOutputKeyFloatingSubnetID]
	}

	return state, nil
}

// StatusFromTerraformState computes an InfrastructureStatus from the given
// Terraform variables.
func StatusFromTerraformState(state *TerraformState) *apiv1alpha1.InfrastructureStatus {
	floatingPoolStatus := apiv1alpha1.FloatingPoolStatus{
		ID: state.FloatingNetworkID,
	}
	if state.FloatingPoolSubnetID != "" {
		floatingPoolStatus.SubnetID = &state.FloatingPoolSubnetID
	}

	var status = &apiv1alpha1.InfrastructureStatus{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
			Kind:       "InfrastructureStatus",
		},
		Networks: apiv1alpha1.NetworkStatus{
			ID:           state.NetworkID,
			FloatingPool: floatingPoolStatus,
			Router: apiv1alpha1.RouterStatus{
				ID: state.RouterID,
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

	return status
}

// ComputeStatus computes the status based on the Terraformer and the given InfrastructureConfig.
func ComputeStatus(ctx context.Context, tf terraformer.Terraformer, config *api.InfrastructureConfig) (*apiv1alpha1.InfrastructureStatus, error) {
	state, err := ExtractTerraformState(ctx, tf, config)
	if err != nil {
		return nil, err
	}

	status := StatusFromTerraformState(state)
	status.Networks.FloatingPool.Name = config.FloatingPoolName
	return status, nil
}
