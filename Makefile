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
TOOLSDIR=$(CURDIR)/bin
BUILDDIR=$(CURDIR)/build/_output
GOFILES=$(shell find . -name "*.go" | grep -vE "(\/vendor\/)|(_test.go)")
PKGS=$(or $(PKG),$(shell $(GO) list ./... | grep -v "^$(PACKAGE)/vendor/"))
TESTPKGS = $(shell $(GO) list -f '{{ if or .TestGoFiles .XTestGoFiles }}{{ .ImportPath }}{{ end }}' $(PKGS))
ENVTEST_K8S_VERSION=1.24

# Version
VERSION?=master
DATE=`date -Iseconds`
COMMIT?=`git rev-parse --verify HEAD`
LDFLAGS="-X github.com/Mellanox/network-operator/version.Version=$(VERSION) -X github.com/Mellanox/network-operator/version.Commit=$(COMMIT) -X github.com/Mellanox/network-operator/version.Date=$(DATE)"

BUILD_VERSION := $(strip $(shell [ -d .git ] && git describe --always --tags --dirty))
BUILD_TIMESTAMP := $(shell date -u +"%Y-%m-%dT%H:%M:%S%Z")
VCS_BRANCH := $(strip $(shell git rev-parse --abbrev-ref HEAD))
VCS_REF := $(strip $(shell [ -d .git ] && git rev-parse --short HEAD))

# Docker
IMAGE_BUILDER?=@docker
IMAGEDIR=$(CURDIR)/images
DOCKERFILE?=$(CURDIR)/Dockerfile
TAG?=mellanox/network-operator
IMAGE_BUILD_OPTS?=
BUNDLE_IMG?=network-operator-bundle:$(VERSION)
# BUNDLE_GEN_FLAGS are the flags passed to the operator-sdk generate bundle command
BUNDLE_GEN_FLAGS ?= -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)

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

# Go tools
GO      = go
GOLANGCI_LINT = $(TOOLSDIR)/golangci-lint
# golangci-lint version should be updated periodically
# we keep it fixed to avoid it from unexpectedly failing on the project
# in case of a version bump
GOLANGCI_LINT_VER = v1.52.2

HADOLINT = $(TOOLSDIR)/hadolint
HADOLINT_VER = v1.23.0

HELM = $(TOOLSDIR)/helm
GET_HELM = $(TOOLSDIR)/get_helm.sh
HELM_VER = v3.5.3

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

$(BUILDDIR): ; $(info Creating build directory...)
	mkdir -p $@

build: generate $(BUILDDIR)/$(BINARY_NAME) ; $(info Building $(BINARY_NAME)...) @ ## Build executable file
	$(info Done!)

$(BUILDDIR)/$(BINARY_NAME): $(GOFILES) | $(BUILDDIR)
	CGO_ENABLED=0 $(GO) build -o $(BUILDDIR)/$(BINARY_NAME) -tags no_openssl -v -ldflags=$(LDFLAGS)

# Tools

$(GOLANGCI_LINT): ; $(info  installing golangci-lint...)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VER))

GOVERALLS = $(TOOLSDIR)/goveralls
$(GOVERALLS): | $(TOOLSDIR) ; $(info  installing goveralls...)
	$(call go-install-tool,$(GOVERALLS),github.com/mattn/goveralls@latest)

$(HADOLINT): | $(TOOLSDIR) ; $(info  install hadolint...)
	$Q curl -sSfL -o $(HADOLINT)  https://github.com/hadolint/hadolint/releases/download/$(HADOLINT_VER)/hadolint-Linux-x86_64
	$Q chmod +x $(HADOLINT)

$(HELM): | $(TOOLSDIR) ; $(info  install helm...)
	$Q curl -fsSL -o $(GET_HELM) https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3
	$Q chmod +x $(GET_HELM)
	$Q env HELM_INSTALL_DIR=$(TOOLSDIR) PATH=$(PATH):$(TOOLSDIR) $(GET_HELM) --no-sudo -v $(HELM_VER)
	$Q rm -f $(GET_HELM)

# Tests

.PHONY: lint
lint: | $(GOLANGCI_LINT) ; $(info  running golangci-lint...) @ ## Run golangci-lint
	$Q $(GOLANGCI_LINT) run --timeout=10m

.PHONY: lint-dockerfile
lint-dockerfile: $(HADOLINT) ; $(info  running Dockerfile lint with hadolint...) @ ## Run hadolint
# DL3018 - allow installing apks without explicit version
# DL3006 - Always tag the version of an image explicitly (until https://github.com/hadolint/hadolint/issues/339 is fixed)

	$Q $(HADOLINT) --ignore DL3018 --ignore DL3006 Dockerfile

.PHONY: lint-helm
lint-helm: $(HELM) ; $(info  running lint for helm charts...) @ ## Run helm lint
	$Q $(HELM) lint $(CHART_PATH)

.PHONY: check-manifests
check-manifests: generate manifests
	$(info checking for git diff after running 'make manifests')
	git diff --quiet ; if [ $$? -eq 1 ] ; then echo "Please, commit manifests after running 'make manifests' command"; exit 1 ; fi

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
ENVTEST_ASSETS_DIR=$(shell pwd)/testbin
check test tests: generate lint manifests ; $(info  running $(NAME:%=% )tests...) @ ## Run tests
	mkdir -p ${ENVTEST_ASSETS_DIR}
	test -f ${ENVTEST_ASSETS_DIR}/setup-envtest.sh || curl -sSLo ${ENVTEST_ASSETS_DIR}/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/v0.8.0/hack/setup-envtest.sh
	. ${ENVTEST_ASSETS_DIR}/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); $(GO) test -timeout $(TIMEOUT)s $(ARGS) $(TESTPKGS)

