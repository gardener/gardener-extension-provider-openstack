// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"regexp"

	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TODO (georgibaltiev): Remove after the release of version 1.49.0
func purgeMachineControllerManagerRBACResources(ctx context.Context, client client.Client) error {
	var (
		clusterRoleBindingList = &rbacv1.ClusterRoleBindingList{}
		clusterRoleList        = &rbacv1.ClusterRoleList{}
		nameRegex              *regexp.Regexp
	)

	nameRegex, err := regexp.Compile("extensions.gardener.cloud:provider-openstack:shoot--.*:machine-controller-manager")
	if err != nil {
		return fmt.Errorf("failed to compile regex: %w", err)
	}

	if err := client.List(ctx, clusterRoleBindingList); err != nil {
		return fmt.Errorf("failed to list clusterRoleBindings: %w", err)
	}

	for _, clusterRoleBinding := range clusterRoleBindingList.Items {
		if nameRegex.Match([]byte(clusterRoleBinding.Name)) {
			if err := client.Delete(ctx, clusterRoleBinding.DeepCopy()); err != nil {
				return fmt.Errorf("failed to delete clusterRoleBinding: %w", err)
			}
		}
	}

	if err := client.List(ctx, clusterRoleList); err != nil {
		return fmt.Errorf("failed to list clusterRoles: %w", err)
	}

	for _, clusterRole := range clusterRoleList.Items {
		if nameRegex.Match([]byte(clusterRole.Name)) {
			if err := client.Delete(ctx, clusterRole.DeepCopy()); err != nil {
				return fmt.Errorf("failed to delete clusterRole: %w", err)
			}
		}
	}
	return nil
}
