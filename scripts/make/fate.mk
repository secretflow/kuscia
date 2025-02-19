#
# Copyright 2024 Ant Group Co., Ltd.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#   https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

# Image URL to use all building image targets
DATETIME = $(shell date +"%Y%m%d%H%M%S")
COMMIT_ID = $(shell git log -1 --pretty="format:%h")
TAG = 0.0.1
DEPLOY_IMG ?= secretflow/fate-deploy-basic:${TAG}
ADAPTER_IMG ?= secretflow/fate-adapter:${TAG}

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

##@ Fate

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

.PHONY: fate-clean
fate-clean: # clean build and test product.
	rm -rf ./thirdparty/fate/build/app

.PHONY: fate-build
fate-build: ## Build kuscia binary.
fate-build: export GOARCH=amd64
fate-build: fmt vet
fate-build:
	bash ./thirdparty/fate/hack/build.sh


.PHONY: fate-adaptor-app-image
fate-adaptor-app-image: ## Build docker image with the manager.
fate-adaptor-app-image: export GOARCH=amd64
fate-adaptor-app-image: fate-build
fate-adaptor-app-image:
	docker build -t ${ADAPTER_IMG} -f ./thirdparty/fate/build/dockerfile/kuscia-job-adapter.Dockerfile ./thirdparty/fate


.PHONY: deploy-image
deploy-image:
	docker build -t ${DEPLOY_IMG} -f ./thirdparty/fate/build/dockerfile/deploy.Dockerfile ./thirdparty/fate
