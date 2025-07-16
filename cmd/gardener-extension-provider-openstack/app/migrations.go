// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"regexp"

	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/go-logr/logr"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var nameRegex = regexp.MustCompile("extensions.gardener.cloud:provider-openstack:shoot--.*:machine-controller-manager")

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
