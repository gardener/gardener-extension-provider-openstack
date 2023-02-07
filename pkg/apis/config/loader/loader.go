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
