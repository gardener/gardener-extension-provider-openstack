// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	"fmt"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"

	openstackapi "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/controller/infrastructure/infraflow/access"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/controller/infrastructure/infraflow/shared"
	osclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
)

const (

	// IdentifierRouter is the key for the router id
	IdentifierRouter = "Router"
	// IdentifierNetwork is the key for the network id
	IdentifierNetwork = "Network"
	// IdentifierSubnet is the key for the subnet id
	IdentifierSubnet = "Subnet"
	// IdentifierFloatingNetwork is the key for the floating network id
	IdentifierFloatingNetwork = "FloatingNetwork"
	// IdentifierSecGroup is the key for the security group id
	IdentifierSecGroup = "SecurityGroup"
	// IdentifierShareNetwork is the key for the share network id
	IdentifierShareNetwork = "ShareNetwork"

	// NameFloatingNetwork is the key for the floating network name
	NameFloatingNetwork = "FloatingNetworkName"
	// NameFloatingPoolSubnet is the name/regex for the floating pool subnets
	NameFloatingPoolSubnet = "FloatingPoolSubnetName"
	// NameNetwork is the name of the network
	NameNetwork = "NetworkName"
	// NameKeyPair is the key for the name of the EC2 key pair resource
	NameKeyPair = "KeyPair"
	// NameSecGroup is the name of the security group
	NameSecGroup = "SecurityGroupName"
	// NameShareNetwork is the name of the shared network
	NameShareNetwork = "ShareNetworkName"

	// RouterIP is the key for the router IP address
	RouterIP = "RouterIP"

	// ObjectSecGroup is the key for the cached security group
	ObjectSecGroup = "SecurityGroup"

	// MarkerMigratedFromTerraform is the key for marking the state for successful state migration from Terraformer
	MarkerMigratedFromTerraform = "MigratedFromTerraform"
	// MarkerTerraformCleanedUp is the key for marking the state for successful cleanup of Terraformer resources.
	MarkerTerraformCleanedUp = "TerraformCleanedUp"
)

// FlowContext contains the logic to reconcile or delete the AWS infrastructure.
type FlowContext struct {
	shared.BasicFlowContext
	state              shared.Whiteboard
	namespace          string
	infraSpec          extensionsv1alpha1.InfrastructureSpec
	config             *openstackapi.InfrastructureConfig
	cloudProfileConfig *openstackapi.CloudProfileConfig
	networking         osclient.Networking
	loadbalancing      osclient.Loadbalancing
	sharedFilesystem   osclient.SharedFilesystem
	access             access.NetworkingAccess
	compute            osclient.Compute
}

// NewFlowContext creates a new FlowContext object
func NewFlowContext(log logr.Logger, clientFactory osclient.Factory,
	infra *extensionsv1alpha1.Infrastructure, config *openstackapi.InfrastructureConfig,
	cloudProfileConfig *openstackapi.CloudProfileConfig,
	oldState shared.FlatMap, persistor shared.FlowStatePersistor) (*FlowContext, error) {

	whiteboard := shared.NewWhiteboard()
	if oldState != nil {
		whiteboard.ImportFromFlatMap(oldState)
	}

	networking, err := clientFactory.Networking(osclient.WithRegion(infra.Spec.Region))
	if err != nil {
		return nil, fmt.Errorf("creating networking client failed: %w", err)
	}
	access, err := access.NewNetworkingAccess(networking, log)
	if err != nil {
		return nil, fmt.Errorf("creating networking access failed: %w", err)
	}
	compute, err := clientFactory.Compute(osclient.WithRegion(infra.Spec.Region))
	if err != nil {
		return nil, fmt.Errorf("creating compute client failed: %w", err)
	}
	loadbalancing, err := clientFactory.Loadbalancing(osclient.WithRegion(infra.Spec.Region))
	if err != nil {
		return nil, err
	}
	sharedFilesytem, err := clientFactory.SharedFilesystem(osclient.WithRegion(infra.Spec.Region))
	if err != nil {
		return nil, err
	}

	flowContext := &FlowContext{
		BasicFlowContext:   *shared.NewBasicFlowContext(log, whiteboard, persistor),
		state:              whiteboard,
		namespace:          infra.Namespace,
		infraSpec:          infra.Spec,
		config:             config,
		cloudProfileConfig: cloudProfileConfig,
		networking:         networking,
		loadbalancing:      loadbalancing,
		access:             access,
		compute:            compute,
		sharedFilesystem:   sharedFilesytem,
	}
	return flowContext, nil
}

// GetInfrastructureConfig returns the InfrastructureConfig object
func (c *FlowContext) GetInfrastructureConfig() *openstackapi.InfrastructureConfig {
	return c.config
}
