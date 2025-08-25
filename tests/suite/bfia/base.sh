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

echo "test suite env: TEST_SUITE_RUN_ROOT_DIR=${TEST_SUITE_RUN_ROOT_DIR}"
echo "test suite env: TEST_BIN_DIR=${TEST_BIN_DIR}"

TEST_SUITE_BFIA_TEST_RUN_DIR=${TEST_SUITE_RUN_ROOT_DIR}/bfia

mkdir -p "${TEST_SUITE_BFIA_TEST_RUN_DIR}"

. ./test/suite/core/functions.sh

function oneTimeSetUp() {
  start_bfia "${TEST_SUITE_BFIA_TEST_RUN_DIR}"
  sleep 10
}

function oneTimeTearDown() {
  stop_bfia
}

function test_bfia_job() {
  local job_id=job-ss-lr
  docker exec -it "${AUTONOMY_ALICE_CONTAINER}" kubectl create -f kuscia-job.yaml
  assertEquals "Kuscia job failed" "Succeeded" "$(wait_kuscia_job_until "${AUTONOMY_ALICE_CONTAINER}" 600 ${job_id})"
  assertEquals "Kuscia data file exist" "Y" "$(exist_container_file "${AUTONOMY_ALICE_CONTAINER}" var/storage/${job_id}-host-0)"
  unset job_id
}

. ./test/vendor/shunit2