test-xml: generate lint manifests | $(GO2XUNIT) ; $(info  running $(NAME:%=% )tests...) @ ## Run tests with xUnit output
	mkdir test
	mkdir -p ${ENVTEST_ASSETS_DIR}
	test -f ${ENVTEST_ASSETS_DIR}/setup-envtest.sh || curl -sSLo ${ENVTEST_ASSETS_DIR}/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/v0.8.0/hack/setup-envtest.sh
	. ${ENVTEST_ASSETS_DIR}/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); 2>&1 $(GO) test -timeout $(TIMEOUT)s -v $(TESTPKGS) | tee test/tests.output
	$(GO2XUNIT) -fail -input test/tests.output -output test/tests.xml

COVERAGE_MODE = count
.PHONY: test-coverage test-coverage-tools
test-coverage-tools: | $(GOVERALLS)
test-coverage: COVERAGE_DIR := $(CURDIR)/test
test-coverage: test-coverage-tools ; $(info  running coverage tests...) @ ## Run coverage tests
	mkdir -p ${ENVTEST_ASSETS_DIR}
	test -f ${ENVTEST_ASSETS_DIR}/setup-envtest.sh || curl -sSLo ${ENVTEST_ASSETS_DIR}/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/v0.8.0/hack/setup-envtest.sh
	. ${ENVTEST_ASSETS_DIR}/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); $(GO) test -covermode=$(COVERAGE_MODE) -coverprofile=network-operator.cover ./...

# Container image
.PHONY: image
image: ; $(info Building Docker image...)  @ ## Build container image
	$(IMAGE_BUILDER) build --build-arg BUILD_DATE="$(BUILD_TIMESTAMP)" \
		--build-arg VERSION="$(BUILD_VERSION)" \
		--build-arg VCS_REF="$(VCS_REF)" \
		--build-arg VCS_BRANCH="$(VCS_BRANCH)" \
		-t $(TAG) -f $(DOCKERFILE)  $(CURDIR) $(IMAGE_BUILD_OPTS)

image-push:
	$(IMAGE_BUILDER) push $(TAG)

.PHONY: chart-build
chart-build: $(HELM) ; $(info Building Helm image...)  @ ## Build Helm Chart
	$Q $(HELM) package deployment/network-operator/ --version $(VERSION)

.PHONY: chart-push
chart-push: $(HELM) ; $(info Pushing Helm image...)  @ ## Push Helm Chart
	ngc registry chart push $(NGC_REPO):$(VERSION)

# Misc

.PHONY: clean
clean: ; $(info  Cleaning...)	 @ ## Cleanup everything
	@rm -rf $(BUILDDIR)
	@rm -rf  test
	@rm -rf $(TOOLSDIR)

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

deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${TAG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -
	kubectl apply -f hack/crds/*

deploy-with-psp: deploy ## Deploy controller to the K8s cluster specified in ~/.kube/config and apply privileged pod security policy
	$(KUSTOMIZE) build config/psp | kubectl apply -f -

undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/default | kubectl delete -f -
	$(KUSTOMIZE) build config/resources-namespace | kubectl delete -f -
	kubectl delete -f hack/crds/*

.PHONY: manifests
manifests: controller-gen	## Generate manifests e.g. CRD, RBAC etc.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	cp config/crd/bases/* deployment/network-operator/crds/

generate: controller-gen ## Generate code
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

CONTROLLER_GEN = $(shell pwd)/bin/controller-gen
controller-gen:	## Download controller-gen locally if necessary
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v0.10.0)

KUSTOMIZE = $(shell pwd)/bin/kustomize
kustomize: ## Download kustomize locally if necessary.
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v4@v4.5.5)

.PHONY: operator-sdk
OPERATOR_SDK = $(TOOLSDIR)/operator-sdk
operator-sdk: ## Download operator-sdk locally if necessary.
ifeq (,$(wildcard $(OPERATOR_SDK)))
ifeq (,$(shell which operator-sdk 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPERATOR_SDK)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	OPERATOR_SDK_DL_URL=https://github.com/operator-framework/operator-sdk/releases/download/v1.22.0 && \
	curl -LO $${OPERATOR_SDK_DL_URL}/operator-sdk_$${OS}_$${ARCH} && \
	chmod +x operator-sdk_$${OS}_$${ARCH} && mv operator-sdk_$${OS}_$${ARCH} ./bin/operator-sdk ;\
	}
else
OPERATOR_SDK = $(shell which operator-sdk)
endif
endif


.PHONY: bundle
bundle: operator-sdk manifests kustomize ## Generate bundle manifests and metadata, then validate generated files.
	$(OPERATOR_SDK) generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(TAG)
	$(KUSTOMIZE) build config/manifests | $(OPERATOR_SDK) generate bundle $(BUNDLE_GEN_FLAGS)
	$(OPERATOR_SDK) bundle validate ./bundle

.PHONY: bundle-build
bundle-build: ## Build the bundle image.
	${IMAGE_BUILDER} build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: bundle-push
bundle-push: ## Push the bundle image.
	${IMAGE_BUILDER} push $(BUNDLE_IMG)

.PHONY: release-build
release-build:
	cd hack && $(GO) run release.go --templateDir ./templates/samples/ --outputDir ../config/samples/
	cd hack && $(GO) run release.go --templateDir ./templates/crs/ --outputDir ../example/crs
	cd hack && $(GO) run release.go --templateDir ./templates/values/ --outputDir ../deployment/network-operator/

# go-install-tool will 'go install' any package $2 and install it to $1.
define go-install-tool
@[ -f $(1) ] || { \
echo "Downloading $(2)" ;\
GOBIN=$(TOOLSDIR) go install $(2) ;\
}
endef
