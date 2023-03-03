// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/client"

	openstackvalidation "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/validation"
)

type secretBinding struct {
	apiReader client.Reader
}

// InjectAPIReader injects the given apiReader into the validator.
func (sb *secretBinding) InjectAPIReader(apiReader client.Reader) error {
	sb.apiReader = apiReader
	return nil
}

// NewSecretBindingValidator returns a new instance of a secret binding validator.
func NewSecretBindingValidator() extensionswebhook.Validator {
	return &secretBinding{}
}

// Validate checks whether the given SecretBinding refers to a Secret with valid OpenStack credentials.
func (sb *secretBinding) Validate(ctx context.Context, newObj, oldObj client.Object) error {
	secretBinding, ok := newObj.(*core.SecretBinding)
	if !ok {
		return fmt.Errorf("wrong object type %T", newObj)
	}

	if oldObj != nil {
		oldSecretBinding, ok := oldObj.(*core.SecretBinding)
		if !ok {
			return fmt.Errorf("wrong object type %T for old object", oldObj)
		}

		// If the provider type did not change, we exit early.
		if oldSecretBinding.Provider != nil && equality.Semantic.DeepEqual(secretBinding.Provider.Type, oldSecretBinding.Provider.Type) {
			return nil
		}
	}

	var (
		secret    = &corev1.Secret{}
		secretKey = kutil.Key(secretBinding.SecretRef.Namespace, secretBinding.SecretRef.Name)
	)
	// Explicitly use the client.Reader to prevent controller-runtime to start Informer for Secrets
	// under the hood. The latter increases the memory usage of the component.
	if err := sb.apiReader.Get(ctx, secretKey, secret); err != nil {
		return err
	}

	return openstackvalidation.ValidateCloudProviderSecret(secret)
}
