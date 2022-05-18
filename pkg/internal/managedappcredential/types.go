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

package managedappcredential

import (
	"time"

	controllerconfig "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/config"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Manager is responsible to manage the lifecycle of the managed appplication
// credentials of an Openstack Shoot cluster.
type Manager struct {
	config        *controllerconfig.ApplicationCredentialConfig
	client        client.Client
	logger        logr.Logger
	identifier    string
	finalizerName string
	namespace     string

	openstackClientFactory openstackclient.FactoryFactory
}

type applicationCredential struct {
	id           string
	name         string
	password     string
	creationTime time.Time

	secret *corev1.Secret
}

type parent struct {
	id     string
	name   string
	secret string

	identityClient openstackclient.Identity
	credentials    *openstack.Credentials
}
