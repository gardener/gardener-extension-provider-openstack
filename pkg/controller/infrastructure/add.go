// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"
	"slices"

	"github.com/gardener/gardener/extensions/pkg/controller/infrastructure"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"k8s.io/apimachinery/pkg/util/sets"
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

	// supportedExtensionClasses are the extension classes supported by the infrastructure controller.
	supportedExtensionClasses = sets.New(extensionsv1alpha1.ExtensionClassShoot)
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
	classes := slices.DeleteFunc(options.ExtensionClasses, func(class extensionsv1alpha1.ExtensionClass) bool {
		return !supportedExtensionClasses.Has(class)
	})
	if len(classes) == 0 {
		log.Log.Info("No supported extension classes left after filtering, skipping infrastructure controller registration")
		return nil
	}

	return infrastructure.Add(mgr, infrastructure.AddArgs{
		Actuator:          NewActuator(mgr, true),
		ConfigValidator:   NewConfigValidator(mgr, openstackclient.FactoryFactoryFunc(openstackclient.NewOpenstackClientFromCredentials), log.Log),
		ControllerOptions: options.Controller,
		Predicates:        infrastructure.DefaultPredicates(ctx, mgr, options.IgnoreOperationAnnotation),
		Type:              openstack.Type,
		KnownCodes:        helper.KnownCodes,
		ExtensionClasses:  classes,
	})
}

// AddToManager adds a controller with the default AddOptions.
func AddToManager(ctx context.Context, mgr manager.Manager) error {
	return AddToManagerWithOptions(ctx, mgr, DefaultAddOptions)
}
