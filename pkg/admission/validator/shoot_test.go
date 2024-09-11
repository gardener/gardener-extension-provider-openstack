// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator_test

import (
	"context"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
	mockmanager "github.com/gardener/gardener/third_party/mock/controller-runtime/manager"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/admission/validator"
)

var _ = Describe("Shoot validator", func() {
	Describe("#Validate", func() {
		const namespace = "garden-dev"

		var (
			shootValidator extensionswebhook.Validator

			ctrl      *gomock.Controller
			mgr       *mockmanager.MockManager
			c         *mockclient.MockClient
			apiReader *mockclient.MockReader
			shoot     *core.Shoot

			ctx = context.TODO()
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())

			scheme := runtime.NewScheme()
			Expect(gardencorev1beta1.AddToScheme(scheme)).To(Succeed())

			c = mockclient.NewMockClient(ctrl)
			apiReader = mockclient.NewMockReader(ctrl)

			mgr = mockmanager.NewMockManager(ctrl)
			mgr.EXPECT().GetScheme().Return(scheme).Times(2)
			mgr.EXPECT().GetClient().Return(c)
			mgr.EXPECT().GetAPIReader().Return(apiReader)
			shootValidator = validator.NewShootValidator(mgr)

			shoot = &core.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: namespace,
				},
				Spec: core.ShootSpec{
					CloudProfile: &core.CloudProfileReference{
						Name: "cloudProfile",
					},
					Provider: core.Provider{
						Type:    "openstack",
						Workers: []core.Worker{},
					},
					Region: "us-west",
					Networking: &core.Networking{
						Nodes: ptr.To("10.250.0.0/16"),
					},
				},
			}
		})

		Context("Workerless Shoot", func() {
			BeforeEach(func() {
				shoot.Spec.Provider.Workers = nil
			})

			It("should not validate", func() {
				err := shootValidator.Validate(ctx, shoot, nil)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
