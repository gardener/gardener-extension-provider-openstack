// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package validator

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	openstackvalidation "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/validation"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type validationContext struct {
	shoot              *core.Shoot
	infraConfig        *openstack.InfrastructureConfig
	cpConfig           *openstack.ControlPlaneConfig
	cloudProfile       *gardencorev1beta1.CloudProfile
	cloudProfileConfig *openstack.CloudProfileConfig
}

var (
	specPath           = field.NewPath("spec")
	providerConfigPath = specPath.Child("providerConfig")
	nwPath             = specPath.Child("networking")
	providerPath       = specPath.Child("provider")
	infraConfigPath    = providerPath.Child("infrastructureConfig")
	cpConfigPath       = providerPath.Child("controlPlaneConfig")
	workersPath        = providerPath.Child("workers")
)

func (v *Shoot) validateShootCreation(ctx context.Context, shoot *core.Shoot) error {
	valContext, err := newValidationContext(ctx, v.decoder, v.client, shoot)
	if err != nil {
		return err
	}

	allErrs := field.ErrorList{}

	allErrs = append(allErrs, openstackvalidation.ValidateInfrastructureConfigAgainstCloudProfile(valContext.infraConfig, valContext.shoot.Spec.Region, valContext.cloudProfileConfig, infraConfigPath)...)
	allErrs = append(allErrs, openstackvalidation.ValidateControlPlaneConfigAgainstCloudProfile(valContext.cpConfig, valContext.shoot.Spec.Region, valContext.cloudProfile, valContext.cloudProfileConfig, cpConfigPath)...)
	allErrs = append(allErrs, v.validateShoot(valContext)...)
	return allErrs.ToAggregate()
}

func (v *Shoot) validateShootUpdate(ctx context.Context, oldShoot, shoot *core.Shoot) error {
	oldValContext, err := newValidationContext(ctx, v.decoder, v.client, oldShoot)
	if err != nil {
		return err
	}

	valContext, err := newValidationContext(ctx, v.decoder, v.client, shoot)
	if err != nil {
		return err
	}

	allErrs := field.ErrorList{}

	allErrs = append(allErrs, openstackvalidation.ValidateInfrastructureConfigUpdate(oldValContext.infraConfig, valContext.infraConfig, infraConfigPath)...)

	// Only validate against cloud profile when related configuration is updated.
	// This ensures that already running shoots won't break after constraints were removed from the cloud profile.
	if oldValContext.infraConfig.FloatingPoolName != valContext.infraConfig.FloatingPoolName {
		allErrs = append(allErrs, openstackvalidation.ValidateInfrastructureConfigAgainstCloudProfile(valContext.infraConfig, valContext.shoot.Spec.Region, valContext.cloudProfileConfig, infraConfigPath)...)
	}

	var (
		oldCpConfig = oldValContext.cpConfig
		cpConfig    = valContext.cpConfig
	)
	if errList := openstackvalidation.ValidateControlPlaneConfigUpdate(oldCpConfig, cpConfig, cpConfigPath); len(errList) != 0 {
		return errList.ToAggregate()
	}

	// Only validate against cloud profile when related configuration is updated.
	// This ensures that already running shoots won't break after constraints were removed from the cloud profile.
	if oldCpConfig.LoadBalancerProvider != cpConfig.LoadBalancerProvider ||
		oldCpConfig.Zone != cpConfig.Zone ||
		!equality.Semantic.DeepEqual(oldCpConfig.LoadBalancerClasses, cpConfig.LoadBalancerClasses) {
		allErrs = append(allErrs, openstackvalidation.ValidateControlPlaneConfigAgainstCloudProfile(cpConfig, valContext.shoot.Spec.Region, valContext.cloudProfile, valContext.cloudProfileConfig, cpConfigPath)...)
	}

	if errList := openstackvalidation.ValidateWorkersUpdate(oldValContext.shoot.Spec.Provider.Workers, valContext.shoot.Spec.Provider.Workers, workersPath); len(errList) > 0 {
		return errList.ToAggregate()
	}

	allErrs = append(allErrs, v.validateShoot(valContext)...)
	return allErrs.ToAggregate()
}

func (v *Shoot) validateShoot(context *validationContext) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, openstackvalidation.ValidateNetworking(context.shoot.Spec.Networking, nwPath)...)
	allErrs = append(allErrs, openstackvalidation.ValidateInfrastructureConfig(context.infraConfig, context.shoot.Spec.Networking.Nodes, infraConfigPath)...)
	allErrs = append(allErrs, openstackvalidation.ValidateControlPlaneConfig(context.cpConfig, cpConfigPath)...)
	allErrs = append(allErrs, openstackvalidation.ValidateWorkers(context.shoot.Spec.Provider.Workers, workersPath)...)
	return allErrs
}

func newValidationContext(ctx context.Context, decoder runtime.Decoder, c client.Client, shoot *core.Shoot) (*validationContext, error) {
	if shoot.Spec.Provider.InfrastructureConfig == nil {
		return nil, field.Required(infraConfigPath, "infrastructureConfig must be set for OpenStack shoots")
	}
	infraConfig, err := decodeInfrastructureConfig(decoder, shoot.Spec.Provider.InfrastructureConfig, infraConfigPath)
	if err != nil {
		return nil, fmt.Errorf("error decoding infrastructureConfig: %v", err)
	}

	if shoot.Spec.Provider.ControlPlaneConfig == nil {
		return nil, field.Required(cpConfigPath, "controlPlaneConfig must be set for OpenStack shoots")
	}
	cpConfig, err := decodeControlPlaneConfig(decoder, shoot.Spec.Provider.ControlPlaneConfig, cpConfigPath)
	if err != nil {
		return nil, fmt.Errorf("error decoding controlPlaneConfig: %v", err)
	}

	cloudProfile := &gardencorev1beta1.CloudProfile{}
	if err := c.Get(ctx, kutil.Key(shoot.Spec.CloudProfileName), cloudProfile); err != nil {
		return nil, err
	}

	if cloudProfile.Spec.ProviderConfig == nil {
		return nil, fmt.Errorf("providerConfig is not given for cloud profile %q", cloudProfile.Name)
	}
	cloudProfileConfig, err := decodeCloudProfileConfig(decoder, cloudProfile.Spec.ProviderConfig, providerConfigPath)
	if err != nil {
		return nil, fmt.Errorf("an error occurred while reading the cloud profile %q: %v", cloudProfile.Name, err)
	}

	return &validationContext{
		shoot:              shoot,
		infraConfig:        infraConfig,
		cpConfig:           cpConfig,
		cloudProfile:       cloudProfile,
		cloudProfileConfig: cloudProfileConfig,
	}, nil
}
