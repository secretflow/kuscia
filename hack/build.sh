#!/bin/bash
#
# Copyright 2025 Ant Group Co., Ltd.
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

app_type=""

ldflags="-s -w -X github.com/secretflow/kuscia/pkg/utils/meta.KusciaVersion=$(git describe --tags --always)"

function build_kuscia() {
  echo "build kuscia binary..."
  mkdir -p build/apps/kuscia
  eval "go build -ldflags=\"$ldflags\" -o build/apps/kuscia/kuscia ./cmd/kuscia"
}

function build_transport() {
  mkdir -p build/apps/transport
  eval "go build -ldflags=\"$ldflags\" -o build/apps/transport/transport ./cmd/transport"
}

usage="$(basename "$0") [OPTIONS]
OPTIONS:
    -h          show this help text
    -t string   build app type. support kuscia, transport
"

while getopts ':ht:' option; do
  case "$option" in
  h)
    echo "$usage"
    exit
    ;;
  t)
    app_type=$OPTARG
    ;;
  :)
    printf "missing argument for -%s\n" "$OPTARG" >&2
    if [[ $OPTARG == "t" ]]; then
      printf "use default argument: kuscia\n"
      app_type="kuscia"
    else
      exit 1
    fi
    ;;
  \?)
    printf "illegal option: -%s\n" "$OPTARG" >&2
    exit 1
    ;;
  esac
done
shift $((OPTIND - 1))

base_dir=$(cd "$(dirname "$0")"/.. && pwd -P)
pushd "$base_dir"

if [[ $app_type == "" ]]; then
  echo "$usage"
  exit 1
elif [[ $app_type == "kuscia" ]]; then
  build_kuscia
elif [[ $app_type == "transport" ]]; then
  build_transport
fi

popd
