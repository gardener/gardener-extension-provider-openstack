// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/install"
)

var (
	// Scheme is a scheme with the types relevant for OpenStack actuators.
	Scheme *runtime.Scheme

	decoder runtime.Decoder

	// lenientDecoder is a decoder that does not use strict mode.
	lenientDecoder runtime.Decoder
)

func init() {
	Scheme = runtime.NewScheme()
	utilruntime.Must(install.AddToScheme(Scheme))

	decoder = serializer.NewCodecFactory(Scheme, serializer.EnableStrict).UniversalDecoder()
	lenientDecoder = serializer.NewCodecFactory(Scheme).UniversalDecoder()
}

// InfrastructureConfigFromInfrastructure extracts the InfrastructureConfig from the
// ProviderConfig section of the given Infrastructure.
func InfrastructureConfigFromInfrastructure(infra *extensionsv1alpha1.Infrastructure) (*api.InfrastructureConfig, error) {
	return InfrastructureConfigFromRawExtension(infra.Spec.ProviderConfig)
}

// InfrastructureConfigFromRawExtension extracts the InfrastructureConfig from the ProviderConfig.
func InfrastructureConfigFromRawExtension(providerConfig *runtime.RawExtension) (*api.InfrastructureConfig, error) {
	config := &api.InfrastructureConfig{}
	if providerConfig != nil && providerConfig.Raw != nil {
		if _, _, err := decoder.Decode(providerConfig.Raw, nil, config); err != nil {
			return nil, err
		}
		return config, nil
	}
	return nil, fmt.Errorf("provider config is not set on the infrastructure resource")
}

// InfrastructureStatusFromRaw extracts the InfrastructureStatus from the
// ProviderStatus section of the given Infrastructure.
func InfrastructureStatusFromRaw(raw *runtime.RawExtension) (*api.InfrastructureStatus, error) {
	config := &api.InfrastructureStatus{}
	if raw != nil && raw.Raw != nil {
		if _, _, err := lenientDecoder.Decode(raw.Raw, nil, config); err != nil {
			return nil, err
		}
		return config, nil
	}
	return nil, fmt.Errorf("provider status is not set on the infrastructure resource")
}

// CloudProfileConfigFromCluster decodes the provider specific cloud profile configuration for a cluster
func CloudProfileConfigFromCluster(cluster *controller.Cluster) (*api.CloudProfileConfig, error) {
	var cloudProfileConfig *api.CloudProfileConfig
	if cluster != nil && cluster.CloudProfile != nil && cluster.CloudProfile.Spec.ProviderConfig != nil && cluster.CloudProfile.Spec.ProviderConfig.Raw != nil {
		cloudProfileSpecifier := fmt.Sprintf("cloudProfile '%q'", k8sclient.ObjectKeyFromObject(cluster.CloudProfile))
		if cluster.Shoot != nil && cluster.Shoot.Spec.CloudProfile != nil {
			cloudProfileSpecifier = fmt.Sprintf("%s '%s/%s'", cluster.Shoot.Spec.CloudProfile.Kind, cluster.Shoot.Namespace, cluster.Shoot.Spec.CloudProfile.Name)
		}
		cloudProfileConfig = &api.CloudProfileConfig{}
		if _, _, err := decoder.Decode(cluster.CloudProfile.Spec.ProviderConfig.Raw, nil, cloudProfileConfig); err != nil {
			return nil, fmt.Errorf("could not decode providerConfig of %s: %w", cloudProfileSpecifier, err)
		}
	}
	return cloudProfileConfig, nil
}

// WorkerConfigFromRawExtension extracts the provider specific configuration for a worker pool.
func WorkerConfigFromRawExtension(raw *runtime.RawExtension) (*api.WorkerConfig, error) {
	poolConfig := &api.WorkerConfig{}

	if raw != nil {
		marshalled, err := raw.MarshalJSON()
		if err != nil {
			return nil, err
		}

		if _, _, err := decoder.Decode(marshalled, nil, poolConfig); err != nil {
			return nil, err
		}
	}

	return poolConfig, nil
}
