// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	. "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/validation"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
)

var _ = Describe("Secret validation", func() {
	Describe("#ValidateCloudProviderSecret", func() {
		const (
			namespace  = "test-namespace"
			secretName = "test-secret"
		)

		var (
			secret  *corev1.Secret
			fldPath *field.Path
		)

		BeforeEach(func() {
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: namespace,
				},
				Data: map[string][]byte{},
			}
			fldPath = field.NewPath("secret")
		})

		Context("Infrastructure secrets (allowDNSKeys=false)", func() {
			It("should pass with valid username/password authentication", func() {
				secret.Data = map[string][]byte{
					openstack.DomainName: []byte("my-domain"),
					openstack.TenantName: []byte("my-tenant"),
					openstack.UserName:   []byte("my-user"),
					openstack.Password:   []byte("my-password"),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, false)
				Expect(errs).To(BeEmpty())
			})

			It("should pass with valid application credential ID authentication", func() {
				secret.Data = map[string][]byte{
					openstack.DomainName:                  []byte("my-domain"),
					openstack.TenantName:                  []byte("my-tenant"),
					openstack.ApplicationCredentialID:     []byte("app-cred-id-123"),
					openstack.ApplicationCredentialSecret: []byte("app-cred-secret"),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, false)
				Expect(errs).To(BeEmpty())
			})

			It("should pass with valid application credential name authentication", func() {
				secret.Data = map[string][]byte{
					openstack.DomainName:                  []byte("my-domain"),
					openstack.TenantName:                  []byte("my-tenant"),
					openstack.UserName:                    []byte("my-user"),
					openstack.ApplicationCredentialName:   []byte("my-app-cred"),
					openstack.ApplicationCredentialSecret: []byte("app-cred-secret"),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, false)
				Expect(errs).To(BeEmpty())
			})

			It("should pass with optional authURL field", func() {
				secret.Data = map[string][]byte{
					openstack.DomainName: []byte("my-domain"),
					openstack.TenantName: []byte("my-tenant"),
					openstack.UserName:   []byte("my-user"),
					openstack.Password:   []byte("my-password"),
					openstack.AuthURL:    []byte("https://keystone.example.com:5000/v3"),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, false)
				Expect(errs).To(BeEmpty())
			})

			It("should pass with optional caCert field", func() {
				secret.Data = map[string][]byte{
					openstack.DomainName: []byte("my-domain"),
					openstack.TenantName: []byte("my-tenant"),
					openstack.UserName:   []byte("my-user"),
					openstack.Password:   []byte("my-password"),
					openstack.CACert:     []byte("-----BEGIN CERTIFICATE-----\nFAKE-CERT\n-----END CERTIFICATE-----"),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, false)
				Expect(errs).To(BeEmpty())
			})

			It("should pass with optional insecure field set to true", func() {
				secret.Data = map[string][]byte{
					openstack.DomainName: []byte("my-domain"),
					openstack.TenantName: []byte("my-tenant"),
					openstack.UserName:   []byte("my-user"),
					openstack.Password:   []byte("my-password"),
					openstack.Insecure:   []byte("true"),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, false)
				Expect(errs).To(BeEmpty())
			})

			It("should pass with optional insecure field set to false", func() {
				secret.Data = map[string][]byte{
					openstack.DomainName: []byte("my-domain"),
					openstack.TenantName: []byte("my-tenant"),
					openstack.UserName:   []byte("my-user"),
					openstack.Password:   []byte("my-password"),
					openstack.Insecure:   []byte("false"),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, false)
				Expect(errs).To(BeEmpty())
			})

			It("should fail when domainName is missing", func() {
				secret.Data = map[string][]byte{
					openstack.TenantName: []byte("my-tenant"),
					openstack.UserName:   []byte("my-user"),
					openstack.Password:   []byte("my-password"),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, false)
				Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("secret.data[domainName]"),
				}))))
			})

			It("should fail when tenantName is missing", func() {
				secret.Data = map[string][]byte{
					openstack.DomainName: []byte("my-domain"),
					openstack.UserName:   []byte("my-user"),
					openstack.Password:   []byte("my-password"),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, false)
				Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("secret.data[tenantName]"),
				}))))
			})

			It("should fail when domainName is empty", func() {
				secret.Data = map[string][]byte{
					openstack.DomainName: []byte(""),
					openstack.TenantName: []byte("my-tenant"),
					openstack.UserName:   []byte("my-user"),
					openstack.Password:   []byte("my-password"),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, false)
				Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("secret.data[domainName]"),
				}))))
			})

			It("should fail when tenantName is empty", func() {
				secret.Data = map[string][]byte{
					openstack.DomainName: []byte("my-domain"),
					openstack.TenantName: []byte(""),
					openstack.UserName:   []byte("my-user"),
					openstack.Password:   []byte("my-password"),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, false)
				Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("secret.data[tenantName]"),
				}))))
			})

			It("should fail when no authentication method is provided", func() {
				secret.Data = map[string][]byte{
					openstack.DomainName: []byte("my-domain"),
					openstack.TenantName: []byte("my-tenant"),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, false)
				Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeRequired),
					"Field":  Equal("secret.data"),
					"Detail": ContainSubstring("must provide one of the following authentication methods"),
				}))))
			})

			It("should fail when password is provided without username", func() {
				secret.Data = map[string][]byte{
					openstack.DomainName: []byte("my-domain"),
					openstack.TenantName: []byte("my-tenant"),
					openstack.Password:   []byte("my-password"),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, false)
				Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("secret.data.username"),
				}))))
			})

			It("should fail when mixing password and application credentials", func() {
				secret.Data = map[string][]byte{
					openstack.DomainName:                  []byte("my-domain"),
					openstack.TenantName:                  []byte("my-tenant"),
					openstack.UserName:                    []byte("my-user"),
					openstack.Password:                    []byte("my-password"),
					openstack.ApplicationCredentialSecret: []byte("app-cred-secret"),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, false)
				Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("secret.data"),
				}))))
			})

			It("should fail when applicationCredentialID is provided without applicationCredentialSecret", func() {
				secret.Data = map[string][]byte{
					openstack.DomainName:              []byte("my-domain"),
					openstack.TenantName:              []byte("my-tenant"),
					openstack.ApplicationCredentialID: []byte("app-cred-id"),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, false)
				Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("secret.data.applicationCredentialSecret"),
				}))))
			})

			It("should fail when applicationCredentialName is provided without username", func() {
				secret.Data = map[string][]byte{
					openstack.DomainName:                  []byte("my-domain"),
					openstack.TenantName:                  []byte("my-tenant"),
					openstack.ApplicationCredentialName:   []byte("my-app-cred"),
					openstack.ApplicationCredentialSecret: []byte("app-cred-secret"),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, false)
				Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("secret.data.username"),
				}))))
			})

			It("should fail when applicationCredentialName is provided without applicationCredentialSecret", func() {
				secret.Data = map[string][]byte{
					openstack.DomainName:                []byte("my-domain"),
					openstack.TenantName:                []byte("my-tenant"),
					openstack.UserName:                  []byte("my-user"),
					openstack.ApplicationCredentialName: []byte("my-app-cred"),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, false)
				Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("secret.data.applicationCredentialSecret"),
				}))))
			})

			It("should fail when username is empty but key is set", func() {
				secret.Data = map[string][]byte{
					openstack.DomainName: []byte("my-domain"),
					openstack.TenantName: []byte("my-tenant"),
					openstack.UserName:   []byte(""),
					openstack.Password:   []byte("my-password"),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, false)
				Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("secret.data[username]"),
				}))))
			})

			It("should fail when password is empty but key is set", func() {
				secret.Data = map[string][]byte{
					openstack.DomainName: []byte("my-domain"),
					openstack.TenantName: []byte("my-tenant"),
					openstack.UserName:   []byte("my-user"),
					openstack.Password:   []byte(""),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, false)
				Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("secret.data[password]"),
				}))))
			})

			It("should fail when domainName contains leading whitespace", func() {
				secret.Data = map[string][]byte{
					openstack.DomainName: []byte(" my-domain"),
					openstack.TenantName: []byte("my-tenant"),
					openstack.UserName:   []byte("my-user"),
					openstack.Password:   []byte("my-password"),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, false)
				Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal("secret.data[domainName]"),
					"BadValue": Equal("(hidden)"),
				}))))
			})

			It("should fail when tenantName is too long", func() {
				secret.Data = map[string][]byte{
					openstack.DomainName: []byte("my-domain"),
					openstack.TenantName: []byte("this-is-a-very-long-tenant-name-that-exceeds-the-maximum-allowed-length-of-64-characters"),
					openstack.UserName:   []byte("my-user"),
					openstack.Password:   []byte("my-password"),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, false)
				Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("secret.data[tenantName]"),
				}))))
			})

			It("should fail when authURL is invalid", func() {
				secret.Data = map[string][]byte{
					openstack.DomainName: []byte("my-domain"),
					openstack.TenantName: []byte("my-tenant"),
					openstack.UserName:   []byte("my-user"),
					openstack.Password:   []byte("my-password"),
					openstack.AuthURL:    []byte("not a valid url"),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, false)
				Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("secret.data[authURL]"),
				}))))
			})

			It("should fail when caCert is missing BEGIN marker", func() {
				secret.Data = map[string][]byte{
					openstack.DomainName: []byte("my-domain"),
					openstack.TenantName: []byte("my-tenant"),
					openstack.UserName:   []byte("my-user"),
					openstack.Password:   []byte("my-password"),
					openstack.CACert:     []byte("FAKE-CERT\n-----END CERTIFICATE-----"),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, false)
				Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("secret.data[caCert]"),
				}))))
			})

			It("should fail when caCert is missing END marker", func() {
				secret.Data = map[string][]byte{
					openstack.DomainName: []byte("my-domain"),
					openstack.TenantName: []byte("my-tenant"),
					openstack.UserName:   []byte("my-user"),
					openstack.Password:   []byte("my-password"),
					openstack.CACert:     []byte("-----BEGIN CERTIFICATE-----\nFAKE-CERT"),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, false)
				Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("secret.data[caCert]"),
				}))))
			})

			It("should fail when insecure has invalid value", func() {
				secret.Data = map[string][]byte{
					openstack.DomainName: []byte("my-domain"),
					openstack.TenantName: []byte("my-tenant"),
					openstack.UserName:   []byte("my-user"),
					openstack.Password:   []byte("my-password"),
					openstack.Insecure:   []byte("yes"),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, false)
				Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeNotSupported),
					"Field":    Equal("secret.data[insecure]"),
					"BadValue": Equal("yes"),
				}))))
			})

			It("should fail when secret contains unexpected keys", func() {
				secret.Data = map[string][]byte{
					openstack.DomainName: []byte("my-domain"),
					openstack.TenantName: []byte("my-tenant"),
					openstack.UserName:   []byte("my-user"),
					openstack.Password:   []byte("my-password"),
					"unexpected-field":   []byte("value"),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, false)
				Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("secret.data[unexpected-field]"),
				}))))
			})
		})

		Context("DNS secrets (allowDNSKeys=true)", func() {
			It("should pass with valid username/password authentication using standard keys", func() {
				secret.Data = map[string][]byte{
					openstack.DomainName: []byte("my-domain"),
					openstack.TenantName: []byte("my-tenant"),
					openstack.UserName:   []byte("my-user"),
					openstack.Password:   []byte("my-password"),
					openstack.AuthURL:    []byte("https://keystone.example.com:5000/v3"),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, true)
				Expect(errs).To(BeEmpty())
			})

			It("should pass with valid username/password authentication using DNS keys", func() {
				secret.Data = map[string][]byte{
					openstack.DNSDomainName: []byte("my-domain"),
					openstack.DNSTenantName: []byte("my-tenant"),
					openstack.DNSUserName:   []byte("my-user"),
					openstack.DNSPassword:   []byte("my-password"),
					openstack.DNSAuthURL:    []byte("https://keystone.example.com:5000/v3"),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, true)
				Expect(errs).To(BeEmpty())
			})

			It("should pass with valid application credential ID authentication using DNS keys", func() {
				secret.Data = map[string][]byte{
					openstack.DNSDomainName:                  []byte("my-domain"),
					openstack.DNSTenantName:                  []byte("my-tenant"),
					openstack.DNSApplicationCredentialID:     []byte("app-cred-id-123"),
					openstack.DNSApplicationCredentialSecret: []byte("app-cred-secret"),
					openstack.DNSAuthURL:                     []byte("https://keystone.example.com:5000/v3"),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, true)
				Expect(errs).To(BeEmpty())
			})

			It("should pass with valid application credential name authentication using DNS keys", func() {
				secret.Data = map[string][]byte{
					openstack.DNSDomainName:                  []byte("my-domain"),
					openstack.DNSTenantName:                  []byte("my-tenant"),
					openstack.DNSUserName:                    []byte("my-user"),
					openstack.DNSApplicationCredentialName:   []byte("my-app-cred"),
					openstack.DNSApplicationCredentialSecret: []byte("app-cred-secret"),
					openstack.DNSAuthURL:                     []byte("https://keystone.example.com:5000/v3"),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, true)
				Expect(errs).To(BeEmpty())
			})

			It("should pass with mixed standard and DNS keys", func() {
				secret.Data = map[string][]byte{
					openstack.DNSDomainName: []byte("my-domain"),
					openstack.TenantName:    []byte("my-tenant"),
					openstack.UserName:      []byte("my-user"),
					openstack.DNSPassword:   []byte("my-password"),
					openstack.AuthURL:       []byte("https://keystone.example.com:5000/v3"),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, true)
				Expect(errs).To(BeEmpty())
			})

			It("should pass with optional caCert using DNS key", func() {
				secret.Data = map[string][]byte{
					openstack.DNSDomainName: []byte("my-domain"),
					openstack.DNSTenantName: []byte("my-tenant"),
					openstack.DNSUserName:   []byte("my-user"),
					openstack.DNSPassword:   []byte("my-password"),
					openstack.DNSAuthURL:    []byte("https://keystone.example.com:5000/v3"),
					openstack.DNS_CA_Bundle: []byte("-----BEGIN CERTIFICATE-----\nFAKE-CERT\n-----END CERTIFICATE-----"),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, true)
				Expect(errs).To(BeEmpty())
			})

			It("should fail when authURL is missing", func() {
				secret.Data = map[string][]byte{
					openstack.DNSDomainName: []byte("my-domain"),
					openstack.DNSTenantName: []byte("my-tenant"),
					openstack.DNSUserName:   []byte("my-user"),
					openstack.DNSPassword:   []byte("my-password"),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, true)
				Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("secret.data[authURL]"),
				}))))
			})

			It("should fail when authURL is empty", func() {
				secret.Data = map[string][]byte{
					openstack.DNSDomainName: []byte("my-domain"),
					openstack.DNSTenantName: []byte("my-tenant"),
					openstack.DNSUserName:   []byte("my-user"),
					openstack.DNSPassword:   []byte("my-password"),
					openstack.DNSAuthURL:    []byte(""),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, true)
				Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("secret.data[OS_AUTH_URL]"),
				}))))
			})

			It("should fail when domainName is missing (neither standard nor DNS key)", func() {
				secret.Data = map[string][]byte{
					openstack.DNSTenantName: []byte("my-tenant"),
					openstack.DNSUserName:   []byte("my-user"),
					openstack.DNSPassword:   []byte("my-password"),
					openstack.DNSAuthURL:    []byte("https://keystone.example.com:5000/v3"),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, true)
				Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("secret.data[domainName]"),
				}))))
			})

			It("should fail when tenantName is missing (neither standard nor DNS key)", func() {
				secret.Data = map[string][]byte{
					openstack.DNSDomainName: []byte("my-domain"),
					openstack.DNSUserName:   []byte("my-user"),
					openstack.DNSPassword:   []byte("my-password"),
					openstack.DNSAuthURL:    []byte("https://keystone.example.com:5000/v3"),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, true)
				Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("secret.data[tenantName]"),
				}))))
			})

			It("should fail when mixing password and application credentials with DNS keys", func() {
				secret.Data = map[string][]byte{
					openstack.DNSDomainName:                  []byte("my-domain"),
					openstack.DNSTenantName:                  []byte("my-tenant"),
					openstack.DNSUserName:                    []byte("my-user"),
					openstack.DNSPassword:                    []byte("my-password"),
					openstack.DNSApplicationCredentialSecret: []byte("app-cred-secret"),
					openstack.DNSAuthURL:                     []byte("https://keystone.example.com:5000/v3"),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, true)
				Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("secret.data"),
				}))))
			})

			It("should fail when secret contains unexpected keys (not in standard or DNS keys)", func() {
				secret.Data = map[string][]byte{
					openstack.DNSDomainName: []byte("my-domain"),
					openstack.DNSTenantName: []byte("my-tenant"),
					openstack.DNSUserName:   []byte("my-user"),
					openstack.DNSPassword:   []byte("my-password"),
					openstack.DNSAuthURL:    []byte("https://keystone.example.com:5000/v3"),
					"unexpected-field":      []byte("value"),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, true)
				Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("secret.data[unexpected-field]"),
				}))))
			})
		})
	})
})
