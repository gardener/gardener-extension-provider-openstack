// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package openstack

import (
	"context"
	"fmt"
	"strings"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
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
	CACert  string

	Insecure bool
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
	var altDomainNameKey, altTenantNameKey, altUserNameKey, altPasswordKey, altAuthURLKey, altCABundleKey *string
	var altApplicationCredentialID, altApplicationCredentialName, altApplicationCredentialSecret *string
	if allowDNSKeys {
		altDomainNameKey = ptr.To(DNSDomainName)
		altTenantNameKey = ptr.To(DNSTenantName)
		altUserNameKey = ptr.To(DNSUserName)
		altPasswordKey = ptr.To(DNSPassword)
		altApplicationCredentialID = ptr.To(DNSApplicationCredentialID)
		altApplicationCredentialName = ptr.To(DNSApplicationCredentialName)
		altApplicationCredentialSecret = ptr.To(DNSApplicationCredentialSecret)
		altAuthURLKey = ptr.To(DNSAuthURL)
		altCABundleKey = ptr.To(DNS_CA_Bundle)
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
	applicationCredentialID := getOptional(secret, ApplicationCredentialID, altApplicationCredentialID)
	applicationCredentialName := getOptional(secret, ApplicationCredentialName, altApplicationCredentialName)
	applicationCredentialSecret := getOptional(secret, ApplicationCredentialSecret, altApplicationCredentialSecret)
	authURL := getOptional(secret, AuthURL, altAuthURLKey)
	caCert := getOptional(secret, CACert, altCABundleKey)

	err = ValidateSecrets(userName, password, applicationCredentialID, applicationCredentialName, applicationCredentialSecret)
	if err != nil {
		return nil, fmt.Errorf("%w in secret %s/%s", err, secret.Namespace, secret.Name)
	}

	return &Credentials{
		DomainName:                  domainName,
		TenantName:                  tenantName,
		Username:                    userName,
		Password:                    password,
		ApplicationCredentialID:     applicationCredentialID,
		ApplicationCredentialName:   applicationCredentialName,
		ApplicationCredentialSecret: applicationCredentialSecret,
		AuthURL:                     authURL,
		CACert:                      caCert,
		Insecure:                    strings.ToLower(strings.TrimSpace(string(secret.Data[Insecure]))) == "true",
	}, nil
}

// ValidateSecrets checks if either basic auth or application credentials are completely provided
func ValidateSecrets(userName, password, appID, appName, appSecret string) error {
	if password != "" {
		if appSecret != "" {
			return fmt.Errorf("cannot specify both '%s' and '%s'", Password, ApplicationCredentialSecret)
		}
		if userName == "" {
			return fmt.Errorf("'%s' is required if '%s' is given", UserName, Password)
		}
	} else {
		if appSecret == "" {
			return fmt.Errorf("must either specify '%s' or '%s'", Password, ApplicationCredentialSecret)
		}
		if appID == "" && (userName == "" || appName == "") {
			return fmt.Errorf("'%s' and '%s' are required if application credentials are used without '%s'",
				ApplicationCredentialName, UserName, ApplicationCredentialID)
		}
	}

	return nil
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
		if altKey == nil {
			return "", fmt.Errorf("missing %q data key in secret %s/%s", key, secret.Namespace, secret.Name)
		}
		value, ok = secret.Data[*altKey]
		if !ok {
			return "", fmt.Errorf("missing %q (or %q) data key in secret %s/%s", key, *altKey, secret.Namespace, secret.Name)
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
