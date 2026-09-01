// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"context"
	"slices"

	"github.com/gardener/gardener/extensions/pkg/controller/bastion"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	controllerconfig "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/config"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
)

var (
	// DefaultAddOptions are the default AddOptions for AddToManager.
	DefaultAddOptions = AddOptions{}

	// supportedExtensionClasses are the extension classes supported by the bastion controller.
	supportedExtensionClasses = sets.New(extensionsv1alpha1.ExtensionClassShoot)
)

// AddOptions are Options to apply when adding the Openstack bastion controller to the manager.
type AddOptions struct {
	// Controller are the controller.Options.
	Controller controller.Options
	// IgnoreOperationAnnotation specifies whether to ignore the operation annotation or not.
	IgnoreOperationAnnotation bool
	// BastionConfig contains config for the Bastion config.
	// Deprecated: Configuring the bastion will be done via CloudProfile in future
	BastionConfig controllerconfig.BastionConfig
	// ExtensionClasses defines the extension classes this extension is responsible for.
	ExtensionClasses []extensionsv1alpha1.ExtensionClass
}

// AddToManagerWithOptions adds a controller with the given Options to the given manager.
// The opts.Reconciler is being set with a newly instantiated actuator.
func AddToManagerWithOptions(mgr manager.Manager, opts AddOptions) error {
	classes := slices.DeleteFunc(opts.ExtensionClasses, func(class extensionsv1alpha1.ExtensionClass) bool {
		return !supportedExtensionClasses.Has(class)
	})
	return bastion.Add(mgr, bastion.AddArgs{
		Actuator:          newActuator(mgr, openstackclient.FactoryFactoryFunc(openstackclient.NewOpenstackClientFromCredentials), &opts.BastionConfig),
		ControllerOptions: opts.Controller,
		Predicates:        bastion.DefaultPredicates(opts.IgnoreOperationAnnotation),
		Type:              openstack.Type,
		ExtensionClasses:  classes,
	})
}

// AddToManager adds a controller with the default Options.
func AddToManager(_ context.Context, mgr manager.Manager) error {
	return AddToManagerWithOptions(mgr, DefaultAddOptions)
}
