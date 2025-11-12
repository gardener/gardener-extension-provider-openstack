// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
)

var _ = Describe("#ErrorCodes", func() {
	Context("dependenciesRegexp", func() {
		It("should match original network deletion error", func() {
			errorMsg := "Unable to complete operation on network 1bf12f9b-5re4-56e1-b240-687c8fa05b0c. There are one or more ports still in use on the network, id for these ports is: 9e46c0a8-6098-49b0-ab4d-y6d07c340f23."
			Expect(KnownCodes[gardencorev1beta1.ErrorInfraDependencies](errorMsg)).To(BeTrue())
		})

		It("should not match unrelated error", func() {
			msg := "Some other error message"
			Expect(KnownCodes[gardencorev1beta1.ErrorInfraDependencies](msg)).To(BeFalse())
		})
	})

	Context("resourcesDepletedRegexp", func() {
		It("should match resource exhausted error", func() {
			errorMsg := "Cloud provider message - machine codes error: code = [ResourceExhausted] message = [error waiting for server [ID=\"redacted\"] to reach target status: server [ID=\"redacted\"] reached unexpected status \"ERROR\",Message:No valid host was found.}]"
			Expect(KnownCodes[gardencorev1beta1.ErrorInfraResourcesDepleted](errorMsg)).To(BeTrue())
		})

		It("should not match unrelated error", func() {
			msg := "Some other error message"
			Expect(KnownCodes[gardencorev1beta1.ErrorInfraResourcesDepleted](msg)).To(BeFalse())
		})
	})
})
