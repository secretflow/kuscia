#!/bin/bash
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

# Exported Variable
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
  local times=$((timeout_seconds / TIMEOUT_DURATION_SECONDS))
  local current=0
  while [ "${current}" -lt "${times}" ]; do
    local job_phase
    job_phase=$(docker exec -it "${ctr}" kubectl get kj -n cross-domain "${job_id}" -o custom-columns=PHASE:.status.phase | sed -n '2p' | tr -d '\n' | tr -d '\r')
    case "${job_phase}" in
    Succeeded)
      echo "Succeeded"
      unset ctr timeout_seconds job_id times  current
      return
      ;;
    Failed)
      echo "Failed"
      unset ctr timeout_seconds job_id times  current
      return
      ;;
    Pending | Running | AwaitingApproval | "" )
      ;;
    *)
      # unexpected
      echo "unexpected: ${job_phase}"
      unset ctr timeout_seconds job_id times  current
      return 100
      ;;
    esac
    ((current++))
    sleep "${TIMEOUT_DURATION_SECONDS}"
  done
  echo "Timeout"
  unset ctr timeout_seconds job_id times  current
}

# Get kuscia api http code on http for healrhZ
#
# Args:
#   addr: the kuscia api ip.
#   kuscia_cert_dir: cert and key location.
# Return:
#   http_code: string
function get_kuscia_api_healthz_http_status_code() {
  local addr=$1
  local kuscia_cert_dir=$2

  curl -k --cert "${kuscia_cert_dir}"/kusciaapi-client.crt --key "${kuscia_cert_dir}"/kusciaapi-client.key \
    --cacert "${kuscia_cert_dir}"/ca.crt -X POST "https://${addr}/healthZ" --header "Token: $(cat "${kuscia_cert_dir}"/token)" \
    --header 'Content-Type: application/json' -s -o /dev/null --write-out '%{http_code}' -d '{}'

  unset addr kuscia_cert_dir
}

# Get kuscia api grpc status message on http for healrhZ
#
# Args:
#   grpcurl_path: grpcurl bin path.
#   addr: the kuscia api ip.
#   kuscia_cert_dir: cert and key location.
# Return:
#   status_message: string
function get_kuscia_api_healthz_grpc_status_message() {
  local bin=$1
  local addr=$2
  local kuscia_cert_dir=$3

  ${bin} -insecure --cert "${kuscia_cert_dir}"/kusciaapi-client.crt --key "${kuscia_cert_dir}"/kusciaapi-client.key \
    --cacert "${kuscia_cert_dir}"/ca.crt -H "Token: $(cat "${kuscia_cert_dir}"/token)" -d '{}' \
    "${addr}" kuscia.proto.api.v1alpha1.kusciaapi.HealthService.healthZ

  unset bin addr kuscia_cert_dir
}

# Get container state, the state is from jsonpath=.state
#
# Args:
#   container_name: watch container.
# Return:
#   state: string
function get_container_state() {
  local container_name=$1
  docker inspect "${container_name}" --format='{{ .State.Status }}' | sed -e 's/"//g'

  unset container_name
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

  unset ctr file_path
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

  unset ctr
}

# Start center mode
#
# Args:
#   test_suite_run_kuscia_dir: location to save kuscia api resource
function start_center_mode() {
  local test_suite_run_kuscia_dir=$1
  local master_container_state
  local lite_alice_container_state
  local lite_bob_container_state
  mkdir -p "${test_suite_run_kuscia_dir}"

  # Run as Center
  ./kuscia.sh center --expose-ports

  # Check centralized container Up
  master_container_state=$(get_container_state "${MASTER_CONTAINER}")
  assertEquals "Container ${MASTER_CONTAINER} not running}" running "${master_container_state}"
  lite_alice_container_state=$(get_container_state "${LITE_ALICE_CONTAINER}")
  assertEquals "Container ${LITE_ALICE_CONTAINER} not running}" running "${lite_alice_container_state}"
  lite_bob_container_state=$(get_container_state "${LITE_BOB_CONTAINER}")
  assertEquals "Container ${LITE_BOB_CONTAINER} not running}" running "${lite_bob_container_state}"

  # get kuscia api resource
  mkdir -p "${test_suite_run_kuscia_dir}"/master
  ## generate client certs
  docker exec -it "${MASTER_CONTAINER}" sh scripts/deploy/init_kusciaapi_client_certs.sh
  docker cp "${MASTER_CONTAINER}":/home/kuscia/var/certs/kusciaapi-client.key "${test_suite_run_kuscia_dir}"/master
  docker cp "${MASTER_CONTAINER}":/home/kuscia/var/certs/kusciaapi-client.crt "${test_suite_run_kuscia_dir}"/master
  docker cp "${MASTER_CONTAINER}":/home/kuscia/var/certs/ca.crt "${test_suite_run_kuscia_dir}"/master
  docker cp "${MASTER_CONTAINER}":/home/kuscia/var/certs/token "${test_suite_run_kuscia_dir}"/master

  unset test_suite_run_kuscia_dir master_container_state lite_alice_container_state lite_bob_container_state
}

