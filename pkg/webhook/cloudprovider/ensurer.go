// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cloudprovider

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/webhook/cloudprovider"
	gcontext "github.com/gardener/gardener/extensions/pkg/webhook/context"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	types "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
)

// NewEnsurer creates cloudprovider ensurer.
func NewEnsurer(mgr manager.Manager) cloudprovider.Ensurer {
	return &ensurer{
		decoder: serializer.NewCodecFactory(mgr.GetScheme(), serializer.EnableStrict).UniversalDecoder(),
	}
}

type ensurer struct {
	decoder runtime.Decoder
}

func (e *ensurer) EnsureCloudProviderSecret(
	ctx context.Context,
	ectx gcontext.GardenContext,
	newSecret, _ *corev1.Secret,
) error {
	cluster, err := ectx.GetCluster(ctx)
	if err != nil {
		return fmt.Errorf("could not get cluster object from ensurer context: %v", err)
	}

	// do not do anything if the providerConfig is missing from cluster
	if cluster.CloudProfile.Spec.ProviderConfig == nil {
		return nil
	}

	config := &openstack.CloudProfileConfig{}
	raw, err := cluster.CloudProfile.Spec.ProviderConfig.MarshalJSON()
	if err != nil {
		return fmt.Errorf("could not decode cluster object's providerConfig: %v", err)
	}
	if _, _, err := e.decoder.Decode(raw, nil, config); err != nil {
		return fmt.Errorf("could not decode cluster object's providerConfig: %v", err)
	}

	keyStoneURL, err := helper.FindKeyStoneURL(config.KeyStoneURLs, config.KeyStoneURL, cluster.Shoot.Spec.Region)
	if err != nil {
		return fmt.Errorf("could not find KeyStoneUrl: %v", err)
	}
	keyStoneCABundle := helper.FindKeyStoneCACert(config.KeyStoneURLs, config.KeyStoneCACert, cluster.Shoot.Spec.Region)

	if newSecret.Data == nil {
		newSecret.Data = make(map[string][]byte)
	}
	newSecret.Data[types.AuthURL] = []byte(keyStoneURL)
	if keyStoneCABundle != nil {
		newSecret.Data[types.CACert] = []byte(*keyStoneCABundle)
	}

	// remove key from user
	delete(newSecret.Data, types.Insecure)
	if config.KeyStoneForceInsecure {
		newSecret.Data[types.Insecure] = []byte("true")
	}
	return nil
}
