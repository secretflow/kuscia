#
# Copyright 2025 Ant Group Co., Ltd.
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

# ========================================== common.mk ===============================================
# All make targets related to common variables are defined in this file.
# ========================================== common.mk ===============================================

# Turn off .INTERMEDIATE file removal by marking all files as
# .SECONDARY.  .INTERMEDIATE file removal is a space-saving hack from
# a time when drives were small; on modern computers with plenty of
# storage, it causes nothing but headaches.
#
# https://news.ycombinator.com/item?id=16486331
.SECONDARY:

SHELL:=/bin/bash

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

# ====================================================================================================
# ROOT Options:
# ====================================================================================================

# Kuscia version
DATETIME = $(shell date +"%Y%m%d%H%M%S")
KUSCIA_VERSION_TAG = $(shell git describe --tags --always)

# Architecture settings. e.g. make xxx ARCH=amd123
ifeq ($(origin ARCH), undefined)
# Get current architecture information
UNAME_M_OUTPUT := $(shell uname -m)
# To configure the ARCH variable to either arm64 or amd64 or UNAME_M_OUTPUT
ARCH = $(if $(filter aarch64 arm64,$(UNAME_M_OUTPUT)),arm64,$(if $(filter amd64 x86_64,$(UNAME_M_OUTPUT)),amd64,$(UNAME_M_OUTPUT)))
endif

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Set GOOS
export GOOS=linux

# Log the running target
LOG_TARGET = echo -e "\033[0;32m==================> Running $@ ============> ... \033[0m"
# Log debugging info
define log
echo -e "\033[36m==================>$1\033[0m"
endef
# Log error info
define errorLog
echo -e "\033[0;31m==================>$1\033[0m"
endef

.PHONY: manifests
manifests: # Generate CustomResourceDefinition objects.
	@$(LOG_TARGET)
	bash hack/generate-crds.sh


.PHONY: gen-clientset
gen-clientset:  # Generate CRD runtime.Object and Clientset\Informer|Listers.
	@$(LOG_TARGET)
	bash hack/update-codegen.sh


.PHONY: gen-proto-code
gen-proto-code:  # Generate protobuf golang code that Kuscia needs.
	@$(LOG_TARGET)
	bash hack/proto-to-go.sh


##@ Common

.PHONY: generate
generate: ## Generate all code that Kuscia needs.
generate: manifests gen-clientset gen-proto-code


## help: Show this help info.
.PHONY: help
help:
	@echo -e "\033[1;3;34mKuscia (Kubernetes-based Secure Collaborative Infra) is a lightweight privacy computing task orchestration framework based on K3s.\033[0m\n"
	@echo -e "Usage:\n  make \033[36m<Target>\033[0m \033[36m<Option>\033[0m\n\nTargets:"
	@awk 'BEGIN {FS = ":.*##"; printf ""} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
