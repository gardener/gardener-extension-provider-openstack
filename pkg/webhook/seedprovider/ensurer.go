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
	var (
		mainStorageConfig   *config.ETCDStorage
		eventsStorageConfig *config.ETCDStorage
		capacity            = resource.MustParse("1Gi")
		class               string
	)

	switch newObj.Name {
	case v1beta1constants.ETCDMain:
		mainStorageConfig = e.mainStorage
	case v1beta1constants.ETCDEvents:
		eventsStorageConfig = e.eventsStorage
	default:
		e.logger.Info("Unknown ETCD name, skipping storage configuration", "name", newObj.Name)
		return nil
	}

	if mainStorageConfig != nil {
		if mainStorageConfig.Capacity != nil {
			capacity = *mainStorageConfig.Capacity
		}
		if mainStorageConfig.ClassName != nil {
			class = *mainStorageConfig.ClassName
		}
	}

	if eventsStorageConfig != nil {
		if eventsStorageConfig.Capacity != nil {
			capacity = *eventsStorageConfig.Capacity
		}
		if eventsStorageConfig.ClassName != nil {
			class = *eventsStorageConfig.ClassName
		}
	}

	newObj.Spec.StorageClass = &class
	newObj.Spec.StorageCapacity = &capacity

	return nil
}
