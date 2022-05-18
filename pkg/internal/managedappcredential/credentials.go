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
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetCredentials returns the credentials and the secret reference for the in-use
// application credential. If no application credential exits nil will be returned.
func GetCredentials(ctx context.Context, client client.Client, namespace string) (*openstack.Credentials, *corev1.SecretReference, error) {
	appCredential, err := readApplicationCredential(ctx, client, namespace)
	if err != nil {
		return nil, nil, err
	}

	if appCredential == nil {
		return nil, nil, nil
	}

	return &openstack.Credentials{
			ApplicationCredentialID:     readSecretKey(appCredential.secret, openstack.ApplicationCredentialID),
			ApplicationCredentialName:   readSecretKey(appCredential.secret, openstack.ApplicationCredentialName),
			ApplicationCredentialSecret: readSecretKey(appCredential.secret, openstack.ApplicationCredentialSecret),
			DomainName:                  readSecretKey(appCredential.secret, openstack.DomainName),
			TenantName:                  readSecretKey(appCredential.secret, openstack.TenantName),
			AuthURL:                     readSecretKey(appCredential.secret, openstack.AuthURL),
		}, &corev1.SecretReference{
			Name:      applicationCredentialSecretName,
			Namespace: namespace,
		}, nil
}
