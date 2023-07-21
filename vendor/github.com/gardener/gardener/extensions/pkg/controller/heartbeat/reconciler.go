// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package heartbeat

import (
	"context"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/extensions"
)

type reconciler struct {
	client               client.Client
	extensionName        string
	renewIntervalSeconds int32
	namespace            string
	clock                clock.Clock
}

// NewReconciler creates a new reconciler that will renew the heartbeat lease resource.
func NewReconciler(mgr manager.Manager, extensionName string, namespace string, renewIntervalSeconds int32, clock clock.Clock) reconcile.Reconciler {
	return &reconciler{
		client:               mgr.GetClient(),
		extensionName:        extensionName,
		renewIntervalSeconds: renewIntervalSeconds,
		namespace:            namespace,
		clock:                clock,
	}
}

// Reconcile renews the heartbeat lease resource.
func (r *reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      extensions.HeartBeatResourceName,
			Namespace: r.namespace,
		},
	}

	if err := r.client.Get(ctx, client.ObjectKeyFromObject(lease), lease); err != nil {
		if apierrors.IsNotFound(err) {
			lease.Spec = coordinationv1.LeaseSpec{
				HolderIdentity:       &r.extensionName,
				LeaseDurationSeconds: &r.renewIntervalSeconds,
				RenewTime:            &metav1.MicroTime{Time: r.clock.Now().UTC()},
			}
			log.V(1).Info("Creating heartbeat Lease", "lease", client.ObjectKeyFromObject(lease))
			return reconcile.Result{RequeueAfter: time.Duration(r.renewIntervalSeconds) * time.Second}, r.client.Create(ctx, lease)
		}
		return reconcile.Result{}, err
	}

	lease.Spec = coordinationv1.LeaseSpec{
		HolderIdentity:       &r.extensionName,
		LeaseDurationSeconds: &r.renewIntervalSeconds,
		RenewTime:            &metav1.MicroTime{Time: r.clock.Now().UTC()},
	}

	log.V(1).Info("Renewing heartbeat Lease", "lease", client.ObjectKeyFromObject(lease))
	return reconcile.Result{RequeueAfter: time.Duration(r.renewIntervalSeconds) * time.Second}, r.client.Update(ctx, lease)
}
