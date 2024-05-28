// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
)

// Delete the Infrastructure config.
func (a *actuator) Delete(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster) error {
	return util.DetermineError(a.delete(ctx, log, OnDelete, infra, cluster), helper.KnownCodes)
}

// ForceDelete forcefully deletes the Infrastructure.
func (a *actuator) ForceDelete(_ context.Context, _ logr.Logger, _ *extensionsv1alpha1.Infrastructure, _ *extensionscontroller.Cluster) error {
	return nil
}

func (a *actuator) delete(ctx context.Context, log logr.Logger, selectorFn SelectorFunc, infra *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster) error {
	useFlow, err := selectorFn(infra, cluster)
	if err != nil {
		return err
	}

	factory := ReconcilerFactoryImpl{
		log: log,
		a:   a,
	}

	reconciler, err := factory.Build(useFlow)
	if err != nil {
		return err
	}

	return reconciler.Delete(ctx, infra, cluster)
}
