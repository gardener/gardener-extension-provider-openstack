# Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

ENSURE_GARDENER_MOD         := $(shell go get github.com/gardener/gardener@$$(go list -m -f "{{.Version}}" github.com/gardener/gardener))
GARDENER_HACK_DIR    		:= $(shell go list -m -f "{{.Dir}}" github.com/gardener/gardener)/hack
EXTENSION_PREFIX            := gardener-extension
NAME                        := provider-openstack
ADMISSION_NAME              := admission-openstack
REGISTRY                    := europe-docker.pkg.dev/gardener-project/public
IMAGE_PREFIX                := $(REGISTRY)/gardener/extensions
REPO_ROOT                   := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
HACK_DIR                    := $(REPO_ROOT)/hack
VERSION                     := $(shell cat "$(REPO_ROOT)/VERSION")
EFFECTIVE_VERSION           := $(VERSION)-$(shell git rev-parse HEAD)
LD_FLAGS                    := "-w $(shell bash $(GARDENER_HACK_DIR)/get-build-ld-flags.sh k8s.io/component-base $(REPO_ROOT)/VERSION $(EXTENSION_PREFIX))"
LEADER_ELECTION             := false
IGNORE_OPERATION_ANNOTATION := true
PLATFORM 					:= linux/amd64
EXTENSION_NAMESPACE			:= garden

WEBHOOK_CONFIG_PORT	:= 8443
WEBHOOK_CONFIG_MODE	:= url
WEBHOOK_CONFIG_URL	:= host.docker.internal:$(WEBHOOK_CONFIG_PORT)
EXTENSION_NAMESPACE	:=

WEBHOOK_PARAM := --webhook-config-url=$(WEBHOOK_CONFIG_URL)
ifeq ($(WEBHOOK_CONFIG_MODE), service)
  WEBHOOK_PARAM := --webhook-config-namespace=$(EXTENSION_NAMESPACE)
endif

REGION             := .kube-secrets/openstack/region.secret
AUTH_URL           := .kube-secrets/openstack/auth_url.secret
DOMAIN_NAME        := .kube-secrets/openstack/domain_name.secret
FLOATING_POOL_NAME := .kube-secrets/openstack/floating_pool_name.secret
PASSWORD           := .kube-secrets/openstack/password.secret
TENANT_NAME        := .kube-secrets/openstack/tenant_name.secret
USER_NAME          := .kube-secrets/openstack/user_name.secret

ifneq ($(strip $(shell git status --porcelain 2>/dev/null)),)
	EFFECTIVE_VERSION := $(EFFECTIVE_VERSION)-dirty
endif

#########################################
# Tools                                 #
#########################################

TOOLS_DIR := $(HACK_DIR)/tools
include $(GARDENER_HACK_DIR)/tools.mk

#########################################
# Rules for local development scenarios #
#########################################

.PHONY: start
start:
	@LEADER_ELECTION_NAMESPACE=$(EXTENSION_NAMESPACE) go run \
		-ldflags $(LD_FLAGS) \
		./cmd/$(EXTENSION_PREFIX)-$(NAME) \
		--config-file=./example/00-componentconfig.yaml \
		--ignore-operation-annotation=$(IGNORE_OPERATION_ANNOTATION) \
		--leader-election=$(LEADER_ELECTION) \
		--webhook-config-server-host=0.0.0.0 \
		--webhook-config-server-port=$(WEBHOOK_CONFIG_PORT) \
		--webhook-config-mode=$(WEBHOOK_CONFIG_MODE) \
		--webhook-config-service-port=443 \
		--gardener-version="v1.39.0" \
		--heartbeat-namespace=$(EXTENSION_NAMESPACE) \
		--heartbeat-renew-interval-seconds=30 \
		--metrics-bind-address=:8080 \
		--health-bind-address=:8081 \
		$(WEBHOOK_PARAM)

.PHONY: start-admission
start-admission:
	@LEADER_ELECTION_NAMESPACE=$(EXTENSION_NAMESPACE) go run \
		-ldflags $(LD_FLAGS) \
		./cmd/$(EXTENSION_PREFIX)-$(ADMISSION_NAME) \
		--webhook-config-server-host=0.0.0.0 \
		--webhook-config-server-port=$(WEBHOOK_CONFIG_PORT) \
		--webhook-config-mode=$(WEBHOOK_CONFIG_MODE) \
		$(WEBHOOK_PARAM)

.PHONY: hook-me
hook-me:
	@bash $(GARDENER_HACK_DIR)/hook-me.sh $(EXTENSION_NAMESPACE) $(EXTENSION_PREFIX)-$(NAME) $(WEBHOOK_CONFIG_PORT)

#################################################################
# Rules related to binary build, Docker image build and release #
#################################################################

.PHONY: install
install:
	@LD_FLAGS=$(LD_FLAGS) EFFECTIVE_VERSION=$(EFFECTIVE_VERSION) \
	bash $(GARDENER_HACK_DIR)/install.sh ./...

.PHONY: docker-login
docker-login:
	@gcloud auth activate-service-account --key-file .kube-secrets/gcr/gcr-readwrite.json

