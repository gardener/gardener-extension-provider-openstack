// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"net/url"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
)

// ValidateCloudProviderSecret validates OpenStack credentials in a secret for both infrastructure and DNS use cases.
// It checks required fields (domainName, tenantName), validates authentication methods (username/password or application credentials),
// and ensures all field values meet format requirements. When allowDNSKeys is true, DNS-specific key aliases (e.g., OS_DOMAIN_NAME) are accepted.
func ValidateCloudProviderSecret(secret *corev1.Secret, fldPath *field.Path, allowDNSKeys bool) field.ErrorList {
	allErrs := field.ErrorList{}
	dataPath := fldPath.Child("data")

	// DNS specific keys
	var (
		domainNameDNSKey, tenantNameDNSKey, userNameDNSKey, passwordDNSKey *string
		appCredIDDNSKey, appCredNameDNSKey, appCredSecretDNSKey            *string
		authURLDNSKey, caBundleDNSKey                                      *string
	)

	// expected keys to ensure no unexpected keys are present
	var expectedKeys []string
	expectedKeys = []string{openstack.DomainName, openstack.TenantName, openstack.UserName, openstack.Password,
		openstack.ApplicationCredentialID, openstack.ApplicationCredentialName, openstack.ApplicationCredentialSecret,
		openstack.AuthURL, openstack.CACert, openstack.Insecure}

	// set DNS key variables if allowed
	if allowDNSKeys {
		domainNameDNSKey = ptr.To(openstack.DNSDomainName)
		tenantNameDNSKey = ptr.To(openstack.DNSTenantName)
		userNameDNSKey = ptr.To(openstack.DNSUserName)
		passwordDNSKey = ptr.To(openstack.DNSPassword)
		appCredIDDNSKey = ptr.To(openstack.DNSApplicationCredentialID)
		appCredNameDNSKey = ptr.To(openstack.DNSApplicationCredentialName)
		appCredSecretDNSKey = ptr.To(openstack.DNSApplicationCredentialSecret)
		authURLDNSKey = ptr.To(openstack.DNSAuthURL)
		caBundleDNSKey = ptr.To(openstack.DNS_CA_Bundle)

		// extend expected keys with DNS keys
		expectedKeys = append(expectedKeys, openstack.DNSDomainName, openstack.DNSTenantName,
			openstack.DNSUserName, openstack.DNSPassword, openstack.DNSApplicationCredentialID,
			openstack.DNSApplicationCredentialName, openstack.DNSApplicationCredentialSecret,
			openstack.DNSAuthURL, openstack.DNS_CA_Bundle)
	}

	// validate required fields (domainName, tenantName)
	domainName, domainNameKey, errs := validateRequiredField(secret, dataPath, openstack.DomainName, domainNameDNSKey)
	allErrs = append(allErrs, errs...)
	allErrs = append(allErrs, validateDomainName(domainName, dataPath.Key(domainNameKey))...)

	tenantName, tenantNameKey, errs := validateRequiredField(secret, dataPath, openstack.TenantName, tenantNameDNSKey)
	allErrs = append(allErrs, errs...)
	allErrs = append(allErrs, validateTenantName(tenantName, dataPath.Key(tenantNameKey))...)

	// get optional fields
	userName, userNameKey, errs := getOptionalField(secret, dataPath, openstack.UserName, userNameDNSKey)
	allErrs = append(allErrs, errs...)
	allErrs = append(allErrs, validateUserName(userName, dataPath.Key(userNameKey))...)

	password, passwordKey, errs := getOptionalField(secret, dataPath, openstack.Password, passwordDNSKey)
	allErrs = append(allErrs, errs...)
	allErrs = append(allErrs, validatePassword(password, dataPath.Key(passwordKey))...)

	appCredID, appCredIDKey, errs := getOptionalField(secret, dataPath, openstack.ApplicationCredentialID, appCredIDDNSKey)
	allErrs = append(allErrs, errs...)
	allErrs = append(allErrs, validateAppCredentialID(appCredID, dataPath.Key(appCredIDKey))...)

	appCredName, appCredNameKey, errs := getOptionalField(secret, dataPath, openstack.ApplicationCredentialName, appCredNameDNSKey)
	allErrs = append(allErrs, errs...)
	allErrs = append(allErrs, validateAppCredentialName(appCredName, dataPath.Key(appCredNameKey))...)

	appCredSecret, appCredSecretKey, errs := getOptionalField(secret, dataPath, openstack.ApplicationCredentialSecret, appCredSecretDNSKey)
	allErrs = append(allErrs, errs...)
	allErrs = append(allErrs, validateAppCredentialSecret(appCredSecret, dataPath.Key(appCredSecretKey))...)

	// authURL is required for DNS secrets, optional for infrastructure secrets
	var authURL, authURLKey string
	if allowDNSKeys {
		authURL, authURLKey, errs = validateRequiredField(secret, dataPath, openstack.AuthURL, authURLDNSKey)
		allErrs = append(allErrs, errs...)
	} else {
		authURL, authURLKey, errs = getOptionalField(secret, dataPath, openstack.AuthURL, authURLDNSKey)
		allErrs = append(allErrs, errs...)
	}
	allErrs = append(allErrs, validateHTTPURL(authURL, dataPath.Key(authURLKey))...)

	caCert, caCertKey, errs := getOptionalField(secret, dataPath, openstack.CACert, caBundleDNSKey)
	allErrs = append(allErrs, errs...)
	allErrs = append(allErrs, validateCACert(caCert, dataPath.Key(caCertKey))...)

	insecure, insecureKey, errs := getOptionalField(secret, dataPath, openstack.Insecure, nil)
	allErrs = append(allErrs, errs...)
	allErrs = append(allErrs, validateInsecure(insecure, dataPath.Key(insecureKey))...)

	// ensure valid authentication method is available
	allErrs = append(allErrs, validateAuthCombination(
		userName, userNameKey, password, passwordKey,
		appCredID, appCredIDKey, appCredName, appCredNameKey, appCredSecret, appCredSecretKey, dataPath)...)

	// make sure that only expected keys exist
	allErrs = append(allErrs, validateNoUnexpectedKeys(secret, dataPath, expectedKeys)...)

	return allErrs
}

