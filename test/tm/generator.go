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

// package contains the generators for provider specific shoot configuration
package main

import (
	"flag"
	"os"
	"reflect"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"

	"github.com/gardener/gardener/extensions/test/tm/generator"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	log "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	infrastructureProviderConfigPath = flag.String("infrastructure-provider-config-filepath", "", "filepath to the provider specific infrastructure config")
	controlplaneProviderConfigPath   = flag.String("controlplane-provider-config-filepath", "", "filepath to the provider specific controlplane config")

	floatingPoolName  = flag.String("floating-pool-name", "", "set the name of the floating pool")
	networkWorkerCidr = flag.String("network-worker-cidr", "10.250.0.0/19", "worker network cidr")

	loadBalancerProvider = flag.String("loadbalancer-provider", "", "loadbalancer provider for the shoot's loadbalancers")
)

func main() {
	log.SetLogger(zap.Logger(false))
	logger := log.Log.WithName("openstack-generator")
	flag.Parse()
	if err := validate(); err != nil {
		logger.Error(err, "error validating input flags")
		os.Exit(1)
	}

	infra := v1alpha1.InfrastructureConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
			Kind:       reflect.TypeOf(v1alpha1.InfrastructureConfig{}).Name(),
		},
		FloatingPoolName: *floatingPoolName,
		Networks: v1alpha1.Networks{
			Workers: *networkWorkerCidr,
		},
	}

	cp := v1alpha1.ControlPlaneConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
			Kind:       reflect.TypeOf(v1alpha1.ControlPlaneConfig{}).Name(),
		},
		LoadBalancerProvider: *loadBalancerProvider,
	}

	if err := generator.MarshalAndWriteConfig(*infrastructureProviderConfigPath, infra); err != nil {
		logger.Error(err, "unable to write infrastructure config")
		os.Exit(1)
	}
	if err := generator.MarshalAndWriteConfig(*controlplaneProviderConfigPath, cp); err != nil {
		logger.Error(err, "unable to write infrastructure config")
		os.Exit(1)
	}
	logger.Info("successfully written openstack provider configuration", "infra", *infrastructureProviderConfigPath, "controlplane", *controlplaneProviderConfigPath)
}

func validate() error {
	if err := generator.ValidateString(infrastructureProviderConfigPath); err != nil {
		return errors.Wrap(err, "error validating infrastructure provider config path")
	}
	if err := generator.ValidateString(controlplaneProviderConfigPath); err != nil {
		return errors.Wrap(err, "error validating controlplane provider config path")
	}
	if err := generator.ValidateString(networkWorkerCidr); err != nil {
		return errors.Wrap(err, "error validating worker CIDR")
	}
	if err := generator.ValidateString(floatingPoolName); err != nil {
		return errors.Wrap(err, "error floating pool name")
	}
	if err := generator.ValidateString(loadBalancerProvider); err != nil {
		return errors.Wrap(err, "error loadbalancer provider")
	}
	return nil
}
