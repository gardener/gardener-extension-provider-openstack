// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package openstack_test

import (
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
})
