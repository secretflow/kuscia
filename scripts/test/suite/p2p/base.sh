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

TEST_SUITE_P2P_TEST_RUN_DIR=${TEST_SUITE_RUN_ROOT_DIR}/p2p
TEST_SUITE_P2P_TEST_RUN_KUSCIA_DIR=${TEST_SUITE_P2P_TEST_RUN_DIR}/kusciaapi

mkdir -p "${TEST_SUITE_P2P_TEST_RUN_DIR}"

. ./test/suite/core/functions.sh

. ./test/suite/p2p/kuscia_images.sh

function oneTimeSetUp() {
  start_p2p_mode "${TEST_SUITE_P2P_TEST_RUN_KUSCIA_DIR}"
}

# Note: This will be call twice on master, because of a bug: https://github.com/kward/shunit2/issues/112. And it is open now.
# we have a bugfix on shunit2, line number: [1370-1371]
function oneTimeTearDown() {
  stop_p2p_mode
}

function test_p2p_kuscia_job() {
  local alice_job_id=secretflow-psi-p2p-alice
  docker exec -it "${AUTONOMY_ALICE_CONTAINER}" scripts/user/create_example_job.sh PSI ${alice_job_id}
  assertEquals "Kuscia job failed" "Succeeded" "$(wait_kuscia_job_until "${AUTONOMY_ALICE_CONTAINER}" 600 ${alice_job_id})"
  assertEquals "Kuscia data file exist" "Y" "$(exist_container_file "${AUTONOMY_ALICE_CONTAINER}" var/storage/data/psi-output.csv)"

  unset alice_job_id
}

function test_p2p_kuscia_api_http_available() {
  local alice_http_port
  local alice_http_status_code
  local autonomy_bob_container_ip
  local bob_http_port
  local bob_http_status_code

  alice_http_port=$(docker inspect --format='{{(index (index .NetworkSettings.Ports "8082/tcp") 0).HostPort}}' "${AUTONOMY_ALICE_CONTAINER}")
  alice_http_status_code=$(get_kuscia_api_healthz_http_status_code "127.0.0.1:${alice_http_port}" "${TEST_SUITE_P2P_TEST_RUN_KUSCIA_DIR}"/alice)
  assertEquals "KusciaApi healthZ http code" "200" "${alice_http_status_code}"

  unset alice_http_status_code

  autonomy_bob_container_ip=$(get_container_ip "${AUTONOMY_BOB_CONTAINER}")
  bob_http_port=$(docker inspect --format='{{(index (index .NetworkSettings.Ports "8082/tcp") 0).HostPort}}' "${AUTONOMY_BOB_CONTAINER}")
  bob_http_status_code=$(get_kuscia_api_healthz_http_status_code "127.0.0.1:${bob_http_port}" "${TEST_SUITE_P2P_TEST_RUN_KUSCIA_DIR}"/bob)
  assertEquals "KusciaApi healthZ http code" "200" "${bob_http_status_code}"

  unset bob_http_status_code

  unset ipv4
}

function test_p2p_kuscia_api_grpc_available() {
  local alice_grpc_port
  local alice_status_message
  local autonomy_bob_container_ip
  local bob_grpc_port
  local bob_status_message
  alice_grpc_port=$(docker inspect --format='{{(index (index .NetworkSettings.Ports "8083/tcp") 0).HostPort}}' "${AUTONOMY_ALICE_CONTAINER}")
  alice_status_message=$(get_kuscia_api_healthz_grpc_status_message "${TEST_BIN_DIR}"/grpcurl "127.0.0.1:${alice_grpc_port}" "${TEST_SUITE_P2P_TEST_RUN_KUSCIA_DIR}"/alice)
  assertEquals "KusciaApi healthZ grpc status message" "success" "$(echo "${alice_status_message}" | "${TEST_BIN_DIR}"/jq .status.message | sed -e 's/"//g')"

  unset alice_status_message

  autonomy_bob_container_ip=$(get_container_ip "${AUTONOMY_BOB_CONTAINER}")
  bob_grpc_port=$(docker inspect --format='{{(index (index .NetworkSettings.Ports "8083/tcp") 0).HostPort}}' "${AUTONOMY_BOB_CONTAINER}")
  bob_status_message=$(get_kuscia_api_healthz_grpc_status_message "${TEST_BIN_DIR}"/grpcurl "127.0.0.1:${bob_grpc_port}" "${TEST_SUITE_P2P_TEST_RUN_KUSCIA_DIR}"/bob)
  assertEquals "KusciaApi healthZ grpc status message" "success" "$(echo "${bob_status_message}" | "${TEST_BIN_DIR}"/jq .status.message | sed -e 's/"//g')"

  unset bob_status_message

  unset ipv4
}

# set and verify cluster domain route rolling period.
# Args:
#   ctr: container name
#   cdr_name: cluster domain route name
#   loop_count: period count to be verified
function try_p2p_token_rolling() {
  local src_ctr=$1
  local dst_ctr=$2
  local cdr_name=$3
  local loop_count=$4
  local period=15

  # set rolling period(s)
  set_cdr_token_rolling_period "${src_ctr}" "${cdr_name}" "${period}"

  local prev_src_revision
  prev_src_revision=$(get_cdr_src_token_revision "${src_ctr}" "${cdr_name}")
  local prev_dst_revision
  prev_dst_revision=$(get_cdr_dst_token_revision "${dst_ctr}" "${cdr_name}")

  for ((i = 1; i < loop_count; i++)); do
    # wait for period(s)
    sleep $((period+3))

    # get new token reversion
    local src_revision
    src_revision=$(get_cdr_src_token_revision "${src_ctr}" "${cdr_name}")
    local dst_revision
    dst_revision=$(get_cdr_dst_token_revision "${dst_ctr}" "${cdr_name}")

    assertNotEquals "source token revision must change" "${src_revision}" "${prev_src_revision}"
    assertNotEquals "destination token revision must change" "${dst_revision}" "${prev_dst_revision}"

    prev_src_revision=$src_revision
    prev_dst_revision=$dst_revision
  done
}

