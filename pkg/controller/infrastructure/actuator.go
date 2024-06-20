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
	"github.com/go-logr/logr"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/internal"
	infrainternal "github.com/gardener/gardener-extension-provider-openstack/pkg/internal/infrastructure"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
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

func (a *actuator) cleanupTerraformerResources(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure) error {
	credentials, err := openstack.GetCredentials(ctx, a.client, infra.Spec.SecretRef, false)
	if err != nil {
		return err
	}

	tf, err := internal.NewTerraformerWithAuth(log, a.restConfig, infrainternal.TerraformerPurpose, infra, credentials, a.disableProjectedTokenMount)
	if err != nil {
		return fmt.Errorf("could not create terraformer object: %w", err)
	}

	return CleanupTerraformerResources(ctx, tf)
}

// CleanupTerraformerResources deletes terraformer artifacts (config, state, secrets).
func CleanupTerraformerResources(ctx context.Context, tf terraformer.Terraformer) error {
	if err := tf.EnsureCleanedUp(ctx); err != nil {
		return nil
	}
	if err := tf.CleanupConfiguration(ctx); err != nil {
		return err
	}
	return tf.RemoveTerraformerFinalizerFromConfig(ctx)
}
