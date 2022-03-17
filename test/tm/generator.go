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
	"fmt"
	"os"
	"reflect"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"

	"github.com/gardener/gardener/extensions/test/testmachinery/generator"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	log "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const (
	defaultNetworkWorkerCidr = "10.250.0.0/19"
)

var (
	cfg    *generatorConfig
	logger logr.Logger
)

type generatorConfig struct {
	floatingPoolName                 string
	loadBalancerProvider             string
	networkWorkerCidr                string
	infrastructureProviderConfigPath string
	controlplaneProviderConfigPath   string
}

func addFlags() {
	cfg = &generatorConfig{}
	flag.StringVar(&cfg.infrastructureProviderConfigPath, "infrastructure-provider-config-filepath", "", "filepath to the provider specific infrastructure config")
	flag.StringVar(&cfg.controlplaneProviderConfigPath, "controlplane-provider-config-filepath", "", "filepath to the provider specific controlplane config")

	flag.StringVar(&cfg.floatingPoolName, "floating-pool-name", "", "set the name of the floating pool")
	flag.StringVar(&cfg.networkWorkerCidr, "network-worker-cidr", "", "worker network cidr")

	flag.StringVar(&cfg.loadBalancerProvider, "loadbalancer-provider", "", "loadbalancer provider for the shoot's loadbalancers")
}

func main() {
	addFlags()
	flag.Parse()
	log.SetLogger(zap.New(zap.UseDevMode(false)))
	logger = log.Log.WithName("openstack-generator")
	if err := validate(); err != nil {
		logger.Error(err, "error validating input flags")
		os.Exit(1)
	}

	infra := v1alpha1.InfrastructureConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
			Kind:       reflect.TypeOf(v1alpha1.InfrastructureConfig{}).Name(),
		},
		FloatingPoolName: cfg.floatingPoolName,
		Networks: v1alpha1.Networks{
			Workers: cfg.networkWorkerCidr,
		},
	}

	cp := v1alpha1.ControlPlaneConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
			Kind:       reflect.TypeOf(v1alpha1.ControlPlaneConfig{}).Name(),
		},
		LoadBalancerProvider: cfg.loadBalancerProvider,
	}

	if err := generator.MarshalAndWriteConfig(cfg.infrastructureProviderConfigPath, infra); err != nil {
		logger.Error(err, "unable to write infrastructure config")
		os.Exit(1)
	}
	if err := generator.MarshalAndWriteConfig(cfg.controlplaneProviderConfigPath, cp); err != nil {
		logger.Error(err, "unable to write controlplane config")
		os.Exit(1)
	}
	logger.Info("successfully written openstack provider configuration", "infra", cfg.infrastructureProviderConfigPath, "controlplane", cfg.controlplaneProviderConfigPath)
}

func validate() error {
	if err := generator.ValidateString(&cfg.infrastructureProviderConfigPath); err != nil {
		return fmt.Errorf("error validating infrastructure provider config path: %w", err)
	}
	if err := generator.ValidateString(&cfg.controlplaneProviderConfigPath); err != nil {
		return fmt.Errorf("error validating controlplane provider config path: %w", err)
	}
	if err := generator.ValidateString(&cfg.floatingPoolName); err != nil {
		return fmt.Errorf("error floating pool name: %w", err)
	}
	if err := generator.ValidateString(&cfg.loadBalancerProvider); err != nil {
		return fmt.Errorf("error loadbalancer provider: %w", err)
	}
	// Optional Parameters
	if err := generator.ValidateString(&cfg.networkWorkerCidr); err != nil {
		logger.Info("Parameter network-worker-cidr is not set, using default.", "value", defaultNetworkWorkerCidr)
		cfg.networkWorkerCidr = defaultNetworkWorkerCidr
	}
	return nil
}