# Stop center mode
function stop_center_mode() {
  docker stop "${MASTER_CONTAINER}" "${LITE_ALICE_CONTAINER}" "${LITE_BOB_CONTAINER}"
  docker rm "${MASTER_CONTAINER}" "${LITE_ALICE_CONTAINER}" "${LITE_BOB_CONTAINER}"
  docker volume rm "${LITE_ALICE_CONTAINER}-containerd" "${LITE_BOB_CONTAINER}-containerd"
}

# Start p2p mode
#
# Args:
#   test_suite_run_kuscia_dir: location to save kuscia api resource
function start_p2p_mode() {
  local test_suite_run_kuscia_dir=$1
  local autonomy_alice_container_state
  local autonomy_bob_container_state
  mkdir -p "${test_suite_run_kuscia_dir}"

  # Run as P2P
  ./kuscia.sh p2p --expose-ports

  # Check p2p container Up
  autonomy_alice_container_state=$(get_container_state "${AUTONOMY_ALICE_CONTAINER}")
  assertEquals "Container ${AUTONOMY_ALICE_CONTAINER} not running}" running "${autonomy_alice_container_state}"
  autonomy_bob_container_state=$(get_container_state "${AUTONOMY_BOB_CONTAINER}")
  assertEquals "Container ${AUTONOMY_BOB_CONTAINER} not running}" running "${autonomy_bob_container_state}"

  # get kuscia api resource
  mkdir -p "${test_suite_run_kuscia_dir}"
  mkdir -p "${test_suite_run_kuscia_dir}"/alice
  mkdir -p "${test_suite_run_kuscia_dir}"/bob
  ## generate client certs
  docker exec -it "${AUTONOMY_ALICE_CONTAINER}" sh scripts/deploy/init_kusciaapi_client_certs.sh
  docker cp "${AUTONOMY_ALICE_CONTAINER}":/home/kuscia/var/certs/kusciaapi-client.key "${test_suite_run_kuscia_dir}"/alice
  docker cp "${AUTONOMY_ALICE_CONTAINER}":/home/kuscia/var/certs/kusciaapi-client.crt "${test_suite_run_kuscia_dir}"/alice
  docker cp "${AUTONOMY_ALICE_CONTAINER}":/home/kuscia/var/certs/ca.crt "${test_suite_run_kuscia_dir}"/alice
  ## generate client certs
  docker cp "${AUTONOMY_ALICE_CONTAINER}":/home/kuscia/var/certs/token "${test_suite_run_kuscia_dir}"/alice
  docker exec -it "${AUTONOMY_BOB_CONTAINER}" sh scripts/deploy/init_kusciaapi_client_certs.sh
  docker cp "${AUTONOMY_BOB_CONTAINER}":/home/kuscia/var/certs/kusciaapi-client.key "${test_suite_run_kuscia_dir}"/bob
  docker cp "${AUTONOMY_BOB_CONTAINER}":/home/kuscia/var/certs/kusciaapi-client.crt "${test_suite_run_kuscia_dir}"/bob
  docker cp "${AUTONOMY_BOB_CONTAINER}":/home/kuscia/var/certs/ca.crt "${test_suite_run_kuscia_dir}"/bob
  docker cp "${AUTONOMY_BOB_CONTAINER}":/home/kuscia/var/certs/token "${test_suite_run_kuscia_dir}"/bob

  unset test_suite_run_kuscia_dir autonomy_alice_container_state autonomy_bob_container_state
}

# Stop p2p mode
function stop_p2p_mode() {
  docker stop "${AUTONOMY_ALICE_CONTAINER}" "${AUTONOMY_BOB_CONTAINER}"
  docker rm "${AUTONOMY_ALICE_CONTAINER}" "${AUTONOMY_BOB_CONTAINER}"
  docker volume rm "${AUTONOMY_ALICE_CONTAINER}-containerd" "${AUTONOMY_BOB_CONTAINER}-containerd"
}

# Get IpV4 Address
function get_ipv4_address() {
  local ipv4=""
  arch=$(uname -s || true)
  case $arch in
  "Linux")
    ipv4=$(ip -4 addr show eth0 | grep -oP '(?<=inet\s)\d+(\.\d+){3}') || true
    ;;
  "Darwin")
    ipv4=$(ipconfig getifaddr en0) || true
    ;;
  esac
  echo "$ipv4"
}


