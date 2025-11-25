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

# ========================================== image.mk ===============================================
# All make targets related to image should be defined here.
# ========================================== image.mk ===============================================

TAG = ${KUSCIA_VERSION_TAG}-${DATETIME}
IMG := secretflow/kuscia:${TAG}

# start_docker_buildx: check if the builder named kuscia exists, if not create it.
define start_docker_buildx
  if [[ ! -n $$(docker buildx inspect kuscia) ]]; then\
	echo "create kuscia builder";\
	docker buildx create --name kuscia --platform linux/arm64,linux/amd64;\
  fi;
  docker buildx use kuscia;
endef

PROOT_IMAGE ?= secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/proot
ENVOY_IMAGE ?= secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia-envoy:0.6.2b0
DEPS_IMAGE ?= secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia-deps:0.7.0b0

##@ Image

.PHONY: proot
proot: export GOARCH=${ARCH}# export ARCH=amd64 or export ARCH=arm64; if not specified, ARCH will auto-detected
proot:
	DOCKER_BUILDKIT=1
	@$(call start_docker_buildx)
	docker buildx build -t ${PROOT_IMAGE} -f ./build/dockerfile/proot-build.Dockerfile . --platform linux/${ARCH} --load


.PHONY: deps-image
deps-image:
	DOCKER_BUILDKIT=1
	@$(call start_docker_buildx)
	docker buildx build -t ${DEPS_IMAGE} -f ./build/dockerfile/base/kuscia-deps.Dockerfile . --platform linux/${ARCH} --load


.PHONY: image
image: export GOARCH=${ARCH}# export ARCH=amd64 or export ARCH=arm64; if not specified, ARCH will auto-detected
image: ## Build docker image with the manager.
image: build
image:
	DOCKER_BUILDKIT=1
	@$(call start_docker_buildx)
	docker buildx build -t ${IMG} --build-arg KUSCIA_ENVOY_IMAGE=${ENVOY_IMAGE} --build-arg DEPS_IMAGE=${DEPS_IMAGE} -f ./build/dockerfile/kuscia-anolis.Dockerfile . --platform linux/${ARCH} --load

.PHONY: build-monitor
build-monitor:
	docker build -t secretflow/kuscia-monitor -f ./build/dockerfile/kuscia-monitor.Dockerfile .





