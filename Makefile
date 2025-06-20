# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

ENSURE_GARDENER_MOD         := $(shell go get github.com/gardener/gardener@$$(go list -m -f "{{.Version}}" github.com/gardener/gardener))
GARDENER_HACK_DIR    		    := $(shell go list -m -f "{{.Dir}}" github.com/gardener/gardener)/hack
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
PLATFORM 					          := linux/amd64
EXTENSION_NAMESPACE			    := garden

TEST_RECONCILER           := tf
TEST_LOGLEVEL             := info
TEST_USE_EXISTING_CLUSTER := false # set to true if you want to use an existing cluster for backupbucket integration tests

WEBHOOK_CONFIG_PORT	:= 8443
WEBHOOK_CONFIG_MODE	:= url
WEBHOOK_CONFIG_URL	:= host.docker.internal:$(WEBHOOK_CONFIG_PORT)

WEBHOOK_PARAM := --webhook-config-url=$(WEBHOOK_CONFIG_URL)
ifeq ($(WEBHOOK_CONFIG_MODE), service)
  WEBHOOK_PARAM := --webhook-config-namespace=$(EXTENSION_NAMESPACE)
endif

REGION             := .kube-secrets/openstack/region.secret
AUTH_URL           := .kube-secrets/openstack/auth_url.secret
DOMAIN_NAME        := .kube-secrets/openstack/domain_name.secret
FLOATING_POOL_NAME := .kube-secrets/openstack/floating_pool_name.secret
TENANT_NAME        := .kube-secrets/openstack/tenant_name.secret
# either authenticate with username/password credentials
USER_NAME          := .kube-secrets/openstack/user_name.secret
PASSWORD           := .kube-secrets/openstack/password.secret
# or application credentials
APP_ID             := .kube-secrets/openstack/app_id.secret
APP_NAME           := .kube-secrets/openstack/app_name.secret
APP_SECRET         := .kube-secrets/openstack/app_secret.secret

INFRA_TEST_FLAGS   := --v -ginkgo.v -ginkgo.progress \
                      --kubeconfig=${KUBECONFIG} \
                      --auth-url='$(shell cat $(AUTH_URL))' \
                      --domain-name='$(shell cat $(DOMAIN_NAME))' \
                      --floating-pool-name='$(shell cat $(FLOATING_POOL_NAME))' \
                      --password='$(shell cat $(PASSWORD))' \
                      --tenant-name='$(shell cat $(TENANT_NAME))' \
                      --user-name='$(shell cat $(USER_NAME))' \
                      --region='$(shell cat $(REGION))' \
                      --app-id='$(shell cat $(APP_ID))' \
                      --app-name='$(shell cat $(APP_NAME))' \
                      --app-secret='$(shell cat $(APP_SECRET))'

BACKUPBUCKET_TEST_FLAGS   := --v -ginkgo.v -ginkgo.show-node-events \
                      --kubeconfig=${KUBECONFIG} \
                      --auth-url='$(shell cat $(AUTH_URL))' \
                      --domain-name='$(shell cat $(DOMAIN_NAME))' \
                      --tenant-name='$(shell cat $(TENANT_NAME))' \
                      --region='$(shell cat $(REGION))' \
		                  --use-existing-cluster=$(TEST_USE_EXISTING_CLUSTER) \
		                  --log-level=$(TEST_LOGLEVEL) \
                      --password='$(shell cat $(PASSWORD))' \
                      --user-name='$(shell cat $(USER_NAME))'

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
	@go run \
		-ldflags $(LD_FLAGS) \
		./cmd/$(EXTENSION_PREFIX)-$(ADMISSION_NAME) \
		--webhook-config-server-host=0.0.0.0 \
		--webhook-config-server-port=$(WEBHOOK_CONFIG_PORT) \
		--webhook-config-mode=$(WEBHOOK_CONFIG_MODE) \
		--leader-election-namespace=$(EXTENSION_NAMESPACE) \
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
docker-image-provider:
	@docker buildx build --platform $(PLATFORM) --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION) -t $(IMAGE_PREFIX)/$(NAME):$(VERSION)           -t $(IMAGE_PREFIX)/$(NAME):latest           -f Dockerfile -m 6g --target $(EXTENSION_PREFIX)-$(NAME)           .

.PHONY: docker-image-admission
docker-image-admission:
	@docker buildx build --platform $(PLATFORM) --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION) -t $(IMAGE_PREFIX)/$(ADMISSION_NAME):$(VERSION) -t $(IMAGE_PREFIX)/$(ADMISSION_NAME):latest -f Dockerfile -m 6g --target $(EXTENSION_PREFIX)-$(ADMISSION_NAME) .

.PHONY: docker-images
docker-images: docker-image-provider docker-image-admission

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

.PHONY: sast
sast: $(GOSEC)
	@bash $(GARDENER_HACK_DIR)/sast.sh

.PHONY: sast-report
sast-report: $(GOSEC)
	@bash $(GARDENER_HACK_DIR)/sast.sh --gosec-report true

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
verify: check format test sast

.PHONY: verify-extended
verify-extended: check-generate check format test-cov test-clean sast-report

.PHONY: integration-test-infra
integration-test-infra:
	@go test -timeout=0 ./test/integration/infrastructure \
		--reconciler='$(TEST_RECONCILER)' \
		$(INFRA_TEST_FLAGS)

.PHONY: integration-test-bastion
integration-test-bastion:
	@go test -timeout=0 ./test/integration/bastion \
		$(INFRA_TEST_FLAGS)

.PHONY: integration-test-backupbucket
integration-test-backupbucket:
	@go test -timeout=0 ./test/integration/backupbucket \
		$(BACKUPBUCKET_TEST_FLAGS)

