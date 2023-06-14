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

function build_exec() {
  cmd="$@"
  echo "$cmd"
  if [ "$exec_type" == "docker" ]; then
    docker exec kuscia-dev-$(whoami) bash -lc \
      "cd /home/admin/dev && $cmd"
  else
    eval $cmd
  fi
}

function build_kuscia() {
  mkdir -p build/apps/kuscia
  build_exec go build -ldflags=\"$ldflags\" -o build/apps/kuscia/kuscia ./cmd/kuscia
}

usage="$(basename "$0") [OPTIONS]
OPTIONS:
    -h          show this help text
    -w string   exec env: docker, local (default: docker)"

while getopts ':hw:' option; do
  case "$option" in
  h)
    echo "$usage"
    exit
    ;;
  w)
    exec_type=$OPTARG
    if [[ "$exec_type" != "local" && "$exec_type" != "docker" ]]; then
      printf "unknown env name: %s\n" "$exec_type" >&2
      exit 1
    fi
    ;;
  :)
    printf "missing argument for -%s\n" "$OPTARG" >&2
    exit 1
    ;;
  \?)
    printf "illegal option: -%s\n" "$OPTARG" >&2
    exit 1
    ;;
  esac
done
shift $((OPTIND - 1))

base_dir=$(cd "$(dirname "$0")"/.. && pwd -P)
pushd $base_dir

if [[ $exec_type == "docker" ]]; then
  build_exec bash hack/build.sh -w local
else # run on local
  build_kuscia
fi

