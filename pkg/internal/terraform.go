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

	"github.com/gardener/gardener-extension-provider-openstack/pkg/imagevector"
	"github.com/gardener/gardener/extensions/pkg/terraformer"

	"github.com/gardener/gardener/pkg/logger"
	"k8s.io/client-go/rest"
)

const (
	// TerraformVarNameUserName maps to terraform internal var representation.
	TerraformVarNameUserName = "TF_VAR_USER_NAME"
	// TerraformVarNamePassword maps to terraform internal var representation.
	TerraformVarNamePassword = "TF_VAR_PASSWORD"
)

// TerraformerVariablesEnvironmentFromCredentials computes the Terraformer variables environment from the
// given Credentials.
func TerraformerVariablesEnvironmentFromCredentials(creds *Credentials) map[string]string {
	return map[string]string{
		TerraformVarNameUserName: creds.Username,
		TerraformVarNamePassword: creds.Password,
	}
}

// NewTerraformer initializes a new Terraformer.
func NewTerraformer(
	restConfig *rest.Config,
	purpose,
	namespace,
	name string,
) (terraformer.Terraformer, error) {
	tf, err := terraformer.NewForConfig(logger.NewLogger("info"), restConfig, purpose, namespace, name, imagevector.TerraformerImage())
	if err != nil {
		return nil, err
	}

	return tf.
		SetTerminationGracePeriodSeconds(630).
		SetDeadlineCleaning(5 * time.Minute).
		SetDeadlinePod(15 * time.Minute), nil
}

// NewTerraformerWithAuth initializes a new Terraformer that has the credentials.
func NewTerraformerWithAuth(
	restConfig *rest.Config,
	purpose,
	namespace,
	name string,
	creds *Credentials,
) (terraformer.Terraformer, error) {
	tf, err := NewTerraformer(restConfig, purpose, namespace, name)
	if err != nil {
		return nil, err
	}

	return tf.SetVariablesEnvironment(TerraformerVariablesEnvironmentFromCredentials(creds)), nil
}
