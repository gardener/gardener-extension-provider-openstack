// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package healthcheck

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/util"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Actuator contains all the health checks and the means to execute them
type Actuator struct {
	logger logr.Logger

	restConfig *rest.Config
	seedClient client.Client
	scheme     *runtime.Scheme
	decoder    runtime.Decoder

	provider            string
	extensionKind       string
	getExtensionObjFunc GetExtensionObjectFunc
	healthChecks        []ConditionTypeToHealthCheck
}

// NewActuator creates a new Actuator.
func NewActuator(provider, extensionKind string, getExtensionObjFunc GetExtensionObjectFunc, healthChecks []ConditionTypeToHealthCheck) HealthCheckActuator {
	return &Actuator{
		healthChecks:        healthChecks,
		getExtensionObjFunc: getExtensionObjFunc,
		provider:            provider,
		extensionKind:       extensionKind,
		logger:              log.Log.WithName(fmt.Sprintf("%s-%s-healthcheck-actuator", provider, extensionKind)),
	}
}

// InjectScheme injects the given runtime.Scheme into this Actuator.
func (a *Actuator) InjectScheme(scheme *runtime.Scheme) error {
	a.scheme = scheme
	a.decoder = serializer.NewCodecFactory(a.scheme).UniversalDecoder()
	return nil
}

// InjectClient injects the given client.Client into this Actuator.
func (a *Actuator) InjectClient(client client.Client) error {
	a.seedClient = client
	return nil
}

// InjectConfig injects the given rest.Config into this Actuator.
func (a *Actuator) InjectConfig(config *rest.Config) error {
	a.restConfig = config
	return nil
}

type healthCheckUnsuccessful struct {
	detail string
}

type healthCheckProgressing struct {
	detail    string
	threshold *time.Duration
}

type channelResult struct {
	healthConditionType string
	healthCheckResult   *SingleCheckResult
	error               error
}

type checkResultForConditionType struct {
	failedChecks       []error
	unsuccessfulChecks []healthCheckUnsuccessful
	progressingChecks  []healthCheckProgressing
	successfulChecks   int
	codes              []gardencorev1beta1.ErrorCode
}

