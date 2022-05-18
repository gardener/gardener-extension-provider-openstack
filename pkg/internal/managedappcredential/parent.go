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
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"

	corev1 "k8s.io/api/core/v1"
)

func (m *Manager) newParentFromCredetials(credentials *openstack.Credentials) (*parent, error) {
	factory, err := m.openstackClientFactory.NewFactory(credentials)
	if err != nil {
		return nil, err
	}
	identityClient, err := factory.Identity()
	if err != nil {
		return nil, err
	}

	parentToken, err := identityClient.GetClientUser()
	if err != nil {
		return nil, err
	}

	return &parent{
		id:     parentToken.ID,
		name:   credentials.Username,
		secret: credentials.Password,

		credentials:    credentials,
		identityClient: identityClient,
	}, nil
}

func (m *Manager) newParentFromSecret(secret *corev1.Secret) (*parent, error) {
	return m.newParentFromCredetials(&openstack.Credentials{
		DomainName: readSecretKey(secret, openstack.DomainName),
		TenantName: readSecretKey(secret, openstack.TenantName),
		Password:   readSecretKey(secret, applicationCredentialSecretParentSecret),
		Username:   readSecretKey(secret, applicationCredentialSecretParentName),
		AuthURL:    readSecretKey(secret, openstack.AuthURL),
	})
}

func (p *parent) isApplicationCredential() bool {
	if p.credentials.ApplicationCredentialID != "" || p.credentials.ApplicationCredentialName != "" {
		return true
	}
	return false
}

func (p *parent) toSecretData() map[string][]byte {
	return map[string][]byte{
		applicationCredentialSecretParentID:     []byte(p.id),
		applicationCredentialSecretParentSecret: []byte(p.secret),
		applicationCredentialSecretParentName:   []byte(p.name),
		openstack.DomainName:                    []byte(p.credentials.DomainName),
		openstack.AuthURL:                       []byte(p.credentials.AuthURL),
		openstack.TenantName:                    []byte(p.credentials.TenantName),
	}
}
