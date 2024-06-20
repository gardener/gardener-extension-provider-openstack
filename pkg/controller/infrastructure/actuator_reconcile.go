// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
)

// Reconcile the Infrastructure config.
func (a *actuator) Reconcile(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	return util.DetermineError(a.reconcile(ctx, log, OnReconcile, infra, cluster), helper.KnownCodes)
}

func (a *actuator) reconcile(ctx context.Context, logger logr.Logger, selectorFn SelectorFunc, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	useFlow, err := selectorFn(infra, cluster)
	if err != nil {
		return err
	}

	factory := ReconcilerFactoryImpl{
		log: logger,
		a:   a,
	}

	reconciler, err := factory.Build(useFlow)
	if err != nil {
		return err
	}
	return reconciler.Reconcile(ctx, infra, cluster)
}
