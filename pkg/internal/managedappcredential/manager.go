// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package managedappcredential

import (
	"context"
	"fmt"
	"strings"

	controllerconfig "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/config"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewManager returns a new manager to manage the lifecycle of
// the managed appplication credentials of an Openstack Shoot cluster.
func NewManager(openstackClientFactory openstackclient.FactoryFactory, config *controllerconfig.ApplicationCredentialConfig, client client.Client, namespace, identifier, finalizerName string, logger logr.Logger) *Manager {
	return &Manager{
		client:                 client,
		config:                 config,
		finalizerName:          finalizerName,
		identifier:             identifier,
		logger:                 logger,
		namespace:              namespace,
		openstackClientFactory: openstackClientFactory,
	}
}

func (m *Manager) runGarbageCollection(ctx context.Context, parent *parent, inUseAppCredentialID string, deleteInUseAppCredential bool) error {
	appCredentialList, err := parent.identityClient.ListApplicationCredentials(ctx, parent.id)
	if err != nil {
		return err
	}

	var errorList []error
	for _, ac := range appCredentialList {
		// Ignore application credentials which name is not matching to the managers identifier.
		if !strings.HasPrefix(ac.Name, m.identifier) {
			continue
		}

		// Skip the is-use application credential.
		if ac.ID == inUseAppCredentialID && !deleteInUseAppCredential {
			continue
		}

		if err := parent.identityClient.DeleteApplicationCredential(ctx, parent.id, ac.ID); err != nil {
			errorList = append(errorList, fmt.Errorf("could not delete application credential %q owned by user %q: %w", ac.ID, parent.id, err))
		}
	}

	return errors.Flatten(errors.NewAggregate(errorList))
}
