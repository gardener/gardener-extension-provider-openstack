// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"time"

	"github.com/gardener/gardener/extensions/pkg/terraformer"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	"github.com/gardener/gardener-extension-provider-openstack/imagevector"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
)

const (
	// TerraformVarNameDomainName maps to terraform internal var representation.
	TerraformVarNameDomainName = "TF_VAR_DOMAIN_NAME"
	// TerraformVarNameProjectName maps to terraform internal var representation.
	TerraformVarNameProjectName = "TF_VAR_TENANT_NAME"
	// TerraformVarNameUserName maps to terraform internal var representation.
	TerraformVarNameUserName = "TF_VAR_USER_NAME"
	// TerraformVarNamePassword maps to terraform internal var representation.
	TerraformVarNamePassword = "TF_VAR_PASSWORD"
	// TerraformVarNameApplicationCredentialId maps to terraform internal var representation.
	TerraformVarNameApplicationCredentialId = "TF_VAR_APPLICATION_CREDENTIAL_ID" // #nosec G101 -- No credential.
	// TerraformVarNameApplicationCredentialName maps to terraform internal var representation.
	TerraformVarNameApplicationCredentialName = "TF_VAR_APPLICATION_CREDENTIAL_NAME" // #nosec G101 -- No credential.
	// TerraformVarNameApplicationCredentialSecret maps to terraform internal var representation.
	TerraformVarNameApplicationCredentialSecret = "TF_VAR_APPLICATION_CREDENTIAL_SECRET" // #nosec G101 -- No credential.
	// TerraformVarCACert  maps to terraform internal var representation.
	TerraformVarCACert = "TF_VAR_CA_CERT"
)

// TerraformerEnvVars computes the Terraformer environment variables from the given secret reference.
func TerraformerEnvVars(secretRef corev1.SecretReference, credentials *openstack.Credentials) []corev1.EnvVar {
	var envVars []corev1.EnvVar
	if credentials.CACert != "" {
		envVars = append(envVars, createEnvVar(secretRef, TerraformVarCACert, openstack.CACert))
	}

	if credentials.ApplicationCredentialSecret != "" {
		envVars = append(envVars,
			createEnvVar(secretRef, TerraformVarNameDomainName, openstack.DomainName),
			createEnvVar(secretRef, TerraformVarNameProjectName, openstack.TenantName),
			createEnvVar(secretRef, TerraformVarNameApplicationCredentialSecret, openstack.ApplicationCredentialSecret))
		if credentials.ApplicationCredentialID != "" {
			envVars = append(envVars, createEnvVar(secretRef, TerraformVarNameApplicationCredentialId, openstack.ApplicationCredentialID))
		}
		if credentials.ApplicationCredentialName != "" {
			envVars = append(envVars, createEnvVar(secretRef, TerraformVarNameApplicationCredentialName, openstack.ApplicationCredentialName))
		}
		if credentials.Username != "" {
			envVars = append(envVars, createEnvVar(secretRef, TerraformVarNameUserName, openstack.UserName))
		}
		return envVars
	}

	return append(envVars,
		createEnvVar(secretRef, TerraformVarNameDomainName, openstack.DomainName),
		createEnvVar(secretRef, TerraformVarNameProjectName, openstack.TenantName),
		createEnvVar(secretRef, TerraformVarNameUserName, openstack.UserName),
		createEnvVar(secretRef, TerraformVarNamePassword, openstack.Password),
	)
}

// NewTerraformer initializes a new Terraformer.
func NewTerraformer(
	logger logr.Logger,
	restConfig *rest.Config,
	purpose string,
	infra *extensionsv1alpha1.Infrastructure,
	disableProjectedTokenMount bool,
) (
	terraformer.Terraformer,
	error,
) {
	tf, err := terraformer.NewForConfig(logger, restConfig, purpose, infra.Namespace, infra.Name, imagevector.TerraformerImage())
	if err != nil {
		return nil, err
	}

	owner := metav1.NewControllerRef(infra, extensionsv1alpha1.SchemeGroupVersion.WithKind(extensionsv1alpha1.InfrastructureResource))
	return tf.
		UseProjectedTokenMount(!disableProjectedTokenMount).
		SetTerminationGracePeriodSeconds(630).
		SetDeadlineCleaning(5 * time.Minute).
		SetDeadlinePod(15 * time.Minute).
		SetOwnerRef(owner), nil
}

// NewTerraformerWithAuth initializes a new Terraformer that has the credentials.
func NewTerraformerWithAuth(
	logger logr.Logger,
	restConfig *rest.Config,
	purpose string,
	infra *extensionsv1alpha1.Infrastructure,
	credentials *openstack.Credentials,
	disableProjectedTokenMount bool,
) (
	terraformer.Terraformer,
	error,
) {
	tf, err := NewTerraformer(logger, restConfig, purpose, infra, disableProjectedTokenMount)
	if err != nil {
		return nil, err
	}

	return tf.SetEnvVars(TerraformerEnvVars(infra.Spec.SecretRef, credentials)...), nil
}

func createEnvVar(secretRef corev1.SecretReference, name, key string) corev1.EnvVar {
	return corev1.EnvVar{
		Name: name,
		ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: secretRef.Name,
			},
			Key: key,
		}},
	}
}
