// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package validation_test

import (
	"strings"

	. "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/validation"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Secret validation", func() {

	DescribeTable("#ValidateCloudProviderSecret",
		func(data map[string][]byte, matcher gomegatypes.GomegaMatcher) {
			secret := &corev1.Secret{
				Data: data,
			}
			err := ValidateCloudProviderSecret(secret)

			Expect(err).To(matcher)
		},

		Entry("should return error when the domain name field is missing",
			map[string][]byte{
				openstack.TenantName: []byte("tenant"),
				openstack.UserName:   []byte("user"),
				openstack.Password:   []byte("password"),
			},
			HaveOccurred(),
		),

		Entry("should return error when the domain name is empty",
			map[string][]byte{
				openstack.DomainName: {},
				openstack.TenantName: []byte("tenant"),
				openstack.UserName:   []byte("user"),
				openstack.Password:   []byte("password"),
			},
			HaveOccurred(),
		),

		Entry("should return error when the domain name contains a trailing space",
			map[string][]byte{
				openstack.DomainName: []byte("domain "),
				openstack.TenantName: []byte("tenant"),
				openstack.UserName:   []byte("user"),
				openstack.Password:   []byte("password"),
			},
			HaveOccurred(),
		),

		Entry("should return error when the tenant name field is missing",
			map[string][]byte{
				openstack.DomainName: []byte("domain"),
				openstack.UserName:   []byte("user"),
				openstack.Password:   []byte("password"),
			},
			HaveOccurred(),
		),

		Entry("should return error when the tenant name is empty",
			map[string][]byte{
				openstack.DomainName: []byte("domain"),
				openstack.TenantName: {},
				openstack.UserName:   []byte("user"),
				openstack.Password:   []byte("password"),
			},
			HaveOccurred(),
		),

		Entry("should return error when the tenant name contains a trailing space",
			map[string][]byte{
				openstack.DomainName: []byte("domain"),
				openstack.TenantName: []byte("tenant "),
				openstack.UserName:   []byte("user"),
				openstack.Password:   []byte("password"),
			},
			HaveOccurred(),
		),

		Entry("should return error when the tenant name is too long",
			map[string][]byte{
				openstack.DomainName: []byte("domain"),
				openstack.TenantName: []byte(strings.Repeat("a", 65)),
				openstack.UserName:   []byte("user"),
				openstack.Password:   []byte("password"),
			},
			HaveOccurred(),
		),

		Entry("should return error when the user name field is missing",
			map[string][]byte{
				openstack.DomainName: []byte("domain"),
				openstack.TenantName: []byte("tenant"),
				openstack.Password:   []byte("password"),
			},
			HaveOccurred(),
		),

		Entry("should return error when the user name is empty",
			map[string][]byte{
				openstack.DomainName: []byte("domain"),
				openstack.TenantName: []byte("tenant"),
				openstack.UserName:   {},
				openstack.Password:   []byte("password"),
			},
			HaveOccurred(),
		),

		Entry("should return error when the user name contains a trailing space",
			map[string][]byte{
				openstack.DomainName: []byte("domain"),
				openstack.TenantName: []byte("tenant"),
				openstack.UserName:   []byte("user "),
				openstack.Password:   []byte("password"),
			},
			HaveOccurred(),
		),

		Entry("should return error when the password field is missing",
			map[string][]byte{
				openstack.DomainName: []byte("domain"),
				openstack.TenantName: []byte("tenant"),
				openstack.UserName:   []byte("user"),
			},
			HaveOccurred(),
		),

		Entry("should return error when the password is empty",
			map[string][]byte{
				openstack.DomainName: []byte("domain"),
				openstack.TenantName: []byte("tenant"),
				openstack.UserName:   []byte("user"),
				openstack.Password:   {},
			},
			HaveOccurred(),
		),

		Entry("should return error when the password contains a trailing new line",
			map[string][]byte{
				openstack.DomainName: []byte("domain"),
				openstack.TenantName: []byte("tenant"),
				openstack.UserName:   []byte("user"),
				openstack.Password:   []byte("password\n"),
			},
			HaveOccurred(),
		),

		Entry("should return error when the auth URL is not a valid URL",
			map[string][]byte{
				openstack.DomainName: []byte("domain"),
				openstack.TenantName: []byte("tenant"),
				openstack.UserName:   []byte("user"),
				openstack.Password:   []byte("password"),
				openstack.AuthURL:    []byte("foo#%(bar"),
			},
			HaveOccurred(),
		),

		Entry("should succeed when the client credentials are valid (without AuthURL)",
			map[string][]byte{
				openstack.DomainName: []byte("domain"),
				openstack.TenantName: []byte("tenant"),
				openstack.UserName:   []byte("user"),
				openstack.Password:   []byte("password"),
			},
			BeNil(),
		),

		Entry("should succeed when the client credentials are valid (with AuthURL)",
			map[string][]byte{
				openstack.DomainName: []byte("domain"),
				openstack.TenantName: []byte("tenant"),
				openstack.UserName:   []byte("user"),
				openstack.Password:   []byte("password"),
				openstack.AuthURL:    []byte("https://foo.bar"),
			},
			BeNil(),
		),

		Entry("should return error when the application credential id contains a trailing new line",
			map[string][]byte{
				openstack.DomainName:                  []byte("domain"),
				openstack.TenantName:                  []byte("tenant"),
				openstack.ApplicationCredentialID:     []byte("app-id\n"),
				openstack.ApplicationCredentialSecret: []byte("app-secret"),
			},
			HaveOccurred(),
		),

		Entry("should return error when the application credential secret contains a trailing new line",
			map[string][]byte{
				openstack.DomainName:                  []byte("domain"),
				openstack.TenantName:                  []byte("tenant"),
				openstack.ApplicationCredentialID:     []byte("app-id"),
				openstack.ApplicationCredentialSecret: []byte("app-secret\n"),
			},
			HaveOccurred(),
		),

		Entry("should return error when neither username nor application credential id is given",
			map[string][]byte{
				openstack.DomainName: []byte("domain"),
				openstack.TenantName: []byte("tenant"),
			},
			HaveOccurred(),
		),

		Entry("should return error when both username and application credential id is given",
			map[string][]byte{
				openstack.DomainName:              []byte("domain"),
				openstack.TenantName:              []byte("tenant"),
				openstack.UserName:                []byte("user"),
				openstack.ApplicationCredentialID: []byte("app-id"),
			},
			HaveOccurred(),
		),

		Entry("should return error when application credential secret is missing",
			map[string][]byte{
				openstack.DomainName:              []byte("domain"),
				openstack.TenantName:              []byte("tenant"),
				openstack.ApplicationCredentialID: []byte("app-id"),
			},
			HaveOccurred(),
		),

		Entry("should succeed when the client application credentials are valid (with AuthURL)",
			map[string][]byte{
				openstack.DomainName:                  []byte("domain"),
				openstack.TenantName:                  []byte("tenant"),
				openstack.ApplicationCredentialID:     []byte("app-id"),
				openstack.ApplicationCredentialSecret: []byte("app-secret"),
				openstack.AuthURL:                     []byte("https://foo.bar"),
			},
			BeNil(),
		),
	)
})
