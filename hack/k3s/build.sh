#!/bin/bash

# Copyright 2023 Ant Group Co., Ltd.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -ex

k3s_version="v1.26.11+k3s2"

function build_k3s() {
  echo "build k3s binary..."
  if [[ $(ls -A "build/k3s") = "" ]];then
    git clone -b $k3s_version --depth 1 https://github.com/k3s-io/k3s.git build/k3s
    KINE_VERSION="github.com/k3s-io/kine@v0.11.3"
    pushd build/k3s
    go mod edit -replace github.com/k3s-io/kine="${KINE_VERSION}"
    pushd
  fi

  cp -rf hack/k3s/build-kuscia-k3s-binary.sh build/k3s/scripts
  chmod +x build/k3s/scripts/build-kuscia-k3s-binary.sh
  cp -rf hack/k3s/Makefile.rebuild_k3s build/k3s/Makefile

  pushd build/k3s
  go mod tidy
  go mod vendor
  make build-kuscia-k3s
  popd
}

build_k3s