// ExecuteHealthCheckFunctions executes all the health check functions, injects clients and logger & aggregates the results.
// returns an Result for each HealthConditionType (e.g  ControlPlaneHealthy)
func (a *Actuator) ExecuteHealthCheckFunctions(ctx context.Context, request types.NamespacedName) (*[]Result, error) {
	var (
		shootClient client.Client
		channel     = make(chan channelResult, len(a.healthChecks))
		wg          sync.WaitGroup
	)

	for _, hc := range a.healthChecks {
		// clone to avoid problems during parallel execution
		check := hc.HealthCheck.DeepCopy()
		SeedClientInto(a.seedClient, check)
		if _, ok := check.(ShootClient); ok {
			if shootClient == nil {
				var err error
				_, shootClient, err = util.NewClientForShoot(ctx, a.seedClient, request.Namespace, client.Options{})
				if err != nil {
					// don't return here, as we might have started some goroutines already to prevent leakage
					channel <- channelResult{
						healthCheckResult: &SingleCheckResult{
							Status: gardencorev1beta1.ConditionFalse,
							Detail: fmt.Sprintf("failed to create shoot client: %v", err),
						},
						error:               err,
						healthConditionType: hc.ConditionType,
					}
					continue
				}
			}
			ShootClientInto(shootClient, check)
		}

		check.SetLoggerSuffix(a.provider, a.extensionKind)

		wg.Add(1)
		go func(ctx context.Context, request types.NamespacedName, check HealthCheck, preCheckFunc PreCheckFunc, healthConditionType string) {
			defer wg.Done()

			if preCheckFunc != nil {
				obj := a.getExtensionObjFunc()
				if err := a.seedClient.Get(ctx, client.ObjectKey{Namespace: request.Namespace, Name: request.Name}, obj); err != nil {
					channel <- channelResult{
						healthCheckResult: &SingleCheckResult{
							Status: gardencorev1beta1.ConditionFalse,
							Detail: fmt.Sprintf("failed to read the extension resource: %v", err),
						},
						error:               err,
						healthConditionType: healthConditionType,
					}
					return
				}

				cluster, err := extensionscontroller.GetCluster(ctx, a.seedClient, request.Namespace)
				if err != nil {
					channel <- channelResult{
						healthCheckResult: &SingleCheckResult{
							Status: gardencorev1beta1.ConditionFalse,
							Detail: fmt.Sprintf("failed to read the cluster resource: %v", err),
						},
						error:               err,
						healthConditionType: healthConditionType,
					}
					return
				}

				if !preCheckFunc(ctx, a.seedClient, obj, cluster) {
					a.logger.V(6).Info("Skipping health check as pre check function returned false", "conditionType", healthConditionType)
					channel <- channelResult{
						healthCheckResult: &SingleCheckResult{
							Status: gardencorev1beta1.ConditionTrue,
						},
						error:               nil,
						healthConditionType: healthConditionType,
					}
					return
				}
			}

			healthCheckResult, err := check.Check(ctx, request)
			channel <- channelResult{
				healthCheckResult:   healthCheckResult,
				error:               err,
				healthConditionType: healthConditionType,
			}
		}(ctx, request, check, hc.PreCheckFunc, hc.ConditionType)
	}

	// close channel when wait group has 0 counter
	go func() {
		wg.Wait()
		close(channel)
	}()

	groupedHealthCheckResults := make(map[string]*checkResultForConditionType)
	// loop runs until channel is closed
	for channelResult := range channel {
		if groupedHealthCheckResults[channelResult.healthConditionType] == nil {
			groupedHealthCheckResults[channelResult.healthConditionType] = &checkResultForConditionType{}
		}
		if channelResult.error != nil {
			groupedHealthCheckResults[channelResult.healthConditionType].failedChecks = append(groupedHealthCheckResults[channelResult.healthConditionType].failedChecks, channelResult.error)
			continue
		}
		if channelResult.healthCheckResult.Status == gardencorev1beta1.ConditionFalse {
			groupedHealthCheckResults[channelResult.healthConditionType].unsuccessfulChecks = append(groupedHealthCheckResults[channelResult.healthConditionType].unsuccessfulChecks, healthCheckUnsuccessful{detail: channelResult.healthCheckResult.Detail})
			groupedHealthCheckResults[channelResult.healthConditionType].codes = append(groupedHealthCheckResults[channelResult.healthConditionType].codes, channelResult.healthCheckResult.Codes...)
			continue
		}
		if channelResult.healthCheckResult.Status == gardencorev1beta1.ConditionProgressing {
			groupedHealthCheckResults[channelResult.healthConditionType].progressingChecks = append(groupedHealthCheckResults[channelResult.healthConditionType].progressingChecks, healthCheckProgressing{detail: channelResult.healthCheckResult.Detail, threshold: channelResult.healthCheckResult.ProgressingThreshold})
			groupedHealthCheckResults[channelResult.healthConditionType].codes = append(groupedHealthCheckResults[channelResult.healthConditionType].codes, channelResult.healthCheckResult.Codes...)
			continue
		}
		groupedHealthCheckResults[channelResult.healthConditionType].successfulChecks++
	}

	var checkResults []Result
	for conditionType, result := range groupedHealthCheckResults {
		if len(result.unsuccessfulChecks) > 0 || len(result.failedChecks) > 0 {
			var details strings.Builder
			result.appendFailedChecksDetails(&details)
			result.appendUnsuccessfulChecksDetails(&details)
			result.appendProgressingChecksDetails(&details)

			checkResults = append(checkResults, Result{
				HealthConditionType: conditionType,
				Status:              gardencorev1beta1.ConditionFalse,
				Detail:              pointer.String(trimTrailingWhitespace(details.String())),
				SuccessfulChecks:    result.successfulChecks,
				UnsuccessfulChecks:  len(result.unsuccessfulChecks),
				FailedChecks:        len(result.failedChecks),
				Codes:               result.codes,
			})
			continue
		}

		if len(result.progressingChecks) > 0 {
			var (
				details   strings.Builder
				threshold *time.Duration
			)

			for index, check := range result.progressingChecks {
				if len(result.progressingChecks) == 1 {
					details.WriteString(ensureTrailingDot(check.detail))
				} else {
					details.WriteString(fmt.Sprintf("%d) %s ", index+1, ensureTrailingDot(check.detail)))
				}

				if check.threshold != nil && (threshold == nil || *threshold > *check.threshold) {
					threshold = check.threshold
				}
			}

			checkResults = append(checkResults, Result{
				HealthConditionType:  conditionType,
				Status:               gardencorev1beta1.ConditionProgressing,
				ProgressingThreshold: threshold,
				Detail:               pointer.String(trimTrailingWhitespace(details.String())),
				SuccessfulChecks:     result.successfulChecks,
				ProgressingChecks:    len(result.progressingChecks),
				Codes:                result.codes,
			})
			continue
		}

		checkResults = append(checkResults, Result{
			HealthConditionType: conditionType,
			Status:              gardencorev1beta1.ConditionTrue,
			SuccessfulChecks:    result.successfulChecks,
		})
	}

	return &checkResults, nil
}