// validateRequiredField checks if a required field exists in the secret and is non-empty.
// It first looks for the standard key, then falls back to the DNS key if provided and allowDNSKeys is true.
// Returns the field value, the actual key used (standard or DNS), and any validation errors.
func validateRequiredField(secret *corev1.Secret, fldPath *field.Path, key string, dnsKey *string) (string, string, field.ErrorList) {
	allErrs := field.ErrorList{}

	value, ok := secret.Data[key]
	actualKey := key
	if !ok && dnsKey != nil {
		actualKey = *dnsKey
		value, ok = secret.Data[actualKey]
	}

	if !ok {
		allErrs = append(allErrs, field.Required(fldPath.Key(key), fmt.Sprintf("missing required field %q", key)))
		return "", key, allErrs
	}

	if ok && len(value) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Key(actualKey), "", fmt.Sprintf("field %q cannot be empty", actualKey)))
	}

	return string(value), actualKey, allErrs
}

// getOptionalField extracts an optional field value from the secret.
// It first checks for the standard key, then falls back to the DNS key if provided.
// Returns the field value (empty string if not found) and the actual key used.
// Returns an error if the key is used but no value provided.
func getOptionalField(secret *corev1.Secret, fldPath *field.Path, key string, dnsKey *string) (string, string, field.ErrorList) {
	allErrs := field.ErrorList{}

	value, ok := secret.Data[key]
	actualKey := key
	if !ok && dnsKey != nil {
		actualKey = *dnsKey
		if value, ok = secret.Data[*dnsKey]; !ok {
			return "", key, allErrs
		}
	}

	if ok && len(value) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Key(actualKey), "", fmt.Sprintf("field %q cannot be empty when key is set", actualKey)))
	}

	return string(value), actualKey, allErrs
}

