// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"context"

	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	machinescheme "github.com/gardener/machine-controller-manager/pkg/client/clientset/versioned/scheme"
	apiextensionsscheme "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/scheme"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
)

var (
	// DefaultAddOptions are the default AddOptions for AddToManager.
	DefaultAddOptions = AddOptions{}
)

// AddOptions are options to apply when adding the OpenStack worker controller to the manager.
type AddOptions struct {
	// Controller are the controller.Options.
	Controller controller.Options
	// IgnoreOperationAnnotation specifies whether to ignore the operation annotation or not.
	IgnoreOperationAnnotation bool
	// GardenCluster is the garden cluster object.
	GardenCluster cluster.Cluster
	// ExtensionClasses defines the extension classes this extension is responsible for.
	ExtensionClasses []extensionsv1alpha1.ExtensionClass
	// SelfHostedShootCluster indicates whether the extension runs in an self-hosted shoot cluster.
	SelfHostedShootCluster bool
}

// AddToManagerWithOptions adds a controller with the given Options to the given manager.
// The opts.Reconciler is being set with a newly instantiated actuator.
func AddToManagerWithOptions(ctx context.Context, mgr manager.Manager, opts AddOptions) error {
	schemeBuilder := runtime.NewSchemeBuilder(
		apiextensionsscheme.AddToScheme,
		machinescheme.AddToScheme,
	)
	if err := schemeBuilder.AddToScheme(mgr.GetScheme()); err != nil {
		return err
	}

	return worker.Add(ctx, mgr, worker.AddArgs{
		Actuator:               NewActuator(mgr, opts.GardenCluster),
		ControllerOptions:      opts.Controller,
		Predicates:             worker.DefaultPredicates(ctx, mgr, opts.IgnoreOperationAnnotation),
		Type:                   openstack.Type,
		ExtensionClasses:       opts.ExtensionClasses,
		SelfHostedShootCluster: opts.SelfHostedShootCluster,
	})
}

// AddToManager adds a controller with the default Options.
func AddToManager(ctx context.Context, mgr manager.Manager) error {
	return AddToManagerWithOptions(ctx, mgr, DefaultAddOptions)
}
