
# Image URL to use all building image targets
DATETIME = $(shell date +"%Y%m%d%H%M%S")
KUSCIA_VERSION_TAG = $(shell git describe --tags --always)
# Get current architecture information
UNAME_M_OUTPUT := $(shell uname -m)

# To configure the ARCH variable to either arm64 or amd64 or UNAME_M_OUTPUT
ARCH := $(if $(filter aarch64 arm64,$(UNAME_M_OUTPUT)),arm64,$(if $(filter amd64 x86_64,$(UNAME_M_OUTPUT)),amd64,$(UNAME_M_OUTPUT)))


TAG = ${KUSCIA_VERSION_TAG}-${DATETIME}
IMG := secretflow/kuscia:${TAG}

# TEST_SUITE used by integration test
TEST_SUITE ?= all

ENVOY_IMAGE ?= secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia-envoy:0.5.0b0
DEPS_IMAGE ?= secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia-deps:0.5.0b0

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: ## Generate CustomResourceDefinition objects.
	bash hack/generate-crds.sh

.PHONY: generate
generate: gen-clientset gen-proto-code  ## Generate all code that Kuscia needs.

.PHONY: gen-clientset
gen-clientset:  # Generate CRD runtime.Object and Clientset\Informer|Listers.
	bash hack/update-codegen.sh

.PHONY: gen-proto-code
gen-proto-code:  # Generate protobuf golang code that Kuscia needs.
	bash hack/proto-to-go.sh

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: verify_error_code
verify_error_code: ## Verify integrity of error code i18n configuration.
	bash hack/errorcode/gen_error_code_doc.sh verify pkg/kusciaapi/errorcode/error_code.go hack/errorcode/i18n/errorcode.zh-CN.toml

.PHONY: gen_error_code_doc
gen_error_code_doc: verify_error_code ## Generate error code markdown doc.
	bash hack/errorcode/gen_error_code_doc.sh doc pkg/kusciaapi/errorcode/error_code.go hack/errorcode/i18n/errorcode.zh-CN.toml docs/reference/apis/error_code_cn.md

.PHONY: test
test: verify_error_code fmt vet ## Run tests.
	rm -rf ./test-results
	mkdir -p test-results
	go test ./cmd/... -gcflags="all=-N -l" -coverprofile=test-results/cmd.covprofile.out | tee test-results/cmd.output.txt
	go test ./pkg/... -gcflags="all=-N -l" -coverprofile=test-results/pkg.covprofile.out | tee test-results/pkg.output.txt

	cat ./test-results/cmd.output.txt | go-junit-report > ./test-results/TEST-cmd.xml
	cat ./test-results/pkg.output.txt | go-junit-report > ./test-results/TEST-pkg.xml

	echo "mode: set" > ./test-results/coverage.out && cat ./test-results/*.covprofile.out | grep -v mode: | sort -r | awk '{if($$1 != last) {print $0;last=$$1}}' >> ./test-results/coverage.out
	cat ./test-results/coverage.out | gocover-cobertura > ./test-results/coverage.xml

.PHONY: clean
clean: # clean build and test product.
	-rm -rf ./test-results
	-rm -rf ./build/apps
	-rm -rf ./build/framework
	-rm -rf ./tmp-crd-code
	-rm -rf ./build/linux

##@ Build

.PHONY: build
build: verify_error_code fmt vet ## Build kuscia binary.
	bash hack/build.sh -t kuscia
	mkdir -p build/linux/${ARCH}
	cp -rp build/apps build/linux/${ARCH}
.PHONY: docs
docs: gen_error_code_doc ## Build docs.
	cd docs && pip install -r requirements.txt && make html

.PHONY: deps-build
deps-build:
	bash hack/k3s/build.sh
	mkdir -p build/linux/${ARCH}/k3s
	cp -rp build/k3s/bin build/linux/${ARCH}/k3s


.PHONY: deps-image
deps-image: deps-build
	docker build -t ${DEPS_IMAGE} -f ./build/dockerfile/base/kuscia-deps.Dockerfile .

.PHONY: image
image: export GOOS=linux
image: export GOARCH=${ARCH}
image: build ## Build docker image with the manager.
	docker build -t ${IMG} --build-arg KUSCIA_ENVOY_IMAGE=${ENVOY_IMAGE} --build-arg DEPS_IMAGE=${DEPS_IMAGE} -f ./build/dockerfile/kuscia-anolis.Dockerfile .

.PHONY: build-monitor
build-monitor:
	docker build -t secretflow/kusica-monitor -f ./build/dockerfile/kuscia-monitor.Dockerfile .

.PHONY: integration_test
integration_test: image ## Run Integration Test
	mkdir -p run/test
	cd run && KUSCIA_IMAGE=${IMG} docker run --rm ${IMG} cat /home/kuscia/scripts/test/integration_test.sh > ./test/integration_test.sh && chmod u+x ./test/integration_test.sh
	cd run && KUSCIA_IMAGE=${IMG} ./test/integration_test.sh ${TEST_SUITE}
