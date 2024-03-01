// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"net/url"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
)

const (
	tenantNameMaxLen = 64
)

// ValidateCloudProviderSecret checks whether the given secret contains a valid OpenStack credentials.
func ValidateCloudProviderSecret(secret *corev1.Secret) error {
	credentials, err := openstack.ExtractCredentials(secret, false)
	if err != nil {
		return err
	}

	secretKey := fmt.Sprintf("%s/%s", secret.Namespace, secret.Name)

	// domainName, tenantName, and userName must not contain leading or trailing whitespace
	for key, value := range map[string]string{
		openstack.DomainName:                  credentials.DomainName,
		openstack.TenantName:                  credentials.TenantName,
		openstack.UserName:                    credentials.Username,
		openstack.ApplicationCredentialID:     credentials.ApplicationCredentialID,
		openstack.ApplicationCredentialName:   credentials.ApplicationCredentialName,
		openstack.ApplicationCredentialSecret: credentials.ApplicationCredentialSecret,
	} {
		if strings.TrimSpace(value) != value {
			return fmt.Errorf("field %q in secret %s must not contain leading or traling whitespace", key, secretKey)
		}
	}

	// tenantName must not be longer than 64 characters, see https://docs.openstack.org/api-ref/identity/v3/?expanded=show-project-details-detail
	if len(credentials.TenantName) > tenantNameMaxLen {
		return fmt.Errorf("field %q in secret %s cannot be longer than %d characters", openstack.TenantName, secretKey, tenantNameMaxLen)
	}

	// password must not contain leading or trailing new lines, as they are known to cause issues
	// Other whitespace characters such as spaces are intentionally not checked for,
	// since there is no documentation indicating that they would not be valid
	if strings.Trim(credentials.Password, "\n\r") != credentials.Password {
		return fmt.Errorf("field %q in secret %s must not contain leading or traling new lines", openstack.Password, secretKey)
	}

	// authURL must be a valid URL if present
	if credentials.AuthURL != "" {
		if _, err := url.Parse(credentials.AuthURL); err != nil {
			return fmt.Errorf("field %q in secret %s must be a valid URL when present: %v", openstack.AuthURL, secretKey, err)
		}
	}

	return nil
}
