// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"context"
	"fmt"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorehelper "github.com/gardener/gardener/pkg/apis/core/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	openstackvalidation "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/validation"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
)

// NewShootValidator returns a new instance of a shoot validator.
func NewShootValidator(mgr manager.Manager) extensionswebhook.Validator {
	return &shoot{
		client:         mgr.GetClient(),
		apiReader:      mgr.GetAPIReader(),
		decoder:        serializer.NewCodecFactory(mgr.GetScheme(), serializer.EnableStrict).UniversalDecoder(),
		lenientDecoder: serializer.NewCodecFactory(mgr.GetScheme()).UniversalDecoder(),
	}
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

// Validate validates the given shoot object.
func (s *shoot) Validate(ctx context.Context, new, old client.Object) error {
	shoot, ok := new.(*core.Shoot)
	if !ok {
		return fmt.Errorf("wrong object type %T", new)
	}

	// Skip if it's a workerless Shoot
	if gardencorehelper.IsWorkerless(shoot) {
		return nil
	}

	var credentials *openstack.Credentials
	if shoot.Spec.SecretBindingName != nil {
		secret, err := s.getCloudProviderSecretForShoot(ctx, shoot)
		if err != nil {
			return fmt.Errorf("failed to get cloud provider credentials: %v", err)
		}

		credentials, err = openstack.ExtractCredentials(secret, false)
		if err != nil {
			return fmt.Errorf("invalid cloud credentials: %v", err)
		}
	}

	if old != nil {
		oldShoot, ok := old.(*core.Shoot)
		if !ok {
			return fmt.Errorf("wrong object type %T for old object", old)
		}
		return s.validateShootUpdate(ctx, oldShoot, shoot, credentials)
	}

	return s.validateShootCreation(ctx, shoot, credentials)
}

func (s *shoot) validateShootCreation(ctx context.Context, shoot *core.Shoot, credentials *openstack.Credentials) error {
	valContext, err := newValidationContext(ctx, s.decoder, s.client, shoot)
	if err != nil {
		return err
	}

	allErrs := field.ErrorList{}

	if credentials != nil {
		allErrs = append(allErrs, openstackvalidation.ValidateInfrastructureConfigAgainstCloudProfile(nil, valContext.infraConfig, credentials.DomainName, valContext.shoot.Spec.Region, valContext.cloudProfileConfig, infraConfigPath)...)
		allErrs = append(allErrs, openstackvalidation.ValidateControlPlaneConfigAgainstCloudProfile(nil, valContext.cpConfig, credentials.DomainName, valContext.shoot.Spec.Region, valContext.infraConfig.FloatingPoolName, valContext.cloudProfileConfig, cpConfigPath)...)
	}
	allErrs = append(allErrs, s.validateShoot(valContext)...)
	return allErrs.ToAggregate()
}

func (s *shoot) validateShootUpdate(ctx context.Context, oldShoot, shoot *core.Shoot, credentials *openstack.Credentials) error {
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
	if credentials != nil {
		allErrs = append(allErrs, openstackvalidation.ValidateInfrastructureConfigAgainstCloudProfile(oldValContext.infraConfig, valContext.infraConfig, credentials.DomainName, valContext.shoot.Spec.Region, valContext.cloudProfileConfig, infraConfigPath)...)
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
	if credentials != nil &&
		oldCpConfig.LoadBalancerProvider != cpConfig.LoadBalancerProvider ||
		oldCpConfig.Zone != cpConfig.Zone ||
		!equality.Semantic.DeepEqual(oldCpConfig.LoadBalancerClasses, cpConfig.LoadBalancerClasses) ||
		oldValContext.infraConfig.FloatingPoolName != valContext.infraConfig.FloatingPoolName {
		allErrs = append(allErrs, openstackvalidation.ValidateControlPlaneConfigAgainstCloudProfile(oldCpConfig, cpConfig, credentials.DomainName, valContext.shoot.Spec.Region, valContext.infraConfig.FloatingPoolName, valContext.cloudProfileConfig, cpConfigPath)...)
	}

	if errList := openstackvalidation.ValidateWorkersUpdate(oldValContext.shoot.Spec.Provider.Workers, valContext.shoot.Spec.Provider.Workers, workersPath); len(errList) > 0 {
		return errList.ToAggregate()
	}

	allErrs = append(allErrs, s.validateShoot(valContext)...)
	return allErrs.ToAggregate()
}

func (s *shoot) validateShoot(context *validationContext) field.ErrorList {
	allErrs := field.ErrorList{}
	if context.shoot.Spec.Networking != nil {
		allErrs = append(allErrs, openstackvalidation.ValidateNetworking(context.shoot.Spec.Networking, nwPath)...)
		allErrs = append(allErrs, openstackvalidation.ValidateInfrastructureConfig(context.infraConfig, context.shoot.Spec.Networking.Nodes, infraConfigPath)...)
	}
	allErrs = append(allErrs, openstackvalidation.ValidateControlPlaneConfig(context.cpConfig, context.infraConfig, context.shoot.Spec.Kubernetes.Version, cpConfigPath)...)
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

	if shoot.Spec.CloudProfile == nil {
		return nil, fmt.Errorf("shoot.spec.cloudprofile must not be nil <nil>")
	}

	if err := c.Get(ctx, client.ObjectKey{Name: shoot.Spec.CloudProfile.Name}, cloudProfile); err != nil {
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
		secretBindingKey = client.ObjectKey{Namespace: shoot.Namespace, Name: *shoot.Spec.SecretBindingName}
	)
	if err := kutil.LookupObject(ctx, s.client, s.apiReader, secretBindingKey, secretBinding); err != nil {
		return nil, err
	}

	var (
		secret    = &corev1.Secret{}
		secretKey = client.ObjectKey{Namespace: secretBinding.SecretRef.Namespace, Name: secretBinding.SecretRef.Name}
	)
	// Explicitly use the client.Reader to prevent controller-runtime to start Informer for Secrets
	// under the hood. The latter increases the memory usage of the component.
	if err := s.apiReader.Get(ctx, secretKey, secret); err != nil {
		return nil, err
	}

	return secret, nil
}
