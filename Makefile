# Copyright 2021 NVIDIA
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Package related
BINARY_NAME=network-operator
PACKAGE=network-operator
ORG_PATH=github.com/Mellanox
REPO_PATH=$(ORG_PATH)/$(PACKAGE)
CHART_PATH=$(CURDIR)/deployment/$(PACKAGE)
TOOLSDIR=$(CURDIR)/hack/tools/bin
MANIFESTDIR=$(CURDIR)/hack/manifests
HACKTMPDIR=$(CURDIR)/hack/tmp
BUILDDIR=$(CURDIR)/build/_output
GOFILES=$(shell find . -name "*.go" | grep -vE "(\/vendor\/)|(_test.go)")
TESTPKGS=./...
ENVTEST_K8S_VERSION=1.28
ARCH ?= $(shell go env GOARCH)
OS ?= $(shell go env GOOS)

# Version
VERSION?=master
DATE=`date -Iseconds`
COMMIT?=`git rev-parse --verify HEAD`
LDFLAGS="-X github.com/Mellanox/network-operator/version.Version=$(BUILD_VERSION) -X github.com/Mellanox/network-operator/version.Commit=$(COMMIT) -X github.com/Mellanox/network-operator/version.Date=$(DATE)"
GCFLAGS=""
BUILD_VERSION := $(strip $(shell [ -d .git ] && git describe --always --tags --dirty))
BUILD_TIMESTAMP := $(shell date -u +"%Y-%m-%dT%H:%M:%S%Z")
VCS_BRANCH := $(strip $(shell git rev-parse --abbrev-ref HEAD))
VCS_REF := $(strip $(shell [ -d .git ] && git rev-parse --short HEAD))
DOCA_DRIVER_RELEASE_URL := https://raw.githubusercontent.com/Mellanox/doca-driver-build/refs/heads/main/release_manifests/

# Docker
IMAGE_BUILDER?=docker
DOCKERFILE?=$(CURDIR)/Dockerfile
TAG?=mellanox/network-operator
REGISTRY?=docker.io
IMAGE_NAME?=network-operator
CONTROLLER_IMAGE=$(REGISTRY)/$(IMAGE_NAME)
IMAGE_BUILD_OPTS?=
BUNDLE_IMG?=network-operator-bundle:$(VERSION)
BUNDLE_OCP_VERSIONS=v4.14-v4.17
# BUNDLE_GEN_FLAGS are the flags passed to the operator-sdk generate bundle command
BUNDLE_GEN_FLAGS ?= -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)
BUILD_ARCH= amd64 arm64

# USE_IMAGE_DIGESTS defines if images are resolved via tags or digests
# You can enable this value if you would like to use SHA Based Digests
# To enable set flag to true
USE_IMAGE_DIGESTS ?= false
ifeq ($(USE_IMAGE_DIGESTS), true)
	BUNDLE_GEN_FLAGS += --use-image-digests
endif

# Accept proxy settings for docker
# To pass proxy for Docker invoke it as 'make image HTTP_POXY=http://192.168.0.1:8080'
DOCKERARGS=
ifdef HTTP_PROXY
	DOCKERARGS += --build-arg http_proxy=$(HTTP_PROXY)
endif
ifdef HTTPS_PROXY
	DOCKERARGS += --build-arg https_proxy=$(HTTPS_PROXY)
endif
IMAGE_BUILD_OPTS += $(DOCKERARGS)

# timeout for tests, seconds
TIMEOUT = 120
Q = $(if $(filter 1,$V),,@)

## Options for 'bundle-build'
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

admissionReviewVersions=v1

ifndef ignore-not-found
	ignore-not-found = false
endif

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

.PHONY: all
all: lint build

$(TOOLSDIR):
	@mkdir -p $@

$(MANIFESTDIR):
	@mkdir -p $@

$(BUILDDIR): ; $(info Creating build directory...)
	mkdir -p $@

$(HACKTMPDIR):
	@mkdir -p $@

build: generate $(BUILDDIR)/$(BINARY_NAME) ; $(info Building $(BINARY_NAME)...) @ ## Build executable file
	$(info Done!)

