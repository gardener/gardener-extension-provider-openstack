#!/bin/bash
#
# Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

CODE_GEN_DIR=$(go list -m -f '{{.Dir}}' k8s.io/code-generator)

# setup virtual GOPATH
source "$GARDENER_HACK_DIR"/vgopath-setup.sh

# We need to explicitly pass GO111MODULE=off to k8s.io/code-generator as it is significantly slower otherwise,
# see https://github.com/kubernetes/code-generator/issues/100.
export GO111MODULE=off

rm -f $GOPATH/bin/*-gen

bash "${CODE_GEN_DIR}"/generate-internal-groups.sh \
  deepcopy,defaulter \
  github.com/gardener/gardener-extension-provider-openstack/pkg/client \
  github.com/gardener/gardener-extension-provider-openstack/pkg/apis \
  github.com/gardener/gardener-extension-provider-openstack/pkg/apis \
  "openstack:v1alpha1" \
  --go-header-file "${GARDENER_HACK_DIR}/LICENSE_BOILERPLATE.txt"

bash "${CODE_GEN_DIR}"/generate-internal-groups.sh \
  conversion \
  github.com/gardener/gardener-extension-provider-openstack/pkg/client \
  github.com/gardener/gardener-extension-provider-openstack/pkg/apis \
  github.com/gardener/gardener-extension-provider-openstack/pkg/apis \
  "openstack:v1alpha1" \
  --extra-peer-dirs=github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack,github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1,k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/conversion,k8s.io/apimachinery/pkg/runtime \
  --go-header-file "${GARDENER_HACK_DIR}/LICENSE_BOILERPLATE.txt"

bash "${CODE_GEN_DIR}"/generate-internal-groups.sh \
  deepcopy,defaulter \
  github.com/gardener/gardener-extension-provider-openstack/pkg/client/componentconfig \
  github.com/gardener/gardener-extension-provider-openstack/pkg/apis \
  github.com/gardener/gardener-extension-provider-openstack/pkg/apis \
  "config:v1alpha1" \
  --go-header-file "${GARDENER_HACK_DIR}/LICENSE_BOILERPLATE.txt"

bash "${CODE_GEN_DIR}"/generate-internal-groups.sh \
  conversion \
  github.com/gardener/gardener-extension-provider-openstack/pkg/client/componentconfig \
  github.com/gardener/gardener-extension-provider-openstack/pkg/apis \
  github.com/gardener/gardener-extension-provider-openstack/pkg/apis \
  "config:v1alpha1" \
  --extra-peer-dirs=github.com/gardener/gardener-extension-provider-openstack/pkg/apis/config,github.com/gardener/gardener-extension-provider-openstack/pkg/apis/config/v1alpha1,k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/conversion,k8s.io/apimachinery/pkg/runtime,github.com/gardener/gardener/extensions/pkg/apis/config/v1alpha1 \
  --go-header-file "${GARDENER_HACK_DIR}/LICENSE_BOILERPLATE.txt"
