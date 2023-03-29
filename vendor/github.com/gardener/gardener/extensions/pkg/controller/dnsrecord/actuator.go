// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package dnsrecord

import (
	"context"

	"github.com/go-logr/logr"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// Actuator acts upon DNSRecord resources.
type Actuator interface {
	// Reconcile reconciles the DNSRecord.
	Reconcile(context.Context, logr.Logger, *extensionsv1alpha1.DNSRecord, *extensionscontroller.Cluster) error
	// Delete deletes the DNSRecord.
	Delete(context.Context, logr.Logger, *extensionsv1alpha1.DNSRecord, *extensionscontroller.Cluster) error
	// Restore restores the DNSRecord.
	Restore(context.Context, logr.Logger, *extensionsv1alpha1.DNSRecord, *extensionscontroller.Cluster) error
	// Migrate migrates the DNSRecord.
	Migrate(context.Context, logr.Logger, *extensionsv1alpha1.DNSRecord, *extensionscontroller.Cluster) error
}
