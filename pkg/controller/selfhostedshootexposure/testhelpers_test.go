// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package selfhostedshootexposure_test

import (
	"context"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
)

// fakeFactoryFactory is a test double for openstackclient.FactoryFactory that always returns the
// same pre-built (mock-backed) factory.
type fakeFactoryFactory struct {
	factory openstackclient.Factory
}

func (f fakeFactoryFactory) NewFactory(_ context.Context, _ *openstack.Credentials) (openstackclient.Factory, error) {
	return f.factory, nil
}
