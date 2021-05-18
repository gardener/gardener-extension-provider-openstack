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

		Entry("should succeed when the client credentials are valid",
			map[string][]byte{
				openstack.DomainName: []byte("domain"),
				openstack.TenantName: []byte("tenant"),
				openstack.UserName:   []byte("user"),
				openstack.Password:   []byte("password"),
			},
			BeNil(),
		),
	)
})
