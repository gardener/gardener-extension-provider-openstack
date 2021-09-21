// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package operation

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/api/extensions"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils/retry"
	"github.com/gardener/gardener/test/framework"
)

// WaitForExtensionCondition waits for the extension to contain the condition type, status and reason
func WaitForExtensionCondition(ctx context.Context, logger *logrus.Logger, seedClient client.Client, groupVersionKind schema.GroupVersionKind, namespacedName types.NamespacedName, conditionType gardencorev1beta1.ConditionType, conditionStatus gardencorev1beta1.ConditionStatus, conditionReason string) error {
	return retry.Until(ctx, 2*time.Second, func(ctx context.Context) (done bool, err error) {
		rawExtension := unstructured.Unstructured{}
		rawExtension.SetGroupVersionKind(groupVersionKind)

		if err := seedClient.Get(ctx, namespacedName, &rawExtension); err != nil {
			logger.Infof("unable to retrieve extension from seed (ns: %s, name: %s, kind %s): %v", namespacedName.Namespace, namespacedName.Name, groupVersionKind.Kind, err)
			return retry.MinorError(fmt.Errorf("unable to retrieve extension from seed (ns: %s, name: %s, kind %s)", namespacedName.Namespace, namespacedName.Name, groupVersionKind.Kind))
		}

		acc, err := extensions.Accessor(rawExtension.DeepCopyObject())
		if err != nil {
			return retry.MinorError(err)
		}

		for _, condition := range acc.GetExtensionStatus().GetConditions() {
			logger.Infof("extension (ns: %s, name: %s, kind %s) has condition: ConditionType: %s, ConditionStatus: %s, ConditionReason: %s))", namespacedName.Namespace, namespacedName.Name, groupVersionKind.Kind, condition.Type, condition.Status, condition.Reason)
			if condition.Type == conditionType && condition.Status == conditionStatus && condition.Reason == conditionReason {
				logger.Infof("found expected condition.")
				return retry.Ok()
			}
		}
		logger.Infof("extension (ns: %s, name: %s, kind %s) does not yet contain expected condition. EXPECTED: (conditionType: %s, conditionStatus: %s, conditionReason: %s))", namespacedName.Namespace, namespacedName.Name, groupVersionKind.Kind, conditionType, conditionStatus, conditionReason)
		return retry.MinorError(fmt.Errorf("extension (ns: %s, name: %s, kind %s) does not yet contain expected condition. EXPECTED: (conditionType: %s, conditionStatus: %s, conditionReason: %s))", namespacedName.Namespace, namespacedName.Name, groupVersionKind.Kind, conditionType, conditionStatus, conditionReason))
	})
}

// ScaleGardenerResourceManager scales the gardener-resource-manager to the desired replicas
func ScaleGardenerResourceManager(ctx context.Context, namespace string, client client.Client, desiredReplicas *int32) (*int32, error) {
	return framework.ScaleDeployment(ctx, client, desiredReplicas, v1beta1constants.DeploymentNameGardenerResourceManager, namespace)
}
