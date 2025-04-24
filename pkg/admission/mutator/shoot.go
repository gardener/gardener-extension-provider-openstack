// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package mutator

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// NewShootMutator returns a new instance of a shoot mutator.
func NewShootMutator(mgr manager.Manager) extensionswebhook.Mutator {
	return &shoot{
		decoder: serializer.NewCodecFactory(mgr.GetScheme(), serializer.EnableStrict).UniversalDecoder(),
	}
}

type shoot struct {
	decoder runtime.Decoder
}

const (
	overlayKey         = "overlay"
	enabledKey         = "enabled"
	createPodRoutesKey = "createPodRoutes"
	calicoReleaseName  = "calico"
	ciliumReleaseName  = "cilium"
)

var (
	// EnableOverlayAsDefaultForCalico enables the overlay network for all new calico shoot clusters on openstack
	EnableOverlayAsDefaultForCalico bool
	// EnableOverlayAsDefaultForCilium enables the overlay network for all new cilium shoot clusters on openstack
	EnableOverlayAsDefaultForCilium bool
)

// Mutate mutates the given shoot object.
func (s *shoot) Mutate(_ context.Context, newObj, oldObj client.Object) error {
	shoot, ok := newObj.(*gardencorev1beta1.Shoot)
	if !ok {
		return fmt.Errorf("wrong object type %T", newObj)
	}

	// Skip if shoot is in restore or migration phase
	if wasShootRescheduledToNewSeed(shoot) {
		return nil
	}

	var oldShoot *gardencorev1beta1.Shoot
	if oldObj != nil {
		oldShoot, ok = oldObj.(*gardencorev1beta1.Shoot)
		if !ok {
			return fmt.Errorf("wrong object type %T", oldObj)
		}
	}

	if oldShoot != nil && isShootInMigrationOrRestorePhase(shoot) {
		return nil
	}

	// Skip if specs are matching
	if oldShoot != nil && reflect.DeepEqual(shoot.Spec, oldShoot.Spec) {
		return nil
	}

	// Skip if it's a workerless Shoot
	if gardencorev1beta1helper.IsWorkerless(shoot) {
		return nil
	}

	// Skip if shoot is in deletion phase
	if shoot.DeletionTimestamp != nil || oldShoot != nil && oldShoot.DeletionTimestamp != nil {
		return nil
	}
	if shoot.Spec.Networking != nil && shoot.Spec.Networking.Type != nil {
		overlayConfig := map[string]interface{}{enabledKey: false}
		if (*shoot.Spec.Networking.Type == calicoReleaseName && EnableOverlayAsDefaultForCalico) || (*shoot.Spec.Networking.Type == ciliumReleaseName && EnableOverlayAsDefaultForCilium) {
			overlayConfig = map[string]interface{}{enabledKey: true}
		}

		networkConfig, err := s.decodeNetworkConfig(shoot.Spec.Networking.ProviderConfig)
		if err != nil {
			return err
		}

		if oldShoot == nil && networkConfig[overlayKey] == nil {
			networkConfig[overlayKey] = overlayConfig
		}

		if oldShoot != nil && networkConfig[overlayKey] == nil {
			oldNetworkConfig, err := s.decodeNetworkConfig(oldShoot.Spec.Networking.ProviderConfig)
			if err != nil {
				return err
			}

			if oldNetworkConfig[overlayKey] != nil {
				networkConfig[overlayKey] = oldNetworkConfig[overlayKey]
			}
		}

		if currentOverlayConfig, ok := networkConfig["overlay"].(map[string]interface{}); ok {
			overlayConfig = currentOverlayConfig
		}

		if !overlayConfig["enabled"].(bool) && overlayConfig[createPodRoutesKey] == nil {
			overlayConfig[createPodRoutesKey] = true
			networkConfig[overlayKey] = overlayConfig
		}

		modifiedJSON, err := json.Marshal(networkConfig)
		if err != nil {
			return err
		}
		shoot.Spec.Networking.ProviderConfig = &runtime.RawExtension{
			Raw: modifiedJSON,
		}
	}

	// Disable TCP to upstream DNS queries by default on OpenStack. DNS over TCP may cause performance issues on larger clusters.
	if shoot.Spec.SystemComponents != nil {
		if shoot.Spec.SystemComponents.NodeLocalDNS != nil {
			if shoot.Spec.SystemComponents.NodeLocalDNS.Enabled {
				if shoot.Spec.SystemComponents.NodeLocalDNS.ForceTCPToUpstreamDNS == nil {
					shoot.Spec.SystemComponents.NodeLocalDNS.ForceTCPToUpstreamDNS = ptr.To(false)
				}
			}
		}
	}

	return nil
}

func (s *shoot) decodeNetworkConfig(network *runtime.RawExtension) (map[string]interface{}, error) {
	var networkConfig map[string]interface{}
	if network == nil || network.Raw == nil {
		return map[string]interface{}{}, nil
	}
	if err := json.Unmarshal(network.Raw, &networkConfig); err != nil {
		return nil, err
	}
	return networkConfig, nil
}

// wasShootRescheduledToNewSeed returns true if the shoot.Spec.SeedName has been changed, but the migration operation has not started yet.
func wasShootRescheduledToNewSeed(shoot *gardencorev1beta1.Shoot) bool {
	return shoot.Status.LastOperation != nil &&
		shoot.Status.LastOperation.Type != gardencorev1beta1.LastOperationTypeMigrate &&
		shoot.Spec.SeedName != nil &&
		shoot.Status.SeedName != nil &&
		*shoot.Spec.SeedName != *shoot.Status.SeedName
}

// isShootInMigrationOrRestorePhase returns true if the shoot is currently being migrated or restored.
func isShootInMigrationOrRestorePhase(shoot *gardencorev1beta1.Shoot) bool {
	return shoot.Status.LastOperation != nil &&
		(shoot.Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeRestore &&
			shoot.Status.LastOperation.State != gardencorev1beta1.LastOperationStateSucceeded ||
			shoot.Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeMigrate)
}
