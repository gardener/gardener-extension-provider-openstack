// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	"context"
	"fmt"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	openstackapi "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	openstackv1alpha1 "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/controller/infrastructure/infraflow/access"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/controller/infrastructure/infraflow/shared"
	infrainternal "github.com/gardener/gardener-extension-provider-openstack/pkg/internal/infrastructure"
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
	// IdentifierEgressCIDRs is the key for the slice containing egress CIDRs strings.
	IdentifierEgressCIDRs = "EgressCIDRs"

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

	// CreatedResourcesExistKey marks that there are infrastructure resources created by Gardener.
	CreatedResourcesExistKey = "resource_exist"
)

// Opts contain options to initiliaze a FlowContext
type Opts struct {
	Log            logr.Logger
	ClientFactory  osclient.Factory
	Infrastructure *extensionsv1alpha1.Infrastructure
	Cluster        *extensionscontroller.Cluster
	State          *openstackapi.InfrastructureState
	Client         client.Client
}

// FlowContext contains the logic to reconcile or delete the infrastructure.
type FlowContext struct {
	state                  shared.Whiteboard
	client                 client.Client
	openstackClientFactory osclient.Factory
	log                    logr.Logger
	infra                  *extensionsv1alpha1.Infrastructure
	config                 *openstackapi.InfrastructureConfig
	cloudProfileConfig     *openstackapi.CloudProfileConfig
	networking             osclient.Networking
	loadbalancing          osclient.Loadbalancing
	access                 access.NetworkingAccess
	compute                osclient.Compute

	*shared.BasicFlowContext
}

// NewFlowContext creates a new FlowContext object
func NewFlowContext(opts Opts) (*FlowContext, error) {
	whiteboard := shared.NewWhiteboard()
	if opts.State != nil {
		whiteboard.ImportFromFlatMap(opts.State.Data)
	}

	networking, err := opts.ClientFactory.Networking(osclient.WithRegion(opts.Infrastructure.Spec.Region))
	if err != nil {
		return nil, fmt.Errorf("creating networking client failed: %w", err)
	}
	access, err := access.NewNetworkingAccess(networking, opts.Log)
	if err != nil {
		return nil, fmt.Errorf("creating networking access failed: %w", err)
	}
	compute, err := opts.ClientFactory.Compute(osclient.WithRegion(opts.Infrastructure.Spec.Region))
	if err != nil {
		return nil, fmt.Errorf("creating compute client failed: %w", err)
	}
	loadbalancing, err := opts.ClientFactory.Loadbalancing(osclient.WithRegion(opts.Infrastructure.Spec.Region))
	if err != nil {
		return nil, err
	}
	infraConfig, err := helper.InfrastructureConfigFromInfrastructure(opts.Infrastructure)
	if err != nil {
		return nil, err
	}
	cloudProfileConfig, err := helper.CloudProfileConfigFromCluster(opts.Cluster)
	if err != nil {
		return nil, err
	}

	flowContext := &FlowContext{
		state:                  whiteboard,
		infra:                  opts.Infrastructure,
		config:                 infraConfig,
		cloudProfileConfig:     cloudProfileConfig,
		networking:             networking,
		loadbalancing:          loadbalancing,
		access:                 access,
		compute:                compute,
		log:                    opts.Log,
		client:                 opts.Client,
		openstackClientFactory: opts.ClientFactory,
	}
	return flowContext, nil
}

func (fctx *FlowContext) persistState(ctx context.Context) error {
	return infrainternal.PatchProviderStatusAndState(ctx, fctx.client, fctx.infra, nil, fctx.computeInfrastructureState())
}

func (fctx *FlowContext) computeInfrastructureState() *runtime.RawExtension {
	return &runtime.RawExtension{
		Object: &openstackv1alpha1.InfrastructureState{
			TypeMeta: metav1.TypeMeta{
				APIVersion: openstackv1alpha1.SchemeGroupVersion.String(),
				Kind:       "InfrastructureState",
			},
			Data: fctx.state.ExportAsFlatMap(),
		},
	}
}

func (fctx *FlowContext) computeInfrastructureStatus() *openstackv1alpha1.InfrastructureStatus {
	status := &openstackv1alpha1.InfrastructureStatus{
		TypeMeta: infrainternal.StatusTypeMeta,
	}

	status.Networks.FloatingPool.ID = ptr.Deref(fctx.state.Get(IdentifierFloatingNetwork), "")
	status.Networks.FloatingPool.Name = ptr.Deref(fctx.state.Get(NameFloatingNetwork), "")

	status.Networks.ID = ptr.Deref(fctx.state.Get(IdentifierNetwork), "")
	status.Networks.Name = ptr.Deref(fctx.state.Get(NameNetwork), "")

	status.Networks.Router.ID = ptr.Deref(fctx.state.Get(IdentifierRouter), "")
	status.Networks.Router.ExternalFixedIPs = fctx.state.GetObject(IdentifierEgressCIDRs).([]string)
	// backwards compatibility change for the deprecated field
	if len(status.Networks.Router.ExternalFixedIPs) > 0 {
		status.Networks.Router.IP = status.Networks.Router.ExternalFixedIPs[0]
	}

	status.Node.KeyName = ptr.Deref(fctx.state.Get(NameKeyPair), "")

	if v := fctx.state.Get(IdentifierShareNetwork); v != nil {
		status.Networks.ShareNetwork = &openstackv1alpha1.ShareNetworkStatus{
			ID:   *v,
			Name: ptr.Deref(fctx.state.Get(NameShareNetwork), ""),
		}
	}

	if v := fctx.state.Get(IdentifierSubnet); v != nil {
		status.Networks.Subnets = []openstackv1alpha1.Subnet{
			{
				Purpose: openstackv1alpha1.PurposeNodes,
				ID:      *v,
			},
		}
	}

	if v := fctx.state.Get(IdentifierSecGroup); v != nil {
		status.SecurityGroups = []openstackv1alpha1.SecurityGroup{
			{
				Purpose: openstackv1alpha1.PurposeNodes,
				ID:      *v,
				Name:    ptr.Deref(fctx.state.Get(NameSecGroup), ""),
			},
		}
	}

	return status
}
