// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	"strings"

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

			It("should allow interior spaces in domainName (Keystone domains like 'Default Domain')", func() {
				secret.Data = map[string][]byte{
					openstack.DomainName: []byte("Default Domain"),
					openstack.TenantName: []byte("my project"),
					openstack.UserName:   []byte("Jane Doe"),
					openstack.Password:   []byte("my-password"),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, false)
				Expect(errs).To(BeEmpty())
			})

			DescribeTable("should fail when an identifier field contains an interior newline",
				func(data map[string][]byte, invalidField string) {
					secret.Data = data

					errs := ValidateCloudProviderSecret(secret, fldPath, false)
					Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(invalidField),
					}))))
				},
				Entry("domainName", map[string][]byte{
					openstack.DomainName: []byte("my-domain\nextra"),
					openstack.TenantName: []byte("my-tenant"),
					openstack.UserName:   []byte("my-user"),
					openstack.Password:   []byte("my-password"),
				}, "secret.data[domainName]"),
				Entry("tenantName", map[string][]byte{
					openstack.DomainName: []byte("my-domain"),
					openstack.TenantName: []byte("my-tenant\nextra"),
					openstack.UserName:   []byte("my-user"),
					openstack.Password:   []byte("my-password"),
				}, "secret.data[tenantName]"),
				Entry("username", map[string][]byte{
					openstack.DomainName: []byte("my-domain"),
					openstack.TenantName: []byte("my-tenant"),
					openstack.UserName:   []byte("my-user\nextra"),
					openstack.Password:   []byte("my-password"),
				}, "secret.data[username]"),
				Entry("applicationCredentialID", map[string][]byte{
					openstack.DomainName:                  []byte("my-domain"),
					openstack.TenantName:                  []byte("my-tenant"),
					openstack.ApplicationCredentialID:     []byte("my-id\nextra"),
					openstack.ApplicationCredentialSecret: []byte("my-app-secret"),
				}, "secret.data[applicationCredentialID]"),
				Entry("applicationCredentialName", map[string][]byte{
					openstack.DomainName:                  []byte("my-domain"),
					openstack.TenantName:                  []byte("my-tenant"),
					openstack.UserName:                    []byte("my-user"),
					openstack.ApplicationCredentialName:   []byte("my-name\nextra"),
					openstack.ApplicationCredentialSecret: []byte("my-app-secret"),
				}, "secret.data[applicationCredentialName]"),
			)

			DescribeTable("should fail when an identifier field contains a NUL or other non-whitespace control character",
				func(data map[string][]byte, invalidField string) {
					secret.Data = data

					errs := ValidateCloudProviderSecret(secret, fldPath, false)
					Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(invalidField),
					}))))
				},
				Entry("domainName with NUL", map[string][]byte{
					openstack.DomainName: []byte("my-domain\x00"),
					openstack.TenantName: []byte("my-tenant"),
					openstack.UserName:   []byte("my-user"),
					openstack.Password:   []byte("my-password"),
				}, "secret.data[domainName]"),
				Entry("tenantName with NUL", map[string][]byte{
					openstack.DomainName: []byte("my-domain"),
					openstack.TenantName: []byte("my-tenant\x00"),
					openstack.UserName:   []byte("my-user"),
					openstack.Password:   []byte("my-password"),
				}, "secret.data[tenantName]"),
				Entry("username with control char (U+0001)", map[string][]byte{
					openstack.DomainName: []byte("my-domain"),
					openstack.TenantName: []byte("my-tenant"),
					openstack.UserName:   []byte("my-user\x01"),
					openstack.Password:   []byte("my-password"),
				}, "secret.data[username]"),
				Entry("applicationCredentialID with NUL", map[string][]byte{
					openstack.DomainName:                  []byte("my-domain"),
					openstack.TenantName:                  []byte("my-tenant"),
					openstack.ApplicationCredentialID:     []byte("my-id\x00"),
					openstack.ApplicationCredentialSecret: []byte("my-app-secret"),
				}, "secret.data[applicationCredentialID]"),
				Entry("applicationCredentialName with NUL", map[string][]byte{
					openstack.DomainName:                  []byte("my-domain"),
					openstack.TenantName:                  []byte("my-tenant"),
					openstack.UserName:                    []byte("my-user"),
					openstack.ApplicationCredentialName:   []byte("my-name\x00"),
					openstack.ApplicationCredentialSecret: []byte("my-app-secret"),
				}, "secret.data[applicationCredentialName]"),
			)

			DescribeTable("should fail when a secret-like field contains a control character or Unicode line separator",
				func(data map[string][]byte, invalidField string) {
					secret.Data = data

					errs := ValidateCloudProviderSecret(secret, fldPath, false)
					Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":     Equal(field.ErrorTypeInvalid),
						"Field":    Equal(invalidField),
						"BadValue": Equal("(hidden)"),
					}))))
				},
				Entry("password with newline", map[string][]byte{
					openstack.DomainName: []byte("my-domain"),
					openstack.TenantName: []byte("my-tenant"),
					openstack.UserName:   []byte("my-user"),
					openstack.Password:   []byte("my-password\n        privileged: true"),
				}, "secret.data[password]"),
				Entry("applicationCredentialSecret with newline", map[string][]byte{
					openstack.DomainName:                  []byte("my-domain"),
					openstack.TenantName:                  []byte("my-tenant"),
					openstack.ApplicationCredentialID:     []byte("my-app-id"),
					openstack.ApplicationCredentialSecret: []byte("my-app-secret\n        privileged: true"),
				}, "secret.data[applicationCredentialSecret]"),
				Entry("password with Unicode line separator (U+2028)", map[string][]byte{
					openstack.DomainName: []byte("my-domain"),
					openstack.TenantName: []byte("my-tenant"),
					openstack.UserName:   []byte("my-user"),
					openstack.Password:   []byte("my-password injected: true"),
				}, "secret.data[password]"),
				Entry("password with Unicode paragraph separator (U+2029)", map[string][]byte{
					openstack.DomainName: []byte("my-domain"),
					openstack.TenantName: []byte("my-tenant"),
					openstack.UserName:   []byte("my-user"),
					openstack.Password:   []byte("my-password injected: true"),
				}, "secret.data[password]"),
				Entry("applicationCredentialSecret with Unicode line separator (U+2028)", map[string][]byte{
					openstack.DomainName:                  []byte("my-domain"),
					openstack.TenantName:                  []byte("my-tenant"),
					openstack.ApplicationCredentialID:     []byte("my-app-id"),
					openstack.ApplicationCredentialSecret: []byte("my-app-secret injected: true"),
				}, "secret.data[applicationCredentialSecret]"),
			)

			It("should allow a password containing spaces", func() {
				secret.Data = map[string][]byte{
					openstack.DomainName: []byte("my-domain"),
					openstack.TenantName: []byte("my-tenant"),
					openstack.UserName:   []byte("my-user"),
					openstack.Password:   []byte("p@ss word with spaces"),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, false)
				Expect(errs).To(BeEmpty())
			})

			It("should allow double-quote characters in the password (gcfg-escaped at render time)", func() {
				secret.Data = map[string][]byte{
					openstack.DomainName: []byte("my-domain"),
					openstack.TenantName: []byte("my-tenant"),
					openstack.UserName:   []byte("my-user"),
					openstack.Password:   []byte(`p@ss word "with" 'quotes'`),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, false)
				Expect(errs).To(BeEmpty())
			})

			It("should allow double-quote characters in identifier fields (gcfg-escaped at render time)", func() {
				secret.Data = map[string][]byte{
					openstack.DomainName: []byte(`my"domain`),
					openstack.TenantName: []byte("my-tenant"),
					openstack.UserName:   []byte("my-user"),
					openstack.Password:   []byte("my-password"),
				}

				errs := ValidateCloudProviderSecret(secret, fldPath, false)
				Expect(errs).To(BeEmpty())
			})

			DescribeTable("should fail when a credential field exceeds its maximum length",
				func(data map[string][]byte, invalidField string) {
					secret.Data = data

					errs := ValidateCloudProviderSecret(secret, fldPath, false)
					Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(invalidField),
					}))))
				},
				Entry("domainName longer than 64 characters", map[string][]byte{
					openstack.DomainName: []byte(strings.Repeat("a", 65)),
					openstack.TenantName: []byte("my-tenant"),
					openstack.UserName:   []byte("my-user"),
					openstack.Password:   []byte("my-password"),
				}, "secret.data[domainName]"),
				Entry("tenantName longer than 64 characters", map[string][]byte{
					openstack.DomainName: []byte("my-domain"),
					openstack.TenantName: []byte(strings.Repeat("a", 65)),
					openstack.UserName:   []byte("my-user"),
					openstack.Password:   []byte("my-password"),
				}, "secret.data[tenantName]"),
				Entry("username longer than 255 characters", map[string][]byte{
					openstack.DomainName: []byte("my-domain"),
					openstack.TenantName: []byte("my-tenant"),
					openstack.UserName:   []byte(strings.Repeat("a", 256)),
					openstack.Password:   []byte("my-password"),
				}, "secret.data[username]"),
				Entry("applicationCredentialName longer than 255 characters", map[string][]byte{
					openstack.DomainName:                  []byte("my-domain"),
					openstack.TenantName:                  []byte("my-tenant"),
					openstack.UserName:                    []byte("my-user"),
					openstack.ApplicationCredentialName:   []byte(strings.Repeat("a", 256)),
					openstack.ApplicationCredentialSecret: []byte("my-app-secret"),
				}, "secret.data[applicationCredentialName]"),
			)

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
