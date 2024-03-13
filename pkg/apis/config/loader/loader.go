// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package loader

import (
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/runtime/serializer/versioning"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/config"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/config/install"
)

var (
	codec  runtime.Codec
	scheme *runtime.Scheme
)

func init() {
	scheme = runtime.NewScheme()
	install.Install(scheme)
	yamlSerializer := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme, scheme)
	codec = versioning.NewDefaultingCodecForScheme(
		scheme,
		yamlSerializer,
		yamlSerializer,
		schema.GroupVersion{Version: "v1alpha1"},
		runtime.InternalGroupVersioner,
	)
}

// LoadFromFile takes a filename and de-serializes the contents into ControllerConfiguration object.
func LoadFromFile(filename string) (*config.ControllerConfiguration, error) {
	bytes, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	return Load(bytes)
}

// Load takes a byte slice and de-serializes the contents into ControllerConfiguration object.
// Encapsulates de-serialization without assuming the source is a file.
func Load(data []byte) (*config.ControllerConfiguration, error) {
	cfg := &config.ControllerConfiguration{}

	if len(data) == 0 {
		return cfg, nil
	}

	decoded, _, err := codec.Decode(data, &schema.GroupVersionKind{Version: "v1alpha1", Kind: "Config"}, cfg)
	if err != nil {
		return nil, err
	}

	return decoded.(*config.ControllerConfiguration), nil
}
