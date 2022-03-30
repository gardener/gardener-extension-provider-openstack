// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cloudprovider

import (
	"fmt"

	"github.com/Masterminds/semver"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/extensions/pkg/webhook/cloudprovider"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
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
			Mutator:              cloudprovider.NewMutator(logger, NewEnsurer(logger)),
			EnableObjectSelector: enable,
		})
	}
}
