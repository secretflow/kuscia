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

[[ ${ROOT} == "" ]] && ROOT=${PWD}

GREEN='\033[0;32m'
NC='\033[0m'
RED='\033[31m'

IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia
if [ "${KUSCIA_IMAGE}" != "" ]; then
  IMAGE=${KUSCIA_IMAGE}
fi

echo -e "IMAGE=${IMAGE}"

if [ "$SECRETFLOW_IMAGE" != "" ]; then
  echo -e "SECRETFLOW_IMAGE=${SECRETFLOW_IMAGE}"
fi

CTR_PREFIX=${USER}-kuscia
CTR_ROOT=/home/kuscia
CTR_CERT_ROOT=${CTR_ROOT}/var/certs
FORCE_START=false
MASTER_MEMORY_LIMIT=2G
LITE_MEMORY_LIMIT=4G
AUTONOMY_MEMORY_LIMIT=6G
SF_IMAGE_NAME="secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/secretflow-lite-anolis8"
SF_IMAGE_TAG="1.7.0b0"
SF_IMAGE_REGISTRY=""
NETWORK_NAME="kuscia-exchange"
VOLUME_PATH="${ROOT}"

function log() {
  local log_content=$1
  echo -e "${GREEN}${log_content}${NC}"
}

function pre_check() {
  if ! mkdir -p "$1" 2>/dev/null; then
    echo -e "${RED}User does not have access to create the directory: $1${NC}"
    exit 1
  fi
}

function arch_check() {
  local arch
  arch=$(uname -a)
  if [[ $arch == *"ARM"* ]] || [[ $arch == *"aarch64"* ]]; then
    echo "Warning: arm64 architecture. Continuing..."
  elif [[ $arch == *"x86_64"* ]]; then
    echo -e "${GREEN}x86_64 architecture. Continuing...${NC}"
  elif [[ $arch == *"amd64"* ]]; then
    echo "Warning: amd64 architecture. Continuing..."
  else
    echo -e "${RED}$arch architecture is not supported by kuscia currently${NC}"
    exit 1
  fi
}

