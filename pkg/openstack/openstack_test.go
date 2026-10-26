// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package openstack_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
)

var _ = Describe("Openstack", func() {
	testUser := "user"
	testPassword := "pass"
	testAppID := "appID"
	testAppName := "appName"
	testAppSecret := "appSecret"
	Describe("ValidateSecrets", func() {
		It("should fail if both basic auth and app credentials are provided", func() {
			err := openstack.ValidateSecrets(testUser, testPassword, testAppID, testAppName, testAppSecret)
			Expect(err).To(HaveOccurred())
		})
		It("should fail if no password and no app secret are provided", func() {
			err := openstack.ValidateSecrets(testUser, "", testAppID, testAppName, "")
			Expect(err).To(HaveOccurred())
		})
		It("should fail if username but no password are given", func() {
			err := openstack.ValidateSecrets(testUser, "", "", "", "")
			Expect(err).To(HaveOccurred())
		})
		It("should be successful if only basic auth credentials are provided", func() {
			err := openstack.ValidateSecrets(testUser, testPassword, "", "", "")
			Expect(err).To(Succeed())
		})
		It("should be successful if only app credentials are provided", func() {
			err := openstack.ValidateSecrets("", "", testAppID, testAppName, testAppSecret)
			Expect(err).To(Succeed())
		})
	})

	Describe("ExtractCredentialsFromData", func() {
		baseData := func() map[string][]byte {
			return map[string][]byte{
				openstack.DomainName: []byte("my-domain"),
				openstack.TenantName: []byte("my-tenant"),
				openstack.UserName:   []byte("my-user"),
				openstack.Password:   []byte("my-password"),
			}
		}

		It("should extract valid credentials", func() {
			creds, err := openstack.ExtractCredentialsFromData(baseData(), false)
			Expect(err).NotTo(HaveOccurred())
			Expect(creds.DomainName).To(Equal("my-domain"))
		})

		baseAppCredData := func() map[string][]byte {
			return map[string][]byte{
				openstack.DomainName:                  []byte("my-domain"),
				openstack.TenantName:                  []byte("my-tenant"),
				openstack.ApplicationCredentialID:     []byte("my-app-id"),
				openstack.ApplicationCredentialSecret: []byte("my-app-secret"),
			}
		}

		DescribeTable("should fail closed when a credential field contains a control character",
			func(key string, dataFn func() map[string][]byte) {
				data := dataFn()
				data[key] = []byte("value\ntls-insecure=true")

				_, err := openstack.ExtractCredentialsFromData(data, false)
				Expect(err).To(MatchError(ContainSubstring("must not contain control characters")))
			},
			Entry("domainName", openstack.DomainName, baseData),
			Entry("tenantName", openstack.TenantName, baseData),
			Entry("username", openstack.UserName, baseData),
			Entry("password", openstack.Password, baseData),
			Entry("applicationCredentialID", openstack.ApplicationCredentialID, baseAppCredData),
			Entry("applicationCredentialName", openstack.ApplicationCredentialName, baseAppCredData),
			Entry("applicationCredentialSecret", openstack.ApplicationCredentialSecret, baseAppCredData),
		)

		It("should reject double-quote characters in credential fields (breaks INI quoting)", func() {
			data := baseData()
			data[openstack.Password] = []byte(`pw"with'quotes`)

			_, err := openstack.ExtractCredentialsFromData(data, false)
			Expect(err).To(HaveOccurred())
		})
	})
})
