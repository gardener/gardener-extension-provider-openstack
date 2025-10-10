//  SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
//  SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"github.com/gardener/gardener/extensions/pkg/controller/infrastructure"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type actuator struct {
	client                     client.Client
	restConfig                 *rest.Config
	disableProjectedTokenMount bool
}

// NewActuator creates a new Actuator that updates the status of the handled Infrastructure resources.
func NewActuator(mgr manager.Manager, disableProjectedTokenMount bool) infrastructure.Actuator {
	return &actuator{
		disableProjectedTokenMount: disableProjectedTokenMount,
		client:                     mgr.GetClient(),
		restConfig:                 mgr.GetConfig(),
	}
}
