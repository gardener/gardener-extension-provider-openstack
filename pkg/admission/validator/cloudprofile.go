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

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	openstackvalidation "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/validation"
)

// NewCloudProfileValidator returns a new instance of a cloud profile validator.
func NewCloudProfileValidator(mgr manager.Manager) extensionswebhook.Validator {
	return &cloudProfile{
		decoder: serializer.NewCodecFactory(mgr.GetScheme(), serializer.EnableStrict).UniversalDecoder(),
	}
}

type cloudProfile struct {
	decoder runtime.Decoder
}

// Validate validates the given cloud profile objects.
func (cp *cloudProfile) Validate(_ context.Context, new, _ client.Object) error {
	cloudProfile, ok := new.(*core.CloudProfile)
	if !ok {
		return fmt.Errorf("wrong object type %T", new)
	}

	providerConfigPath := field.NewPath("spec").Child("providerConfig")
	if cloudProfile.Spec.ProviderConfig == nil {
		return field.Required(providerConfigPath, "providerConfig must be set for OpenStack cloud profiles")
	}

	cpConfig, err := decodeCloudProfileConfig(cp.decoder, cloudProfile.Spec.ProviderConfig)
	if err != nil {
		return err
	}

	return openstackvalidation.ValidateCloudProfileConfig(cpConfig, providerConfigPath).ToAggregate()
}