// validateAuthCombination ensures that exactly one valid authentication method is provided.
// Valid methods are:
// (1) applicationCredentialID + applicationCredentialSecret
// (2) username + domainName + applicationCredentialName + applicationCredentialSecret
// (3) username + password + domainName + tenantName
// Note: domainName and tenantName are validated separately as always-required fields.
// It also prevents mixing password-based and application credential authentication.
// See https://docs.openstack.org/python-openstackclient/latest/cli/authentication.html
// and https://docs.openstack.org/keystone/latest/user/application_credentials.html#using-application-credentials
func validateAuthCombination(
	userName, userNameKey, pw, pwKey,
	appCredID, appCredIDKey, appCredName, appCredNameKey,
	appCredSecret, appCredSecretKey string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Check for invalid mixing of password and application credentials
	if pw != "" && appCredSecret != "" {
		return field.ErrorList{field.Invalid(fldPath, "",
			fmt.Sprintf("cannot specify both %s and %s", pwKey, appCredSecretKey))}
	}

	// Determine which authentication method is being used
	if appCredID != "" {
		// Method 1: Application Credential ID
		if appCredSecret == "" {
			allErrs = append(allErrs, field.Required(
				fldPath.Child(openstack.ApplicationCredentialSecret),
				fmt.Sprintf("%s is required when %s is provided", appCredSecretKey, appCredIDKey)))
		}
		return allErrs
	}

	if appCredName != "" {
		// Method 2: Application Credential Name (requires username and domainName)
		if appCredSecret == "" {
			allErrs = append(allErrs, field.Required(
				fldPath.Child(openstack.ApplicationCredentialSecret),
				fmt.Sprintf("%s is required when %s is provided", appCredSecretKey, appCredNameKey)))
		}
		if userName == "" {
			allErrs = append(allErrs, field.Required(
				fldPath.Child(openstack.UserName),
				fmt.Sprintf("%s is required when %s is provided", userNameKey, appCredNameKey)))
		}
		return allErrs
	}

	if pw != "" {
		// Method 3: Username/Password authentication (requires domainName and tenantName)
		if userName == "" {
			allErrs = append(allErrs, field.Required(
				fldPath.Child(openstack.UserName),
				fmt.Sprintf("%s is required when %s is provided", userNameKey, pwKey)))
		}
		return allErrs
	}

	// No valid authentication method provided
	return field.ErrorList{field.Required(fldPath,
		fmt.Sprintf("must provide one of the following authentication methods: "+
			"(1) %s + %s, "+
			"(2) %s + %s + %s, "+
			"(3) %s + %s",
			appCredIDKey, appCredSecretKey,
			userNameKey, appCredNameKey, appCredSecretKey,
			userNameKey, pwKey))}
}

// validateNoUnexpectedKeys ensures that the secret contains only expected keys.
// Any key not in the expectedKeys list is reported as a forbidden field.
func validateNoUnexpectedKeys(secret *corev1.Secret, fldPath *field.Path, expectedKeys []string) field.ErrorList {
	allErrs := field.ErrorList{}
	expectedKeysSet := sets.NewString(expectedKeys...)

	for k := range secret.Data {
		if !expectedKeysSet.Has(k) {
			allErrs = append(allErrs, field.Forbidden(fldPath.Key(k),
				fmt.Sprintf("unexpected field %q", k)))
		}
	}

	return allErrs
}

// validateHTTPURL validates that a string is a valid HTTP or HTTPS URL
func validateHTTPURL(urlStr string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if urlStr == "" {
		return allErrs
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, urlStr, "must be a valid URL"))
		return allErrs
	}

	if parsedURL.Scheme != "https" && parsedURL.Scheme != "http" {
		allErrs = append(allErrs, field.Invalid(fldPath, urlStr, "must use http:// or https:// scheme"))
	}

	if parsedURL.Host == "" {
		allErrs = append(allErrs, field.Invalid(fldPath, urlStr, "must include a host"))
	}

	return allErrs
}

// validateCACert validates that a CA certificate is in valid PEM format.
// It checks for the presence of PEM certificate header and footer markers.
// This field is optional, so empty values are allowed.
func validateCACert(cert string, fldPath *field.Path) field.ErrorList {
	if cert == "" {
		return nil // Optional field
	}

	allErrs := field.ErrorList{}

	const (
		pemHeader = "-----BEGIN CERTIFICATE-----"
		pemFooter = "-----END CERTIFICATE-----"
	)

	if !strings.Contains(cert, pemHeader) {
		allErrs = append(allErrs, field.Invalid(fldPath, "", "must contain '-----BEGIN CERTIFICATE-----'"))
	}

	if !strings.Contains(cert, pemFooter) {
		allErrs = append(allErrs, field.Invalid(fldPath, "", "must contain '-----END CERTIFICATE-----'"))
	}

	return allErrs
}
