// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package v1alpha1_test

import (
	"time"

	. "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/config/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Defaults", func() {
	Context("#SetDefaults_ControllerConfiguration", func() {
		var obj *ControllerConfiguration

		BeforeEach(func() {
			obj = &ControllerConfiguration{}
		})

		It("should default the controller configuration", func() {
			SetDefaults_ControllerConfiguration(obj)

			Expect(obj.ApplicationCredentialConfig).NotTo(BeNil())
		})
	})

	Context("#SetDefaults_ApplicationCredentialConfig", func() {
		var obj *ApplicationCredentialConfig

		BeforeEach(func() {
			obj = &ApplicationCredentialConfig{}
		})

		It("should default the application crendential config", func() {
			SetDefaults_ApplicationCrendentialConfig(obj)

			Expect(*obj.Lifetime).To(Equal(metav1.Duration{Duration: time.Hour * 48}))
			Expect(*obj.OpenstackExpirationPeriod).To(Equal(metav1.Duration{Duration: time.Hour * 24 * 30}))
			Expect(*obj.RenewThreshold).To(Equal(metav1.Duration{Duration: time.Hour * 24 * 3}))
		})
	})

})
