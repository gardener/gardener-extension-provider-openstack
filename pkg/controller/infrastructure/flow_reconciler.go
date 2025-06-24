// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/controller/infrastructure/infraflow"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/internal"
	infrainternal "github.com/gardener/gardener-extension-provider-openstack/pkg/internal/infrastructure"
	openstackutils "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
)

// FlowReconciler an implementation of an infrastructure reconciler using native SDKs.
type FlowReconciler struct {
	client                     client.Client
	restConfig                 *rest.Config
	log                        logr.Logger
	disableProjectedTokenMount bool
}

// NewFlowReconciler creates a new flow reconciler.
func NewFlowReconciler(client client.Client, restConfig *rest.Config, log logr.Logger, projToken bool) Reconciler {
	return &FlowReconciler{
		client:                     client,
		restConfig:                 restConfig,
		log:                        log,
		disableProjectedTokenMount: projToken,
	}
}

// Reconcile reconciles the infrastructure and updates the Infrastructure status (state of the world), the state (input for the next loops) or reports any errors that occurred.
func (f *FlowReconciler) Reconcile(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	var (
		infraState *openstack.InfrastructureState
		err        error
	)

	// when the function is called, we may have: a. no state, b. terraform state (migration) or c. flow state. In case of a TF state
	// because no explicit migration to the new flow format is necessary, we simply return an empty state.
	fsOk, err := hasFlowState(infra.Status.State)
	if err != nil {
		return err
	}

	if fsOk {
		// if it had a flow state, then we just decode it.
		infraState, err = f.infrastructureStateFromRaw(infra)
		if err != nil {
			return err
		}
	} else {
		// otherwise migrate it from the terraform state if needed.
		infraState, err = f.migrateFromTerraform(ctx, infra)
		if err != nil {
			return err
		}
	}

	credentials, err := openstackutils.GetCredentials(ctx, f.client, infra.Spec.SecretRef, false)
	if err != nil {
		return fmt.Errorf("could not get Openstack credentials: %w", err)
	}
	clientFactory, err := openstackclient.NewOpenstackClientFromCredentials(ctx, credentials)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	fctx, err := infraflow.NewFlowContext(infraflow.Opts{
		Log:            f.log,
		Infrastructure: infra,
		State:          infraState,
		Cluster:        cluster,
		ClientFactory:  clientFactory,
		Client:         f.client,
	})
	if err != nil {
		return fmt.Errorf("failed to create flow context: %w", err)
	}

	return fctx.Reconcile(ctx)
}

// Delete deletes the infrastructure resource using the flow reconciler.
func (f *FlowReconciler) Delete(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	infraState, err := f.infrastructureStateFromRaw(infra)
	if err != nil {
		return err
	}

	credentials, err := openstackutils.GetCredentials(ctx, f.client, infra.Spec.SecretRef, false)
	if err != nil {
		return fmt.Errorf("could not get Openstack credentials: %w", err)
	}
	clientFactory, err := openstackclient.NewOpenstackClientFromCredentials(ctx, credentials)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	fctx, err := infraflow.NewFlowContext(infraflow.Opts{
		Log:            f.log,
		Infrastructure: infra,
		State:          infraState,
		Cluster:        cluster,
		ClientFactory:  clientFactory,
		Client:         f.client,
	})
	if err != nil {
		return fmt.Errorf("failed to create flow context: %w", err)
	}
	err = fctx.Delete(ctx)
	if err != nil {
		return err
	}

	tf, err := internal.NewTerraformer(f.log, f.restConfig, infrainternal.TerraformerPurpose, infra, f.disableProjectedTokenMount)
	if err != nil {
		return err
	}
	return CleanupTerraformerResources(ctx, tf)
}

// Restore implements the restoration of an infrastructure resource during the control plane migration.
func (f *FlowReconciler) Restore(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	return f.Reconcile(ctx, infra, cluster)
}

func (f *FlowReconciler) infrastructureStateFromRaw(infra *extensionsv1alpha1.Infrastructure) (*openstack.InfrastructureState, error) {
	state := &openstack.InfrastructureState{}
	raw := infra.Status.State

	if raw != nil {
		jsonBytes, err := raw.MarshalJSON()
		if err != nil {
			return nil, err
		}

		// todo(ka): for now we won't use the actuator decoder because the flow state kind was registered as "FlowState" and not "InfrastructureState". So we
		// shall use the simple json unmarshal for this release.
		if err := json.Unmarshal(jsonBytes, state); err != nil {
			return nil, err
		}
	}

	return state, nil
}

func (f *FlowReconciler) migrateFromTerraform(ctx context.Context, infra *extensionsv1alpha1.Infrastructure) (*openstack.InfrastructureState, error) {
	var (
		state = &openstack.InfrastructureState{
			Data: map[string]string{},
		}
	)
	// we want to prevent the deletion of Infrastructure CR if there may be still resources in the cloudprovider. We will initialize the data
	// with a specific "marker" so that deletion attempts will not skip the deletion if we are certain that terraform had created infra resources
	// in past reconciliation.
	tf, err := internal.NewTerraformer(f.log, f.restConfig, infrainternal.TerraformerPurpose, infra, f.disableProjectedTokenMount)
	if err != nil {
		return nil, err
	}

	// nothing to do if state is empty
	if tf.IsStateEmpty(ctx) {
		return state, nil
	}

	// this is a special case when migrating from Terraform. If TF had created any resources (meaning there is an actual tf.state written)
	// we mark that there are infra resources created.
	state.Data[infraflow.CreatedResourcesExistKey] = "true"

	return state, infrainternal.PatchProviderStatusAndState(ctx, f.client, infra, nil, &runtime.RawExtension{Object: state})
}
