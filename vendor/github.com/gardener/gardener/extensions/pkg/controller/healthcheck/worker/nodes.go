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

package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/gardener/gardener/extensions/pkg/controller/healthcheck"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// DefaultHealthChecker all the information for the Worker HealthCheck.
// This check assumes that the MachineControllerManager (https://github.com/gardener/machine-controller-manager) has been
// deployed by the Worker extension controller.
type DefaultHealthChecker struct {
	logger logr.Logger
	// Needs to be set by actuator before calling the Check function
	seedClient client.Client
	// make sure shoot client is instantiated
	shootClient client.Client
	// scaleUpProgressingThreshold is the progressing threshold when the health check detects a scale-up situation.
	scaleUpProgressingThreshold *time.Duration
	// scaleDownProgressingThreshold is the progressing threshold when the health check detects a scale-down situation.
	scaleDownProgressingThreshold *time.Duration
}

// NewNodesChecker is a health check function which performs certain checks about the nodes registered in the cluster.
// It implements the healthcheck.HealthCheck interface.
func NewNodesChecker() *DefaultHealthChecker {
	scaleUpProgressingThreshold := 5 * time.Minute
	scaleDownProgressingThreshold := 15 * time.Minute

	return &DefaultHealthChecker{
		scaleUpProgressingThreshold:   &scaleUpProgressingThreshold,
		scaleDownProgressingThreshold: &scaleDownProgressingThreshold,
	}
}

// WithScaleUpProgressingThreshold sets the scaleUpProgressingThreshold property.
func (h *DefaultHealthChecker) WithScaleUpProgressingThreshold(d time.Duration) *DefaultHealthChecker {
	h.scaleUpProgressingThreshold = &d
	return h
}

// WithScaleDownProgressingThreshold sets the scaleDownProgressingThreshold property.
func (h *DefaultHealthChecker) WithScaleDownProgressingThreshold(d time.Duration) *DefaultHealthChecker {
	h.scaleDownProgressingThreshold = &d
	return h
}

// InjectSeedClient injects the seed client.
func (h *DefaultHealthChecker) InjectSeedClient(seedClient client.Client) {
	h.seedClient = seedClient
}

// InjectShootClient injects the shoot client.
func (h *DefaultHealthChecker) InjectShootClient(shootClient client.Client) {
	h.shootClient = shootClient
}

// SetLoggerSuffix injects the logger.
func (h *DefaultHealthChecker) SetLoggerSuffix(provider, extension string) {
	h.logger = log.Log.WithName(fmt.Sprintf("%s-%s-healthcheck-nodes", provider, extension))
}

// DeepCopy clones the healthCheck struct by making a copy and returning the pointer to that new copy.
func (h *DefaultHealthChecker) DeepCopy() healthcheck.HealthCheck {
	copy := *h
	return &copy
}

// Check executes the health check.
func (h *DefaultHealthChecker) Check(ctx context.Context, request types.NamespacedName) (*healthcheck.SingleCheckResult, error) {
	machineDeploymentList := &machinev1alpha1.MachineDeploymentList{}
	if err := h.seedClient.List(ctx, machineDeploymentList, client.InNamespace(request.Namespace)); err != nil {
		err := fmt.Errorf("unable to check nodes. Failed to list machine deployments in namespace %q: %w", request.Namespace, err)
		h.logger.Error(err, "Health check failed")
		return nil, err
	}

	nodeList := &corev1.NodeList{}
	if err := h.shootClient.List(ctx, nodeList); err != nil {
		err := fmt.Errorf("unable to check nodes. Failed to list shoot nodes: %w", err)
		h.logger.Error(err, "Health check failed")
		return nil, err
	}

	var (
		readyNodes      int
		registeredNodes = len(nodeList.Items)
		desiredMachines = getDesiredMachineCount(machineDeploymentList.Items)
	)

	for _, node := range nodeList.Items {
		if node.Spec.Unschedulable {
			continue
		}
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
				readyNodes++
			}
		}
	}

	machineList := &machinev1alpha1.MachineList{}
	if registeredNodes != desiredMachines || readyNodes != desiredMachines {
		if err := h.seedClient.List(ctx, machineList, client.InNamespace(request.Namespace)); err != nil {
			err := fmt.Errorf("unable to check nodes. Failed to list machines in namespace %q: %w", request.Namespace, err)
			h.logger.Error(err, "Health check failed")
			return nil, err
		}
	}

	// First check if the MachineDeployments report failed machines. If false then check if the MachineDeployments are
	// "available". If false then check if there is a regular scale-up happening or if there are machines with an erroneous
	// phase. Only then check the other MachineDeployment conditions. As last check, check if there is a scale-down happening
	// (e.g., in case of an rolling-update).

	checkScaleUp := false

	for _, deployment := range machineDeploymentList.Items {
		for _, failedMachine := range deployment.Status.FailedMachines {
			err := fmt.Errorf("machine %q failed: %s", failedMachine.Name, failedMachine.LastOperation.Description)
			h.logger.Error(err, "Health check failed")
			return &healthcheck.SingleCheckResult{
				Status: gardencorev1beta1.ConditionFalse,
				Detail: err.Error(),
				Codes:  gardencorev1beta1helper.DetermineErrorCodes(err),
			}, nil
		}

		for _, condition := range deployment.Status.Conditions {
			if condition.Type == machinev1alpha1.MachineDeploymentAvailable && condition.Status != machinev1alpha1.ConditionTrue {
				checkScaleUp = true
				break
			}
		}
	}

	if checkScaleUp {
		if status, err := checkNodesScalingUp(machineList, readyNodes, desiredMachines); status != gardencorev1beta1.ConditionTrue {
			h.logger.Error(err, "Health check failed")
			return &healthcheck.SingleCheckResult{
				Status:               status,
				Detail:               err.Error(),
				Codes:                gardencorev1beta1helper.DetermineErrorCodes(err),
				ProgressingThreshold: h.scaleUpProgressingThreshold,
			}, nil
		}
	}

	if isHealthy, err := checkMachineDeploymentsHealthy(machineDeploymentList.Items); !isHealthy {
		h.logger.Error(err, "Health check failed")
		return &healthcheck.SingleCheckResult{
			Status: gardencorev1beta1.ConditionFalse,
			Detail: err.Error(),
			Codes:  gardencorev1beta1helper.DetermineErrorCodes(err),
		}, nil
	}

	if status, err := checkNodesScalingDown(machineList, nodeList, registeredNodes, desiredMachines); status != gardencorev1beta1.ConditionTrue {
		h.logger.Error(err, "Health check failed")
		return &healthcheck.SingleCheckResult{
			Status:               status,
			Detail:               err.Error(),
			Codes:                gardencorev1beta1helper.DetermineErrorCodes(err),
			ProgressingThreshold: h.scaleDownProgressingThreshold,
		}, nil
	}

	return &healthcheck.SingleCheckResult{Status: gardencorev1beta1.ConditionTrue}, nil
}