function init_sf_image_info() {
  if [ "$SECRETFLOW_IMAGE" != "" ]; then
    SF_IMAGE_TAG=${SECRETFLOW_IMAGE##*:}
    path_separator_count="$(echo "$SECRETFLOW_IMAGE" | tr -cd "/" | wc -c)"
    if [ "${path_separator_count}" == 1 ]; then
      SF_IMAGE_NAME=${SECRETFLOW_IMAGE//:${SF_IMAGE_TAG}/}
    elif [ "${path_separator_count}" == 2 ]; then
      registry=$(echo "${SECRETFLOW_IMAGE}" | cut -d "/" -f 1)
      bucket=$(echo "${SECRETFLOW_IMAGE}" | cut -d "/" -f 2)
      name_and_tag=$(echo "${SECRETFLOW_IMAGE}" | cut -d "/" -f 3)
      name=${name_and_tag//:${SF_IMAGE_TAG}/}
      SF_IMAGE_REGISTRY="$registry/$bucket"
      SF_IMAGE_NAME="$name"
    fi
  fi
}

init_sf_image_info

function need_start_docker_container() {
  ctr=$1

  if [[ ! "$(docker ps -a -q -f name=^/"${ctr}"$)" ]]; then
    # need start your container
    return 0
  fi

  if $FORCE_START; then
    log "Remove container '${ctr}' ..."
    docker rm -f "${ctr}" >/dev/null 2>&1
    # need start your container
    return 0
  fi

  read -rp "$(echo -e "${GREEN}"The container \'"${ctr}"\' already exists. Do you need to recreate it? [y/n]: "${NC}")" yn
  case $yn in
  [Yy]*)
    echo -e "${GREEN}Remove container ${ctr} ...${NC}"
    docker rm -f "${ctr}"
    # need start your container
    return 0
    ;;
  *)
    return 1
    ;;
  esac
}

function do_http_probe() {
  local ctr=$1
  local endpoint=$2
  local max_retry=$3
  local enable_mtls=$4
  local cert_config
  if [[ "$enable_mtls" == "true" ]]; then
    cert_config="--cacert ${CTR_CERT_ROOT}/ca.crt --cert ${CTR_CERT_ROOT}/ca.crt --key ${CTR_CERT_ROOT}/ca.key"
  fi

  local retry=0
  while [[ "$retry" -lt "$max_retry" ]]; do
    local status_code
    status_code=$(docker exec -it "${ctr}" curl -k --write-out '%{http_code}' --silent --output /dev/null "${endpoint}" "${cert_config}")
    if [[ "${status_code}" -eq 200 || "${status_code}" -eq 404 || "${status_code}" -eq 401 ]]; then
      return 0
    fi
    sleep 1
    retry=$((retry + 1))
  done

  return 1
}

function probe_k3s() {
  local domain_ctr=$1

  if ! do_http_probe "${domain_ctr}" "https://127.0.0.1:6443" 60; then
    echo "[Error] Probe k3s in container '${domain_ctr}' failed. Please check k3s log in container, path: /home/kuscia/var/logs/k3s.log" >&2
    exit 1
  fi
}

function probe_gateway_crd() {
  local master=$1
  local domain=$2
  local gw_name=$3
  local max_retry=$4
  probe_k3s "${master}"

  local retry=0
  while [ $retry -lt "$max_retry" ]; do
    local line_num
    line_num=$(docker exec -it "${master}" kubectl get gateways -n "${domain}" | grep -c -i "${gw_name}" | xargs)
    if [[ "${line_num}" == "1" ]]; then
      return
    fi
    sleep 1
    retry=$((retry + 1))
  done
  echo "[Error] Probe gateway in namespace '$domain' failed. Please check envoy log in container, path: /home/kuscia/var/logs/envoy" >&2
  exit 1
}

function generate_env_flag() {
  local env_flag
  local env_file=${ROOT}/env.list
  if [ -e "${env_file}" ]; then
    env_flag="--env-file ${env_file}"
  else
    env_flag="--env REGISTRY_ENDPOINT=${SF_IMAGE_REGISTRY}"
  fi
  echo "${env_flag}"
}

function copy_between_containers() {
  local src_file=$1
  local dest_file=$2
  local dest_volume=$3
  local temp_file
  temp_file=$(basename "${dest_file}")
  docker cp "${src_file}" /tmp/"${temp_file}" >/dev/null
  docker cp /tmp/"${temp_file}" "${dest_file}" >/dev/null
  rm /tmp/"${temp_file}"
  echo "Copy file successfully src_file:'$src_file' to dest_file:'$dest_file'"
}

function createVolume() {
  local VOLUME_NAME=$1
  if ! docker volume ls --format '{{.Name}}' | grep "^${VOLUME_NAME}$"; then
    docker volume create "${VOLUME_NAME}"
  fi
}

function copy_container_file_to_volume() {
  local src_file=$1
  local dest_volume=$2
  local dest_file=$3

  createVolume "${dest_volume}"

  docker run -d --rm --name "${CTR_PREFIX}-dummy" -v "${dest_volume}":/tmp/kuscia "${IMAGE}" tail -f /dev/null >/dev/null 2>&1
  copy_between_containers "${src_file}" "${CTR_PREFIX}-dummy:/tmp/kuscia/${dest_file}" >/dev/null
  docker rm -f "${CTR_PREFIX}-dummy" >/dev/null 2>&1
  echo "Copy file successfully src_file:'$src_file' to dest_file:'$dest_volume:$CTR_CERT_ROOT/$dest_file'"
}

function copy_volume_file_to_container() {
  local src_volume=$1
  local src_file=$2
  local dest_file=$3

  createVolume "${src_volume}"

  docker run -d --rm --name "${CTR_PREFIX}-dummy" -v "${src_volume}":/tmp/kuscia "${IMAGE}" tail -f /dev/null >/dev/null 2>&1
  copy_between_containers "${CTR_PREFIX}-dummy:/tmp/kuscia/${src_file}" "${dest_file}" >/dev/null
  docker rm -f "${CTR_PREFIX}-dummy" >/dev/null 2>&1
  echo "Copy file successfully src_file:'$src_volume/$src_file' to dest_file:'$dest_file'"
}

function create_secretflow_app_image() {
  local ctr=$1
  docker exec -it "${ctr}" scripts/deploy/create_sf_app_image.sh "${SF_IMAGE_NAME}" "${SF_IMAGE_TAG}"
  log "create secretflow app image done"
}

function create_domaindatagrant_alice2bob() {
  local ctr=$1
  probe_datamesh "${ctr}"
  docker exec -it "${ctr}" curl https://127.0.0.1:8070/api/v1/datamesh/domaindatagrant/create -X POST -H 'content-type: application/json' -d '{"author":"alice","domaindata_id":"alice-table","grant_domain":"bob"}' \
    --cacert "${CTR_CERT_ROOT}/ca.crt" --cert "${CTR_CERT_ROOT}/ca.crt" --key "${CTR_CERT_ROOT}/ca.key"
  echo
  docker exec -it "${ctr}" curl https://127.0.0.1:8070/api/v1/datamesh/domaindatagrant/create -X POST -H 'content-type: application/json' -d '{"author":"alice","domaindata_id":"alice-dp-table","grant_domain":"bob"}' \
    --cacert "${CTR_CERT_ROOT}/ca.crt" --cert "${CTR_CERT_ROOT}/ca.crt" --key "${CTR_CERT_ROOT}/ca.key"
  echo
}

function create_domaindata_alice_table() {
  local ctr=$1
  local domain_id=$2
  local data_path="/home/kuscia/var/storage/data"

  # create domain data alice table
  docker exec -it "${ctr}" scripts/deploy/create_domaindata_alice_table.sh "${domain_id}"
  log "create domaindata alice's table done default stored path: '${data_path}'"
}

function create_domaindatagrant_bob2alice() {
  local ctr=$1
  probe_datamesh "${ctr}"
  docker exec -it "${ctr}" curl https://127.0.0.1:8070/api/v1/datamesh/domaindatagrant/create -X POST -H 'content-type: application/json' -d '{"author":"bob","domaindata_id":"bob-table","grant_domain":"alice"}' \
    --cacert "${CTR_CERT_ROOT}/ca.crt" --cert "${CTR_CERT_ROOT}/ca.crt" --key "${CTR_CERT_ROOT}/ca.key"
  echo
  docker exec -it "${ctr}" curl https://127.0.0.1:8070/api/v1/datamesh/domaindatagrant/create -X POST -H 'content-type: application/json' -d '{"author":"bob","domaindata_id":"bob-dp-table","grant_domain":"alice"}' \
    --cacert "${CTR_CERT_ROOT}/ca.crt" --cert "${CTR_CERT_ROOT}/ca.crt" --key "${CTR_CERT_ROOT}/ca.key"
  echo
}

function create_domaindata_bob_table() {
  local ctr=$1
  local domain_id=$2
  local data_path="/home/kuscia/var/storage/data"

  # create domain data bob table
  docker exec -it "${ctr}" scripts/deploy/create_domaindata_bob_table.sh "${domain_id}"
  log "create domaindata bob's table done default stored path: '${data_path}'"
}

function probe_datamesh() {
  local domain_ctr=$1
  if ! do_http_probe "$domain_ctr" "https://127.0.0.1:8070/healthZ" 30 true; then
    echo "[Error] Probe datamesh in container '$domain_ctr' failed." >&2
    echo "You cloud run command that 'docker logs $domain_ctr' to check the log" >&2
  fi
  log "Probe datamesh successfully"
}

function start_lite() {
  local master_ctr=$1
  local master_domain=$2
  local domain_id=$3
  local master_endpoint=$4
  local domain_ctr=${CTR_PREFIX}-lite-${domain_id}
  local externalPort=$5
  local internalPort=$6
  local httpPort=$7
  local grpcPort=$8
  local volume_path=$9
  local kuscia_config_file=${volume_path}/kuscia.yaml
  local data_path=${volume_path}/data

  if need_start_docker_container "${domain_ctr}"; then
    log "Starting container '$domain_ctr' ..."
    local conf_dir=${ROOT}/${domain_ctr}
    env_flag=$(generate_env_flag)
    local mount_volume_param="-v /tmp:/tmp"
    if [ "$volume_path" != "" ]; then
      mount_volume_param="-v /tmp:/tmp  -v ${data_path}:/home/kuscia/var/storage/data"
    fi

    # init kuscia.yaml
    pre_check "${data_path}"
    csr_token=$(docker exec -it "${master_ctr}" scripts/deploy/add_domain_lite.sh "${domain_id}" "${master_domain}" | tr -d '\r\n')
    docker run -it --rm "${IMAGE}" kuscia init --mode Lite --domain "${domain_id}" --master-endpoint "${master_endpoint}" --lite-deploy-token "${csr_token}" >"${kuscia_config_file}" 2>&1 || cat "${kuscia_config_file}"
    wrap_kuscia_config_file "${kuscia_config_file}"

    createVolume "${domain_ctr}-containerd"

    # start kuscia
    docker run -dit --privileged --name="${domain_ctr}" --hostname="${domain_ctr}" --restart=always --network="${NETWORK_NAME}" -m "${LITE_MEMORY_LIMIT}" "${env_flag}" \
      -v "${domain_ctr}-containerd:${CTR_ROOT}/containerd" \
      -e NAMESPACE="${domain_id}" \
      "${mount_volume_param}" \
      -p "${externalPort}":1080 \
      -p "${internalPort}":80 \
      -p "${httpPort}":8082 \
      -p "${grpcPort}":8083 \
      -v "${kuscia_config_file}":/home/kuscia/etc/conf/kuscia.yaml \
      "${IMAGE}" bin/kuscia start -c etc/conf/kuscia.yaml
    probe_gateway_crd "${master_ctr}" "${domain_id}" "${domain_ctr}" 60
    log "Lite domain '${domain_id}' started successfully docker container name:'${domain_ctr}'"
    docker run -it --rm "${IMAGE}" cat /home/kuscia/var/storage/data/"${domain_id}.csv" >"${data_path}/${domain_id}.csv"
  fi
}

function start_master() {
  local master_ctr=$1
  local master_domain=$2
  local port=$3
  local httpPort=$4
  local grpcPort=$5
  if need_start_docker_container "${master_ctr}"; then
    log "Starting container '$master_ctr' ..."
    local certs_volume="${master_ctr}-certs"
    local conf_dir="${ROOT}/${master_ctr}"
    local kuscia_config_file="${conf_dir}/kuscia.yaml"
    env_flag=$(generate_env_flag)

    # init kuscia.yaml
    mkdir -p "${conf_dir}"
    kuscia_config_file="${conf_dir}/kuscia.yaml"
    docker run -it --rm "${IMAGE}" kuscia init --mode Master --domain "${master_domain}" >"${kuscia_config_file}" 2>&1 || cat "${kuscia_config_file}"

    # start kuscia
    docker run -dit --name="${master_ctr}" --hostname="${master_ctr}" --restart=always --network="${NETWORK_NAME}" -m "${MASTER_MEMORY_LIMIT}" "${env_flag}" \
      -e NAMESPACE="${master_domain}" \
      -p "${port}":1080 \
      -p "${httpPort}":8082 \
      -p "${grpcPort}":8083 \
      -v "${kuscia_config_file}":/home/kuscia/etc/conf/kuscia.yaml \
      -v /tmp:/tmp \
      "${IMAGE}" bin/kuscia start -c etc/conf/kuscia.yaml
    probe_gateway_crd "${master_ctr}" "${master_domain}" "${master_ctr}" 60
    log "Master '${master_domain}' started successfully"
    FORCE_START=true
  fi
}

function create_cluster_domain_route() {
  local master_ctr=$1
  local src_domain=$2
  local dest_domain=$3
  local dest_ctr=${CTR_PREFIX}-lite-${dest_domain}
  log "Starting create cluster domain route from '${src_domain}' to '${dest_domain}'"

  docker exec -it "${master_ctr}" scripts/deploy/create_cluster_domain_route.sh "${src_domain}" "${dest_domain}" "http://${dest_ctr}:1080"
  log "Cluster domain route from '${src_domain}' to '${dest_domain}' created successfully dest_endpoint: '${dest_ctr}':1080"
}

function check_sf_image() {
  local domain_id=$1
  local domain_ctr=$2
  local volume_path=$3
  local env_file=${ROOT}/env.list
  local default_repo=${SF_IMAGE_REGISTRY}
  local repo
  if [ -e "${env_file}" ]; then
    repo=$(awk -F "=" '/REGISTRY_ENDPOINT/ {print $2}' "${env_file}")
  fi
  local sf_image="${SF_IMAGE_NAME}:${SF_IMAGE_TAG}"
  if [ "${repo}" != "" ]; then
    sf_image="${repo}/${SF_IMAGE_NAME##*/}:${SF_IMAGE_TAG}"
  elif [ "${default_repo}" != "" ]; then
    sf_image="${default_repo}/${SF_IMAGE_NAME##*/}:${SF_IMAGE_TAG}"
  fi
  if [ "${SECRETFLOW_IMAGE}" != "" ]; then
    sf_image=$SECRETFLOW_IMAGE
  fi

  if docker exec -it "${domain_ctr}" crictl inspecti "${sf_image}" >/dev/null 2>&1; then
    log "Image '${sf_image}' already exists in domain '${domain_id}'"
    return
  fi

  local has_sf_image=false
  if docker image inspect "${sf_image}" >/dev/null 2>&1; then
    has_sf_image=true
  fi

  if [ "${has_sf_image}" == true ]; then
    log "Found the secretflow image '${sf_image}' on host"
  else
    log "Not found the secretflow image '${sf_image}' on host"
    if [ "${repo}" != "" ]; then
      docker login "${repo}"
    fi
    log "Start pulling image '${sf_image}' ..."
    docker pull "${sf_image}"
  fi

  log "Start importing image '${sf_image}' Please be patient..."
  local image_id
  image_id=$(docker images --filter="reference=${sf_image}" --format "{{.ID}}")
  local image_tar
  image_tar="/tmp/$(echo "${sf_image}" | sed 's/\//_/g').${image_id}.tar"
  if [ ! -e "${image_tar}" ]; then
    docker save "${sf_image}" -o "${image_tar}"
  fi
  docker exec -it "${domain_ctr}" ctr -a="${CTR_ROOT}/containerd/run/containerd.sock" -n=k8s.io images import "${image_tar}"
  log "Successfully imported image '${sf_image}' to container '${domain_ctr}' ..."
}

function run_centralized() {
  build_kuscia_network
  local master_domain="kuscia-system"
  local alice_domain="alice"
  local bob_domain="bob"
  local master_ctr=${CTR_PREFIX}-master
  local alice_ctr=${CTR_PREFIX}-lite-${alice_domain}
  local bob_ctr=${CTR_PREFIX}-lite-${bob_domain}

  start_master "${master_ctr}" "${master_domain}" 18080 18082 18083

  start_lite "${master_ctr}" "${master_domain}" "${alice_domain}" "https://${master_ctr}:1080" 28080 28081 28082 28083 "${ROOT}/${alice_ctr}"
  start_lite "${master_ctr}" "${master_domain}" "${bob_domain}" "https://${master_ctr}:1080" 38080 38081 38082 38083 "${ROOT}/${bob_ctr}"

  create_cluster_domain_route "${master_ctr}" "${alice_domain}" "${bob_domain}"
  create_cluster_domain_route "${master_ctr}" "${bob_domain}" "${alice_domain}"

  check_sf_image "${alice_domain}" "${alice_ctr}" "${alice_ctr}"
  check_sf_image "${bob_domain}" "${bob_ctr}" "${bob_ctr}"

  # create demo data
  create_domaindata_alice_table "${master_ctr}" "${alice_domain}"
  create_domaindata_bob_table "${master_ctr}" "${bob_domain}"
  create_domaindatagrant_alice2bob "${alice_ctr}"
  create_domaindatagrant_bob2alice "${bob_ctr}"
  # create secretflow app image
  create_secretflow_app_image "${master_ctr}"

  log "Kuscia centralized cluster started successfully"
}

function run_hybrid_centerX2() {
  build_kuscia_network
  local transit=$1
  local alice_master_domain="master-alice"
  local bob_master_domain="master-bob"
  local alice_domain="alice"
  local bob_domain="bob"
  local alice_master_ctr=${CTR_PREFIX}-master-alice
  local bob_master_ctr=${CTR_PREFIX}-master-bob
  local alice_ctr=${CTR_PREFIX}-lite-${alice_domain}
  local bob_ctr=${CTR_PREFIX}-lite-${bob_domain}
  local p2p_protocol="kuscia"

  start_master "${alice_master_ctr}" "${alice_master_domain}" 8080 8082 8083
  start_master "${bob_master_ctr}" "${bob_master_domain}" 18080 18082 18083

  start_lite "${alice_master_ctr}" "${alice_master_domain}" "${alice_domain}" "https://${alice_master_ctr}:1080" 28080 28081 28082 28083 "${ROOT}/${alice_ctr}"
  start_lite "${bob_master_ctr}" "${bob_master_domain}" "${bob_domain}" "https://${bob_master_ctr}:1080" 38080 38081 38082 38083 "${ROOT}/${bob_ctr}"

  build_interconn "${bob_master_ctr}" "${alice_master_ctr}" "${alice_master_domain}" "${bob_master_domain}" "${p2p_protocol}"
  build_interconn "${alice_master_ctr}" "${bob_master_ctr}" "${bob_master_domain}" "${alice_master_domain}" "${p2p_protocol}"

  copy_between_containers "${alice_ctr}:${CTR_CERT_ROOT}/domain.crt" "${bob_master_ctr}:${CTR_CERT_ROOT}/${alice_domain}.domain.crt"
  copy_between_containers "${bob_ctr}:${CTR_CERT_ROOT}/domain.crt" "${alice_master_ctr}:${CTR_CERT_ROOT}/${bob_domain}.domain.crt"

  docker exec -it "${alice_master_ctr}" scripts/deploy/add_domain.sh "${bob_domain}" p2p "${p2p_protocol}" "${bob_master_domain}"
  docker exec -it "${bob_master_ctr}" scripts/deploy/add_domain.sh "${alice_domain}" p2p "${p2p_protocol}" "${alice_master_domain}"

  if [[ $transit == false ]]; then
    # alice-bob@alice-master
    docker exec -it "${alice_master_ctr}" scripts/deploy/join_to_host.sh "${alice_domain}" "${bob_domain}" "http://${bob_ctr}:1080" -i false -p "${p2p_protocol}"
    # bob-alice@bob-master
    docker exec -it "${bob_master_ctr}" scripts/deploy/join_to_host.sh "${bob_domain}" "${alice_domain}" "http://${alice_ctr}:1080" -i false -p "${p2p_protocol}"
    # alice-bob@bob-master
    docker exec -it "${bob_master_ctr}" scripts/deploy/join_to_host.sh "${alice_domain}" "${bob_domain}" "http://${bob_ctr}:1080" -i false -p "${p2p_protocol}"
    # bob-alice@alice-master
    docker exec -it "${alice_master_ctr}" scripts/deploy/join_to_host.sh "${bob_domain}" "${alice_domain}" "http://${alice_ctr}:1080" -i false -p "${p2p_protocol}"
  else
    # alice to bob =
    # bob-master to bob +
    # alice-master to bob transit by bob-master +
    # alice to bob transit by alice-master
    docker exec -it "${bob_master_ctr}" scripts/deploy/join_to_host.sh "${bob_master_domain}" "${bob_domain}" "http://${bob_ctr}:1080" -i false -p "${p2p_protocol}"
    docker exec -it "${alice_master_ctr}" scripts/deploy/join_to_host.sh "${alice_master_domain}" "${bob_domain}" "http://${bob_ctr}:1080" -i false -p "${p2p_protocol}" -x "${bob_master_domain}"
    docker exec -it "${alice_master_ctr}" scripts/deploy/join_to_host.sh "${alice_domain}" "${bob_domain}" "http://${bob_ctr}:1080" -i false -p "${p2p_protocol}" -x "${alice_master_domain}"
    # cdr declaration for handshake
    docker exec -it "${bob_master_ctr}" scripts/deploy/join_to_host.sh "${alice_domain}" "${bob_domain}" "http://${bob_ctr}:1080" -i false -p "${p2p_protocol}" -x "${alice_master_domain}"
    docker exec -it "${bob_master_ctr}" scripts/deploy/join_to_host.sh "${alice_master_domain}" "${bob_domain}" "http://${bob_ctr}:1080" -i false -p "${p2p_protocol}" -x "${bob_master_domain}"
    # bob to alice =
    # alice-master to alice +
    # bob-master to alice transit by alice-master +
    # bob to alice transit by bob-master
    docker exec -it "${alice_master_ctr}" scripts/deploy/join_to_host.sh "${alice_master_domain}" "${alice_domain}" "http://${alice_ctr}:1080" -i false -p "${p2p_protocol}"
    docker exec -it "${bob_master_ctr}" scripts/deploy/join_to_host.sh "${bob_master_domain}" "${alice_domain}" "http://${alice_ctr}:1080" -i false -p "${p2p_protocol}" -x "${alice_master_domain}"
    docker exec -it "${bob_master_ctr}" scripts/deploy/join_to_host.sh "${bob_domain}" "${alice_domain}" "http://${alice_ctr}:1080" -i false -p "${p2p_protocol}" -x "${bob_master_domain}"
    # cdr declaration for handshake
    docker exec -it "${alice_master_ctr}" scripts/deploy/join_to_host.sh "${bob_domain}" "${alice_domain}" "http://${alice_ctr}:1080" -i false -p "${p2p_protocol}" -x "${bob_master_domain}"
    docker exec -it "${alice_master_ctr}" scripts/deploy/join_to_host.sh "${bob_master_domain}" "${alice_domain}" "http://${alice_ctr}:1080" -i false -p "${p2p_protocol}" -x "${alice_master_domain}"
  fi

  check_sf_image "${alice_domain}" "${alice_ctr}" "${alice_ctr}"
  check_sf_image "${bob_domain}" "${bob_ctr}" "${bob_ctr}"

  # create demo data
  create_domaindata_alice_table "${alice_master_ctr}" "${alice_domain}"
  create_domaindata_bob_table "${bob_master_ctr}" "${bob_domain}"
  create_domaindatagrant_alice2bob "${alice_ctr}"
  create_domaindatagrant_bob2alice "${bob_ctr}"
  # create secretflow app image
  create_secretflow_app_image "${alice_master_ctr}"
  create_secretflow_app_image "${bob_master_ctr}"

  log "Kuscia hybrid cluster centerX2 started successfully"
}

function run_hybrid_centerXp2p() {
  build_kuscia_network
  local transit=$1
  local alice_master_domain="master-alice"
  local bob_master_domain="bob"
  local alice_domain="alice"
  local bob_domain="bob"
  local alice_master_ctr=${CTR_PREFIX}-master-alice
  local bob_master_ctr=${CTR_PREFIX}-autonomy-bob
  local alice_ctr=${CTR_PREFIX}-lite-${alice_domain}
  local bob_ctr=${CTR_PREFIX}-autonomy-${bob_domain}
  local p2p_protocol="kuscia"

  start_master "$alice_master_ctr" "$alice_master_domain" 8080 8082 8083
  start_lite "${alice_master_ctr}" "${alice_master_domain}" "${alice_domain}" "https://${alice_master_ctr}:1080" 18080 18081 18082 18083 "${ROOT}/${alice_ctr}"
  start_autonomy "${bob_domain}" 12081 12082 12083 "${p2p_protocol}"

  build_interconn "${bob_master_ctr}" "${alice_master_ctr}" "${alice_master_domain}" "${bob_master_domain}" "${p2p_protocol}"
  build_interconn "${alice_master_ctr}" "${bob_master_ctr}" "${bob_master_domain}" "${alice_master_domain}" "${p2p_protocol}"

  copy_between_containers "${alice_ctr}:${CTR_CERT_ROOT}/domain.crt" "${bob_master_ctr}:${CTR_CERT_ROOT}/${alice_domain}.domain.crt"
  docker exec -it "${bob_master_ctr}" scripts/deploy/add_domain.sh "${alice_domain}" p2p "${p2p_protocol}" "${alice_master_domain}"

  if [[ $transit == false ]]; then
    docker exec -it "${alice_master_ctr}" scripts/deploy/join_to_host.sh "${alice_domain}" "${bob_domain}" "https://${bob_ctr}:1080" -i false -p "${p2p_protocol}"
    docker exec -it "${bob_master_ctr}" scripts/deploy/join_to_host.sh "${bob_domain}" "${alice_domain}" "http://${alice_ctr}:1080" -i false -p "${p2p_protocol}"
    docker exec -it "${bob_master_ctr}" scripts/deploy/join_to_host.sh "${alice_domain}" "${bob_domain}" "http://${bob_ctr}:1080" -i false -p "${p2p_protocol}"
    docker exec -it "${alice_master_ctr}" scripts/deploy/join_to_host.sh "${bob_domain}" "${alice_domain}" "http://${alice_ctr}:1080" -i false -p "${p2p_protocol}"
  else
    # alice to bob =
    # alice-master to bob + (generated already by build interconn)
    # alice to bob transit by alice-master
    docker exec -it "${alice_master_ctr}" scripts/deploy/join_to_host.sh "${alice_domain}" "${bob_domain}" "https://${bob_ctr}:1080" -i false -p "${p2p_protocol}" -x "${alice_master_domain}"
    # bob to alice =
    # alice-master to alice +
    # bob to alice transit by alice-master
    docker exec -it "${alice_master_ctr}" scripts/deploy/join_to_host.sh "${alice_master_domain}" "${alice_domain}" "http://${alice_ctr}:1080" -i false -p "${p2p_protocol}"
    docker exec -it "${bob_master_ctr}" scripts/deploy/join_to_host.sh "${bob_domain}" "${alice_domain}" "http://${alice_ctr}:1080" -i false -p "${p2p_protocol}" -x "${alice_master_domain}"
    # cdr declaration for handshake
    docker exec -it "${bob_master_ctr}" scripts/deploy/join_to_host.sh "${alice_domain}" "${bob_domain}" "https://${bob_ctr}:1080" -i false -p "${p2p_protocol}"
    docker exec -it "${alice_master_ctr}" scripts/deploy/join_to_host.sh "${bob_domain}" "${alice_domain}" "http://${alice_ctr}:1080" -i false -p "${p2p_protocol}"
  fi

  check_sf_image "${alice_domain}" "${alice_ctr}" "${alice_ctr}"
  check_sf_image "${bob_domain}" "${bob_ctr}" "${bob_ctr}"

  # create demo data
  create_domaindata_alice_table "${alice_master_ctr}" "${alice_domain}"
  create_domaindata_bob_table "${bob_master_ctr}" "${bob_domain}"
  create_domaindatagrant_alice2bob "${alice_ctr}"
  create_domaindatagrant_bob2alice "${bob_ctr}"
  # create secretflow app image
  create_secretflow_app_image "${alice_master_ctr}"
  create_secretflow_app_image "${bob_master_ctr}"

  log "Kuscia hybrid cluster centerXp2p started successfully"
}

function start_autonomy() {
  local domain_id=$1
  local domain_ctr=${CTR_PREFIX}-autonomy-${domain_id}
  local internalPort=$2
  local kusciaapi_http_port=$3
  local kusciaapi_grpc_port=$4
  local p2p_protocol=$5
  local conf_dir=${ROOT}/${domain_ctr}
  local kuscia_config_file=${conf_dir}/kuscia.yaml
  local data_path=${conf_dir}/data
  if need_start_docker_container "${domain_ctr}"; then
    log "Starting container '${domain_ctr}' ..."
    env_flag=$(generate_env_flag "${domain_id}")

    # init kuscia.yaml
    pre_check "${data_path}"
    docker run -it --rm "${IMAGE}" kuscia init --mode Autonomy --domain "${domain_id}" >"${kuscia_config_file}" 2>&1 || cat "${kuscia_config_file}"
    wrap_kuscia_config_file "${kuscia_config_file}" "${p2p_protocol}" "${domain_id}"

    createVolume "${domain_ctr}-containerd"

    docker run -dit --privileged --name="${domain_ctr}" --hostname="${domain_ctr}" --restart=always --network="${NETWORK_NAME}" -m "${AUTONOMY_MEMORY_LIMIT}" "${env_flag}" \
      --env NAMESPACE="${domain_id}" \
      -v "${domain_ctr}-containerd":${CTR_ROOT}/containerd \
      -p "${internalPort}":80 \
      -p "${kusciaapi_http_port}":8082 \
      -p "${kusciaapi_grpc_port}":8083 \
      -v "${kuscia_config_file}":/home/kuscia/etc/conf/kuscia.yaml \
      -v /tmp:/tmp \
      -v "${data_path}":/home/kuscia/var/storage/data \
      "${IMAGE}" bin/kuscia start -c etc/conf/kuscia.yaml
    probe_gateway_crd "${domain_ctr}" "${domain_id}" "${domain_ctr}" 60
    log "Autonomy domain '${domain_id}' started successfully docker container name: '${domain_ctr}'"
    docker run -it --rm "${IMAGE}" cat /home/kuscia/var/storage/data/"${domain_id}".csv >"${data_path}/${domain_id}.csv"
  fi
}

function build_interconn() {
  local host_ctr=$1
  local member_ctr=$2
  local member_domain=$3
  local host_domain=$4
  local interconn_protocol=$5

  log "Starting build internet connect from '${member_domain}' to '${host_domain}'"
  copy_between_containers "${member_ctr}:${CTR_CERT_ROOT}/domain.crt" "${host_ctr}:${CTR_CERT_ROOT}/${member_domain}.domain.crt"
  docker exec -it "${host_ctr}" scripts/deploy/add_domain.sh "${member_domain}" p2p "${interconn_protocol}" "${master_domain}"

  docker exec -it "${member_ctr}" scripts/deploy/join_to_host.sh "${member_domain}" "${host_domain}" "https://${host_ctr}:1080" -p "${interconn_protocol}"
  log "Build internet connect from '${member_domain}' to '${host_domain}' successfully protocol: '${interconn_protocol}' dest host: '${host_ctr}':1080"
}

function run_p2p() {
  local p2p_protocol=$1
  local alice_domain="alice"
  local bob_domain="bob"
  local alice_ctr=${CTR_PREFIX}-autonomy-${alice_domain}
  local bob_ctr=${CTR_PREFIX}-autonomy-${bob_domain}
  build_kuscia_network
  arch_check

  start_autonomy "${alice_domain}" 11081 11082 11083 "${p2p_protocol}"
  start_autonomy "${bob_domain}" 12081 12082 12083 "${p2p_protocol}"

  build_interconn "${bob_ctr}" "${alice_ctr}" "${alice_domain}" "${bob_domain}" "${p2p_protocol}"
  build_interconn "${alice_ctr}" "${bob_ctr}" "${bob_domain}" "${alice_domain}" "${p2p_protocol}"

  check_sf_image "${alice_domain}" "${alice_ctr}"
  check_sf_image "${bob_domain}" "${bob_ctr}"

  create_secretflow_app_image "${alice_ctr}"
  create_secretflow_app_image "${bob_ctr}"

  # create demo data
  create_domaindata_alice_table "${alice_ctr}" "${alice_domain}"
  create_domaindata_bob_table "${bob_ctr}" "${bob_domain}"
  create_domaindatagrant_alice2bob "${alice_ctr}"
  create_domaindatagrant_bob2alice "${bob_ctr}"
  log "Kuscia p2p cluster started successfully"
}

function build_kuscia_network() {
  if [[ ! "$(docker network ls -q -f name=${NETWORK_NAME})" ]]; then
    docker network create "${NETWORK_NAME}"
  fi
}

function wrap_kuscia_config_file() {
  local kuscia_config_file=$1
  local p2p_protocol=$2
  local domain=$3

  PRIVILEGED_CONFIG="
agent:
  allowPrivileged: true
  "

  BFIA_CONFIG="
agent:
  allowPrivileged: ${ALLOW_PRIVILEGED}
  plugins:
    - name: cert-issuance
    - name: config-render
    - name: env-import
      config:
        usePodLabels: false
        envList:
          - envs:
              - name: system.transport
                value: transport.${domain}.svc
              - name: system.storage
                value: file:///home/kuscia/var/storage
            selectors:
              - key: maintainer
                value: secretflow-contact@service.alipay.com
  "

  if [[ $p2p_protocol == "bfia" ]]; then
    echo -e "$BFIA_CONFIG" >>"$kuscia_config_file"
  else
    if [[ $ALLOW_PRIVILEGED == "true" ]]; then
      echo -e "$PRIVILEGED_CONFIG" >>"$kuscia_config_file"
    fi
  fi
}

usage() {
  echo "$(basename "$0") NETWORK_MODE [OPTIONS]
NETWORK_MODE:
    center,centralized       centralized network mode (default)
    p2p                      p2p network mode

Common Options:
    -h                  show this help text
    -p                  interconnection protocol, must be 'kuscia' or 'bfia', default is 'kuscia'. In current quickstart script, center mode just support 'bfia' protocol."
}

mode=
case "$1" in
center | centralized | p2p | cxc | cxp)
  mode=$1
  shift
  ;;
esac

interconn_protocol=
transit=false
while getopts 'p:th' option; do
  case "$option" in
  p)
    interconn_protocol=$OPTARG
    [[ "$interconn_protocol" == "bfia" || "$interconn_protocol" == "kuscia" ]] && continue
    printf "illegal value for -%s\n" "$option" >&2
    usage
    exit
    ;;
  t)
    transit=true
    ;;
  h)
    usage
    exit
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

[ "$interconn_protocol" == "bfia" ] || interconn_protocol="kuscia"
[ "$mode" == "" ] && mode=$1
[[ "$mode" == "" || "$mode" == "centralized" ]] && mode="center"
if [[ "$mode" == "center" && "$interconn_protocol" != "kuscia" ]]; then
  printf "In current quickstart script, center mode just support 'kuscia'\n" >&2
  exit 1
fi

case "$mode" in
center)
  run_centralized
  ;;
p2p)
  run_p2p $interconn_protocol
  ;;
cxc)
  run_hybrid_centerX2 $transit
  ;;
cxp)
  run_hybrid_centerXp2p $transit
  ;;
*)
  printf "unsupported network mode: %s\n" "$mode" >&2
  exit 1
  ;;
esac