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
	"fmt"
	"strings"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/terraformer"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	openstackv1alpha1 "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/controller/infrastructure/infraflow"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/controller/infrastructure/infraflow/shared"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/internal"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/internal/infrastructure"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
)

func (a *actuator) Reconcile(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster) error {
	flowState, err := a.getStateFromInfraStatus(ctx, infra)
	if err != nil {
		return err
	}
	if flowState != nil {
		return a.reconcileWithFlow(ctx, log, infra, cluster, flowState)
	}
	if a.shouldUseFlow(infra, cluster) {
		flowState, err = a.migrateFromTerraformerState(ctx, log, infra)
		if err != nil {
			return err
		}
		return a.reconcileWithFlow(ctx, log, infra, cluster, flowState)
	}
	return a.reconcileWithTerraformer(ctx, log, infra, cluster, terraformer.StateConfigMapInitializerFunc(terraformer.CreateState))
}

func (a *actuator) shouldUseFlow(infrastructure *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster) bool {
	return (infrastructure.Annotations != nil && strings.EqualFold(infrastructure.Annotations[AnnotationKeyUseFlow], "true")) ||
		(cluster.Shoot != nil && cluster.Shoot.Annotations != nil && strings.EqualFold(cluster.Shoot.Annotations[AnnotationKeyUseFlow], "true"))
}

func (a *actuator) getStateFromInfraStatus(_ context.Context, infrastructure *extensionsv1alpha1.Infrastructure) (*infraflow.PersistentState, error) {
	if infrastructure.Status.State != nil {
		return infraflow.NewPersistentStateFromJSON(infrastructure.Status.State.Raw)
	}
	return nil, nil
}

func (a *actuator) migrateFromTerraformerState(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure) (*infraflow.PersistentState, error) {
	log.Info("starting terraform state migration")

	// just explore the infrastructure objects by starting with an empty state
	state := infraflow.NewPersistentState()

	if err := a.updateStatusState(ctx, infra, state); err != nil {
		return nil, fmt.Errorf("updating status state failed: %w", err)
	}
	log.Info("terraform state migrated successfully")

	return state, nil
}

func (a *actuator) reconcileWithFlow(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure,
	cluster *extensionscontroller.Cluster, oldState *infraflow.PersistentState) error {

	log.Info("reconcileWithFlow")

	flowContext, err := a.createFlowContext(ctx, log, infra, cluster, oldState)
	if err != nil {
		return err
	}
	if err = flowContext.Reconcile(ctx); err != nil {
		_ = flowContext.PersistState(ctx, true)
		return util.DetermineError(err, helper.KnownCodes)
	}
	return flowContext.PersistState(ctx, true)
}

func (a *actuator) createFlowContext(ctx context.Context, log logr.Logger,
	infra *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster,
	oldState *infraflow.PersistentState) (*infraflow.FlowContext, error) {

	cloudProfileConfig, err := helper.CloudProfileConfigFromCluster(cluster)
	if err != nil {
		return nil, err
	}

	if oldState.MigratedFromTerraform() && !oldState.TerraformCleanedUp() {
		err := a.cleanupTerraformerResources(ctx, log, infra)
		if err != nil {
			return nil, fmt.Errorf("cleaning up terraformer resources failed: %w", err)
		}
		oldState.SetTerraformCleanedUp()
		if err := a.updateStatusState(ctx, infra, oldState); err != nil {
			return nil, fmt.Errorf("updating status state failed: %w", err)
		}
	}

	config, err := helper.InfrastructureConfigFromInfrastructure(infra)
	if err != nil {
		return nil, err
	}

	credentials, err := openstack.GetCredentials(ctx, a.client, infra.Spec.SecretRef, false)
	if err != nil {
		return nil, fmt.Errorf("could not get Openstack credentials: %w", err)
	}
	clientFactory, err := openstackclient.NewOpenstackClientFromCredentials(credentials)
	if err != nil {
		return nil, err
	}

	infraObjectKey := client.ObjectKey{
		Namespace: infra.Namespace,
		Name:      infra.Name,
	}
	persistor := func(ctx context.Context, flatState shared.FlatMap) error {
		state := infraflow.NewPersistentStateFromFlatMap(flatState)
		infra := &extensionsv1alpha1.Infrastructure{}
		if err := a.client.Get(ctx, infraObjectKey, infra); err != nil {
			return err
		}
		return a.updateStatusState(ctx, infra, state)
	}

	var oldFlatState shared.FlatMap
	if oldState != nil {
		if valid, err := oldState.HasValidVersion(); !valid {
			return nil, err
		}
		oldFlatState = oldState.ToFlatMap()
	}

	return infraflow.NewFlowContext(log, clientFactory, infra, config, cloudProfileConfig, oldFlatState, persistor)
}

