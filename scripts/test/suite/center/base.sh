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

echo "test suite env: TEST_SUITE_RUN_ROOT_DIR=${TEST_SUITE_RUN_ROOT_DIR}"
echo "test suite env: TEST_BIN_DIR=${TEST_BIN_DIR}"

TEST_SUITE_CENTER_TEST_RUN_DIR=${TEST_SUITE_RUN_ROOT_DIR}/center
TEST_SUITE_RUN_KUSCIA_DIR=${TEST_SUITE_CENTER_TEST_RUN_DIR}/kusciaapi

mkdir -p "${TEST_SUITE_CENTER_TEST_RUN_DIR}"

. ./test/suite/core/functions.sh

function oneTimeSetUp() {
  start_center_mode "${TEST_SUITE_RUN_KUSCIA_DIR}"
}

# Note: This will be call twice on master, because of a bug: https://github.com/kward/shunit2/issues/112. And it is open now.
# we have a bugfix on shunit2, line number: [1370-1371]
function oneTimeTearDown() {
  stop_center_mode
}

function test_centralized_example_kuscia_job() {
  local job_id=secretflow-psi
  docker exec -it "${MASTER_CONTAINER}" scripts/user/create_example_job.sh PSI ${job_id}

  assertEquals "Kuscia job failed" "Succeeded" "$(wait_kuscia_job_until "${MASTER_CONTAINER}" 600 ${job_id})"
  assertEquals "Kuscia job output file not exist" "Y" "$(exist_container_file "${LITE_ALICE_CONTAINER}" var/storage/data/psi-output.csv)"

  unset job_id
}

function test_centralized_kuscia_api_http_available() {
  local http_status_code=$(get_kuscia_api_healthz_http_status_code "127.0.0.1:18082" "${TEST_SUITE_RUN_KUSCIA_DIR}"/master)
  assertEquals "KusciaApi healthZ http code" "200" "${http_status_code}"

  unset ipv4 http_status_code
}

function test_centralized_kuscia_api_grpc_available() {
  local status_message=$(get_kuscia_api_healthz_grpc_status_message "${TEST_BIN_DIR}"/grpcurl "127.0.0.1:18083" "${TEST_SUITE_RUN_KUSCIA_DIR}"/master)
  assertEquals "KusciaApi healthZ grpc status message" "success" "$(echo "${status_message}" | "${TEST_BIN_DIR}"/jq .status.message | sed -e 's/"//g')"

  unset ipv4 status_message
}

. ./test/vendor/shunit2