// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var nameRegex = regexp.MustCompile("extensions.gardener.cloud:provider-openstack:shoot-.*:machine-controller-manager")

// TODO (georgibaltiev): Remove after the release of version 1.50.0
func purgeMachineControllerManagerRBACResources(ctx context.Context, c client.Client, log logr.Logger) error {
	log.Info("Starting the deletion of obsolete ClusterRoles and ClusterRoleBindings")

	var (
		clusterRoleBindingList = &rbacv1.ClusterRoleBindingList{}
		clusterRoleList        = &rbacv1.ClusterRoleList{}
	)

	if err := c.List(ctx, clusterRoleBindingList); err != nil {
		return fmt.Errorf("failed to list ClusterRoleBindings: %w", err)
	}

	for _, clusterRoleBinding := range clusterRoleBindingList.Items {
		if nameRegex.Match([]byte(clusterRoleBinding.Name)) {
			log.Info("Deleting ClusterRoleBinding", "clusterRoleBinding", client.ObjectKeyFromObject(&clusterRoleBinding))
			if err := kutil.DeleteObject(
				ctx,
				c,
				&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleBinding.Name}},
			); err != nil {
				return fmt.Errorf("failed to delete ClusterRoleBinding %s: %w", client.ObjectKeyFromObject(&clusterRoleBinding), err)
			}
		}
	}

	if err := c.List(ctx, clusterRoleList); err != nil {
		return fmt.Errorf("failed to list ClusterRoles: %w", err)
	}

	for _, clusterRole := range clusterRoleList.Items {
		if nameRegex.Match([]byte(clusterRole.Name)) {
			log.Info("Deleting ClusterRole", "clusterRole", client.ObjectKeyFromObject(&clusterRole))
			if err := kutil.DeleteObject(
				ctx,
				c,
				&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: clusterRole.Name}},
			); err != nil {
				return fmt.Errorf("failed to delete ClusterRole %s: %w", client.ObjectKeyFromObject(&clusterRole), err)
			}
		}
	}

	log.Info("Successfully deleted the obsolete ClusterRoles and ClusterRoleBindings")
	return nil
}

// TODO (kon-angelo): Remove after the release of version 1.46.0
func purgeTerraformerRBACResources(ctx context.Context, c client.Client, log logr.Logger) error {
	log.Info("Starting the deletion of obsolete terraformer resources")

	const (
		terraformerRoleName = "gardener.cloud:system:terraformer"
	)

	var (
		roleBindingList    = &rbacv1.RoleBindingList{}
		roleList           = &rbacv1.RoleList{}
		serviceAccountList = &corev1.ServiceAccountList{}
	)

	// list serviceAccount bindings in all namespaces
	if err := c.List(ctx, roleBindingList); err != nil {
		return fmt.Errorf("failed to list RoleBindings: %w", err)
	}

	for _, roleBinding := range roleBindingList.Items {
		if strings.EqualFold(roleBinding.Name, terraformerRoleName) {
			log.Info("Deleting RoleBinding", "roleBinding", client.ObjectKeyFromObject(&roleBinding))
			if err := kutil.DeleteObject(
				ctx,
				c,
				&rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Namespace: roleBinding.Namespace, Name: roleBinding.Name}},
			); err != nil {
				return fmt.Errorf("failed to delete roleBinding %s: %w", client.ObjectKeyFromObject(&roleBinding), err)
			}
		}
	}
	log.Info("Successfully deleted the obsolete RoleBindings for terraformer")

	if err := c.List(ctx, roleList); err != nil {
		return fmt.Errorf("failed to list roles: %w", err)
	}

	for _, role := range roleList.Items {
		if strings.EqualFold(role.Name, terraformerRoleName) {
			log.Info("Deleting Role", "role", client.ObjectKeyFromObject(&role))
			if err := kutil.DeleteObject(
				ctx,
				c,
				&rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Namespace: role.Namespace, Name: role.Name}},
			); err != nil {
				return fmt.Errorf("failed to delete Role %s: %w", client.ObjectKeyFromObject(&role), err)
			}
		}
	}
	log.Info("Successfully deleted the obsolete Roles for terraformer")

	if err := c.List(ctx, serviceAccountList); err != nil {
		return fmt.Errorf("failed to list roles: %w", err)
	}

	for _, serviceAccount := range serviceAccountList.Items {
		if strings.EqualFold(serviceAccount.Name, "terraformer") {
			log.Info("Deleting ServiceAccount", "serviceAccount", client.ObjectKeyFromObject(&serviceAccount))
			if err := kutil.DeleteObject(
				ctx,
				c,
				&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: serviceAccount.Namespace, Name: serviceAccount.Name}},
			); err != nil {
				return fmt.Errorf("failed to delete ServiceAccount %s: %w", client.ObjectKeyFromObject(&serviceAccount), err)
			}
		}
	}
	log.Info("Successfully deleted the obsolete ServiceAccounts for terraformer")

	return nil
}
