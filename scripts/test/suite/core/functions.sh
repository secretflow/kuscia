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
# export variable
MASTER_CONTAINER=${USER}-kuscia-master
LITE_ALICE_CONTAINER=${USER}-kuscia-lite-alice
LITE_BOB_CONTAINER=${USER}-kuscia-lite-bob
AUTONOMY_ALICE_CONTAINER=${USER}-kuscia-autonomy-alice
AUTONOMY_BOB_CONTAINER=${USER}-kuscia-autonomy-bob

TIMEOUT_DURATION_SECONDS=10

# Wait kuscia job until succeeded, failed, timeout or unexpected state. watch duration is 10s.
#
# Args:
#   ctr: which container.
#   timeout_seconds: watch timeout.
#   job_id: which job that you want to watch.
# Return:
#   state: string
function wait_kuscia_job_until() {
  local ctr=$1
  local timeout_seconds=$2
  local job_id=$3
  local times=$(("${timeout_seconds}" / "${TIMEOUT_DURATION_SECONDS}"))
  local current=0
  while [ "${current}" -lt "${times}" ]; do
    local job_phase=$(docker exec -it "${ctr}" kubectl get kj "${job_id}" -o custom-columns=PHASE:.status.phase | sed -n '2p' | tr -d '\n' | tr -d '\r')
    case "${job_phase}" in
    Succeeded)
      echo "Succeeded"
      return
      ;;
    Failed)
      echo "Failed"
      return
      ;;
    Pending | Running | "" )
      ;;
    *)
      # unexpected
      echo "unexpected: ${job_phase}"
      return 100
      ;;
    esac
    ((current++))
    sleep "${TIMEOUT_DURATION_SECONDS}"
  done
  echo "Timeout"
}

# Get kuscia api http code on http for healrhZ
#
# Args:
#   ip: the kuscia api ip.
#   kuscia_cert_dir: cert and key location.
# Return:
#   http_code: string
function get_kuscia_api_healthz_http_status_code() {
  local ip=$1
  local kuscia_cert_dir=$2

  curl --cert "${kuscia_cert_dir}"/kusciaapi-client.crt --key "${kuscia_cert_dir}"/kusciaapi-client.key \
    --cacert "${kuscia_cert_dir}"/ca.crt -X POST "https://${ip}:8082/api/v1/domain/query" --header "Token: $(cat "${kuscia_cert_dir}"/token)" \
    --header 'Content-Type: application/json' -s -o /dev/null --write-out '%{http_code}' -d '{"domain_id": "alice"}'
}

# Get kuscia api grpc status message on http for healrhZ
#
# Args:
#   grpcurl_path: grpcurl bin path.
#   ip: the kuscia api ip.
#   kuscia_cert_dir: cert and key location.
# Return:
#   status_message: string
function get_kuscia_api_healthz_grpc_status_message() {
  local bin=$1
  local ip=$2
  local kuscia_cert_dir=$3

  ${bin} --cert "${kuscia_cert_dir}"/kusciaapi-client.crt --key "${kuscia_cert_dir}"/kusciaapi-client.key \
    --cacert "${kuscia_cert_dir}"/ca.crt -H "Token: $(cat "${kuscia_cert_dir}"/token)" -d '{}' \
    "${ip}":8083 kuscia.proto.api.v1alpha1.kusciaapi.HealthService.healthZ
}

# Get container state, the state is from jsonpath=.state
#
# Args:
#   container_name: watch container.
# Return:
#   state: string
function get_container_state() {
  local container_name=$1
  docker container ls --filter name="${container_name}" --format='{{json .State}}' | sed -e 's/"//g'
}

# Exist container file
#
# Args:
#   ctr: which container.
#   file_path: file location path.
# Return:
#   exist: enum string, [Y,N]
function exist_container_file() {
  local ctr=$1
  local file_path=$2
  docker exec -it "${ctr}" test -e "${file_path}" && echo "Y" || echo "N"
}

# Get container ip
#
# Args:
#   ctr: which container.
# Return:
#   ip: for example, 172.18.0.2
function get_container_ip() {
  local ctr=$1
  docker inspect "${ctr}" --format '{{ (index .NetworkSettings.Networks "kuscia-exchange").IPAddress }}'
}

