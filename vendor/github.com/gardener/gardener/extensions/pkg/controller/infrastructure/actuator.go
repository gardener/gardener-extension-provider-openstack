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

package infrastructure

import (
	"context"

	"github.com/go-logr/logr"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// Actuator acts upon Infrastructure resources.
type Actuator interface {
	// Reconcile the Infrastructure config.
	Reconcile(context.Context, logr.Logger, *extensionsv1alpha1.Infrastructure, *extensionscontroller.Cluster) error
	// Delete the Infrastructure config.
	Delete(context.Context, logr.Logger, *extensionsv1alpha1.Infrastructure, *extensionscontroller.Cluster) error
	// Restore takes the state of the Infrastrucure resource and applies it to the terraform pod's output state
	Restore(context.Context, logr.Logger, *extensionsv1alpha1.Infrastructure, *extensionscontroller.Cluster) error
	// Migrate deletes the terraform k8s resources without deleting the corresponding resources in the IaaS provider
	Migrate(context.Context, logr.Logger, *extensionsv1alpha1.Infrastructure, *extensionscontroller.Cluster) error
}
