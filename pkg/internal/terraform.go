// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package internal

import (
	"time"

	"github.com/gardener/gardener/extensions/pkg/terraformer"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/imagevector"
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
	TerraformVarNameApplicationCredentialId = "TF_VAR_APPLICATION_CREDENTIAL_ID"
	// TerraformVarNameApplicationCredentialSecret maps to terraform internal var representation.
	TerraformVarNameApplicationCredentialSecret = "TF_VAR_APPLICATION_CREDENTIAL_SECRET"
)

// TerraformerEnvVars computes the Terraformer environment variables from the given secret reference.
func TerraformerEnvVars(secretRef corev1.SecretReference, useApplicationCredentials bool) []corev1.EnvVar {
	name1 := TerraformVarNameUserName
	key1 := openstack.UserName
	name2 := TerraformVarNamePassword
	key2 := openstack.Password

	if useApplicationCredentials {
		name1 = TerraformVarNameApplicationCredentialId
		key1 = openstack.ApplicationCredentialID
		name2 = TerraformVarNameApplicationCredentialSecret
		key2 = openstack.ApplicationCredentialSecret
	}

	return []corev1.EnvVar{
		createEnvVar(secretRef, TerraformVarNameDomainName, openstack.DomainName),
		createEnvVar(secretRef, TerraformVarNameProjectName, openstack.TenantName),
		createEnvVar(secretRef, name1, key1),
		createEnvVar(secretRef, name2, key2),
	}
}

// NewTerraformer initializes a new Terraformer.
func NewTerraformer(
	logger logr.Logger,
	restConfig *rest.Config,
	purpose string,
	infra *extensionsv1alpha1.Infrastructure,
) (terraformer.Terraformer, error) {
	tf, err := terraformer.NewForConfig(logger, restConfig, purpose, infra.Namespace, infra.Name, imagevector.TerraformerImage())
	if err != nil {
		return nil, err
	}

	owner := metav1.NewControllerRef(infra, extensionsv1alpha1.SchemeGroupVersion.WithKind(extensionsv1alpha1.InfrastructureResource))
	return tf.
		UseV2(true).
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
	useApplicationCredentials bool,
) (terraformer.Terraformer, error) {
	tf, err := NewTerraformer(logger, restConfig, purpose, infra)
	if err != nil {
		return nil, err
	}

	return tf.SetEnvVars(TerraformerEnvVars(infra.Spec.SecretRef, useApplicationCredentials)...), nil
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