# @return domain route destination token
# Args:
#   ctr: container name
#   dr_name: dormain route name
function get_dr_dst_token_value() {
  local ctr=$1
  local dr_name=$2
  docker exec "${ctr}" kubectl get dr "$dr_name" -o jsonpath='{.status.tokenStatus.revisionToken.token}'
}

# @return domain route source token
# Args:
#   ctr: container name
#   dr_name: dormain route name
function get_dr_src_token_value() {
  local ctr=$1
  local dr_name=$2
  docker exec "${ctr}" kubectl get dr "$dr_name" -o jsonpath='{.status.tokenStatus.revisionToken.token}'
}


# @return cluster domain route destination token
# Args:
#   ctr: container name
#   cdr_name: cluster dormain route name
function get_cdr_dst_token_value() {
  local ctr=$1
  local cdr_name=$2
  docker exec "${ctr}" kubectl get cdr "$cdr_name" -o jsonpath='{.status.tokenStatus.destinationTokens[-1].token}'
}


# @return cluster domain route source token
# Args:
#   ctr: container name
#   dr_name: cluster dormain route name
function get_cdr_src_token_value() {
  local ctr=$1
  local cdr_name=$2
  docker exec "${ctr}" kubectl get cdr "$cdr_name" -o jsonpath='{.status.tokenStatus.sourceTokens[-1].token}'
}


# @return cluster domain route destination token revision
# Args:
#   ctr: container name
#   cdr_name: cluster dormain route name
function get_cdr_dst_token_revision() {
  local ctr=$1
  local cdr_name=$2
  docker exec "${ctr}" kubectl get cdr "$cdr_name" -o jsonpath='{.status.tokenStatus.destinationTokens[-1].revision}'
}


# @return cluster domain route source token revision
# Args:
#   ctr: container name
#   cdr_name: cluster dormain route name
function get_cdr_src_token_revision() {
  local ctr=$1
  local cdr_name=$2
  docker exec "${ctr}" kubectl get cdr "$cdr_name" -o jsonpath='{.status.tokenStatus.sourceTokens[-1].revision}'
}


# @return cluster domain route token status
# Args:
#   ctr: container name
#   cdr_name: cluster dormain route name
function get_cdr_token_status() {
  local ctr=$1
  local cdr_name=$2
  docker exec "${ctr}" kubectl get cdr "$cdr_name" -o jsonpath='{.status.conditions[0].type}'
}


# @return domain route party token ready status
# Args:
#   ctr: container name
#   dr_name: dormain route name
#   domain: party
function get_dr_revision_token_ready() {
  local ctr=$1
  local dr_name=$2
  local domain=$3
  docker exec "${ctr}" kubectl get dr "$dr_name" -n "$domain" -o jsonpath='{.status.tokenStatus.revisionToken.isReady}'
}


# Args:
#   ctr: container name
#   cdr_name: cluster dormain route name
#   period: target token rolling period(s)
function set_cdr_token_rolling_period() {
  local ctr=$1
  local cdr_name=$2
  local peroid=$3
  docker exec "${ctr}" kubectl patch cdr "$cdr_name" --type json -p="[{\"op\": \"replace\", \"path\": \"/spec/tokenConfig/rollingUpdatePeriod\", \"value\": ${peroid}}]"
}


# @return cluster domain route rolling period(s)
# Args:
#   ctr: container name
#   cdr_name: cluster dormain route name
function get_cdr_token_rolling_period() {
  local ctr=$1
  local cdr_name=$2
  docker exec "${ctr}" kubectl get cdr "$cdr_name" -o jsonpath='{.spec.tokenConfig.rollingUpdatePeriod}'
}

# @return get images
# Args:
#   images_tag: image tag
#   container_name: container name
#   runtime: runtime runc or runp
function get_images() {
  local images_tag=$1
  local container_name=$2
  local runtime=$3
  if [ -n "${runtime}" ]; then
    docker exec -i "${container_name}" bash -c "kuscia image --runtime=${runtime} ls 2>&1 | awk '{print \$1\":\"\$2}' | grep ${images_tag}"
  else
    docker exec -i "${container_name}" bash -c "kuscia image ls 2>&1 | awk '{print \$1\":\"\$2}' | grep ${images_tag}"
  fi
}


# @return get image ID
# Args:
#   images_tag: image tag
#   container_name: container name
#   runtime: runtime runc or runp
function get_image_ID() {
    local images_tag=$1
    local container_name=$2
    local runtime=$3
    if [ -n "${runtime}" ]; then
      docker exec -i "${container_name}" bash -c "kuscia image --runtime=${runtime} ls 2>&1 | grep ${images_tag} | awk '{print \$3}'"
    else
      docker exec -i "${container_name}" bash -c "kuscia image ls 2>&1 | grep ${images_tag} | awk '{print \$3}'"
    fi
}