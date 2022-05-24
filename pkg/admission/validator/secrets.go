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

package validator

import (
	"context"
	"fmt"

	openstackvalidation "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/validation"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type secret struct{}

// NewSecretValidator returns a new instance of a secret validator.
func NewSecretValidator() extensionswebhook.Validator {
	return &secret{}
}

// Validate checks whether the given new secret contains valid OpenStack credentials.
func (s *secret) Validate(ctx context.Context, newObj, oldObj client.Object) error {
	secret, ok := newObj.(*corev1.Secret)
	if !ok {
		return fmt.Errorf("wrong object type %T", newObj)
	}

	if oldObj != nil {
		oldSecret, ok := oldObj.(*corev1.Secret)
		if !ok {
			return fmt.Errorf("wrong object type %T for old object", oldObj)
		}

		if equality.Semantic.DeepEqual(secret.Data, oldSecret.Data) {
			return nil
		}
	}

	return openstackvalidation.ValidateCloudProviderSecret(secret)
}
