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

EXTENSION_PREFIX            := gardener-extension
NAME                        := provider-openstack
REGISTRY                    := eu.gcr.io/gardener-project
IMAGE_PREFIX                := $(REGISTRY)/extensions
REPO_ROOT                   := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
HACK_DIR                    := $(REPO_ROOT)/hack
VERSION                     := $(shell cat "$(REPO_ROOT)/VERSION")
LD_FLAGS                    := "-w -X github.com/gardener/$(EXTENSION_PREFIX)-$(NAME)/pkg/version.Version=$(IMAGE_TAG)"
VERIFY                      := true
LEADER_ELECTION             := false
IGNORE_OPERATION_ANNOTATION := true

WEBHOOK_CONFIG_MODE	:= service
WEBHOOK_CONFIG_URL	:= docker.for.mac.localhost
EXTENSION_NAMESPACE	:=

WEBHOOK_PARAM := --webhook-config-url=$(WEBHOOK_CONFIG_URL)
ifeq ($(WEBHOOK_CONFIG_MODE), service)
  WEBHOOK_PARAM := --webhook-config-namespace=$(EXTENSION_NAMESPACE)
endif

### Build commands

.PHONY: format
format:
	@$(REPO_ROOT)/vendor/github.com/gardener/gardener-extensions/hack/format.sh ./cmd ./pkg ./test

.PHONY: clean
clean:
	@$(REPO_ROOT)/vendor/github.com/gardener/gardener-extensions/hack/clean.sh ./pkg/...

.PHONY: generate
generate:
	@$(REPO_ROOT)/vendor/github.com/gardener/gardener-extensions/hack/generate.sh ./...

.PHONY: check
check:
	@$(REPO_ROOT)/vendor/github.com/gardener/gardener-extensions/hack/check.sh ./...

.PHONY: test
test:
	@$(REPO_ROOT)/vendor/github.com/gardener/gardener-extensions/hack/test.sh --skipPackage test/e2e/networkpolicies,test/integration -r ./...

.PHONY: verify
verify: check generate test format

.PHONY: install
install:
	@LD_FLAGS="-w -X github.com/gardener/$(EXTENSION_PREFIX)-$(NAME)/pkg/version.Version=$(VERSION)" \
	$(REPO_ROOT)/vendor/github.com/gardener/gardener-extensions/hack/install.sh ./...

.PHONY: install-requirements
install-requirements:
	@go install -mod=vendor $(REPO_ROOT)/vendor/github.com/ahmetb/gen-crd-api-reference-docs
	@go install -mod=vendor $(REPO_ROOT)/vendor/github.com/gobuffalo/packr/v2/packr2
	@go install -mod=vendor $(REPO_ROOT)/vendor/github.com/golang/mock/mockgen
	@go install -mod=vendor $(REPO_ROOT)/vendor/github.com/onsi/ginkgo/ginkgo
	@$(REPO_ROOT)/vendor/github.com/gardener/gardener-extensions/hack/install-requirements.sh

.PHONY: all
ifeq ($(VERIFY),true)
all: verify generate install
else
all: generate install
endif

### Docker commands

.PHONY: docker-login
docker-login:
	@gcloud auth activate-service-account --key-file .kube-secrets/gcr/gcr-readwrite.json

.PHONY: docker-images
docker-images:
	@docker build -t $(IMAGE_PREFIX)/$(NAME):$(VERSION) -t $(IMAGE_PREFIX)/$(NAME):latest -f Dockerfile -m 6g --target $(EXTENSION_PREFIX)-$(NAME) .

### Debug / Development commands

.PHONY: revendor
revendor:
	@GO111MODULE=on go mod vendor
	@GO111MODULE=on go mod tidy
	@chmod +x $(REPO_ROOT)/vendor/github.com/gardener/gardener-extensions/hack/*
	@chmod +x $(REPO_ROOT)/vendor/github.com/gardener/gardener-extensions/hack/.ci/*

.PHONY: start
start:
	@LEADER_ELECTION_NAMESPACE=garden GO111MODULE=on go run \
		-mod=vendor \
		-ldflags $(LD_FLAGS) \
		./cmd/$(EXTENSION_PREFIX)-$(NAME) \
		--config-file=./example/00-componentconfig.yaml \
		--ignore-operation-annotation=$(IGNORE_OPERATION_ANNOTATION) \
		--leader-election=$(LEADER_ELECTION) \
		--webhook-config-server-host=0.0.0.0 \
		--webhook-config-server-port=8443 \
		--webhook-config-mode=$(WEBHOOK_CONFIG_MODE) \
		$(WEBHOOK_PARAM)
