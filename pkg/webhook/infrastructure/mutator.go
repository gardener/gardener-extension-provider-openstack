// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"
	"fmt"
	"strings"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	extensionscontextwebhook "github.com/gardener/gardener/extensions/pkg/webhook/context"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
)

type mutator struct {
	logger logr.Logger
	client client.Client
}

// New returns a new Infrastructure mutator that uses mutateFunc to perform the mutation.
func New(mgr manager.Manager, logger logr.Logger) extensionswebhook.Mutator {
	return &mutator{
		client: mgr.GetClient(),
		logger: logger,
	}
}

// Mutate mutates the given object on creation and adds the annotation `openstack.provider.extensions.gardener.cloud/use-flow=true`
// if the seed has the label `openstack.provider.extensions.gardener.cloud/use-flow` == `new`.
func (m *mutator) Mutate(ctx context.Context, newObj, _ client.Object) error {
	if newObj.GetDeletionTimestamp() != nil {
		return nil
	}

	newInfra, ok := newObj.(*extensionsv1alpha1.Infrastructure)
	if !ok {
		return fmt.Errorf("could not mutate: object is not of type Infrastructure")
	}

	if m.isInMigrationOrRestorePhase(newInfra) {
		return nil
	}

	gctx := extensionscontextwebhook.NewGardenContext(m.client, newObj)
	cluster, err := gctx.GetCluster(ctx)
	if err != nil {
		return err
	}

	// skip if shoot is being deleted
	if cluster.Shoot.DeletionTimestamp != nil {
		return nil
	}

	if newInfra.Annotations == nil {
		newInfra.Annotations = map[string]string{}
	}
	if cluster.Seed.Annotations == nil {
		cluster.Seed.Annotations = map[string]string{}
	}
	if cluster.Shoot.Annotations == nil {
		cluster.Shoot.Annotations = map[string]string{}
	}

	mutated := false
	if v, ok := cluster.Shoot.Annotations[openstack.GlobalAnnotationKeyUseFlow]; ok {
		newInfra.Annotations[openstack.AnnotationKeyUseFlow] = v
		mutated = true
	} else if v, ok := cluster.Shoot.Annotations[openstack.AnnotationKeyUseFlow]; ok {
		newInfra.Annotations[openstack.AnnotationKeyUseFlow] = v
		mutated = true
	} else if v := cluster.Seed.Annotations[openstack.SeedAnnotationKeyUseFlow]; !strings.EqualFold(v, "false") {
		newInfra.Annotations[openstack.AnnotationKeyUseFlow] = "true"
		mutated = true
	}
	if mutated {
		extensionswebhook.LogMutation(logger, newInfra.Kind, newInfra.Namespace, newInfra.Name)
	}

	return nil
}

func (m *mutator) isInMigrationOrRestorePhase(infra *extensionsv1alpha1.Infrastructure) bool {
	// During the restore phase, the infrastructure object is created without status or state (including the operation type information).
	// Instead, the object is annotated to indicate that the status and state information will be patched in a subsequent operation.
	// Therefore, while the GardenerOperationWaitForState  annotation exists, do nothing.
	if infra.GetAnnotations()[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationWaitForState {
		return true
	}

	operationType := v1beta1helper.ComputeOperationType(infra.ObjectMeta, infra.Status.LastOperation)
	return operationType == v1beta1.LastOperationTypeMigrate || operationType == v1beta1.LastOperationTypeRestore
}
