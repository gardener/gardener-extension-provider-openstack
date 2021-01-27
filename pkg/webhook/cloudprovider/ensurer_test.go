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

package cloudprovider

import (
	"context"
	"testing"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/install"
	openstackv1alpha1 "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"
	types "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/webhook/cloudprovider"
	gcontext "github.com/gardener/gardener/extensions/pkg/webhook/context"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CloudProvider Webhook Suite")
}

var _ = Describe("Ensurer", func() {
	var (
		ctx     = context.TODO()
		ectx    gcontext.GardenContext
		ensurer cloudprovider.Ensurer
		scheme  *runtime.Scheme

		authUrl = "foo://bar"
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		install.Install(scheme)

		ectx = gcontext.NewInternalGardenContext(
			&extensionscontroller.Cluster{
				CloudProfile: &gardencorev1beta1.CloudProfile{
					TypeMeta: metav1.TypeMeta{
						APIVersion: gardencorev1beta1.GroupName,
						Kind:       "CloudProfile",
					},
					Spec: gardencorev1beta1.CloudProfileSpec{
						ProviderConfig: &runtime.RawExtension{
							Object: &openstackv1alpha1.CloudProfileConfig{
								TypeMeta: metav1.TypeMeta{
									Kind:       "CloudProfileConfig",
									APIVersion: openstackv1alpha1.SchemeGroupVersion.String(),
								},
								KeyStoneURL: authUrl,
							},
						},
					},
				},
				Shoot: &gardencorev1beta1.Shoot{},
			},
		)
		ensurer = NewEnsurer(logger)
		err := ensurer.(inject.Scheme).InjectScheme(scheme)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Should ensure auth_url if present in cluster object", func() {
		var (
			new = &corev1.Secret{}
		)
		err := ensurer.EnsureCloudProviderSecret(ctx, ectx, new, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(new.Data[types.AuthURL])).To(Equal(authUrl))
	})

	It("Should return an error if no authURL is found", func() {
		var (
			new = &corev1.Secret{}
		)
		ectx := gcontext.NewInternalGardenContext(
			&extensionscontroller.Cluster{
				CloudProfile: &gardencorev1beta1.CloudProfile{
					Spec: gardencorev1beta1.CloudProfileSpec{
						ProviderConfig: &runtime.RawExtension{
							Object: &openstackv1alpha1.CloudProfileConfig{},
						},
					},
				},
				Shoot: &gardencorev1beta1.Shoot{},
			},
		)
		err := ensurer.EnsureCloudProviderSecret(ctx, ectx, new, nil)
		Expect(err).To(HaveOccurred())
	})
})