$(BUILDDIR)/$(BINARY_NAME): $(GOFILES) | $(BUILDDIR)
	CGO_ENABLED=0 $(GO) build -o $(BUILDDIR)/$(BINARY_NAME) -tags no_openssl -v -ldflags=$(LDFLAGS)

# Tools
GO = go

# golangci-lint is used to lint go code.
GOLANGCI_LINT_PKG=github.com/golangci/golangci-lint/cmd/golangci-lint
GOLANGCI_LINT_BIN= golangci-lint
GOLANGCI_LINT_VER = v1.61.0
GOLANGCI_LINT = $(TOOLSDIR)/$(GOLANGCI_LINT_BIN)-$(GOLANGCI_LINT_VER)
$(GOLANGCI_LINT):
	$(call go-install-tool,$(GOLANGCI_LINT_PKG),$(GOLANGCI_LINT_BIN),$(GOLANGCI_LINT_VER))

# controller gen is used to generate manifests and code for Kubernetes controllers.
CONTROLLER_GEN_PKG = sigs.k8s.io/controller-tools/cmd/controller-gen
CONTROLLER_GEN_BIN = controller-gen
CONTROLLER_GEN_VER = v0.16.4
CONTROLLER_GEN = $(TOOLSDIR)/$(CONTROLLER_GEN_BIN)-$(CONTROLLER_GEN_VER)
$(CONTROLLER_GEN):
	$(call go-install-tool,$(CONTROLLER_GEN_PKG),$(CONTROLLER_GEN_BIN),$(CONTROLLER_GEN_VER))

# kustomize is used to generate manifests for OpenShift bundles and developer deployments.
KUSTOMIZE_PKG = sigs.k8s.io/kustomize/kustomize/v5
KUSTOMIZE_BIN = kustomize
KUSTOMIZE_VER = v5.5.0
KUSTOMIZE = $(TOOLSDIR)/$(KUSTOMIZE_BIN)-$(KUSTOMIZE_VER)
$(KUSTOMIZE):
	$(call go-install-tool,$(KUSTOMIZE_PKG),$(KUSTOMIZE_BIN),$(KUSTOMIZE_VER))

# setup-envtest is used to install test Kubernetes control plane components for envtest-based tests.
SETUP_ENVTEST_PKG := sigs.k8s.io/controller-runtime/tools/setup-envtest
SETUP_ENVTEST_BIN := setup-envtest
SETUP_ENVTEST_VER := v0.0.0-20240110160329-8f8247fdc1c3
SETUP_ENVTEST := $(abspath $(TOOLSDIR)/$(SETUP_ENVTEST_BIN)-$(SETUP_ENVTEST_VER))
$(SETUP_ENVTEST):
	$(call go-install-tool,$(SETUP_ENVTEST_PKG),$(SETUP_ENVTEST_BIN),$(SETUP_ENVTEST_VER))

# hadolint is used to lint docker files.
HADOLINT_BIN = hadolint
HADOLINT_VER = v2.12.0
HADOLINT = $(abspath $(TOOLSDIR)/$(HADOLINT_BIN)-$(HADOLINT_VER))
$(HADOLINT): | $(TOOLSDIR)
	$Q echo "Installing hadolint-$(HADOLINT_VER) to $(TOOLSDIR)"
	$Q curl -sSfL -o $(HADOLINT)  https://github.com/hadolint/hadolint/releases/download/$(HADOLINT_VER)/hadolint-Linux-x86_64
	$Q chmod +x $(HADOLINT)

# helm is used to manage helm deployments and artifacts.
GET_HELM = $(TOOLSDIR)/get_helm.sh
HELM_VER = v3.13.3
HELM_BIN = helm
HELM = $(abspath $(TOOLSDIR)/$(HELM_BIN)-$(HELM_VER))
$(HELM): | $(TOOLSDIR)
	$Q echo "Installing helm-$(HELM_VER) to $(TOOLSDIR)"
	$Q curl -fsSL -o $(GET_HELM) https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3
	$Q chmod +x $(GET_HELM)
	$Q env HELM_INSTALL_DIR=$(TOOLSDIR) PATH=$(PATH):$(TOOLSDIR) $(GET_HELM) --no-sudo -v $(HELM_VER)
	$Q mv $(TOOLSDIR)/$(HELM_BIN) $(TOOLSDIR)/$(HELM_BIN)-$(HELM_VER)
	$Q rm -f $(GET_HELM)

