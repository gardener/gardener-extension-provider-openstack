// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"
	"fmt"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// cleanupTerraformerResources removes artifacts left behind when infrastructure state was migrated.
// Their finalizers must be removed so that they cannot block deletion of the shoot namespace.
func (a *actuator) cleanupTerraformerResources(ctx context.Context, infra *extensionsv1alpha1.Infrastructure) error {
	const finalizer = "gardener.cloud/terraformer"

	for _, obj := range []client.Object{
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: infra.Namespace, Name: infra.Name + ".infra.tf-state"}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: infra.Namespace, Name: infra.Name + ".infra.tf-vars"}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: infra.Namespace, Name: infra.Name + ".infra.tf-config"}},
	} {
		key := client.ObjectKeyFromObject(obj)
		if err := a.client.Get(ctx, key, obj); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("failed to get legacy Terraformer resource %s: %w", key, err)
		}

		if controllerutil.ContainsFinalizer(obj, finalizer) {
			if err := controllerutils.RemoveFinalizers(ctx, a.client, obj, finalizer); err != nil {
				return fmt.Errorf("failed to remove finalizer from legacy Terraformer resource %s: %w", key, err)
			}
		}
		if err := client.IgnoreNotFound(a.client.Delete(ctx, obj)); err != nil {
			return fmt.Errorf("failed to delete legacy Terraformer resource %s: %w", key, err)
		}
	}
	return nil
}