func (a *actuator) updateStatusState(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, state *infraflow.PersistentState) error {
	status, err := computeProviderStatusFromFlowState(state)
	if err != nil {
		return err
	}

	stateBytes, err := state.ToJSON()
	if err != nil {
		return err
	}

	return a.updateProviderStatus(ctx, infra, status, stateBytes)
}

func (a *actuator) cleanupTerraformerResources(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure) error {
	credentials, err := openstack.GetCredentials(ctx, a.client, infra.Spec.SecretRef, false)
	if err != nil {
		return err
	}

	tf, err := internal.NewTerraformerWithAuth(log, a.restConfig, infrastructure.TerraformerPurpose, infra, credentials, a.disableProjectedTokenMount)
	if err != nil {
		return fmt.Errorf("could not create terraformer object: %w", err)
	}

	if err := tf.CleanupConfiguration(ctx); err != nil {
		return err
	}
	return tf.RemoveTerraformerFinalizerFromConfig(ctx)
}

func (a *actuator) reconcileWithTerraformer(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster, stateInitializer terraformer.StateConfigMapInitializer) error {
	config, err := helper.InfrastructureConfigFromInfrastructure(infra)
	if err != nil {
		return err
	}

	terraformFiles, err := infrastructure.RenderTerraformerTemplate(infra, config, cluster)
	if err != nil {
		return err
	}

	// need to know if application credentials are used
	credentials, err := openstack.GetCredentials(ctx, a.client, infra.Spec.SecretRef, false)
	if err != nil {
		return err
	}

	tf, err := internal.NewTerraformerWithAuth(log, a.restConfig, infrastructure.TerraformerPurpose, infra, credentials, a.disableProjectedTokenMount)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	if err := tf.
		InitializeWith(ctx, terraformer.DefaultInitializer(a.client, terraformFiles.Main, terraformFiles.Variables, terraformFiles.TFVars, stateInitializer)).
		Apply(ctx); err != nil {

		return util.DetermineError(fmt.Errorf("failed to apply the terraform config: %w", err), helper.KnownCodes)
	}

	return a.updateProviderStatusWithTerraformer(ctx, tf, infra, config)
}

func computeProviderStatusFromFlowState(state *infraflow.PersistentState) (*openstackv1alpha1.InfrastructureStatus, error) {
	if len(state.Data) == 0 {
		return nil, nil
	}
	status := &openstackv1alpha1.InfrastructureStatus{
		TypeMeta: metav1.TypeMeta{
			APIVersion: openstackv1alpha1.SchemeGroupVersion.String(),
			Kind:       "InfrastructureStatus",
		},
	}

	status.Networks.ID = shared.ValidValue(state.Data[infraflow.IdentifierNetwork])
	status.Networks.Name = shared.ValidValue(state.Data[infraflow.NameNetwork])
	status.Networks.Router.ID = shared.ValidValue(state.Data[infraflow.IdentifierRouter])
	status.Networks.Router.IP = shared.ValidValue(state.Data[infraflow.RouterIP])
	status.Networks.FloatingPool.ID = shared.ValidValue(state.Data[infraflow.IdentifierFloatingNetwork])
	status.Networks.FloatingPool.Name = shared.ValidValue(state.Data[infraflow.NameFloatingNetwork])
	if v := shared.ValidValue(state.Data[infraflow.IdentifierShareNetwork]); v != "" {
		status.Networks.ShareNetwork = &openstackv1alpha1.ShareNetworkStatus{
			ID:   v,
			Name: shared.ValidValue(state.Data[infraflow.NameShareNetwork]),
		}
	}

	subnetID := shared.ValidValue(state.Data[infraflow.IdentifierSubnet])
	if subnetID != "" {
		status.Networks.Subnets = []openstackv1alpha1.Subnet{
			{
				Purpose: openstackv1alpha1.PurposeNodes,
				ID:      subnetID,
			},
		}
	}

	secGroupID := shared.ValidValue(state.Data[infraflow.IdentifierSecGroup])
	if secGroupID != "" {
		status.SecurityGroups = []openstackv1alpha1.SecurityGroup{
			{
				Purpose: openstackv1alpha1.PurposeNodes,
				ID:      secGroupID,
				Name:    shared.ValidValue(state.Data[infraflow.NameSecGroup]),
			},
		}
	}

	status.Node.KeyName = shared.ValidValue(state.Data[infraflow.NameKeyPair])

	return status, nil
}
