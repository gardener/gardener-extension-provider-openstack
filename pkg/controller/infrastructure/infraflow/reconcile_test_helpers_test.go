// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"go.uber.org/mock/gomock"

	openstackapi "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	accessmocks "github.com/gardener/gardener-extension-provider-openstack/pkg/controller/infrastructure/infraflow/access/mocks"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/controller/infrastructure/infraflow/shared"
)

// newFixture builds a minimal FlowContext for unit testing. It wires a fresh
// Whiteboard and a mock NetworkingAccess, while leaving all other fields nil
// (they are not needed for the functions under test).
func newFixture(ctrl *gomock.Controller, cfg *openstackapi.InfrastructureConfig) (*FlowContext, *accessmocks.MockNetworkingAccess) {
	mockAccess := accessmocks.NewMockNetworkingAccess(ctrl)
	fctx := &FlowContext{
		state:              shared.NewWhiteboard(),
		config:             cfg,
		cloudProfileConfig: &openstackapi.CloudProfileConfig{},
		infra: &extensionsv1alpha1.Infrastructure{
			Spec: extensionsv1alpha1.InfrastructureSpec{
				Region: "test-region",
			},
		},
		access:           mockAccess,
		BasicFlowContext: &shared.BasicFlowContext{},
	}
	return fctx, mockAccess
}

// defaultInfraConfig returns an InfrastructureConfig with a workers CIDR and
// no BYO fields set — the fully-managed baseline.
func defaultInfraConfig() *openstackapi.InfrastructureConfig {
	return &openstackapi.InfrastructureConfig{
		FloatingPoolName: "my-pool",
		Networks: openstackapi.Networks{
			Workers: "10.250.0.0/19",
		},
	}
}
