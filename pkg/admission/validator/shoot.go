// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
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
	cloudProfileSpec   *gardencorev1beta1.CloudProfileSpec
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
func (s *shoot) Validate(ctx context.Context, newObj, oldObj client.Object) error {
	shoot, ok := newObj.(*core.Shoot)
	if !ok {
		return fmt.Errorf("wrong object type %T", newObj)
	}

	// Skip if it's a workerless Shoot
	if gardencorehelper.IsWorkerless(shoot) {
		return nil
	}

	var credentials *openstack.Credentials
	if shoot.Spec.SecretBindingName != nil || shoot.Spec.CredentialsBindingName != nil {
		secret, err := s.getCloudProviderSecretForShoot(ctx, shoot)
		if err != nil {
			return fmt.Errorf("failed to get cloud provider credentials: %v", err)
		}

		credentials, err = openstack.ExtractCredentials(secret, false)
		if err != nil {
			return fmt.Errorf("invalid cloud credentials: %v", err)
		}
	}

	shootV1Beta1 := &gardencorev1beta1.Shoot{}
	err := gardencorev1beta1.Convert_core_Shoot_To_v1beta1_Shoot(shoot, shootV1Beta1, nil)
	if err != nil {
		return err
	}
	cloudProfile, err := gardener.GetCloudProfile(ctx, s.client, shootV1Beta1)
	if err != nil {
		return err
	}
	if cloudProfile == nil {
		return fmt.Errorf("cloudprofile could not be found")
	}

	if oldObj != nil {
		oldShoot, ok := oldObj.(*core.Shoot)
		if !ok {
			return fmt.Errorf("wrong object type %T for old object", oldObj)
		}
		return s.validateShootUpdate(ctx, oldShoot, shoot, credentials, &cloudProfile.Spec)
	}

	return s.validateShootCreation(ctx, shoot, credentials, &cloudProfile.Spec)
}

func (s *shoot) validateShootCreation(ctx context.Context, shoot *core.Shoot, credentials *openstack.Credentials, cloudProfileSpec *gardencorev1beta1.CloudProfileSpec) error {
	valContext, err := newValidationContext(s.decoder, shoot, cloudProfileSpec)
	if err != nil {
		return err
	}

	allErrs := field.ErrorList{}

	if credentials != nil {
		allErrs = append(allErrs, openstackvalidation.ValidateInfrastructureConfigAgainstCloudProfile(nil, valContext.infraConfig, credentials.DomainName, valContext.shoot.Spec.Region, valContext.cloudProfileConfig, infraConfigPath)...)
		allErrs = append(allErrs, openstackvalidation.ValidateControlPlaneConfigAgainstCloudProfile(nil, valContext.cpConfig, credentials.DomainName, valContext.shoot.Spec.Region, valContext.infraConfig.FloatingPoolName, valContext.cloudProfileConfig, cpConfigPath)...)
	}
	allErrs = append(allErrs, s.validateShoot(ctx, valContext)...)
	return allErrs.ToAggregate()
}

func (s *shoot) validateShootUpdate(ctx context.Context, oldShoot, shoot *core.Shoot, credentials *openstack.Credentials, cloudProfileSpec *gardencorev1beta1.CloudProfileSpec) error {
	oldValContext, err := newValidationContext(s.lenientDecoder, oldShoot, cloudProfileSpec)
	if err != nil {
		return err
	}

	valContext, err := newValidationContext(s.decoder, shoot, cloudProfileSpec)
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

	allErrs = append(allErrs, s.validateShoot(ctx, valContext)...)
	return allErrs.ToAggregate()
}

func (s *shoot) validateShoot(ctx context.Context, context *validationContext) field.ErrorList {
	allErrs := field.ErrorList{}
	if context.shoot.Spec.Networking != nil {
		allErrs = append(allErrs, openstackvalidation.ValidateNetworking(context.shoot.Spec.Networking, nwPath)...)
		allErrs = append(allErrs, openstackvalidation.ValidateInfrastructureConfig(context.infraConfig, context.shoot.Spec.Networking.Nodes, infraConfigPath)...)

		// Validate that SubnetPoolID is set for dual-stack shoots
		if !core.IsIPv4SingleStack(context.shoot.Spec.Networking.IPFamilies) && context.infraConfig.SubnetPoolID == nil {
			allErrs = append(allErrs, field.Required(infraConfigPath.Child("subnetPoolID"), "subnetPoolID must be set for dual-stack shoots"))
		}
	}
	allErrs = append(allErrs, openstackvalidation.ValidateControlPlaneConfig(context.cpConfig, context.infraConfig, context.shoot.Spec.Kubernetes.Version, cpConfigPath)...)
	allErrs = append(allErrs, openstackvalidation.ValidateWorkers(context.shoot.Spec.Provider.Workers, context.cloudProfileConfig, workersPath)...)
	allErrs = append(allErrs, s.validateDNS(ctx, context.shoot)...)
	return allErrs
}

