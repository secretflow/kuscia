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

TEST_SUITE_CENTER_TEST_RUN_DIR=${TEST_SUITE_RUN_ROOT_DIR}/center
TEST_SUITE_RUN_KUSCIA_DIR=${TEST_SUITE_CENTER_TEST_RUN_DIR}/kusciaapi

mkdir -p "${TEST_SUITE_CENTER_TEST_RUN_DIR}"

. ./test/suite/core/functions.sh

. ./test/suite/center/kuscia_images.sh

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
  local http_port
  http_port=$(docker inspect --format='{{(index (index .NetworkSettings.Ports "8082/tcp") 0).HostPort}}' "${MASTER_CONTAINER}")
  local http_status_code
  http_status_code=$(get_kuscia_api_healthz_http_status_code "127.0.0.1:${http_port}" "${TEST_SUITE_RUN_KUSCIA_DIR}"/master)
  assertEquals "KusciaApi healthZ http code" "200" "${http_status_code}"

  unset http_status_code
}

function test_centralized_kuscia_api_grpc_available() {
  local grpc_port
  grpc_port=$(docker inspect --format='{{(index (index .NetworkSettings.Ports "8083/tcp") 0).HostPort}}' "${MASTER_CONTAINER}")
  local status_message
  status_message=$(get_kuscia_api_healthz_grpc_status_message "${TEST_BIN_DIR}"/grpcurl "127.0.0.1:${grpc_port}" "${TEST_SUITE_RUN_KUSCIA_DIR}"/master)
  assertEquals "KusciaApi healthZ grpc status message" "success" "$(echo "${status_message}" | "${TEST_BIN_DIR}"/jq .status.message | sed -e 's/"//g')"

  unset status_message
}

# set and verify cluster domain route rolling period.
# Args:
#   ctr: container name
#   cdr_name: cluster domain route name
#   loop_count: period count to be verified
function try_centralized_token_rolling() {
  local ctr=$1
  local cdr_name=$2
  local loop_count=$3
  local period=15

  # set rolling period(s)
  set_cdr_token_rolling_period "${ctr}" "$cdr_name" $period

  # get initial token reversion
  local prev_src_revision
  prev_src_revision=$(get_cdr_src_token_revision "${ctr}" "$cdr_name")
  local prev_dst_revision
  prev_dst_revision=$(get_cdr_dst_token_revision "${ctr}" "$cdr_name")

  for ((i=1; i<loop_count; i++)); do
    # wait for period(s)
    sleep $((period+3))

    # get new token reversion
    local src_revision
    src_revision=$(get_cdr_src_token_revision "${ctr}" "$cdr_name")
    local dst_revision
    dst_revision=$(get_cdr_dst_token_revision "${ctr}" "$cdr_name")

    assertNotEquals "source token revision must change" "$src_revision" "$prev_src_revision"
    assertNotEquals "destination token revision must change" "$dst_revision" "$prev_dst_revision"

    prev_src_revision=$src_revision
    prev_dst_revision=$dst_revision
  done
}


function test_centralized_token_rolling_all() {
  # lite to lite
  try_centralized_token_rolling "${MASTER_CONTAINER}" "alice-bob" 2
  # lite to master
  try_centralized_token_rolling "${MASTER_CONTAINER}" "alice-kuscia-system" 2
  # master to lite
  # try_centralized_token_rolling "root-kuscia-master" "kuscia-system-alice"
}

function test_centralized_token_rolling_party_offline() {
  local master_ctr=${MASTER_CONTAINER}
  local bob_ctr=${LITE_BOB_CONTAINER}
  local cdr_name="alice-bob"
  local dr_name="alice-bob"
  local src_domain="alice"
  local period=30

  local ready
  ready=$(get_dr_revision_token_ready "$master_ctr" $dr_name $src_domain)
  assertEquals "true" "$ready"
  # party offline
  docker stop "$bob_ctr"
  sleep $period

  ready=$(get_dr_revision_token_ready "$master_ctr" $dr_name $src_domain)
  assertEquals "false" "$ready"

  # back online
  docker start "$bob_ctr"
  sleep $period

  ready=$(get_dr_revision_token_ready "$master_ctr" $dr_name $src_domain)
  assertEquals "true" "$ready"

  # run task
  test_centralized_example_kuscia_job
}

function test_kuscia_images() {
    test_runc_kuscia_images
    test_runp_kuscia_images
}

. ./test/vendor/shunit2
