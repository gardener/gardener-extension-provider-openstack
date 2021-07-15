// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package openstack

import (
	"context"
	"fmt"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Credentials contains the necessary OpenStack credential information.
type Credentials struct {
	DomainName string
	TenantName string

	// either authenticate with username/password credentials
	Username string
	Password string

	// or application credentials
	ApplicationCredentialID     string
	ApplicationCredentialName   string
	ApplicationCredentialSecret string

	AuthURL string
}

// GetCredentials computes for a given context and infrastructure the corresponding credentials object.
func GetCredentials(ctx context.Context, c client.Client, secretRef corev1.SecretReference, allowDNSKeys bool) (*Credentials, error) {
	secret, err := extensionscontroller.GetSecretByReference(ctx, c, &secretRef)
	if err != nil {
		return nil, err
	}
	return ExtractCredentials(secret, allowDNSKeys)
}

// ExtractCredentials generates a credentials object for a given provider secret.
func ExtractCredentials(secret *corev1.Secret, allowDNSKeys bool) (*Credentials, error) {

	var altDomainNameKey, altTenantNameKey, altUserNameKey, altPasswordKey, altAuthURLKey *string
	if allowDNSKeys {
		altDomainNameKey = pointer.String(DNSDomainName)
		altTenantNameKey = pointer.String(DNSTenantName)
		altUserNameKey = pointer.String(DNSUserName)
		altPasswordKey = pointer.String(DNSPassword)
		altAuthURLKey = pointer.String(DNSAuthURL)
	}

	if secret.Data == nil {
		return nil, fmt.Errorf("secret does not contain any data")
	}
	domainName, err := getRequired(secret, DomainName, altDomainNameKey)
	if err != nil {
		return nil, err
	}
	tenantName, err := getRequired(secret, TenantName, altTenantNameKey)
	if err != nil {
		return nil, err
	}
	userName := getOptional(secret, UserName, altUserNameKey)
	password := getOptional(secret, Password, altPasswordKey)
	applicationCredentialID := getOptional(secret, ApplicationCredentialID, nil)
	applicationCredentialName := getOptional(secret, ApplicationCredentialName, nil)
	applicationCredentialSecret := getOptional(secret, ApplicationCredentialSecret, nil)
	authURL := getOptional(secret, AuthURL, altAuthURLKey)

	if password != "" {
		if applicationCredentialSecret != "" {
			return nil, fmt.Errorf("cannot specify both '%s' and '%s' in secret %s/%s", Password, ApplicationCredentialSecret, secret.Namespace, secret.Name)
		}
		if userName == "" {
			return nil, fmt.Errorf("'%s' is required if '%s' is given in %s/%s", UserName, Password, secret.Namespace, secret.Name)
		}
	} else {
		if applicationCredentialSecret == "" {
			return nil, fmt.Errorf("must either specify '%s' or '%s' in secret %s/%s", Password, ApplicationCredentialSecret, secret.Namespace, secret.Name)
		}
		if applicationCredentialID == "" {
			if userName == "" || applicationCredentialName == "" {
				return nil, fmt.Errorf("'%s' and '%s' are required if application credentials are used without '%s' in secret %s/%s", ApplicationCredentialName, UserName,
					ApplicationCredentialID, secret.Namespace, secret.Name)
			}
		}
	}

	return &Credentials{
		DomainName:                  domainName,
		TenantName:                  tenantName,
		Username:                    userName,
		Password:                    password,
		ApplicationCredentialID:     applicationCredentialID,
		ApplicationCredentialName:   applicationCredentialName,
		ApplicationCredentialSecret: applicationCredentialSecret,
		AuthURL:                     string(authURL),
	}, nil
}

// getOptional returns optional value for a corresponding key or empty string
func getOptional(secret *corev1.Secret, key string, altKey *string) string {
	if value, ok := secret.Data[key]; ok {
		return string(value)
	}
	if altKey != nil {
		if value, ok := secret.Data[*altKey]; ok {
			return string(value)
		}
	}
	return ""
}

// getRequired checks if the provided map has a valid value for a corresponding key.
func getRequired(secret *corev1.Secret, key string, altKey *string) (string, error) {
	value, ok := secret.Data[key]
	if !ok {
		if altKey != nil {
			value, ok = secret.Data[*altKey]
			if !ok {
				return "", fmt.Errorf("missing %q (or %q) data key in secret %s/%s", key, *altKey, secret.Namespace, secret.Name)
			}
		} else {
			return "", fmt.Errorf("missing %q data key in secret %s/%s", key, secret.Namespace, secret.Name)
		}
	}
	if len(value) == 0 {
		if altKey != nil {
			return "", fmt.Errorf("key %q (or %q) in secret %s/%s cannot be empty", key, *altKey, secret.Namespace, secret.Name)
		}
		return "", fmt.Errorf("key %q in secret %s/%s cannot be empty", key, secret.Namespace, secret.Name)
	}
	return string(value), nil
}