# Start center mode
#
# Args:
#   test_suite_run_kuscia_dir: location to save kuscia api resource
function start_center_mode() {
  local test_suite_run_kuscia_dir=$1
  mkdir -p "${test_suite_run_kuscia_dir}"

  # Run as Center
  ./start_standalone.sh center

    # Check centralized container Up
  local master_container_state=$(get_container_state "${MASTER_CONTAINER}")
  assertEquals "Container ${MASTER_CONTAINER} not running}" running "${master_container_state}"
  local lite_alice_container_state=$(get_container_state "${LITE_ALICE_CONTAINER}")
  assertEquals "Container ${LITE_ALICE_CONTAINER} not running}" running "${lite_alice_container_state}"
  local lite_bob_container_state=$(get_container_state "${LITE_BOB_CONTAINER}")
  assertEquals "Container ${LITE_BOB_CONTAINER} not running}" running "${lite_bob_container_state}"

  # get kuscia api resource
  mkdir -p "${test_suite_run_kuscia_dir}"/master
  docker cp "${MASTER_CONTAINER}":/home/kuscia/etc/certs/kusciaapi-client.key "${test_suite_run_kuscia_dir}"/master
  docker cp "${MASTER_CONTAINER}":/home/kuscia/etc/certs/kusciaapi-client.crt "${test_suite_run_kuscia_dir}"/master
  docker cp "${MASTER_CONTAINER}":/home/kuscia/etc/certs/ca.crt "${test_suite_run_kuscia_dir}"/master
  docker cp "${MASTER_CONTAINER}":/home/kuscia/etc/certs/token "${test_suite_run_kuscia_dir}"/master

  unset test_suite_run_kuscia_dir master_container_state lite_alice_container_state lite_bob_container_state
}

# Stop center mode
function stop_center_mode() {
  docker stop "${MASTER_CONTAINER}" "${LITE_ALICE_CONTAINER}" "${LITE_BOB_CONTAINER}"
  docker rm "${MASTER_CONTAINER}" "${LITE_ALICE_CONTAINER}" "${LITE_BOB_CONTAINER}"
}

# Start p2p mode
#
# Args:
#   test_suite_run_kuscia_dir: location to save kuscia api resource
function start_p2p_mode() {
  local test_suite_run_kuscia_dir=$1
  mkdir -p "${test_suite_run_kuscia_dir}"

  # Run as P2P
  ./start_standalone.sh p2p

  # Check p2p container Up
  local autonomy_alice_container_state=$(get_container_state "${AUTONOMY_ALICE_CONTAINER}")
  assertEquals "Container ${AUTONOMY_ALICE_CONTAINER} not running}" running "${autonomy_alice_container_state}"
  local autonomy_bob_container_state=$(get_container_state "${AUTONOMY_BOB_CONTAINER}")
  assertEquals "Container ${AUTONOMY_BOB_CONTAINER} not running}" running "${autonomy_bob_container_state}"

  # get kuscia api resource
  mkdir -p "${test_suite_run_kuscia_dir}"
  mkdir -p "${test_suite_run_kuscia_dir}"/alice
  mkdir -p "${test_suite_run_kuscia_dir}"/bob
  docker cp "${AUTONOMY_ALICE_CONTAINER}":/home/kuscia/etc/certs/kusciaapi-client.key "${test_suite_run_kuscia_dir}"/alice
  docker cp "${AUTONOMY_ALICE_CONTAINER}":/home/kuscia/etc/certs/kusciaapi-client.crt "${test_suite_run_kuscia_dir}"/alice
  docker cp "${AUTONOMY_ALICE_CONTAINER}":/home/kuscia/etc/certs/ca.crt "${test_suite_run_kuscia_dir}"/alice
  docker cp "${AUTONOMY_ALICE_CONTAINER}":/home/kuscia/etc/certs/token "${test_suite_run_kuscia_dir}"/alice
  docker cp "${AUTONOMY_BOB_CONTAINER}":/home/kuscia/etc/certs/kusciaapi-client.key "${test_suite_run_kuscia_dir}"/bob
  docker cp "${AUTONOMY_BOB_CONTAINER}":/home/kuscia/etc/certs/kusciaapi-client.crt "${test_suite_run_kuscia_dir}"/bob
  docker cp "${AUTONOMY_BOB_CONTAINER}":/home/kuscia/etc/certs/ca.crt "${test_suite_run_kuscia_dir}"/bob
  docker cp "${AUTONOMY_BOB_CONTAINER}":/home/kuscia/etc/certs/token "${test_suite_run_kuscia_dir}"/bob

  unset test_suite_run_kuscia_dir autonomy_alice_container_state autonomy_bob_container_state
}

# Stop p2p mode
function stop_p2p_mode() {
  docker stop "${AUTONOMY_ALICE_CONTAINER}" "${AUTONOMY_BOB_CONTAINER}"
  docker rm "${AUTONOMY_ALICE_CONTAINER}" "${AUTONOMY_BOB_CONTAINER}"
}