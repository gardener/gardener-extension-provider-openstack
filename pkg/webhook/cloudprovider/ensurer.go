/*
 * Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cloudprovider

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/webhook/cloudprovider"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/genericmutator"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
)

const(
	authUrlKey = "auth_url"
)

func NewEnsurer(logger logr.Logger) cloudprovider.Ensurer{
	return &ensurer{
		logger: logger,
	}
}

type ensurer struct{
	logger logr.Logger
	decoder runtime.Decoder
}

func (e *ensurer) InjectScheme(scheme *runtime.Scheme) error {
	e.decoder = serializer.NewCodecFactory(scheme).UniversalDecoder()
	return nil
}

func (e *ensurer) EnsureCloudProviderSecret(
	ctx context.Context,
	ectx genericmutator.EnsurerContext,
	new, _ *corev1.Secret,
) error {
	cluster, err := ectx.GetCluster(ctx)
	if err != nil {
		return fmt.Errorf("could not get cluster object from ensurer context: %v", err)
	}

	// do not do anything if the providerConfig is missing from cluster
	if cluster.CloudProfile.Spec.ProviderConfig == nil {
		return nil
	}

	config := &openstack.CloudProfileConfig{}
	raw, err := cluster.CloudProfile.Spec.ProviderConfig.MarshalJSON()
	if err != nil {
		return fmt.Errorf("could not decode cluster object's providerConfig: %v", err)
	}
	if _ , _, err := e.decoder.Decode(raw, nil, config); err != nil {
		return fmt.Errorf("could not decode cluster object's providerConfig: %v", err)
	}

	keyStoneURL, err := helper.FindKeyStoneURL(config.KeyStoneURLs, config.KeyStoneURL, cluster.Shoot.Spec.Region)
	if err != nil {
		return fmt.Errorf("could not find KeyStoneUrl: %v", err)
	}

	if new.Data == nil {
		new.Data = make(map[string][]byte)
	}
	new.Data[authUrlKey] = []byte(keyStoneURL)
	return nil
}



