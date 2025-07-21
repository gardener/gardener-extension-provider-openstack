// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupentry

import (
	"context"

	"github.com/gardener/gardener/extensions/pkg/controller/backupentry"
	"github.com/gardener/gardener/extensions/pkg/controller/backupentry/genericactuator"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
)

var (
	// DefaultAddOptions are the default DefaultAddOptions for AddToManager.
	DefaultAddOptions = AddOptions{}
)

// AddOptions are options to apply when adding the Openstack backupbucket controller to the manager.
type AddOptions struct {
	// Controller are the controller.Options.
	Controller controller.Options
	// IgnoreOperationAnnotation specifies whether to ignore the operation annotation or not.
	IgnoreOperationAnnotation bool
	// ExtensionClass defines the extension class this extension is responsible for.
	ExtensionClass extensionsv1alpha1.ExtensionClass
}

// AddToManagerWithOptions adds a controller with the given Options to the given manager.
// The opts.Reconciler is being set with a newly instantiated actuator.
func AddToManagerWithOptions(_ context.Context, mgr manager.Manager, opts AddOptions) error {
	return backupentry.Add(mgr, backupentry.AddArgs{
		Actuator:          genericactuator.NewActuator(mgr, newActuator(mgr)),
		ControllerOptions: opts.Controller,
		Predicates:        backupentry.DefaultPredicates(opts.IgnoreOperationAnnotation),
		Type:              openstack.Type,
		ExtensionClass:    opts.ExtensionClass,
	})
}

// AddToManager adds a controller with the default Options.
func AddToManager(ctx context.Context, mgr manager.Manager) error {
	return AddToManagerWithOptions(ctx, mgr, DefaultAddOptions)
}