function test_p2p_token_rolling_all() {
  try_p2p_token_rolling "${AUTONOMY_ALICE_CONTAINER}" "${AUTONOMY_BOB_CONTAINER}" "alice-bob" 2
  try_p2p_token_rolling "${AUTONOMY_BOB_CONTAINER}" "${AUTONOMY_ALICE_CONTAINER}" "bob-alice" 2
}

function test_p2p_token_rolling_party_offline() {
  local alice_ctr="${AUTONOMY_ALICE_CONTAINER}"
  local bob_ctr="${AUTONOMY_BOB_CONTAINER}"
  local cdr_name="alice-bob"
  local dr_name="alice-bob"
  local src_domain="alice"

  local ready
  ready=$(get_dr_revision_token_ready "${alice_ctr}" "${dr_name}" "${src_domain}")
  assertEquals "true" "${ready}"
  # party offline
  docker stop "${bob_ctr}"

  for i in {1..30}; do
    local ready
    ready=$(get_dr_revision_token_ready "${alice_ctr}" "${dr_name}" "${src_domain}")
    if [[ "${ready}" == "false" ]]; then
      break
    fi
    sleep 2
  done
  assertEquals "false" "${ready}"

  # back online
  docker start "${bob_ctr}"

  for i in {1..30}; do
    local ready
    ready=$(get_dr_revision_token_ready "${alice_ctr}" "${dr_name}" "${src_domain}")
    if [[ "${ready}" == "true" ]]; then
      break
    fi
    sleep 2
  done
  assertEquals "true" "${ready}"

  # run task
  test_p2p_kuscia_job
}

function test_p2p_token_rolling_auth_removal() {
  local alice_ctr="${AUTONOMY_ALICE_CONTAINER}"
  local bob_ctr="${AUTONOMY_BOB_CONTAINER}"
  local cdr_name="alice-bob"
  local dr_name="alice-bob"
  local dst_domain="bob"
  local src_domain="alice"

  local ready
  ready=$(get_dr_revision_token_ready "${alice_ctr}" "${dr_name}" "${src_domain}")
  assertEquals "true" "${ready}"

  # save dr
  docker exec "$bob_ctr" kubectl get cdr $cdr_name -o json | "${TEST_BIN_DIR}"/jq 'del(.status)' > tmp.json
  # dr removal
  docker exec "$bob_ctr" kubectl delete cdr $cdr_name

  for i in {1..30}; do
    local ready
    ready=$(get_dr_revision_token_ready "${alice_ctr}" "${dr_name}" "${src_domain}")
    if [[ "${ready}" == "false" ]]; then
      break
    fi
    sleep 2
  done
  assertEquals "false" "${ready}"

  # dr restore
  docker cp tmp.json "${bob_ctr}":/home/kuscia/
  docker exec "${bob_ctr}" kubectl create -f /home/kuscia/tmp.json

  for i in {1..30}; do
    local ready
    ready=$(get_dr_revision_token_ready "${alice_ctr}" "${dr_name}" "${src_domain}")
    if [[ "${ready}" == "true" ]]; then
      break
    fi
    sleep 2
  done
  assertEquals "true" "${ready}"

  # run task
  test_p2p_kuscia_job
}

function test_p2p_token_rolling_cert_misconfig() {
  local alice_ctr="${AUTONOMY_ALICE_CONTAINER}"
  local bob_ctr="${AUTONOMY_BOB_CONTAINER}"
  local dr_name="alice-bob"
  local cdr_name="alice-bob"
  local dst_domain="bob"
  local src_domain="alice"

  local dst_cert
  local mis_cert
  dst_cert=$(docker exec "${alice_ctr}" kubectl get domain "${dst_domain}" -o jsonpath='{.spec.cert}')
  mis_cert=$(docker exec "${alice_ctr}" kubectl get domain "${src_domain}" -o jsonpath='{.spec.cert}') # use alice domain cert as misconfigured cert
  # cert mis config
  docker exec "${alice_ctr}" kubectl patch domain "${dst_domain}" --type json -p="[{\"op\": \"replace\", \"path\": \"/spec/cert\", \"value\": ${mis_cert}}]"

  for i in {1..30}; do
    local ready
    ready=$(get_dr_revision_token_ready "${alice_ctr}" "${dr_name}" "${src_domain}")
    if [[ "${ready}" == "false" ]]; then
      break
    fi
    sleep 2
  done
  assertEquals "false" "${ready}"

  # cert restore
  docker exec "${alice_ctr}" kubectl patch domain "${dst_domain}" --type json -p="[{\"op\": \"replace\", \"path\": \"/spec/cert\", \"value\": ${dst_cert}}]"

  for i in {1..30}; do
    local ready
    ready=$(get_dr_revision_token_ready "${alice_ctr}" "${dr_name}" "${src_domain}")
    if [[ "${ready}" == "true" ]]; then
      break
    fi
    sleep 2
  done
  assertEquals "true" "${ready}"

  # run task
  test_p2p_kuscia_job
}

function test_kuscia_images() {
    test_runc_kuscia_images
    test_runp_kuscia_images
}

. ./test/vendor/shunit2
