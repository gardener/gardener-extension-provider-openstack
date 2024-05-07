// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/controller/infrastructure"
	"github.com/gardener/gardener/extensions/pkg/terraformer"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	openstackv1alpha1 "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"
	infrainternal "github.com/gardener/gardener-extension-provider-openstack/pkg/internal/infrastructure"
)

const (
	// AnnotationKeyUseFlow is the annotation key used to enable reconciliation with flow instead of terraformer.
	AnnotationKeyUseFlow = "openstack.provider.extensions.gardener.cloud/use-flow"
)

type actuator struct {
	client                     client.Client
	restConfig                 *rest.Config
	disableProjectedTokenMount bool
}

// NewActuator creates a new Actuator that updates the status of the handled Infrastructure resources.
func NewActuator(mgr manager.Manager, disableProjectedTokenMount bool) infrastructure.Actuator {
	return &actuator{
		disableProjectedTokenMount: disableProjectedTokenMount,
		client:                     mgr.GetClient(),
		restConfig:                 mgr.GetConfig(),
	}
}

// Helper functions

func (a *actuator) updateProviderStatusWithTerraformer(
	ctx context.Context,
	tf terraformer.Terraformer,
	infra *extensionsv1alpha1.Infrastructure,
	config *api.InfrastructureConfig,
) error {
	status, err := infrainternal.ComputeStatus(ctx, tf, config)
	if err != nil {
		return err
	}

	state, err := tf.GetRawState(ctx)
	if err != nil {
		return err
	}

	stateBytes, err := state.Marshal()
	if err != nil {
		return err
	}

	return a.updateProviderStatus(ctx, infra, status, stateBytes)
}

func (a *actuator) updateProviderStatus(
	ctx context.Context,
	infra *extensionsv1alpha1.Infrastructure,
	status *openstackv1alpha1.InfrastructureStatus,
	stateBytes []byte,
) error {
	if status != nil && status.Networks.Router.IP != "" {
		infra.Status.EgressCIDRs = []string{fmt.Sprintf("%s/32", status.Networks.Router.IP)}
	}
	patch := client.MergeFrom(infra.DeepCopy())
	infra.Status.ProviderStatus = &runtime.RawExtension{Object: status}
	infra.Status.State = &runtime.RawExtension{Raw: stateBytes}
	return a.client.Status().Patch(ctx, infra, patch)
}
