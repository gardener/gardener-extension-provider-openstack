// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	"context"

	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/keypairs"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"k8s.io/utils/ptr"

	clientmocks "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client/mocks"
)

var _ = Describe("ensureSSHKeyPair", func() {
	var (
		ctrl *gomock.Controller
		ctx  context.Context
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		ctx = context.Background()
	})

	AfterEach(func() { ctrl.Finish() })

	It("stores the key pair name and returns early when the public key matches", func() {
		pubKey := "ssh-rsa AAAAB3Nza... user@host"
		fctx, _ := newFixture(ctrl, defaultInfraConfig())
		fctx.infra.Spec.SSHPublicKey = []byte(pubKey)
		mockCompute := clientmocks.NewMockCompute(ctrl)
		fctx.compute = mockCompute

		mockCompute.EXPECT().
			GetKeyPair(ctx, gomock.Any()).
			Return(&keypairs.KeyPair{Name: "my-keypair", PublicKey: pubKey}, nil)
		// DeleteKeyPair and CreateKeyPair must NOT be called

		Expect(fctx.ensureSSHKeyPair(ctx)).To(Succeed())
		Expect(ptr.Deref(fctx.state.Get(NameKeyPair), "")).To(Equal("my-keypair"))
	})

	It("deletes and recreates the key pair when the public key has changed", func() {
		oldKey := "ssh-rsa AAAAB3Nza... old@host"
		newKey := "ssh-rsa CCCCCCCC... new@host"
		fctx, _ := newFixture(ctrl, defaultInfraConfig())
		fctx.infra.Spec.SSHPublicKey = []byte(newKey)
		mockCompute := clientmocks.NewMockCompute(ctrl)
		fctx.compute = mockCompute

		keypairName := fctx.defaultSSHKeypairName()

		mockCompute.EXPECT().
			GetKeyPair(ctx, keypairName).
			Return(&keypairs.KeyPair{Name: keypairName, PublicKey: oldKey}, nil)
		mockCompute.EXPECT().
			DeleteKeyPair(ctx, keypairName).
			Return(nil)
		mockCompute.EXPECT().
			CreateKeyPair(ctx, keypairName, newKey).
			Return(&keypairs.KeyPair{Name: keypairName, PublicKey: newKey}, nil)

		Expect(fctx.ensureSSHKeyPair(ctx)).To(Succeed())
		Expect(ptr.Deref(fctx.state.Get(NameKeyPair), "")).To(Equal(keypairName))
	})

	It("creates a new key pair when none exists", func() {
		pubKey := "ssh-rsa AAAAB3Nza... user@host"
		fctx, _ := newFixture(ctrl, defaultInfraConfig())
		fctx.infra.Spec.SSHPublicKey = []byte(pubKey)
		mockCompute := clientmocks.NewMockCompute(ctrl)
		fctx.compute = mockCompute

		keypairName := fctx.defaultSSHKeypairName()

		mockCompute.EXPECT().
			GetKeyPair(ctx, keypairName).
			Return(nil, nil)
		mockCompute.EXPECT().
			CreateKeyPair(ctx, keypairName, pubKey).
			Return(&keypairs.KeyPair{Name: keypairName, PublicKey: pubKey}, nil)

		Expect(fctx.ensureSSHKeyPair(ctx)).To(Succeed())
		Expect(ptr.Deref(fctx.state.Get(NameKeyPair), "")).To(Equal(keypairName))
	})
})