.PHONY: docker-image-provider
docker-images-provider:
	@docker buildx build --platform $(PLATFORM) --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION) -t $(IMAGE_PREFIX)/$(NAME):$(VERSION)           -t $(IMAGE_PREFIX)/$(NAME):latest           -f Dockerfile -m 6g --target $(EXTENSION_PREFIX)-$(NAME)           .

.PHONY: docker-image-admission
docker-images-admission:
	@docker buildx build --platform $(PLATFORM) --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION) -t $(IMAGE_PREFIX)/$(ADMISSION_NAME):$(VERSION) -t $(IMAGE_PREFIX)/$(ADMISSION_NAME):latest -f Dockerfile -m 6g --target $(EXTENSION_PREFIX)-$(ADMISSION_NAME) .

.PHONY: docker-images
docker-images: docker-images-provider docker-images-admission

#####################################################################
# Rules for verification, formatting, linting, testing and cleaning #
#####################################################################

.PHONY: tidy
tidy:
	@go mod tidy
	@mkdir -p $(REPO_ROOT)/.ci/hack && cp $(GARDENER_HACK_DIR)/.ci/* $(REPO_ROOT)/.ci/hack/ && chmod +xw $(REPO_ROOT)/.ci/hack/*
	@GARDENER_HACK_DIR=$(GARDENER_HACK_DIR) bash $(REPO_ROOT)/hack/update-github-templates.sh
	@cp $(GARDENER_HACK_DIR)/cherry-pick-pull.sh $(HACK_DIR)/cherry-pick-pull.sh && chmod +xw $(HACK_DIR)/cherry-pick-pull.sh

.PHONY: clean
clean:
	@$(shell find ./example -type f -name "controller-registration.yaml" -exec rm '{}' \;)
	@bash $(GARDENER_HACK_DIR)/clean.sh ./cmd/... ./pkg/... ./test/...

.PHONY: check-generate
check-generate:
	@bash $(GARDENER_HACK_DIR)/check-generate.sh $(REPO_ROOT)

.PHONY: check
check: $(GOIMPORTS) $(GOLANGCI_LINT)
	@REPO_ROOT=$(REPO_ROOT) bash $(GARDENER_HACK_DIR)/check.sh --golangci-lint-config=./.golangci.yaml ./cmd/... ./pkg/... ./test/...
	@REPO_ROOT=$(REPO_ROOT) bash $(GARDENER_HACK_DIR)/check-charts.sh ./charts

.PHONY: generate
generate: $(VGOPATH) $(CONTROLLER_GEN) $(GEN_CRD_API_REFERENCE_DOCS) $(HELM) $(MOCKGEN) $(YQ)
	@REPO_ROOT=$(REPO_ROOT) VGOPATH=$(VGOPATH) GARDENER_HACK_DIR=$(GARDENER_HACK_DIR) bash $(GARDENER_HACK_DIR)/generate-sequential.sh ./charts/... ./cmd/... ./example/... ./pkg/...
	$(MAKE) format

.PHONY: format
format: $(GOIMPORTS) $(GOIMPORTSREVISER)
	@bash $(GARDENER_HACK_DIR)/format.sh ./cmd ./pkg ./test

.PHONY: test
test:
	@bash $(GARDENER_HACK_DIR)/test.sh ./cmd/... ./pkg/...

.PHONY: test-cov
test-cov:
	@bash $(GARDENER_HACK_DIR)/test-cover.sh ./cmd/... ./pkg/...

.PHONY: test-clean
test-clean:
	@bash $(GARDENER_HACK_DIR)/test-cover-clean.sh

.PHONY: verify
verify: check format test

.PHONY: verify-extended
verify-extended: check-generate check format test-cov test-clean

.PHONY: integration-test-infra
integration-test-infra:
	@go test -timeout=0 ./test/integration/infrastructure \
		--v -ginkgo.v -ginkgo.progress \
		--kubeconfig=${KUBECONFIG} \
		--auth-url='$(shell cat $(AUTH_URL))' \
		--domain-name='$(shell cat $(DOMAIN_NAME))' \
		--floating-pool-name='$(shell cat $(FLOATING_POOL_NAME))' \
		--password='$(shell cat $(PASSWORD))' \
		--tenant-name='$(shell cat $(TENANT_NAME))' \
		--user-name='$(shell cat $(USER_NAME))' \
		--region='$(shell cat $(REGION))'

.PHONY: integration-test-bastion
integration-test-bastion:
	@go test -timeout=0 ./test/integration/bastion \
		--v -ginkgo.v -ginkgo.progress \
		--kubeconfig=${KUBECONFIG} \
		--auth-url='$(shell cat $(AUTH_URL))' \
		--domain-name='$(shell cat $(DOMAIN_NAME))' \
		--floating-pool-name='$(shell cat $(FLOATING_POOL_NAME))' \
		--password='$(shell cat $(PASSWORD))' \
		--tenant-name='$(shell cat $(TENANT_NAME))' \
		--user-name='$(shell cat $(USER_NAME))' \
		--region='$(shell cat $(REGION))'
