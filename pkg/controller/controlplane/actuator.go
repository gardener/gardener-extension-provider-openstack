// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"context"
	"fmt"

	extensionsconfigv1alpha1 "github.com/gardener/gardener/extensions/pkg/apis/config/v1alpha1"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/controlplane"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	networking "github.com/gardener/gardener-extension-provider-openstack/pkg/utils/networking"
)

const (
	// NetworkUnavailableConditionType is the type of the NetworkUnavailable condition.
	NetworkUnavailableConditionType = "NetworkUnavailable"
	// CalicoIsUpReason is the reason set by Calico when it sets the NetworkUnavailable condition to indicate Calico is up.
	CalicoIsUpReason = "CalicoIsUp"
	// CalicoIsDownReason is the reason set by Calico when it sets the NetworkUnavailable condition to indicate Calico is down.
	CalicoIsDownReason = "CalicoIsDown"
	// AnnotationCalicoCleanupCompleted indicates that Calico condition cleanup has been completed.
	AnnotationCalicoCleanupCompleted = "openstack.provider.extensions.gardener.cloud/calico-cleanup-completed"
)

// NewActuator creates a new Actuator that wraps the generic actuator and adds cleanup logic.
func NewActuator(mgr manager.Manager, a controlplane.Actuator) controlplane.Actuator {
	return &actuator{
		Actuator: a,
		client:   mgr.GetClient(),
	}
}

// actuator is an Actuator that acts upon and updates the status of ControlPlane resources.
type actuator struct {
	controlplane.Actuator
	client client.Client
}

func (a *actuator) Reconcile(
	ctx context.Context,
	log logr.Logger,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) (bool, error) {
	ok, err := a.Actuator.Reconcile(ctx, log, cp, cluster)
	if err != nil {
		return ok, err
	}

	overlayEnabled, err := networking.IsOverlayEnabled(cluster.Shoot.Spec.Networking)
	if err != nil {
		log.Error(err, "Failed to determine if overlay is enabled")
		return ok, err
	}

	// Clean up NetworkUnavailable conditions set by Calico only when overlay is disabled
	// Only run cleanup if it hasn't been completed yet (annotation not present)
	if !overlayEnabled && cp.Annotations[AnnotationCalicoCleanupCompleted] != "true" {
		if err := a.cleanupCalicoNetworkUnavailableConditions(ctx, log, cp.Namespace, cluster); err != nil {
			log.Error(err, "Failed to cleanup Calico NetworkUnavailable conditions")
			return ok, err
		} else {
			// Mark cleanup as completed
			if err := a.markCleanupCompleted(ctx, cp); err != nil {
				log.Error(err, "Failed to mark cleanup as completed")
				return ok, err
			}
		}
	}

	// Remove cleanup annotation when overlay is enabled so cleanup can run again if overlay is disabled later
	if overlayEnabled && cp.Annotations[AnnotationCalicoCleanupCompleted] == "true" {
		if err := a.removeCleanupAnnotation(ctx, cp); err != nil {
			log.Error(err, "Failed to remove cleanup annotation")
			return ok, err
		}
	}

	return ok, nil
}

// cleanupCalicoNetworkUnavailableConditions removes NetworkUnavailable conditions from nodes
// that were set by Calico for example "CalicoIsUp" or "CalicoIsDown".
func (a *actuator) cleanupCalicoNetworkUnavailableConditions(
	ctx context.Context,
	log logr.Logger,
	namespace string,
	cluster *extensionscontroller.Cluster,
) error {
	if extensionscontroller.IsHibernated(cluster) {
		return nil
	}

	_, shootClient, err := util.NewClientForShoot(ctx, a.client, namespace, client.Options{}, extensionsconfigv1alpha1.RESTOptions{})
	if err != nil {
		return fmt.Errorf("could not create shoot client: %w", err)
	}

	nodes := &corev1.NodeList{}
	if err := shootClient.List(ctx, nodes); err != nil {
		return fmt.Errorf("could not list nodes in shoot cluster: %w", err)
	}

	for _, node := range nodes.Items {
		if err := a.cleanupNodeNetworkUnavailableCondition(ctx, log, shootClient, &node); err != nil {
			log.Error(err, "Failed to cleanup NetworkUnavailable condition from node", "node", node.Name)
			return err
		}
	}

	return nil
}

// cleanupNodeNetworkUnavailableCondition removes the NetworkUnavailable condition from a node
// if it was set by Calico.
func (a *actuator) cleanupNodeNetworkUnavailableCondition(
	ctx context.Context,
	log logr.Logger,
	shootClient client.Client,
	node *corev1.Node,
) error {
	// Check if the node has a NetworkUnavailable condition set by Calico
	hasCondition := false
	for _, condition := range node.Status.Conditions {
		if condition.Type == NetworkUnavailableConditionType &&
			(condition.Reason == CalicoIsUpReason || condition.Reason == CalicoIsDownReason) {
			hasCondition = true
			break
		}
	}

	if !hasCondition {
		return nil
	}

	// Remove the NetworkUnavailable condition
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Get the latest version of the node
		currentNode := &corev1.Node{}
		if err := shootClient.Get(ctx, client.ObjectKey{Name: node.Name}, currentNode); err != nil {
			return err
		}

		// Filter out the NetworkUnavailable condition set by Calico
		var newConditions []corev1.NodeCondition
		removed := false
		for _, condition := range currentNode.Status.Conditions {
			if condition.Type == NetworkUnavailableConditionType &&
				(condition.Reason == CalicoIsUpReason || condition.Reason == CalicoIsDownReason) {
				removed = true
				log.Info("Removing NetworkUnavailable condition set by Calico", "node", currentNode.Name, "reason", condition.Reason)
				continue
			}
			newConditions = append(newConditions, condition)
		}

		// Only update if we actually removed a condition
		if !removed {
			return nil
		}

		currentNode.Status.Conditions = newConditions
		return shootClient.Status().Update(ctx, currentNode)
	})
}

// markCleanupCompleted marks the cleanup as completed by adding an annotation to the ControlPlane resource.
func (a *actuator) markCleanupCompleted(ctx context.Context, cp *extensionsv1alpha1.ControlPlane) error {
	patch := client.MergeFrom(cp.DeepCopy())
	if cp.Annotations == nil {
		cp.Annotations = make(map[string]string)
	}
	cp.Annotations[AnnotationCalicoCleanupCompleted] = "true"
	return a.client.Patch(ctx, cp, patch)
}

// removeCleanupAnnotation removes the cleanup completion annotation from the ControlPlane resource.
func (a *actuator) removeCleanupAnnotation(ctx context.Context, cp *extensionsv1alpha1.ControlPlane) error {
	patch := client.MergeFrom(cp.DeepCopy())
	delete(cp.Annotations, AnnotationCalicoCleanupCompleted)
	return a.client.Patch(ctx, cp, patch)
}
