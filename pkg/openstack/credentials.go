// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
	credentials, err := ExtractCredentialsFromData(secret.Data, allowDNSKeys)
	if err != nil {
		return nil, fmt.Errorf("%w in secret %s/%s", err, secret.Namespace, secret.Name)
	}
	return credentials, nil
}

// ExtractCredentialsFromData generates a credentials object from a raw secret data map.
// It is equivalent to ExtractCredentials but accepts a map[string][]byte directly,
// allowing extraction from both corev1.Secret and gardencorev1beta1.InternalSecret data.
func ExtractCredentialsFromData(data map[string][]byte, allowDNSKeys bool) (*Credentials, error) {
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

	if data == nil {
		return nil, fmt.Errorf("secret does not contain any data")
	}
	domainName, err := getRequiredFromData(data, DomainName, altDomainNameKey)
	if err != nil {
		return nil, err
	}
	tenantName, err := getRequiredFromData(data, TenantName, altTenantNameKey)
	if err != nil {
		return nil, err
	}
	userName := getOptionalFromData(data, UserName, altUserNameKey)
	password := getOptionalFromData(data, Password, altPasswordKey)
	applicationCredentialID := getOptionalFromData(data, ApplicationCredentialID, altApplicationCredentialID)
	applicationCredentialName := getOptionalFromData(data, ApplicationCredentialName, altApplicationCredentialName)
	applicationCredentialSecret := getOptionalFromData(data, ApplicationCredentialSecret, altApplicationCredentialSecret)
	authURL := getOptionalFromData(data, AuthURL, altAuthURLKey)
	caCert := getOptionalFromData(data, CACert, altCABundleKey)

	if err := ValidateSecrets(userName, password, applicationCredentialID, applicationCredentialName, applicationCredentialSecret); err != nil {
		return nil, err
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
		Insecure:                    strings.ToLower(strings.TrimSpace(string(data[Insecure]))) == "true",
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

// getOptionalFromData returns optional value for a corresponding key or empty string
func getOptionalFromData(data map[string][]byte, key string, altKey *string) string {
	if value, ok := data[key]; ok {
		return string(value)
	}
	if altKey != nil {
		if value, ok := data[*altKey]; ok {
			return string(value)
		}
	}
	return ""
}

// getRequiredFromData checks if the provided map has a valid value for a corresponding key.
func getRequiredFromData(data map[string][]byte, key string, altKey *string) (string, error) {
	value, ok := data[key]
	if !ok {
		if altKey == nil {
			return "", fmt.Errorf("missing %q data key in secret data", key)
		}
		value, ok = data[*altKey]
		if !ok {
			return "", fmt.Errorf("missing %q (or %q) data key in secret data", key, *altKey)
		}
	}
	if len(value) == 0 {
		if altKey != nil {
			return "", fmt.Errorf("key %q (or %q) in secret data cannot be empty", key, *altKey)
		}
		return "", fmt.Errorf("key %q in secret data cannot be empty", key)
	}
	return string(value), nil
}
