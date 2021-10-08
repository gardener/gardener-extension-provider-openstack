// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/gardener/gardener/charts"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation/botanist/component/logging"
	"github.com/gardener/gardener/pkg/operation/common"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// DeploySeedLogging will install the Helm release "seed-bootstrap/charts/loki" in the Seed clusters.
func (b *Botanist) DeploySeedLogging(ctx context.Context) error {
	if !b.Shoot.IsLoggingEnabled() {
		return common.DeleteShootLoggingStack(ctx, b.K8sSeedClient.Client(), b.Shoot.SeedNamespace)
	}

	images, err := b.InjectSeedSeedImages(map[string]interface{}{},
		charts.ImageNameLoki,
		charts.ImageNameLokiCurator,
		charts.ImageNameKubeRbacProxy,
		charts.ImageNameTelegraf,
	)
	if err != nil {
		return err
	}

	lokiValues := map[string]interface{}{
		"global":   images,
		"replicas": b.Shoot.GetReplicas(1),
	}

	hvpaValues := make(map[string]interface{})
	hvpaEnabled := gardenletfeatures.FeatureGate.Enabled(features.HVPA)
	if b.ManagedSeed != nil {
		hvpaEnabled = gardenletfeatures.FeatureGate.Enabled(features.HVPAForShootedSeed)
	}

	ingressClass, err := getIngressClass(b.Seed.GetInfo())
	if err != nil {
		return err
	}

	if b.isShootNodeLoggingEnabled() {
		lokiValues["rbacSidecarEnabled"] = true
		lokiValues["kubeRBACProxyKubeconfigCheckSum"] = b.LoadCheckSum(logging.SecretNameLokiKubeRBACProxyKubeconfig)
		lokiValues["ingress"] = map[string]interface{}{
			"class": ingressClass,
			"hosts": []map[string]interface{}{
				{
					"hostName":    b.ComputeLokiHost(),
					"secretName":  common.LokiTLS,
					"serviceName": "loki",
					"servicePort": 8080,
					"backendPath": "/loki/api/v1/push",
				},
			},
		}
	} else {
		err := common.DeleteShootNodeLoggingStack(ctx, b.K8sSeedClient.Client(), b.Shoot.SeedNamespace)
		if err != nil {
			return err
		}
	}

	hvpaValues["enabled"] = hvpaEnabled
	lokiValues["hvpa"] = hvpaValues

	if hvpaEnabled {
		currentResources, err := kutil.GetContainerResourcesInStatefulSet(ctx, b.K8sSeedClient.Client(), kutil.Key(b.Shoot.SeedNamespace, "loki"))
		if err != nil {
			return err
		}
		if len(currentResources) != 0 && currentResources["loki"] != nil {
			lokiValues["resources"] = map[string]interface{}{
				"loki": currentResources["loki"],
			}
		}
	}

	return b.K8sSeedClient.ChartApplier().Apply(ctx, filepath.Join(charts.Path, "seed-bootstrap", "charts", "loki"), b.Shoot.SeedNamespace, fmt.Sprintf("%s-logging", b.Shoot.SeedNamespace), kubernetes.Values(lokiValues))
}

func (b *Botanist) isShootNodeLoggingEnabled() bool {
	if b.Shoot != nil && b.Shoot.IsLoggingEnabled() && b.Config != nil &&
		b.Config.Logging != nil && b.Config.Logging.ShootNodeLogging != nil {

		for _, purpose := range b.Config.Logging.ShootNodeLogging.ShootPurposes {
			if gardencore.ShootPurpose(b.Shoot.Purpose) == purpose {
				return true
			}
		}
	}
	return false
}
