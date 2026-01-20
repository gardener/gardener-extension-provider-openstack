// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seedprovider

import (
	druidcorev1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/genericmutator"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/config"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
)

var (
	// DefaultAddOptions are the default AddOptions for AddToManager.
	DefaultAddOptions = AddOptions{}
)

// AddOptions are options to apply when adding the Openstack seedprovider webhook to the manager.
type AddOptions struct {
	// ETCDStorage is the etcd storage configuration.
	ETCDStorage       config.ETCDStorage
	ETCDEventsStorage config.ETCDStorage
}

var logger = log.Log.WithName("openstack-seedprovider-webhook")

// AddToManagerWithOptions creates a webhook with the given options and adds it to the manager.
func AddToManagerWithOptions(mgr manager.Manager, opts AddOptions) (*extensionswebhook.Webhook, error) {
	logger.Info("Adding webhook to manager")
	return controlplane.New(mgr, controlplane.Args{
		Kind:     controlplane.KindSeed,
		Provider: openstack.Type,
		Types: []extensionswebhook.Type{
			{Obj: &druidcorev1alpha1.Etcd{}},
		},
		Mutator: genericmutator.NewMutator(mgr, NewEnsurer(&opts.ETCDStorage, &opts.ETCDEventsStorage, logger), nil, nil, nil, logger),
	})
}

// AddToManager creates a webhook with the default options and adds it to the manager.
func AddToManager(mgr manager.Manager) (*extensionswebhook.Webhook, error) {
	return AddToManagerWithOptions(mgr, DefaultAddOptions)
}