# operator-sdk is used to generate operator-sdk bundles
OPERATOR_SDK_DL_URL=https://github.com/operator-framework/operator-sdk/releases/download
OPERATOR_SDK_BIN = operator-sdk
OPERATOR_SDK_VER = v1.33.0
OPERATOR_SDK = $(abspath $(TOOLSDIR)/$(OPERATOR_SDK_BIN)-$(OPERATOR_SDK_VER))
$(OPERATOR_SDK): | $(TOOLSDIR)
	$Q echo "Installing $(OPERATOR_SDK_BIN)-$(OPERATOR_SDK_VER) to $(TOOLSDIR)"
	$Q curl -sSfL $(OPERATOR_SDK_DL_URL)/$(OPERATOR_SDK_VER)/operator-sdk_$(OS)_$(ARCH) -o $(OPERATOR_SDK)
	$Q chmod +x $(OPERATOR_SDK)

# minikube is used to set-up a local kubernetes cluster for dev work.
MINIKUBE_VER := v0.0.0-20231012212722-e25aeebc7846
MINIKUBE_BIN := minikube
MINIKUBE := $(abspath $(TOOLSDIR)/$(MINIKUBE_BIN)-$(MINIKUBE_VER))
$(MINIKUBE): | $(TOOLSDIR)
	$Q echo "Installing minikube-$(MINIKUBE_VER) to $(TOOLSDIR)"
	$Q curl -fsSL https://storage.googleapis.com/minikube/releases/latest/minikube-$(OS)-$(ARCH) -o $(MINIKUBE)
	$Q chmod +x $(MINIKUBE)

# skaffold is used to run a debug build of the network operator for dev work.
SKAFFOLD_VER := v2.10.0
SKAFFOLD_BIN := skaffold
SKAFFOLD := $(abspath $(TOOLSDIR)/$(SKAFFOLD_BIN)-$(SKAFFOLD_VER))
$(SKAFFOLD): | $(TOOLSDIR)
	$Q echo "Installing skaffold-$(SKAFFOLD_VER) to $(TOOLSDIR)"
	$Q curl -fsSL https://storage.googleapis.com/skaffold/releases/latest/skaffold-$(OS)-$(ARCH) -o $(SKAFFOLD)
	$Q chmod +x $(SKAFFOLD)

# mockery is used to generate mocks for unit tests.
MOCKERY_PKG := github.com/vektra/mockery/v2
MOCKERY_VER := v2.43.2
MOCKERY_BIN := mockery
MOCKERY_PATH = $(abspath $(TOOLSDIR)/$(MOCKERY_BIN))
MOCKERY = $(abspath $(TOOLSDIR)/$(MOCKERY_BIN))-$(MOCKERY_VER)
$(MOCKERY): | $(TOOLSDIR)
	$(call go-install-tool,$(MOCKERY_PKG),$(MOCKERY_BIN),$(MOCKERY_VER))
	$Q ln $(MOCKERY) $(MOCKERY_PATH)

# cert-manager is used for webhook certs in the dev setup.
CERT_MANAGER_YAML=$(MANIFESTDIR)/cert-manager.yaml
CERT_MANAGER_VER=v1.13.3
$(CERT_MANAGER_YAML): | $(MANIFESTDIR)
	curl -fSsL "https://github.com/cert-manager/cert-manager/releases/download/$(CERT_MANAGER_VER)/cert-manager.yaml" -o $(CERT_MANAGER_YAML)

# Tests

.PHONY: lint
lint: | $(GOLANGCI_LINT) ; $(info  running golangci-lint...) @ ## Run golangci-lint
	$Q $(GOLANGCI_LINT) run --timeout=10m

.PHONY: lint-fix
lint-fix: | $(GOLANGCI_LINT) ; $(info  running golangci-lint...) @ ## Run golangci-lint and fix findings where possible
	$Q $(GOLANGCI_LINT) run --timeout=10m --fix

.PHONY: lint-dockerfile
lint-dockerfile: $(HADOLINT) ; $(info  running Dockerfile lint with hadolint...) @ ## Run hadolint
# Ignoring warning DL3029: Do not use --platform flag with FROM
	$Q $(HADOLINT) --ignore DL3029 Dockerfile