func newValidationContext(decoder runtime.Decoder, shoot *core.Shoot, cloudProfileSpec *gardencorev1beta1.CloudProfileSpec) (*validationContext, error) {
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

	if cloudProfileSpec.ProviderConfig == nil {
		return nil, fmt.Errorf("providerConfig is not given for cloud profile %q", shoot.Spec.CloudProfile)
	}
	cloudProfileConfig, err := decodeCloudProfileConfig(decoder, cloudProfileSpec.ProviderConfig)
	if err != nil {
		return nil, fmt.Errorf("an error occurred while reading the cloud profile %q: %v", shoot.Spec.CloudProfile, err)
	}

	return &validationContext{
		shoot:              shoot,
		infraConfig:        infraConfig,
		cpConfig:           cpConfig,
		cloudProfileSpec:   cloudProfileSpec,
		cloudProfileConfig: cloudProfileConfig,
	}, nil
}

func (s *shoot) getCloudProviderSecretForShoot(ctx context.Context, shoot *core.Shoot) (*corev1.Secret, error) {
	var secretKey client.ObjectKey
	if shoot.Spec.SecretBindingName != nil {
		var (
			bindingKey    = client.ObjectKey{Namespace: shoot.Namespace, Name: *shoot.Spec.SecretBindingName}
			secretBinding = &gardencorev1beta1.SecretBinding{}
		)
		if err := kutil.LookupObject(ctx, s.client, s.apiReader, bindingKey, secretBinding); err != nil {
			return nil, err
		}
		secretKey = client.ObjectKey{Namespace: secretBinding.SecretRef.Namespace, Name: secretBinding.SecretRef.Name}
	} else {
		var (
			bindingKey         = client.ObjectKey{Namespace: shoot.Namespace, Name: *shoot.Spec.CredentialsBindingName}
			credentialsBinding = &securityv1alpha1.CredentialsBinding{}
		)
		if err := kutil.LookupObject(ctx, s.client, s.apiReader, bindingKey, credentialsBinding); err != nil {
			return nil, err
		}
		secretKey = client.ObjectKey{Namespace: credentialsBinding.CredentialsRef.Namespace, Name: credentialsBinding.CredentialsRef.Name}
	}

	// Explicitly use the client.Reader to prevent controller-runtime to start Informer for Secrets
	// under the hood. The latter increases the memory usage of the component.
	secret := &corev1.Secret{}
	if err := s.apiReader.Get(ctx, secretKey, secret); err != nil {
		return nil, err
	}

	return secret, nil
}

// validateDNS validates all openstack provider entries in the Shoot spec.
func (s *shoot) validateDNS(ctx context.Context, shoot *core.Shoot) field.ErrorList {
	allErrs := field.ErrorList{}

	if shoot.Spec.DNS == nil {
		return allErrs
	}

	providersPath := specPath.Child("dns").Child("providers")

	for i, p := range shoot.Spec.DNS.Providers {
		if p.Type == nil || *p.Type != openstack.DNSType {
			continue
		}

		// skip non-primary providers
		if p.Primary == nil || !*p.Primary {
			continue
		}

		providerFldPath := providersPath.Index(i)

		if ptr.Deref(p.SecretName, "") == "" {
			allErrs = append(allErrs, field.Required(providerFldPath.Child("secretName"),
				fmt.Sprintf("secretName must be specified for %v provider", openstack.DNSType)))
			continue
		}

		secret := &corev1.Secret{}
		key := client.ObjectKey{Namespace: shoot.Namespace, Name: *p.SecretName}
		if err := s.apiReader.Get(ctx, key, secret); err != nil {
			if apierrors.IsNotFound(err) {
				allErrs = append(allErrs, field.Invalid(providerFldPath.Child("secretName"),
					*p.SecretName, "referenced secret not found"))
			} else {
				allErrs = append(allErrs, field.InternalError(providerFldPath.Child("secretName"), err))
			}
			continue
		}

		allErrs = append(allErrs, openstackvalidation.ValidateCloudProviderSecret(secret, providerFldPath, true)...)
	}

	return allErrs
}
