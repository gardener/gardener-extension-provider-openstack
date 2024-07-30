// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cloudprovider

import (
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/extensions/pkg/webhook/cloudprovider"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
)

var (
	logger = log.Log.WithName("openstack-cloudprovider-webhook")
)

// AddToManager creates the cloudprovider webhook and adds it to the manager.
func AddToManager(mgr manager.Manager) (*extensionswebhook.Webhook, error) {
	logger.Info("Adding webhook to manager")
	return cloudprovider.New(mgr, cloudprovider.Args{
		Provider: openstack.Type,
		Mutator:  cloudprovider.NewMutator(mgr, logger, NewEnsurer(mgr, logger)),
	})
}
