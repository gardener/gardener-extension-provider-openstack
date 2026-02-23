// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package networking

import (
	"encoding/json"
	"fmt"

	v1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// IsOverlayEnabled inspects a Shoot Networking providerConfig and returns whether overlay is enabled
func IsOverlayEnabled(network *v1beta1.Networking) (bool, error) {
	if network == nil || network.ProviderConfig == nil || len(network.ProviderConfig.Raw) == 0 {
		return true, nil
	}

	var networkConfig map[string]interface{}
	if err := json.Unmarshal(network.ProviderConfig.Raw, &networkConfig); err != nil {
		return false, err
	}

	if overlay, ok := networkConfig["overlay"].(map[string]interface{}); ok {
		if enabled, ok2 := overlay["enabled"].(bool); ok2 {
			return enabled, nil
		}
		return false, fmt.Errorf("overlay.enabled is not a boolean")
	}

	return true, nil
}
