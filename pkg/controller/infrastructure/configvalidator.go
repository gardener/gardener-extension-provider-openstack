// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"
	"fmt"
	"slices"

	"github.com/gardener/gardener/extensions/pkg/controller/infrastructure"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/subnets"
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
	clientFactory, err := c.clientFactoryFactory.NewFactory(ctx, credentials)
	if err != nil {
		allErrs = append(allErrs, field.InternalError(nil, fmt.Errorf("could not create Openstack client factory: %+v", err)))
		return allErrs
	}
	networkingClient, err := clientFactory.Networking(openstackclient.WithRegion(infra.Spec.Region))
	if err != nil {
		allErrs = append(allErrs, field.InternalError(nil, fmt.Errorf("could not create Openstack networking client: %+v", err)))
		return allErrs
	}

	// Validate infrastructure config
	logger.Info("Validating infrastructure configuration")
	allErrs = append(allErrs, c.validateFloatingPoolName(ctx, networkingClient, config.FloatingPoolName, field.NewPath("floatingPoolName"))...)
	if config.Networks.ID != nil {
		allErrs = append(allErrs, c.validateNetwork(ctx, networkingClient, *config.Networks.ID, field.NewPath("networks").Child("id"))...)
	}
	if config.Networks.ID != nil && config.Networks.SubnetID != nil {
		allErrs = append(allErrs, c.validateSubnet(ctx, networkingClient, *config.Networks.SubnetID, *config.Networks.ID, field.NewPath("networks").Child("subnetId"))...)
	}
	if config.Networks.Router != nil && config.Networks.Router.ID != "" {
		allErrs = append(allErrs, c.validateRouter(ctx, networkingClient, config.Networks.Router.ID, field.NewPath("networks").Child("router").Child("id"))...)
		if config.Networks.SubnetID != nil {
			allErrs = append(allErrs, c.validateRouterHasSubnetInterface(ctx, networkingClient, config.Networks.Router.ID, *config.Networks.SubnetID, field.NewPath("networks").Child("router"))...)
		}
	}
	if config.Networks.SecurityGroupID != nil {
		allErrs = append(allErrs, c.validateSecurityGroup(ctx, networkingClient, *config.Networks.SecurityGroupID, field.NewPath("networks").Child("securityGroupId"))...)
	}
	if config.Networks.IPv6 != nil && config.Networks.IPv6.NodeSubnetID != nil && config.Networks.ID != nil {
		allErrs = append(allErrs, c.validateSubnet(ctx, networkingClient, *config.Networks.IPv6.NodeSubnetID, *config.Networks.ID, field.NewPath("networks").Child("ipv6").Child("nodeSubnetId"))...)
		if config.Networks.Router != nil && config.Networks.Router.ID != "" {
			allErrs = append(allErrs, c.validateRouterHasSubnetInterface(ctx, networkingClient, config.Networks.Router.ID, *config.Networks.IPv6.NodeSubnetID, field.NewPath("networks").Child("router"))...)
		}
	}
	if config.Networks.ShareNetworkID != nil {
		sharedFilesystemClient, err := clientFactory.SharedFilesystem(openstackclient.WithRegion(infra.Spec.Region))
		if err != nil {
			allErrs = append(allErrs, field.InternalError(nil, fmt.Errorf("could not create OpenStack shared filesystem client: %w", err)))
			return allErrs
		}
		allErrs = append(allErrs, c.validateShareNetwork(ctx, sharedFilesystemClient, *config.Networks.ShareNetworkID, field.NewPath("networks").Child("shareNetworkId"))...)
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
	if !slices.Contains(externalNetworkNames, floatingPoolName) {
		allErrs = append(allErrs, field.NotFound(fldPath, floatingPoolName))
	}

	return allErrs
}

func (c *configValidator) validateNetwork(ctx context.Context, networkingClient openstackclient.Networking, networkID string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	nets, err := networkingClient.ListNetwork(ctx, networks.ListOpts{ID: networkID})
	if err != nil {
		allErrs = append(allErrs, field.InternalError(fldPath, fmt.Errorf("could not list networks: %w", err)))
		return allErrs
	}
	if len(nets) == 0 {
		allErrs = append(allErrs, field.NotFound(fldPath, networkID))
	}

	return allErrs
}

func (c *configValidator) validateSubnet(ctx context.Context, networkingClient openstackclient.Networking, subnetID, networkID string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	snets, err := networkingClient.ListSubnets(ctx, subnets.ListOpts{ID: subnetID, NetworkID: networkID})
	if err != nil {
		allErrs = append(allErrs, field.InternalError(fldPath, fmt.Errorf("could not list subnets: %w", err)))
		return allErrs
	}
	if len(snets) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, subnetID, fmt.Sprintf("subnet %q not found in network %q", subnetID, networkID)))
	}

	return allErrs
}

func (c *configValidator) validateRouter(ctx context.Context, networkingClient openstackclient.Networking, routerID string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	router, err := networkingClient.GetRouterByID(ctx, routerID)
	if err != nil {
		allErrs = append(allErrs, field.InternalError(fldPath, fmt.Errorf("could not get router: %w", err)))
		return allErrs
	}
	if router == nil {
		allErrs = append(allErrs, field.NotFound(fldPath, routerID))
	}

	return allErrs
}

func (c *configValidator) validateRouterHasSubnetInterface(ctx context.Context, networkingClient openstackclient.Networking, routerID, subnetID string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	port, err := networkingClient.GetRouterInterfacePort(ctx, routerID, subnetID)
	if err != nil {
		allErrs = append(allErrs, field.InternalError(fldPath, fmt.Errorf("could not check router interface: %w", err)))
		return allErrs
	}
	if port == nil {
		allErrs = append(allErrs, field.Invalid(fldPath, routerID,
			fmt.Sprintf("router %q does not have an interface to subnet %q; please attach the router to the subnet before creating the shoot", routerID, subnetID)))
	}

	return allErrs
}

func (c *configValidator) validateSecurityGroup(ctx context.Context, networkingClient openstackclient.Networking, securityGroupID string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	sg, err := networkingClient.GetSecurityGroup(ctx, securityGroupID)
	if err != nil {
		allErrs = append(allErrs, field.InternalError(fldPath, fmt.Errorf("could not get security group %q: %w", securityGroupID, err)))
		return allErrs
	}
	if sg == nil {
		allErrs = append(allErrs, field.NotFound(fldPath, securityGroupID))
	}

	return allErrs
}

func (c *configValidator) validateShareNetwork(ctx context.Context, sharedFilesystemClient openstackclient.SharedFilesystem, shareNetworkID string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	sn, err := sharedFilesystemClient.GetShareNetwork(ctx, shareNetworkID)
	if err != nil {
		allErrs = append(allErrs, field.InternalError(fldPath, fmt.Errorf("could not get share network %q: %w", shareNetworkID, err)))
		return allErrs
	}
	if sn == nil {
		allErrs = append(allErrs, field.NotFound(fldPath, shareNetworkID))
	}

	return allErrs
}
