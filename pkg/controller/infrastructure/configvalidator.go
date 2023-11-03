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

package infrastructure

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/controller/infrastructure"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/go-logr/logr"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/layer3/routers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/subnets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
)

// configValidator implements ConfigValidator for openstack infrastructure resources.
type configValidator struct {
	client               client.Client
	clientFactoryFactory openstackclient.FactoryFactory
	logger               logr.Logger
}

// NewConfigValidator creates a new ConfigValidator.
func NewConfigValidator(mgr manager.Manager, clientFactoryFactory openstackclient.FactoryFactory, logger logr.Logger) infrastructure.ConfigValidator {
	return &configValidator{
		client:               mgr.GetClient(),
		clientFactoryFactory: clientFactoryFactory,
		logger:               logger.WithName("openstack-infrastructure-config-validator"),
	}
}

// Validate validates the provider config of the given infrastructure resource with the cloud provider.
func (c *configValidator) Validate(ctx context.Context, infra *extensionsv1alpha1.Infrastructure) field.ErrorList {
	allErrs := field.ErrorList{}

	logger := c.logger.WithValues("infrastructure", client.ObjectKeyFromObject(infra))

	// Get provider config from the infrastructure resource
	config, err := helper.InfrastructureConfigFromInfrastructure(infra)
	if err != nil {
		allErrs = append(allErrs, field.InternalError(nil, err))
		return allErrs
	}

	// Create openstack networking client
	credentials, err := openstack.GetCredentials(ctx, c.client, infra.Spec.SecretRef, false)
	if err != nil {
		allErrs = append(allErrs, field.InternalError(nil, fmt.Errorf("could not get Openstack credentials: %+v", err)))
		return allErrs
	}
	clientFactory, err := c.clientFactoryFactory.NewFactory(credentials)
	if err != nil {
		allErrs = append(allErrs, field.InternalError(nil, fmt.Errorf("could not create Openstack client factory: %+v", err)))
		return allErrs
	}
	networkingClient, err := clientFactory.Networking()
	if err != nil {
		allErrs = append(allErrs, field.InternalError(nil, fmt.Errorf("could not create Openstack networking client: %+v", err)))
		return allErrs
	}

	// Validate infrastructure config
	logger.Info("Validating infrastructure configuration")
	allErrs = append(allErrs, c.validateFloatingPoolName(ctx, networkingClient, config.FloatingPoolName, field.NewPath("floatingPoolName"))...)
	if config.Networks.ID != nil {
		allErrs = append(allErrs, c.validateNetwork(ctx, networkingClient, *config.Networks.ID, field.NewPath("networks.id"))...)
	}
	if config.Networks.SubnetID != nil {
		allErrs = append(allErrs, c.validateSubnet(ctx, networkingClient, *config.Networks.SubnetID, *config.Networks.ID, field.NewPath("networks.subnetId"))...)
	}
	if config.Networks.Router != nil && config.Networks.Router.ID != "" {
		allErrs = append(allErrs, c.validateRouter(ctx, networkingClient, config.Networks.Router.ID, field.NewPath("networks.router.id"))...)
	}

	return allErrs
}

func (c *configValidator) validateFloatingPoolName(ctx context.Context, networkingClient openstackclient.Networking, floatingPoolName string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Get external network names
	externalNetworkNames, err := networkingClient.GetExternalNetworkNames(ctx)
	if err != nil {
		allErrs = append(allErrs, field.InternalError(fldPath, fmt.Errorf("could not get external network names: %w", err)))
		return allErrs
	}

	// Check if floatingPoolName is contained in the list of external network names
	if !utils.ValueExists(floatingPoolName, externalNetworkNames) {
		allErrs = append(allErrs, field.NotFound(fldPath, floatingPoolName))
	}

	return allErrs
}

func (c *configValidator) validateNetwork(ctx context.Context, networkingClient openstackclient.Networking, networkID string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	networks, err := networkingClient.ListNetwork(networks.ListOpts{ID: networkID})
	if err != nil {
		allErrs = append(allErrs, field.InternalError(fldPath, fmt.Errorf("could not get network: %w", err)))
		return allErrs
	}
	if len(networks) == 0 {
		allErrs = append(allErrs, field.NotFound(fldPath, networkID))
		return allErrs
	}
	return allErrs
}

func (c *configValidator) validateSubnet(ctx context.Context, networkingClient openstackclient.Networking, subnetID, networkID string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// validate subnet existence
	subnets, err := networkingClient.ListSubnets(subnets.ListOpts{ID: subnetID})
	if err != nil {
		allErrs = append(allErrs, field.InternalError(fldPath, fmt.Errorf("could not get subnet: %w", err)))
		return allErrs
	}
	if len(subnets) == 0 {
		allErrs = append(allErrs, field.NotFound(fldPath, subnetID))
		return allErrs
	}

	// validate subnet is in defined network
	if subnets[0].NetworkID != networkID {
		allErrs = append(allErrs, field.Invalid(fldPath, subnetID, fmt.Sprintf("specified subnet isn't a child of the specified network %q", networkID)))
	}

	return allErrs
}

func (c *configValidator) validateRouter(ctx context.Context, networkingClient openstackclient.Networking, routerID string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	routers, err := networkingClient.ListRouters(routers.ListOpts{ID: routerID})
	if err != nil {
		allErrs = append(allErrs, field.InternalError(fldPath, fmt.Errorf("could not get subnet: %w", err)))
		return allErrs
	}
	if len(routers) == 0 {
		allErrs = append(allErrs, field.NotFound(fldPath, routerID))
		return allErrs
	}
	return allErrs
}
