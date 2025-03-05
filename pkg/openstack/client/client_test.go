// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client_test

import (
	"github.com/gophercloud/gophercloud/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
)

var _ = Describe("Client", func() {
	Describe("IgnoreNotFoundError", func() {
		It("ignore 404 not found Error should return nil", func() {
			err := gophercloud.ErrUnexpectedResponseCode{
				URL:      "http://example.com",
				Method:   "GET",
				Expected: []int{200},
				Actual:   404,
				Body:     nil,
			}
			Expect(openstackclient.IgnoreNotFoundError(err)).To(BeNil())
		})
	})
})
