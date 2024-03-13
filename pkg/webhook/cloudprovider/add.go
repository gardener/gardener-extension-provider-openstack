// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cloudprovider

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/extensions/pkg/webhook/cloudprovider"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
)

var (
	logger                           = log.Log.WithName("openstack-cloudprovider-webhook")
	versionConstraintGreaterEqual142 *semver.Constraints
)

func init() {
	var err error
	versionConstraintGreaterEqual142, err = semver.NewConstraint(">= 1.42")
	utilruntime.Must(err)
}

// AddToManager creates the cloudprovider webhook and adds it to the manager.
func AddToManager(gardenerVersion *string) func(mgr manager.Manager) (*extensionswebhook.Webhook, error) {
	return func(mgr manager.Manager) (*extensionswebhook.Webhook, error) {
		enable := false
		if gardenerVersion != nil && len(*gardenerVersion) > 0 {
			version, err := semver.NewVersion(*gardenerVersion)
			if err != nil {
				return nil, fmt.Errorf("failed to parse gardener version: %v", err)
			}
			if versionConstraintGreaterEqual142.Check(version) {
				enable = true
			}
		}
		logger.Info("adding webhook to manager")
		return cloudprovider.New(mgr, cloudprovider.Args{
			Provider:             openstack.Type,
			Mutator:              cloudprovider.NewMutator(mgr, logger, NewEnsurer(mgr, logger)),
			EnableObjectSelector: enable,
		})
	}
}
