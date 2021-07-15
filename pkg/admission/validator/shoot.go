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

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	openstackvalidation "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/validation"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewShootValidator returns a new instance of a shoot validator.
func NewShootValidator() extensionswebhook.Validator {
	return &shoot{}
}

type shoot struct {
	client         client.Client
	apiReader      client.Reader
	decoder        runtime.Decoder
	lenientDecoder runtime.Decoder
}

type validationContext struct {
	shoot              *core.Shoot
	infraConfig        *api.InfrastructureConfig
	cpConfig           *api.ControlPlaneConfig
	cloudProfile       *gardencorev1beta1.CloudProfile
	cloudProfileConfig *api.CloudProfileConfig
}

var (
	specPath        = field.NewPath("spec")
	nwPath          = specPath.Child("networking")
	providerPath    = specPath.Child("provider")
	infraConfigPath = providerPath.Child("infrastructureConfig")
	cpConfigPath    = providerPath.Child("controlPlaneConfig")
	workersPath     = providerPath.Child("workers")
)

// InjectClient injects the given client into the validator.
func (s *shoot) InjectClient(client client.Client) error {
	s.client = client
	return nil
}

// InjectAPIReader injects the given apiReader into the validator.
func (s *shoot) InjectAPIReader(apiReader client.Reader) error {
	s.apiReader = apiReader
	return nil
}

// InjectScheme injects the given scheme into the validator.
func (s *shoot) InjectScheme(scheme *runtime.Scheme) error {
	s.decoder = serializer.NewCodecFactory(scheme, serializer.EnableStrict).UniversalDecoder()
	s.lenientDecoder = serializer.NewCodecFactory(scheme).UniversalDecoder()
	return nil
}

// Validate validates the given shoot object.
func (s *shoot) Validate(ctx context.Context, new, old client.Object) error {
	shoot, ok := new.(*core.Shoot)
	if !ok {
		return fmt.Errorf("wrong object type %T", new)
	}

	secret, err := s.getCloudProviderSecretForShoot(ctx, shoot)
	if err != nil {
		return fmt.Errorf("failed to get cloud provider credentials: %v", err)
	}

	credentials, err := openstack.ExtractCredentials(secret, false)
	if err != nil {
		return fmt.Errorf("invalid cloud credentials: %v", err)
	}

	if old != nil {
		oldShoot, ok := old.(*core.Shoot)
		if !ok {
			return fmt.Errorf("wrong object type %T for old object", old)
		}
		return s.validateShootUpdate(ctx, oldShoot, shoot, credentials.DomainName)
	}

	return s.validateShootCreation(ctx, shoot, credentials.DomainName)
}

func (s *shoot) validateShootCreation(ctx context.Context, shoot *core.Shoot, domain string) error {
	valContext, err := newValidationContext(ctx, s.decoder, s.client, shoot)
	if err != nil {
		return err
	}

	allErrs := field.ErrorList{}

	allErrs = append(allErrs, openstackvalidation.ValidateInfrastructureConfigAgainstCloudProfile(nil, valContext.infraConfig, domain, valContext.shoot.Spec.Region, valContext.cloudProfileConfig, infraConfigPath)...)
	allErrs = append(allErrs, openstackvalidation.ValidateControlPlaneConfigAgainstCloudProfile(nil, valContext.cpConfig, domain, valContext.shoot.Spec.Region, valContext.infraConfig.FloatingPoolName, valContext.cloudProfileConfig, cpConfigPath)...)
	allErrs = append(allErrs, s.validateShoot(valContext)...)
	return allErrs.ToAggregate()
}

