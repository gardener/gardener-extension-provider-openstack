// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package components

import (
	"github.com/Masterminds/semver"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/imagevector"
)

// Component is an interface which can be implemented by operating system config components.
type Component interface {
	// Name returns the name of the component.
	Name() string
	// Config takes a configuration context and returns units and files.
	Config(Context) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File, error)
}

// Context contains configuration for the components.
type Context struct {
	CABundle                *string
	ClusterDNSAddress       string
	ClusterDomain           string
	CRIName                 extensionsv1alpha1.CRIName
	Images                  map[string]*imagevector.Image
	NodeLabels              map[string]string
	KubeletCABundle         []byte
	KubeletCLIFlags         ConfigurableKubeletCLIFlags
	KubeletConfigParameters ConfigurableKubeletConfigParameters
	KubeletDataVolumeName   *string
	KubernetesVersion       *semver.Version
	SSHPublicKeys           []string
	SSHAccessEnabled        bool
	LokiIngress             string
	PromtailEnabled         bool
	APIServerURL            string
	Sysctls                 map[string]string
}
