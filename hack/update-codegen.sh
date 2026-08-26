#!/bin/bash
#
# SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

CODE_GEN_DIR=$(GOWORK=off go list -m -modfile "$(go list -m -f '{{.Dir}}' github.com/gardener/gardener/hack/tools)/go.mod" -f '{{.Dir}}' k8s.io/code-generator)
source "${CODE_GEN_DIR}/kube_codegen.sh"

CURRENT_DIR=$(dirname $0)
PROJECT_ROOT="${CURRENT_DIR}"/..

kube::codegen::gen_helpers \
  --boilerplate "${GARDENER_HACK_DIR}/LICENSE_BOILERPLATE.txt" \
  "${PROJECT_ROOT}/pkg/apis/openstack"

kube::codegen::gen_helpers \
  --boilerplate "${GARDENER_HACK_DIR}/LICENSE_BOILERPLATE.txt" \
  "${PROJECT_ROOT}/pkg/apis/config"