.PHONY: lint-helm
lint-helm: $(HELM) ; $(info  running lint for helm charts...) @ ## Run helm lint
	$Q $(HELM) lint $(CHART_PATH)

.PHONY: check-manifests
check-manifests: generate manifests
	$(info checking for git diff after running 'make manifests')
	git diff --quiet ; if [ $$? -eq 1 ] ; then echo "Please, commit manifests after running 'make manifests' and 'make generate' commands"; exit 1 ; fi

.PHONY: check-go-modules
check-go-modules: generate-go-modules
	git diff --quiet HEAD go.sum; if [ $$? -eq 1 ] ; then echo "go.sum is out of date. Please commit after running 'make generate-go-modules' command"; exit 1; fi

.PHONY: generate-go-modules
generate-go-modules:
	go mod tidy

.PHONY: check-release-build
check-release-build: release-build
	$(info checking for git diff after running 'make release-build')
	git diff --quiet ; if [ $$? -eq 1 ] ; then echo "Please, commit templates after running 'make release-build' command"; exit 1 ; fi

TEST_TARGETS := test-default test-bench test-short test-verbose test-race
.PHONY: $(TEST_TARGETS) test-xml check test tests
test-bench:   ARGS=-run=__absolutelynothing__ -bench=. ## Run benchmarks
test-short:   ARGS=-short        ## Run only short tests
test-verbose: ARGS=-v            ## Run tests in verbose mode with coverage reporting
test-race:    ARGS=-race         ## Run tests with race detector
$(TEST_TARGETS): NAME=$(MAKECMDGOALS:test-%=%)
$(TEST_TARGETS): test
check test tests test-xml test-coverage: SHELL:=/bin/bash

.PHONY: setup-envtest
setup-envtest: $(SETUP_ENVTEST)  ## Install envtest binaries
	@echo KUBEBUILDER_ASSETS=`$(SETUP_ENVTEST) use --use-env -p path $(ENVTEST_K8S_VERSION)`

.PHONY: clean-envtest
clean-envtest: setup-envtest ;## Clean up assets installed by setup-envtest
	$Q $(SETUP_ENVTEST) cleanup

check test tests: setup-envtest ; $(info  running $(NAME:%=% )tests...) @ ## Run tests
	KUBEBUILDER_ASSETS=`$(SETUP_ENVTEST) use --use-env -p path $(ENVTEST_K8S_VERSION)` $(GO) test -timeout $(TIMEOUT)s $(ARGS) $(TESTPKGS)

COVERAGE_MODE = count
.PHONY: test-coverage

test-coverage: COVERAGE_DIR := $(CURDIR)/test
test-coverage: setup-envtest; $(info  running coverage tests...) @ ## Run coverage tests
	KUBEBUILDER_ASSETS=`$(SETUP_ENVTEST) use --use-env -p path $(ENVTEST_K8S_VERSION)` $(GO) test -covermode=$(COVERAGE_MODE) -coverpkg=./... -coverprofile=network-operator.cover $(TESTPKGS)

.PHONY: generate-mocks
generate-mocks: | $(MOCKERY) ; $(info  running mockery...) @ ## Run mockery
	$Q PATH=$(TOOLSDIR):$(PATH) go generate ./...

# Container image
.PHONY: image
image: ; $(info Building Docker image...)  @ ## Build container image
	$Q DOCKER_BUILDKIT=1 $(IMAGE_BUILDER) build --build-arg BUILD_DATE="$(BUILD_TIMESTAMP)" \
		--build-arg VERSION="$(BUILD_VERSION)" \
		--build-arg VCS_REF="$(VCS_REF)" \
		--build-arg VCS_BRANCH="$(VCS_BRANCH)" \
		--build-arg LDFLAGS=$(LDFLAGS) \
		--build-arg ARCH="$(ARCH)" \
		--build-arg GCFLAGS="$(GCFLAGS)" \
		-t $(TAG) -f $(DOCKERFILE)  $(CURDIR) $(IMAGE_BUILD_OPTS)

image-push:
	$(IMAGE_BUILDER) push $(TAG)