func (s *shoot) validateShootUpdate(ctx context.Context, oldShoot, shoot *core.Shoot, domain string) error {
	oldValContext, err := newValidationContext(ctx, s.lenientDecoder, s.client, oldShoot)
	if err != nil {
		return err
	}

	valContext, err := newValidationContext(ctx, s.decoder, s.client, shoot)
	if err != nil {
		return err
	}

	allErrs := field.ErrorList{}

	allErrs = append(allErrs, openstackvalidation.ValidateInfrastructureConfigUpdate(oldValContext.infraConfig, valContext.infraConfig, infraConfigPath)...)
	allErrs = append(allErrs, openstackvalidation.ValidateInfrastructureConfigAgainstCloudProfile(oldValContext.infraConfig, valContext.infraConfig, domain, valContext.shoot.Spec.Region, valContext.cloudProfileConfig, infraConfigPath)...)

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
		!equality.Semantic.DeepEqual(oldCpConfig.LoadBalancerClasses, cpConfig.LoadBalancerClasses) ||
		oldValContext.infraConfig.FloatingPoolName != valContext.infraConfig.FloatingPoolName {
		allErrs = append(allErrs, openstackvalidation.ValidateControlPlaneConfigAgainstCloudProfile(oldCpConfig, cpConfig, domain, valContext.shoot.Spec.Region, valContext.infraConfig.FloatingPoolName, valContext.cloudProfileConfig, cpConfigPath)...)
	}

	if errList := openstackvalidation.ValidateWorkersUpdate(oldValContext.shoot.Spec.Provider.Workers, valContext.shoot.Spec.Provider.Workers, workersPath); len(errList) > 0 {
		return errList.ToAggregate()
	}

	allErrs = append(allErrs, s.validateShoot(valContext)...)
	return allErrs.ToAggregate()
}

func (s *shoot) validateShoot(context *validationContext) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, openstackvalidation.ValidateNetworking(context.shoot.Spec.Networking, nwPath)...)
	allErrs = append(allErrs, openstackvalidation.ValidateInfrastructureConfig(context.infraConfig, context.shoot.Spec.Networking.Nodes, infraConfigPath)...)
	allErrs = append(allErrs, openstackvalidation.ValidateControlPlaneConfig(context.cpConfig, context.shoot.Spec.Kubernetes.Version, cpConfigPath)...)
	allErrs = append(allErrs, openstackvalidation.ValidateWorkers(context.shoot.Spec.Provider.Workers, context.cloudProfileConfig, workersPath)...)
	return allErrs
}

func newValidationContext(ctx context.Context, decoder runtime.Decoder, c client.Client, shoot *core.Shoot) (*validationContext, error) {
	if shoot.Spec.Provider.InfrastructureConfig == nil {
		return nil, field.Required(infraConfigPath, "infrastructureConfig must be set for OpenStack shoots")
	}
	infraConfig, err := decodeInfrastructureConfig(decoder, shoot.Spec.Provider.InfrastructureConfig)
	if err != nil {
		return nil, fmt.Errorf("error decoding infrastructureConfig: %v", err)
	}

	if shoot.Spec.Provider.ControlPlaneConfig == nil {
		return nil, field.Required(cpConfigPath, "controlPlaneConfig must be set for OpenStack shoots")
	}
	cpConfig, err := decodeControlPlaneConfig(decoder, shoot.Spec.Provider.ControlPlaneConfig)
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
	cloudProfileConfig, err := decodeCloudProfileConfig(decoder, cloudProfile.Spec.ProviderConfig)
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

func (s *shoot) getCloudProviderSecretForShoot(ctx context.Context, shoot *core.Shoot) (*corev1.Secret, error) {
	var (
		secretBinding    = &gardencorev1beta1.SecretBinding{}
		secretBindingKey = kutil.Key(shoot.Namespace, shoot.Spec.SecretBindingName)
	)
	if err := kutil.LookupObject(ctx, s.client, s.apiReader, secretBindingKey, secretBinding); err != nil {
		return nil, err
	}

	var (
		secret    = &corev1.Secret{}
		secretKey = kutil.Key(secretBinding.SecretRef.Namespace, secretBinding.SecretRef.Name)
	)
	// Explicitly use the client.Reader to prevent controller-runtime to start Informer for Secrets
	// under the hood. The latter increases the memory usage of the component.
	if err := s.apiReader.Get(ctx, secretKey, secret); err != nil {
		return nil, err
	}

	return secret, nil
}
