// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"

	"github.com/gardener/gardener/extensions/pkg/controller/infrastructure"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
)

var (
	// DefaultAddOptions are the default AddOptions for AddToManager.
	DefaultAddOptions = AddOptions{}
)

// AddOptions are options to apply when adding the OpenStack infrastructure controller to the manager.
type AddOptions struct {
	// Controller are the controller.Options.
	Controller controller.Options
	// IgnoreOperationAnnotation specifies whether to ignore the operation annotation or not.
	IgnoreOperationAnnotation bool
	// DisableProjectedTokenMount specifies whether the projected token mount shall be disabled for the terraformer.
	// Used for testing only.
	DisableProjectedTokenMount bool
	// ExtensionClasses defines the extension classes this extension is responsible for.
	ExtensionClasses []extensionsv1alpha1.ExtensionClass
}

// AddToManagerWithOptions adds a controller with the given AddOptions to the given manager.
// The opts.Reconciler is being set with a newly instantiated actuator.
func AddToManagerWithOptions(ctx context.Context, mgr manager.Manager, options AddOptions) error {
	return infrastructure.Add(mgr, infrastructure.AddArgs{
		Actuator:          NewActuator(mgr, true),
		ConfigValidator:   NewConfigValidator(mgr, openstackclient.FactoryFactoryFunc(openstackclient.NewOpenstackClientFromCredentials), log.Log),
		ControllerOptions: options.Controller,
		Predicates:        infrastructure.DefaultPredicates(ctx, mgr, options.IgnoreOperationAnnotation),
		Type:              openstack.Type,
		KnownCodes:        helper.KnownCodes,
		ExtensionClasses:  options.ExtensionClasses,
	})
}

// AddToManager adds a controller with the default AddOptions.
func AddToManager(ctx context.Context, mgr manager.Manager) error {
	return AddToManagerWithOptions(ctx, mgr, DefaultAddOptions)
}
