// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"
	"fmt"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"

	openstackapi "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/controller/infrastructure/infraflow"
	openstackutils "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
)

// Delete the Infrastructure config.
func (a *actuator) Delete(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster) error {
	return util.DetermineError(a.delete(ctx, log, infra, cluster), helper.KnownCodes)
}

// ForceDelete forcefully deletes the Infrastructure.
func (a *actuator) ForceDelete(_ context.Context, _ logr.Logger, _ *extensionsv1alpha1.Infrastructure, _ *extensionscontroller.Cluster) error {
	return nil
}

func (a *actuator) delete(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster) error {
	var infraState *openstackapi.InfrastructureState
	fsOk, err := helper.HasFlowState(infra.Status)
	if err != nil {
		return err
	}
	if fsOk {
		// if it had a flow state, then we just decode it.
		infraState, err = helper.InfrastructureStateFromRaw(infra.Status.State)
		if err != nil {
			return err
		}
	} else {
		// otherwise migrate it from the terraform state if needed.
		infraState, err = a.migrateFromTerraform(ctx, log, infra)
		if err != nil {
			return err
		}
	}

	credentials, err := openstackutils.GetCredentials(ctx, a.client, infra.Spec.SecretRef, false)
	if err != nil {
		return fmt.Errorf("could not get Openstack credentials: %w", err)
	}
	clientFactory, err := openstackclient.NewOpenstackClientFromCredentials(ctx, credentials)
	if err != nil {
		return err
	}

	fctx, err := infraflow.NewFlowContext(infraflow.Opts{
		Log:            log,
		Infrastructure: infra,
		State:          infraState,
		Cluster:        cluster,
		ClientFactory:  clientFactory,
		Client:         a.client,
	})
	if err != nil {
		return fmt.Errorf("failed to create flow context: %w", err)
	}
	err = fctx.Delete(ctx)
	if err != nil {
		return err
	}

	tf, err := newTerraformer(log, a.restConfig, terraformerPurpose, infra, a.disableProjectedTokenMount)
	if err != nil {
		return err
	}
	return cleanupTerraformerResources(ctx, tf)
}
