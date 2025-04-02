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

SELECTED_TEST_SUITE=${1}

# TEST_SUITES hold all test suites.
TEST_SUITES="center.base p2p.base center.example"
WSL_IGNORE_SUITES="center.nsjail p2p.nsjail"
center_base="./test/suite/center/base.sh"
p2p_base="./test/suite/p2p/base.sh"
center_nsjail="./test/suite/center/nsjail.sh"
p2p_nsjail="./test/suite/p2p/nsjail.sh"
center_example="./test/suite/center/example.sh"

TEST_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
TEST_BIN_DIR=${TEST_ROOT}/test_run/bin
TEST_RUN_ROOT_DIR=${TEST_ROOT}/test_run
IS_WSL=false

if [ "$WSL_DISTRO_NAME" ]; then
  IS_WSL=true
fi
echo "detect environment: is_wsl=${IS_WSL}"

# Download grpcurl
#
# Args:
#   output_dir: output dir
function download_grpcurl() {
  local output_dir=$1
  mkdir -p "${output_dir}"
  # if installed on output, do nothing
  if [ -e "${output_dir}/grpcurl" ]; then
    return
  fi
  # if installed on host, just make a link
  if [ "$(which grpcurl)" != "" ]; then
    ln -s "$(which grpcurl)" "${output_dir}"/grpcurl
    return
  fi
  local package_url
  case $(uname -s -m) in
  "Linux x86_64" )
    package_url="https://secretflow-data.oss-cn-shanghai.aliyuncs.com/package/grpcurl_1.8.8_linux_x86_64.tar.gz"
    ;;
  "Darwin x86_64" )
    package_url="https://secretflow-data.oss-cn-shanghai.aliyuncs.com/package/grpcurl_1.8.8_osx_x86_64.tar.gz"
    ;;
  "Linux aarch64" )
    package_url="https://secretflow-data.oss-cn-shanghai.aliyuncs.com/package/grpcurl_1.8.8_linux_arm64.tar.gz"
    ;;
  "Darwin arm64" )
    package_url="https://secretflow-data.oss-cn-shanghai.aliyuncs.com/package/grpcurl_1.8.8_osx_arm64.tar.gz"
    ;;
  *)
    echo "Unsupported OS"
    exit 1
    ;;
  esac
  wget -O "${output_dir}"/grpcurl.tar.gz ${package_url}
  tar -zxf "${output_dir}"/grpcurl.tar.gz -C "${output_dir}"
}

# Download jq
#
# Args:
#   output_dir: output_dir
function download_jq() {
  local output_dir=$1
  mkdir -p "${output_dir}"
  # if installed on output, do nothing
  if [ -e "${output_dir}/jq" ]; then
    return
  fi
  # if installed on host, just make a link
  if [ "$(which jq)" != "" ]; then
    ln -s "$(which jq)" "${output_dir}"/jq
     return
  fi
  case $(uname -s -m) in
  "Linux x86_64" )
    package_url="https://secretflow-data.oss-cn-shanghai.aliyuncs.com/package/jq-linux64"
    ;;
  "Darwin x86_64" )
    package_url="https://secretflow-data.oss-cn-shanghai.aliyuncs.com/package/jq-osx-amd64"
    ;;
  "Linux aarch64" )
    package_url="https://secretflow-data.oss-cn-shanghai.aliyuncs.com/package/jq-linux64-arm64"
    ;;
  "Darwin arm64" )
    package_url="https://secretflow-data.oss-cn-shanghai.aliyuncs.com/package/jq-macos-arm64"
    ;;
  *)
    echo "Unsupported OS"
    exit 1
    ;;
  esac
  wget -O "${output_dir}"/jq ${package_url}
  chmod a+x "${output_dir}"/jq
}

function installRequires() {
  download_grpcurl "${TEST_BIN_DIR}"
  download_jq "${TEST_BIN_DIR}"
}

function copyTestScripts() {
  docker run --name "${USER}"-kuscia-integration-test --entrypoint="" -d "${KUSCIA_IMAGE}" bash
  docker cp "${USER}"-kuscia-integration-test:/home/kuscia/scripts/test/suite ./test
  docker cp "${USER}"-kuscia-integration-test:/home/kuscia/scripts/test/vendor ./test
  docker stop "${USER}"-kuscia-integration-test
  docker rm -v "${USER}"-kuscia-integration-test
}


if [ "${SELECTED_TEST_SUITE}" == "" ]; then
  SELECTED_TEST_SUITE="all"
fi
if [ "${SELECTED_TEST_SUITE}" != "all" ] ; then
  case "${TEST_SUITES}" in
    *"${SELECTED_TEST_SUITE}"*) ;;
    *) echo "can't find test suite: ${SELECTED_TEST_SUITE}" && exit 1;;
  esac
fi

installRequires
copyTestScripts

docker run --rm "${KUSCIA_IMAGE}" cat /home/kuscia/scripts/deploy/kuscia.sh > kuscia.sh && chmod u+x kuscia.sh

if [ "${SELECTED_TEST_SUITE}" == "all" ]; then
  for suite in ${TEST_SUITES}; do
    if [[ "${IS_WSL}" == true && "${WSL_IGNORE_SUITES}" =~ ${suite} ]]; then
      echo "in wsl env: $suite will be skip"
      continue
    fi
    test_suite_run_root_dir="${TEST_RUN_ROOT_DIR}"/"${suite}"
    mkdir -p "${test_suite_run_root_dir}"
    suite_for_path=${suite//./_}
    TEST_SUITE_RUN_ROOT_DIR="${test_suite_run_root_dir}" TEST_BIN_DIR=${TEST_BIN_DIR} ${!suite_for_path}
    rm -rf "${test_suite_run_root_dir}"
  done
else
  test_suite_run_root_dir="${TEST_RUN_ROOT_DIR}"/"${SELECTED_TEST_SUITE}"
  mkdir -p "${test_suite_run_root_dir}"
  SELECTED_TEST_SUITE_FOR_PATH=${SELECTED_TEST_SUITE//./_}
  TEST_SUITE_RUN_ROOT_DIR="${test_suite_run_root_dir}" TEST_BIN_DIR=${TEST_BIN_DIR} ${!SELECTED_TEST_SUITE_FOR_PATH}
  rm -rf "${test_suite_run_root_dir}"
fi