# Targets for multi-arch container builds
# The multi arch build flow first builds containers locally with a suffix in the image tag stating the architecture.
# Each of these images is then tagged and pushed to the registry as `$(CONTROLLER_IMAGE):$(VERSION)`.
# A manifest is created linking all of the images together.
# None of the resulting images in the registry have an architecture specific tag or name.
.PHONY: image-build
image-build: ; $(info Building Docker image...)  @ ## Build container image
	DOCKER_BUILDKIT=1 $(IMAGE_BUILDER) build --build-arg BUILD_DATE="$(BUILD_TIMESTAMP)" \
		--build-arg VERSION="$(BUILD_VERSION)" \
		--build-arg VCS_REF="$(VCS_REF)" \
		--build-arg VCS_BRANCH="$(VCS_BRANCH)" \
		--build-arg LDFLAGS=$(LDFLAGS) \
		--build-arg ARCH="$(ARCH)" \
		--build-arg GCFLAGS="$(GCFLAGS)" \
		-t $(CONTROLLER_IMAGE):$(VERSION)-$(ARCH) -f $(DOCKERFILE)  $(CURDIR) $(IMAGE_BUILD_OPTS)

image-build-%:
	$(MAKE) ARCH=$* image-build

.PHONY: image-build-multiarch
image-build-multiarch: $(addprefix image-build-,$(BUILD_ARCH))

DOCKER_MANIFEST_CREATE_ARGS?="--amend"
image-manifest-for-arch: $(addprefix image-push-for-arch-,$(BUILD_ARCH))
	$(IMAGE_BUILDER) manifest create $(DOCKER_MANIFEST_CREATE_ARGS)  $(CONTROLLER_IMAGE):$(VERSION) $(shell $(IMAGE_BUILDER) inspect --format='{{index .RepoDigests 0}}' $(CONTROLLER_IMAGE):$(VERSION))

image-push-for-arch-%:
	$(IMAGE_BUILDER) tag $(CONTROLLER_IMAGE):$(VERSION)-$(ARCH) $(CONTROLLER_IMAGE):$(VERSION)
	$(IMAGE_BUILDER) push $(CONTROLLER_IMAGE):$(VERSION)

image-manifest-for-arch-%:
	$(MAKE) ARCH=$* image-manifest-for-arch

.PHONY: image-push-multiarch
image-push-multiarch: $(addprefix image-manifest-for-arch-,$(BUILD_ARCH))
	$(IMAGE_BUILDER) manifest push  $(CONTROLLER_IMAGE):$(VERSION)

.PHONY: chart-build
chart-build: $(HELM) ; $(info Building Helm image...)  @ ## Build Helm Chart
	@if [ -z "$(APP_VERSION)" ]; then \
		echo "APP_VERSION is not set, skipping a part of the command."; \
		$(HELM) package --dependency-update deployment/network-operator/ --version $(VERSION); \
	else $(HELM) package --dependency-update deployment/network-operator/ --version $(VERSION) --app-version $(APP_VERSION); \
	fi

.PHONY: chart-push
chart-push: $(HELM) ; $(info Pushing Helm image...)  @ ## Push Helm Chart
	ngc registry chart push $(NGC_REPO):$(VERSION)

# Misc

.PHONY: clean
clean: ; $(info  Cleaning...)	 @ ## Cleanup everything
	@rm -rf $(BUILDDIR)
	@rm -rf $(TOOLSDIR)
	@rm -rf $(MANIFESTDIR)
	@rm -rf $(HACKTMPDIR)

.PHONY: help
help: ## Show this message
	@grep -E '^[ a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

run: generate manifests	## Run against the configured Kubernetes cluster in ~/.kube/config
	go run ./main.go

install: manifests	## Install CRDs into a cluster
	kubectl apply -f config/crd/bases

uninstall: manifests	## Uninstall CRDs from a cluster
	sh kubectl delete --ignore-not-found=$(ignore-not-found) -f -

