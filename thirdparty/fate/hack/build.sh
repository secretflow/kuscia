#!/bin/bash
#
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
#

set -e

exec_type=local
ldflags="-X github.com/secretflow/kuscia/pkg/utils/meta.KusciaVersion=$(git describe --always)"

function build_kuscia() {
  mkdir -p ./build/apps
  go build -ldflags="$ldflags" -o ./build/apps/fate ./cmd
}

base_dir=$(cd "$(dirname "$0")"/.. && pwd -P)
pushd $base_dir

build_kuscia

