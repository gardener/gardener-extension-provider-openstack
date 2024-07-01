// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"

	"github.com/gardener/gardener/extensions/pkg/controller/infrastructure"
	"github.com/gardener/gardener/extensions/pkg/terraformer"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
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
