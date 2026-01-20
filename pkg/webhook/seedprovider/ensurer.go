// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seedprovider

import (
	"context"

	druidcorev1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	gcontext "github.com/gardener/gardener/extensions/pkg/webhook/context"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/genericmutator"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/config"
)

// NewEnsurer creates a new seedprovider ensurer.
func NewEnsurer(mainStorage *config.ETCDStorage, eventsStorage *config.ETCDStorage, logger logr.Logger) genericmutator.Ensurer {
	return &ensurer{
		mainStorage:   mainStorage,
		eventsStorage: eventsStorage,
		logger:        logger.WithName("openstack-seedprovider-ensurer"),
	}
}

type ensurer struct {
	genericmutator.NoopEnsurer
	mainStorage   *config.ETCDStorage
	eventsStorage *config.ETCDStorage
	logger        logr.Logger
}

// EnsureETCD ensures that the etcd conform to the provider requirements.
func (e *ensurer) EnsureETCD(_ context.Context, _ gcontext.GardenContext, newObj, _ *druidcorev1alpha1.Etcd) error {
	capacity := resource.MustParse("10Gi")
	class := ""

	var cfg *config.ETCDStorage
	switch newObj.Name {
	case v1beta1constants.ETCDMain:
		cfg = e.mainStorage
	case v1beta1constants.ETCDEvents:
		cfg = e.eventsStorage
	default:
		e.logger.Info("Unknown ETCD name, skipping storage configuration", "name", newObj.Name)
		return nil
	}

	if cfg != nil {
		if cfg.Capacity != nil {
			capacity = *cfg.Capacity
		}
		if cfg.ClassName != nil {
			class = *cfg.ClassName
		}
	}

	if newObj.Spec.StorageCapacity != nil {
		newObj.Spec.StorageCapacity = &capacity
		if class != "" {
			newObj.Spec.StorageClass = &class
		}
	}

	return nil
}
