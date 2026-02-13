// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package access_test

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/subnets"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/controller/infrastructure/infraflow/access"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
)

type fakeNetworking struct {
	client.Networking

	listSubnetsFn func(ctx context.Context, opts subnets.ListOpts) ([]subnets.Subnet, error)
}

func (f *fakeNetworking) ListSubnets(ctx context.Context, opts subnets.ListOpts) ([]subnets.Subnet, error) {
	if f.listSubnetsFn == nil {
		return nil, fmt.Errorf("listSubnetsFn not set")
	}
	return f.listSubnetsFn(ctx, opts)
}

func newNetworkingAccessWithSubnets(returned []subnets.Subnet) access.NetworkingAccess {
	a, err := access.NewNetworkingAccess(&fakeNetworking{
		listSubnetsFn: func(_ context.Context, opts subnets.ListOpts) ([]subnets.Subnet, error) {
			Expect(opts.NetworkID).To(Equal("net-1"))
			return returned, nil
		},
	}, logr.Discard())
	Expect(err).NotTo(HaveOccurred())
	return a
}

var _ = Describe("LookupFloatingPoolSubnetIDs", func() {
	It("propagates errors from ListSubnets", func() {
		ctx := context.Background()
		sentinelErr := fmt.Errorf("network error")

		a, err := access.NewNetworkingAccess(&fakeNetworking{
			listSubnetsFn: func(_ context.Context, opts subnets.ListOpts) ([]subnets.Subnet, error) {
				Expect(opts.NetworkID).To(Equal("net-1"))
				return nil, sentinelErr
			},
		}, logr.Discard())
		Expect(err).NotTo(HaveOccurred())

		ids, err := a.LookupFloatingPoolSubnetIDs(ctx, "net-1", "ext-*")
		Expect(ids).To(BeNil())
		Expect(err).To(MatchError(sentinelErr))
	})

	It("returns all subnet IDs when the pattern is empty", func() {
		ctx := context.Background()
		a := newNetworkingAccessWithSubnets([]subnets.Subnet{
			{ID: "s-1", Name: ""},
			{ID: "s-2", Name: "ext-a"},
			{ID: "s-3", Name: "int-a"},
		})

		ids, err := a.LookupFloatingPoolSubnetIDs(ctx, "net-1", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(ids).To(Equal([]string{"s-1", "s-2", "s-3"}))
	})

	It("matches by glob pattern against the full subnet name", func() {
		ctx := context.Background()
		a := newNetworkingAccessWithSubnets([]subnets.Subnet{
			{ID: "s-1", Name: "ext-a"},
			{ID: "s-2", Name: "ext-b"},
			{ID: "s-3", Name: "internal"},
			{ID: "s-4", Name: ""},
		})

		ids, err := a.LookupFloatingPoolSubnetIDs(ctx, "net-1", "ext-*")
		Expect(err).NotTo(HaveOccurred())
		Expect(ids).To(Equal([]string{"s-1", "s-2"}))
	})

	It("requires regexp matches to cover the whole subnet name", func() {
		ctx := context.Background()
		a := newNetworkingAccessWithSubnets([]subnets.Subnet{
			{ID: "s-1", Name: "ext-a"},
			{ID: "s-2", Name: "ext-b"},
			{ID: "s-3", Name: "internal"},
		})

		_, err := a.LookupFloatingPoolSubnetIDs(ctx, "net-1", "~ext")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("no subnet found matching"))

		ids, err := a.LookupFloatingPoolSubnetIDs(ctx, "net-1", "~ext-.*")
		Expect(err).NotTo(HaveOccurred())
		Expect(ids).To(Equal([]string{"s-1", "s-2"}))
	})

	It("supports negation patterns and still skips unnamed subnets", func() {
		ctx := context.Background()
		a := newNetworkingAccessWithSubnets([]subnets.Subnet{
			{ID: "s-1", Name: "ext-a"},
			{ID: "s-2", Name: "int-a"},
			{ID: "s-3", Name: ""},
		})

		ids, err := a.LookupFloatingPoolSubnetIDs(ctx, "net-1", "!ext-*")
		Expect(err).NotTo(HaveOccurred())
		Expect(ids).To(Equal([]string{"s-2"}))
	})

	It("supports combined negation and regexp patterns", func() {
		ctx := context.Background()
		a := newNetworkingAccessWithSubnets([]subnets.Subnet{
			{ID: "s-1", Name: "ext-a"},
			{ID: "s-2", Name: "int-a"},
			{ID: "s-3", Name: "internal"},
			{ID: "s-4", Name: ""},
			{ID: "s-5", Name: "ext-b"},
		})

		ids, err := a.LookupFloatingPoolSubnetIDs(ctx, "net-1", "!~ext-.*")
		Expect(err).NotTo(HaveOccurred())
		Expect(ids).To(Equal([]string{"s-2", "s-3"}))
	})

	It("returns an error for invalid regexp patterns", func() {
		ctx := context.Background()
		a := newNetworkingAccessWithSubnets([]subnets.Subnet{{ID: "s-1", Name: "ext-a"}})

		_, err := a.LookupFloatingPoolSubnetIDs(ctx, "net-1", "~[")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("invalid subnet regexp pattern"))
	})

	It("returns a helpful error when no subnets match", func() {
		ctx := context.Background()
		a := newNetworkingAccessWithSubnets([]subnets.Subnet{{ID: "s-1", Name: "int-a"}})

		pat := "ext-*"
		_, err := a.LookupFloatingPoolSubnetIDs(ctx, "net-1", pat)
		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError(ContainSubstring(pat)))
	})
})
