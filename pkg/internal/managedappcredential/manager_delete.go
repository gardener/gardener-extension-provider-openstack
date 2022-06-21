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

	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
)

// Delete deletes the managed application credentials of an Openstack Shoot cluster.
func (m *Manager) Delete(ctx context.Context, credentials *openstack.Credentials) error {
	newParentUser, err := m.newParentFromCredentials(credentials)
	if err != nil {
		return err
	}

	appCredential, err := readApplicationCredential(ctx, m.client, m.namespace)
	if err != nil {
		return err
	}

	if appCredential == nil {
		return m.runGarbageCollection(ctx, newParentUser, nil)
	}

	var oldParentUser *parent
	if user, err := m.newParentFromSecret(appCredential.secret); err == nil && user != nil {
		oldParentUser = user
	}

	if oldParentUser != nil && (newParentUser.id != oldParentUser.id) {
		// Try to clean up the application credentials owned by the old parent user.
		// This might not work as the information about this user could be stale,
		// because the user credentials are rotated, the user is not associated to
		// Openstack project anymore or it is deleted.
		if err := m.runGarbageCollection(ctx, oldParentUser, nil); err != nil {
			m.logger.Error(err, "could not clean up application credential(s) as the owning user has changed and information about owning user might be stale")
		}

		return m.removeApplicationCredentialStore(ctx, appCredential.secret)
	}

	if newParentUser.isApplicationCredential() {
		return m.removeApplicationCredentialStore(ctx, appCredential.secret)
	}

	if err := m.runGarbageCollection(ctx, newParentUser, &appCredential.id); err != nil {
		return err
	}

	return m.removeApplicationCredentialStore(ctx, appCredential.secret)
}