deploy: manifests $(KUSTOMIZE) ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${TAG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -
	kubectl apply -f hack/crds/*

undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/default | kubectl delete -f -
	$(KUSTOMIZE) build config/resources-namespace | kubectl delete -f -
	kubectl delete -f hack/crds/*

.PHONY: manifests
manifests: $(CONTROLLER_GEN)	## Generate manifests e.g. CRD, RBAC etc.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	cp config/crd/bases/* deployment/network-operator/crds/

generate: $(CONTROLLER_GEN) ## Generate code
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: bundle
bundle: $(OPERATOR_SDK) $(KUSTOMIZE) manifests ## Generate bundle manifests and metadata, then validate generated files.
	cd hack && $(GO) run release.go --with-sha256 --templateDir ./templates/config/manager --outputDir ../config/manager/
	cd hack && $(GO) run release.go --with-sha256 --templateDir ./templates/samples/ --outputDir ../config/samples/
	$(OPERATOR_SDK) generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(TAG)
	$(KUSTOMIZE) build config/manifests | $(OPERATOR_SDK) generate bundle $(BUNDLE_GEN_FLAGS)
	git checkout -- config/manager/kustomization.yaml
	GO=$(GO) BUNDLE_OCP_VERSIONS=$(BUNDLE_OCP_VERSIONS) TAG=$(TAG) hack/scripts/ocp-bundle-postprocess.sh
	$(OPERATOR_SDK) bundle validate ./bundle

.PHONY: bundle-build
bundle-build: ## Build the bundle image.
	${IMAGE_BUILDER} build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: bundle-push
bundle-push: ## Push the bundle image.
	${IMAGE_BUILDER} push $(BUNDLE_IMG)

.PHONY: release-build
release-build:
	yq '.[].version'  hack/release.yaml | grep 'latest' && exit 1 || true
	cd hack && $(GO) run release.go --templateDir ./templates/crs/ --outputDir ../example/crs
	cd hack && $(GO) run release.go --templateDir ./templates/values/ --outputDir ../deployment/network-operator/

.PHONY: check-doca-drivers
check-doca-drivers: $(HACKTMPDIR)
	$(eval DRIVERVERSION := $(shell yq '.Mofed.version'  hack/release.yaml | cut -d'-' -f1))
	wget $(DOCA_DRIVER_RELEASE_URL)$(DRIVERVERSION).yaml  -O $(HACKTMPDIR)/doca-driver-matrix.yaml
	cd hack && $(GO) run release.go --doca-driver-check --doca-driver-matrix $(HACKTMPDIR)/doca-driver-matrix.yaml

# dev environment

MINIKUBE_CLUSTER_NAME = net-op-dev
dev-minikube: $(MINIKUBE) ## Create a minikube cluster for development.
	CLUSTER_NAME=$(MINIKUBE_CLUSTER_NAME) MINIKUBE_BIN=$(MINIKUBE) $(CURDIR)/hack/scripts/minikube-install.sh

clean-minikube: $(MINIKUBE)  ## Delete the development minikube cluster.
	$(MINIKUBE) delete -p $(MINIKUBE_CLUSTER_NAME)

SKAFFOLD_REGISTRY=localhost:5000
dev-skaffold: $(SKAFFOLD) $(CERT_MANAGER_YAML) manifests generate dev-minikube ## Create a development minikube cluster and deploy the operator in debug mode.
	## Deploy the network attachment definition CRD.
	kubectl apply -f hack/crds/*
	## Deploy cert manager to provide certificates for webhooks.
	$Q kubectl apply -f $(CERT_MANAGER_YAML) \
	&& echo "Waiting for cert-manager deployment to be ready."\
	&& kubectl wait --for=condition=ready pod -l app=webhook --timeout=60s -n cert-manager
	# Use minikube for docker build and deployment.
	$Q eval $$($(MINIKUBE) -p $(MINIKUBE_CLUSTER_NAME) docker-env); \
	$(SKAFFOLD) debug --default-repo=$(SKAFFOLD_REGISTRY) --detect-minikube=false

# go-install-tool will 'go install' a go module $1 with version $3 and install it with the name $2-$3 to $TOOLSDIR.
define go-install-tool
	$Q echo "Installing $(2)-$(3) to $(TOOLSDIR)"
	$Q GOBIN=$(TOOLSDIR) go install $(1)@$(3)
	$Q mv $(TOOLSDIR)/$(2) $(TOOLSDIR)/$(2)-$(3)
endef
