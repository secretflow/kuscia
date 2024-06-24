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

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'
RED='\033[31m'
CTR_ROOT=/home/kuscia
CTR_CERT_ROOT=${CTR_ROOT}/var/certs
SF_IMAGE_REGISTRY=""
NETWORK_NAME="kuscia-exchange"
CLUSTER_NETWORK_NAME="kuscia-exchange-cluster"
IMPORT_SF_IMAGE=secretflow

function log() {
  local log_content=$1
  echo -e "${GREEN}${log_content}${NC}"
}

if [[ ${KUSCIA_IMAGE} == "" ]]; then
  KUSCIA_IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia:latest
fi
log "KUSCIA_IMAGE=${KUSCIA_IMAGE}"

if [[ "$SECRETFLOW_IMAGE" == "" ]]; then
  SECRETFLOW_IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/secretflow-lite-anolis8:1.6.0b0
fi
log "SECRETFLOW_IMAGE=${SECRETFLOW_IMAGE}"

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

function pre_check() {
  if ! mkdir -p "$1" 2>/dev/null; then
    echo -e "${RED}User does not have access to create the directory: $1${NC}"
    exit 1
  fi
}

function init_k3s_data() {
  if [ ! -d "${K3S_DB_PATH}" ]; then
    pre_check "${K3S_DB_PATH}"
  else
    echo -e "${GREEN}k3s data already exists ${K3S_DB_PATH}...${NC}"
    read -rp "$(echo -e "${GREEN}Whether to retain k3s data?(y/n): ${NC}")" reuse
    reuse=${reuse:-N}
    if [[ "${reuse}" =~ ^([nN][oO]|[nN])$ ]]; then
        rm -rf "${K3S_DB_PATH:?}"/*
    fi
  fi
}

function init_sf_image_info() {
  if [ "$SECRETFLOW_IMAGE" != "" ]; then
    SF_IMAGE_TAG=${SECRETFLOW_IMAGE##*:}
    path_separator_count="$(echo "$SECRETFLOW_IMAGE" | tr -cd "/" | wc -c)"
    if [ ${path_separator_count} == 1 ]; then
      SF_IMAGE_NAME=$(echo "$SECRETFLOW_IMAGE" | sed "s/:${SF_IMAGE_TAG}//")
    elif [ $path_separator_count == 2 ]; then
      registry=$(echo $SECRETFLOW_IMAGE | cut -d "/" -f 1)
      bucket=$(echo $SECRETFLOW_IMAGE | cut -d "/" -f 2)
      name_and_tag=$(echo $SECRETFLOW_IMAGE | cut -d "/" -f 3)
      name=$(echo "$name_and_tag" | sed "s/:${SF_IMAGE_TAG}//")
      SF_IMAGE_REGISTRY="$registry/$bucket"
      SF_IMAGE_NAME="$name"
    fi
  fi
}

init_sf_image_info

function wrap_kuscia_config_file() {
  local kuscia_config_file=$1
  local p2p_protocol=$2

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
                value: transport.${DOMAIN}.svc
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

function need_start_docker_container() {
  local force_start=false
  ctr=$1

  if [[ ! "$(docker ps -a -q -f name=^/${ctr}$)" ]]; then
    # need start your container
    return 0
  fi

  if $force_start; then
    log "Remove container '${ctr}' ..."
    docker rm -f $ctr >/dev/null 2>&1
    # need start your container
    return 0
  fi

  read -rp "$(echo -e ${GREEN}The container \'${ctr}\' already exists. Do you need to recreate it? [y/n]: ${NC})" yn
  case $yn in
  [Yy]*)
    echo -e "${GREEN}Remove container ${ctr} ...${NC}"
    docker rm -f $ctr
    # need start your container
    return 0
    ;;
  *)
    echo -e "${YELLOW}installation exit.${NC}"
    exit 0
    ;;
  esac

  return 1
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
    status_code=$(docker exec -it $ctr curl -k --write-out '%{http_code}' --silent --output /dev/null ${endpoint} ${cert_config})
    if [[ $status_code -eq 200 || $status_code -eq 404 || $status_code -eq 401 ]]; then
      return 0
    fi
    sleep 1
    retry=$((retry + 1))
  done

  return 1
}

function probe_k3s() {
  local domain_ctr=$1

  if ! do_http_probe $domain_ctr "https://127.0.0.1:6443" 60; then
    echo "[Error] Probe k3s in container '$domain_ctr' failed. Please check k3s log in container, path: ${CTR_ROOT}/var/logs/k3s.log" >&2
    exit 1
  fi
}

function probe_gateway_crd() {
  local master=$1
  local domain=$2
  local gw_name=$3
  local max_retry=$4
  probe_k3s $master

  local retry=0
  while [ $retry -lt $max_retry ]; do
    local line_num=$(docker exec -it $master kubectl get gateways -n $domain | grep -i $gw_name | wc -l | xargs)
    if [[ $line_num == "1" ]]; then
      return
    fi
    sleep 1
    retry=$((retry + 1))
  done
  echo "[Error] Probe gateway in namespace '$domain' failed. Please check envoy log in container, path: ${CTR_ROOT}/var/logs/envoy" >&2
  exit 1
}

function generate_env_flag() {
  local env_flag
  local env_file=${ROOT}/env.list
  if [ -e $env_file ]; then
    env_flag="--env-file $env_file"
  else
    env_flag="--env REGISTRY_ENDPOINT=${SF_IMAGE_REGISTRY}"
  fi
  echo $env_flag
}

function createVolume() {
  local VOLUME_NAME=$1
  if ! docker volume ls --format '{{.Name}}' | grep "^${VOLUME_NAME}$"; then
    docker volume create $VOLUME_NAME
  fi
}

function generate_hostname() {
    local prefix=$1
    local local_hostname
    local_hostname=$(hostname)
    local_hostname=$(echo "${local_hostname}" | tr '[:upper:]_.' '[:lower:]--' | sed 's/[^a-z0-9]$//g' )
    echo "${prefix}-${local_hostname}" | cut -c 1-63
}

function copy_between_containers() {
  local src_file=$1
  local dest_file=$2
  local dest_volume=$3
  local temp_file
  temp_file=$(basename $dest_file)
  docker cp $src_file /tmp/${temp_file} >/dev/null
  docker cp /tmp/${temp_file} $dest_file >/dev/null
  rm /tmp/${temp_file}
  echo "Copy file successfully src_file:'$src_file' to dest_file:'$dest_file'"
}

function probe_datamesh() {
  local domain_ctr=$1
  if ! do_http_probe "$domain_ctr" "https://127.0.0.1:8070/healthZ" 30 true; then
    echo -e "${RED}[Error] Probe datamesh in container '$domain_ctr' failed.${NC}" >&2
    echo -e "${RED}You cloud run command that 'docker logs $domain_ctr' to check the log${NC}" >&2
    exit 1
  fi
  log "Probe datamesh successfully"
}

function get_runtime() {
  local conf_file=$1
  local runtime
  runtime=$(grep '^runtime:' ${conf_file} | cut -d':' -f2 | awk '{$1=$1};1' | tr -d '\r\n')
  if [[ $runtime == "" ]]; then
    runtime=runc
  fi
  echo "$runtime"
}

function generate_mount_flag() {
  local mount_flag="-v ${DOMAIN_DATA_DIR}:${CTR_ROOT}/var/storage/data -v ${DOMAIN_LOG_DIR}:${CTR_ROOT}/var/stdout ${k3s_volume}"
  echo "$mount_flag"
}

function create_cluster_domain_route() {
  local ctr_prefix=${USER}-kuscia
  local master_ctr=${ctr_prefix}-master
  local src_domain=$1
  local dest_domain=$2
  log "Starting create cluster domain route from '${src_domain}' to '${dest_domain}'"

  docker exec -it ${master_ctr} scripts/deploy/create_cluster_domain_route.sh ${src_domain} ${dest_domain} http://${ctr_prefix}-lite-${dest_domain}:1080
  log "Cluster domain route from '${src_domain}' to '${dest_domain}' created successfully dest_endpoint: '${ctr_prefix}'-lite-'${dest_domain}':1080"
}

function build_interconn() {
  local host_ctr=$1
  local member_ctr=$2
  local member_domain=$3
  local host_domain=$4
  local interconn_protocol=$5

  log "Starting build internet connect from '${member_domain}' to '${host_domain}'"
  copy_between_containers ${member_ctr}:${CTR_CERT_ROOT}/domain.crt ${host_ctr}:${CTR_CERT_ROOT}/${member_domain}.domain.crt
  docker exec -it ${host_ctr} scripts/deploy/add_domain.sh $member_domain p2p ${interconn_protocol} ${master_domain}

  docker exec -it ${member_ctr} scripts/deploy/join_to_host.sh $member_domain $host_domain https://${host_ctr}:1080
  log "Build internet connect from '${member_domain}' to '${host_domain}' successfully protocol: '${interconn_protocol}' dest host: '${host_ctr}':1080"
}

function init_kuscia_conf_file() {
  local domain_type=$1
  local domain_id=$2
  local domain_ctr=$3
  local kuscia_conf_file=$4
  local master_endpoint=$5
  local master_ctr=$(echo "${master_endpoint}" | cut -d'/' -f3 | cut -d':' -f1)
  pre_check "${PWD}/${domain_ctr}"
  if [[ "${domain_type}" = "lite" ]]; then
    token=$(docker exec -it "${master_ctr}" scripts/deploy/add_domain_lite.sh "${domain_id}" | tr -d '\r\n')
    docker run -it --rm ${KUSCIA_IMAGE} kuscia init --mode "${domain_type}" --domain "${domain_id}" --master-endpoint ${master_endpoint} --lite-deploy-token ${token} > "${kuscia_conf_file}"
  else
    docker run -it --rm ${KUSCIA_IMAGE} kuscia init --mode "${domain_type}" --domain "${domain_id}" > "${kuscia_conf_file}"
  fi
  wrap_kuscia_config_file ${kuscia_conf_file}
}

function init() {
  local domain_ctr=$1
  [[ ${ROOT} == "" ]] && ROOT=${PWD}
  [[ ${DOMAIN_DATA_DIR} == "" ]] && DOMAIN_DATA_DIR="${ROOT}/${domain_ctr}/data"
  [[ ${DOMAIN_LOG_DIR} == "" ]] && DOMAIN_LOG_DIR="${ROOT}/${domain_ctr}/logs"

  pre_check "${DOMAIN_DATA_DIR}"
  pre_check "${DOMAIN_LOG_DIR}"

  log "ROOT=${ROOT}"
  log "DOMAIN_ID=${domain_id}"
  log "DOMAIN_HOST_PORT=${DOMAIN_HOST_PORT}"
  log "DOMAIN_HOST_INTERNAL_PORT=${domain_host_internal_port}"
  log "DOMAIN_DATA_DIR=${DOMAIN_DATA_DIR}"
  log "DOMAIN_LOG_DIR=${DOMAIN_LOG_DIR}"
  log "KUSCIA_IMAGE=${KUSCIA_IMAGE}"
  log "KUSCIAAPI_HTTP_PORT=${kusciaapi_http_port}"
  log "KUSCIAAPI_GRPC_PORT=${kusciaapi_grpc_port}"
}

function start_container() {
  local domain_ctr=$1
  local domain_id=$2
  local env_flag=$3
  local kuscia_conf_file=$4
  local mount_flag=$5
  local memory_limit=$6
  local domain_host_port=$7
  local kusciaapi_http_port=$8
  local kusciaapi_grpc_port=$9
  local domain_host_internal_port=${10}
  local mountcontainerd=""
  local export_port="-p ${domain_host_internal_port}:80 \
    -p ${domain_host_port}:1080 \
    -p ${kusciaapi_http_port}:8082 \
    -p ${kusciaapi_grpc_port}:8083"

  local local_network_name=${NETWORK_NAME}

  if [[ ${mode} != "start" ]] && [[ "${EXPOSE_PORTS}" != true ]]; then
    export_port=""
  fi
  if [[ ${domain_type} != "master" && ${runtime} == "runc" ]]; then
     createVolume "${domain_ctr}-containerd"
     mountcontainerd="-v ${domain_ctr}-containerd:${CTR_ROOT}/containerd"
     privileged_flag=" --privileged"
  fi
  log "domain_hostname=${domain_hostname}"

  if [[ ${mode} == "start" ]] && [[ "${CLUSTERED}" == true ]]; then
    local_network_name=${CLUSTER_NETWORK_NAME}
  fi
  log "network=${local_network_name}"

  docker run -dit${privileged_flag} --name="${domain_ctr}" --hostname="${domain_hostname}" --restart=always --network=${local_network_name} ${memory_limit} \
    ${export_port} ${mountcontainerd} -v /tmp:/tmp\
    -v ${kuscia_conf_file}:${CTR_ROOT}/etc/conf/kuscia.yaml \
    ${env_flag} ${mount_flag} \
    --env NAMESPACE="${domain_id}" \
    "${KUSCIA_IMAGE}" bin/kuscia start -c etc/conf/kuscia.yaml
}

function start_kuscia_container() {
  local domain_type=$1
  local domain_id=$2
  local runtime=$3
  local master_endpoint=$4
  local domain_ctr=$5
  local init_kuscia_conf_file=$6
  local mount_flag=$7
  local domain_host_port=$8
  local kusciaapi_http_port=$9
  local kusciaapi_grpc_port=${10}
  local domain_host_internal_port=${11}
  local env_flag
  local memory_limit
  local limit
  env_flag=$(generate_env_flag)

  local domain_hostname
  domain_hostname=$(generate_hostname "${domain_ctr}") || { echo -e "${RED}Failed to generate hostname${NC}"; exit 1; }

  if [[ ${MEMORY_LIMIT} = "-1" ]]; then
    memory_limit=""
  else
    case "${MEMORY_LIMIT}" in
      "")
        case "${domain_type}" in
          "lite")
            limit="4GiB"
            ;;
          "autonomy")
            limit="6GiB"
            ;;
          "master")
            limit="2GiB"
            ;;
        esac
        ;;
      *)
        limit="${MEMORY_LIMIT}"
        ;;
    esac
    memory_limit="-m ${limit}"
  fi

  build_kuscia_network

  if [[ ${init_kuscia_conf_file} = "true" ]]; then
    local kuscia_conf_file="${PWD}/${domain_ctr}/kuscia.yaml"
    init_kuscia_conf_file "${domain_type}" "${domain_id}" "${domain_ctr}" "${kuscia_conf_file}" "${master_endpoint}"
  fi

  if need_start_docker_container "$domain_ctr"; then
    log "Starting container $domain_ctr ..."
    start_container "${domain_ctr}" "${domain_id}" "${env_flag}" "${kuscia_conf_file}" "${mount_flag}" "${memory_limit}" "${domain_host_port}" "${kusciaapi_http_port}" "${kusciaapi_grpc_port}" "${domain_host_internal_port}" "${domain_hostname}"
    [[ "$domain_type" != "lite" ]] && probe_gateway_crd "${domain_ctr}" "${domain_id}" "${domain_hostname}" 60
    [[ "$domain_type" != "master" ]] && probe_datamesh "${domain_ctr}"
  fi

  if [[ ${IMPORT_SF_IMAGE} = "none"  ]]; then
    echo -e "${GREEN}skip importing sf image${NC}"
  elif [[ ${IMPORT_SF_IMAGE} = "secretflow"  ]]; then
    if [[ "$domain_type" != "master" ]] && [[ ${runtime} == "runc" ]]; then
      docker run --rm $KUSCIA_IMAGE cat ${CTR_ROOT}/scripts/deploy/import_engine_image.sh > import_engine_image.sh && chmod u+x import_engine_image.sh
      bash import_engine_image.sh ${domain_ctr} ${SECRETFLOW_IMAGE}
      rm -rf import_engine_image.sh
    fi
  fi

  if [[ "$domain_type" != "lite" ]]; then
    docker exec -it "${domain_ctr}" scripts/deploy/create_secretflow_app_image.sh "${SECRETFLOW_IMAGE}"
    log "Create secretflow app image done"
  fi
  log "$domain_type domain '${domain_id}' deployed successfully"
}

function get_config_value() {
  local config_file=$1
  local key=$2
  grep "$key:" "$config_file" | awk '{ print $2 }' | sed 's/"//g' | tr -d '\r\n'
}

function start_kuscia() {
  local kuscia_conf_file=${KUSCIA_CONFIG_FILE}
  local domain_id=$(get_config_value "$kuscia_conf_file" "domainID")
  local deploy_mode=$(get_config_value "$kuscia_conf_file" "mode" | tr '[:upper:]' '[:lower:]')
  local master_endpoint=$(get_config_value "$kuscia_conf_file" "masterEndpoint")
  local store_endpoint=$(get_config_value "$kuscia_conf_file" "datastoreEndpoint")
  local runtime=$(get_runtime "$kuscia_conf_file")
  local privileged_flag
  local domain_host_internal_port=${DOMAIN_HOST_INTERNAL_PORT:-13081}
  local kusciaapi_http_port=${KUSCIAAPI_HTTP_PORT:-13082}
  local kusciaapi_grpc_port=${KUSCIAAPI_GRPC_PORT:-13083}
  local k3s_volume=""

  wrap_kuscia_config_file ${kuscia_conf_file}
  local ctr_prefix=${USER}-kuscia
  local master_ctr=${ctr_prefix}-master
  local domain_ctr="${ctr_prefix}-${deploy_mode}-${domain_id}"
  if [[ "${deploy_mode}" == "master" ]]; then
    domain_ctr="${master_ctr}"
  fi

  K3S_DB_PATH="${HOME}/kuscia/${domain_ctr}/k3s"
  [[ ${DOMAIN_HOST_PORT} == "" ]] && { printf "empty domain host port\n" >&2; exit 1; }
  [[ ${deploy_mode} != "lite" && ${store_endpoint} == "" ]] && { init_k3s_data; k3s_volume="-v ${K3S_DB_PATH}:${CTR_ROOT}/var/k3s"; }
  init ${domain_ctr}
  local mount_flag=$(generate_mount_flag)
  start_kuscia_container "${deploy_mode}" "${domain_id}" "$runtime" "$master_endpoint" "${domain_ctr}" "false" "${mount_flag}" "${DOMAIN_HOST_PORT}" "${kusciaapi_http_port}" "${kusciaapi_grpc_port}" "${domain_host_internal_port}"
}

function start_center_cluster() {
  local alice_domain=alice
  local bob_domain=bob
  local ctr_prefix=${USER}-kuscia
  local master_ctr=${ctr_prefix}-master
  local runtime="runc"
  local privileged_flag=" --privileged"
  local alice_ctr=${ctr_prefix}-lite-${alice_domain}
  local bob_ctr=${ctr_prefix}-lite-${bob_domain}
  start_kuscia_container "master" "kuscia-system" "" "" "${master_ctr}" "true" ""
  start_kuscia_container "lite" "${alice_domain}" "${runtime}" "https://${master_ctr}:1080" "${alice_ctr}" "true"
  start_kuscia_container "lite" "${bob_domain}" "${runtime}" "https://${master_ctr}:1080" "${bob_ctr}" "true"
  create_cluster_domain_route ${alice_domain} ${bob_domain}
  create_cluster_domain_route ${bob_domain} ${alice_domain}
  docker exec -it ${alice_ctr} scripts/deploy/init_example_data.sh ${alice_domain}
  docker exec -it ${bob_ctr} scripts/deploy/init_example_data.sh ${bob_domain}
  log "Kuscia ${mode} cluster started successfully"
}

function start_p2p_cluster() {
  local alice_domain=alice
  local bob_domain=bob
  local ctr_prefix=${USER}-kuscia
  local runtime="runc"
  local p2p_protocol=$1
  local privileged_flag=" --privileged"
  local alice_ctr=${ctr_prefix}-autonomy-${alice_domain}
  local bob_ctr=${ctr_prefix}-autonomy-${bob_domain}
  start_kuscia_container "autonomy" "${alice_domain}" "${runtime}" " " "${alice_ctr}" "true"
  start_kuscia_container "autonomy" "${bob_domain}" "${runtime}" " " "${bob_ctr}" "true"
  build_interconn ${bob_ctr} ${alice_ctr} ${alice_domain} ${bob_domain} ${p2p_protocol}
  build_interconn ${alice_ctr} ${bob_ctr} ${bob_domain} ${alice_domain} ${p2p_protocol}
  docker exec -it ${alice_ctr} scripts/deploy/init_example_data.sh ${alice_domain}
  docker exec -it ${bob_ctr} scripts/deploy/init_example_data.sh ${bob_domain}
  log "Kuscia ${mode} cluster started successfully"
}

function start_cxc_cluster() {
  local alice_domain=alice
  local bob_domain=bob
  local ctr_prefix=${USER}-kuscia
  local runtime="runc"
  local privileged_flag=" --privileged"
  local alice_ctr=${ctr_prefix}-lite-cxc-${alice_domain}
  local bob_ctr=${ctr_prefix}-lite-cxc-${bob_domain}
  local alice_master_domain="master-cxc-alice"
  local bob_master_domain="master-cxc-bob"
  local alice_master_ctr=${ctr_prefix}-${alice_master_domain}
  local bob_master_ctr=${ctr_prefix}-${bob_master_domain}
  local p2p_protocol="kuscia"
  local transit=$1

  start_kuscia_container "master" "${alice_master_domain}" "" "" "${alice_master_ctr}" "true"
  start_kuscia_container "master" "${bob_master_domain}" "" "" "${bob_master_ctr}" "true"
  start_kuscia_container "lite" "${alice_domain}" "${runtime}" "https://${alice_master_ctr}:1080" "${alice_ctr}" "true"
  start_kuscia_container "lite" "${bob_domain}" "${runtime}" "https://${bob_master_ctr}:1080" "${bob_ctr}" "true"

  build_interconn ${bob_master_ctr} ${alice_master_ctr} ${alice_master_domain} ${bob_master_domain} ${p2p_protocol}
  build_interconn ${alice_master_ctr} ${bob_master_ctr} ${bob_master_domain} ${alice_master_domain} ${p2p_protocol}
  copy_between_containers ${alice_ctr}:${CTR_CERT_ROOT}/domain.crt ${bob_master_ctr}:${CTR_CERT_ROOT}/${alice_domain}.domain.crt
  copy_between_containers ${bob_ctr}:${CTR_CERT_ROOT}/domain.crt ${alice_master_ctr}:${CTR_CERT_ROOT}/${bob_domain}.domain.crt
  docker exec -it ${alice_master_ctr} scripts/deploy/add_domain.sh ${bob_domain} p2p ${p2p_protocol} ${bob_master_domain}
  docker exec -it ${bob_master_ctr} scripts/deploy/add_domain.sh ${alice_domain} p2p ${p2p_protocol} ${alice_master_domain}
  if [[ $transit == false ]]; then
    docker exec -it ${alice_master_ctr} scripts/deploy/join_to_host.sh ${alice_domain} ${bob_domain} http://${bob_ctr}:1080 -i false -p ${p2p_protocol}
    docker exec -it ${bob_master_ctr} scripts/deploy/join_to_host.sh ${bob_domain} ${alice_domain} http://${alice_ctr}:1080 -i false -p ${p2p_protocol}
    docker exec -it ${bob_master_ctr} scripts/deploy/join_to_host.sh ${alice_domain} ${bob_domain} http://${bob_ctr}:1080 -i false -p ${p2p_protocol}
    docker exec -it ${alice_master_ctr} scripts/deploy/join_to_host.sh ${bob_domain} ${alice_domain} http://${alice_ctr}:1080 -i false -p ${p2p_protocol}
  else
    docker exec -it ${bob_master_ctr} scripts/deploy/join_to_host.sh ${bob_master_domain} ${bob_domain} http://${bob_ctr}:1080 -i false -p ${p2p_protocol}
    docker exec -it ${alice_master_ctr} scripts/deploy/join_to_host.sh ${alice_master_domain} ${bob_domain} http://${bob_ctr}:1080 -i false -x ${bob_master_domain} -p ${p2p_protocol}
    docker exec -it ${alice_master_ctr} scripts/deploy/join_to_host.sh ${alice_domain} ${bob_domain} http://${bob_ctr}:1080 -i false -x ${alice_master_domain} -p ${p2p_protocol}
    docker exec -it ${bob_master_ctr} scripts/deploy/join_to_host.sh ${alice_domain} ${bob_domain} http://${bob_ctr}:1080 -i false -p ${p2p_protocol} -x $alice_master_domain
    docker exec -it ${bob_master_ctr} scripts/deploy/join_to_host.sh $alice_master_domain ${bob_domain} http://${bob_ctr}:1080 -i false -p ${p2p_protocol} -x $bob_master_domain
    docker exec -it ${alice_master_ctr} scripts/deploy/join_to_host.sh ${alice_master_domain} ${alice_domain} http://${alice_ctr}:1080 -i false -p ${p2p_protocol}
    docker exec -it ${bob_master_ctr} scripts/deploy/join_to_host.sh ${bob_master_domain} ${alice_domain} http://${alice_ctr}:1080 -i false -x ${alice_master_domain} -p ${p2p_protocol}
    docker exec -it ${bob_master_ctr} scripts/deploy/join_to_host.sh ${bob_domain} ${alice_domain} http://${alice_ctr}:1080 -i false -x ${bob_master_domain} -p ${p2p_protocol}
    docker exec -it ${alice_master_ctr} scripts/deploy/join_to_host.sh ${bob_domain} ${alice_domain} http://${alice_ctr}:1080 -i false -p ${p2p_protocol} -x $bob_master_domain
    docker exec -it ${alice_master_ctr} scripts/deploy/join_to_host.sh $bob_master_domain ${alice_domain} http://${alice_ctr}:1080 -i false -p ${p2p_protocol} -x $alice_master_domain
  fi
  docker exec -it ${alice_ctr} scripts/deploy/init_example_data.sh ${alice_domain}
  docker exec -it ${bob_ctr} scripts/deploy/init_example_data.sh ${bob_domain}
  log "Kuscia ${mode} cluster started successfully"
}

function start_cxp_cluster() {
  local alice_domain=alice
  local bob_domain=bob
  local ctr_prefix=${USER}-kuscia
  local runtime="runc"
  local privileged_flag=" --privileged"
  local alice_ctr=${ctr_prefix}-lite-cxp-${alice_domain}
  local bob_ctr=${ctr_prefix}-autonomy-cxp-${bob_domain}
  local alice_master_domain="master-cxp-alice"
  local alice_master_ctr=${ctr_prefix}-${alice_master_domain}
  local p2p_protocol="kuscia"
  local transit=$1

  start_kuscia_container "master" "${alice_master_domain}" "" "" "${alice_master_ctr}" "true"
  start_kuscia_container "lite" "${alice_domain}" "${runtime}" "https://${alice_master_ctr}:1080" "${alice_ctr}" "true"
  start_kuscia_container "autonomy" "${bob_domain}" "${runtime}" "https://${alice_master_ctr}:1080" "${bob_ctr}" "true"

  build_interconn ${bob_ctr} ${alice_master_ctr} ${alice_master_domain} ${bob_domain} ${p2p_protocol}
  build_interconn ${alice_master_ctr} ${bob_ctr} ${bob_domain} ${alice_master_domain} ${p2p_protocol}
  copy_between_containers ${alice_ctr}:${CTR_CERT_ROOT}/domain.crt ${bob_ctr}:${CTR_CERT_ROOT}/${alice_domain}.domain.crt
  docker exec -it ${bob_ctr} scripts/deploy/add_domain.sh ${alice_domain} p2p ${p2p_protocol} ${alice_master_domain}

  if [[ $transit == false ]]; then
    docker exec -it ${alice_master_ctr} scripts/deploy/join_to_host.sh ${alice_domain} ${bob_domain} https://${bob_ctr}:1080 -i false -p ${p2p_protocol}
    docker exec -it ${bob_ctr} scripts/deploy/join_to_host.sh ${bob_domain} ${alice_domain} http://${alice_ctr}:1080 -i false -p ${p2p_protocol}
    docker exec -it ${bob_ctr} scripts/deploy/join_to_host.sh ${alice_domain} ${bob_domain} http://${bob_ctr}:1080 -i false -p ${p2p_protocol}
    docker exec -it ${alice_master_ctr} scripts/deploy/join_to_host.sh ${bob_domain} ${alice_domain} http://${alice_ctr}:1080 -i false -p ${p2p_protocol}
  else
    docker exec -it ${alice_master_ctr} scripts/deploy/join_to_host.sh ${alice_domain} ${bob_domain} https://${bob_ctr}:1080 -i false -x ${alice_master_domain} -p ${p2p_protocol}
    docker exec -it ${alice_master_ctr} scripts/deploy/join_to_host.sh $alice_master_domain ${alice_domain} http://${alice_ctr}:1080 -i false -p ${p2p_protocol}
    docker exec -it ${bob_ctr} scripts/deploy/join_to_host.sh ${bob_domain} ${alice_domain} http://${alice_ctr}:1080 -i false -x ${alice_master_domain} -p ${p2p_protocol}
    docker exec -it ${bob_ctr} scripts/deploy/join_to_host.sh ${alice_domain} ${bob_domain} https://${bob_ctr}:1080 -i false -x ${alice_master_domain} -p ${p2p_protocol}
    docker exec -it ${alice_master_ctr} scripts/deploy/join_to_host.sh ${bob_domain} ${alice_domain} http://${alice_ctr}:1080 -i false -x ${alice_master_domain} -p ${p2p_protocol}
  fi

  docker exec -it ${alice_ctr} scripts/deploy/init_example_data.sh ${alice_domain}
  docker exec -it ${bob_ctr} scripts/deploy/init_example_data.sh ${bob_domain}
  log "Kuscia ${mode} cluster started successfully"
}

function build_kuscia_network() {
  if [[ ${mode} == "start" ]] && [[ "${CLUSTERED}" == true ]]; then
    # Clustered mode does not create network.
    pre_check_cluster_network
  elif [[ ! "$(docker network ls -q -f name=^${NETWORK_NAME}$)" ]]; then
    docker network create ${NETWORK_NAME}
  fi
}

# pre check docker network
function pre_check_cluster_network() {

  local container_id
  local rm_container=false
  if [[ ! "$(docker network ls -q -f name=${CLUSTER_NETWORK_NAME})" ]]; then
    container_id=$(docker run -dit --rm --network ${CLUSTER_NETWORK_NAME} ${KUSCIA_IMAGE} bash)
    rm_container=true
  fi

  local network_info
  network_info=$(docker network inspect "$CLUSTER_NETWORK_NAME")

  if ! (echo "${network_info}" | grep '"Driver": "overlay"' > /dev/null); then
    echo -e "${RED}Network '${CLUSTER_NETWORK_NAME}' exists, but its Driver is not 'overlay'.${NC}"
    exit 1
  fi

  if ! (echo "${network_info}" | grep '"Attachable": true' > /dev/null); then
    echo -e "${RED}Network '${CLUSTER_NETWORK_NAME}' exists, but its Attachable is not 'true'.${NC}"
    exit 1
  fi

  log "Network '${CLUSTER_NETWORK_NAME}' is overlay and swarm scope type."
  if [ $rm_container = true ] ;then
    docker rm -f "${container_id}" || true
  fi
}

function get_absolute_path() {
  echo "$(
    cd "$(dirname -- "$1")" >/dev/null
    pwd -P
  )/$(basename -- "$1")"
}

usage() {
  echo "$(basename "$0") DEPLOY_MODE [OPTIONS]
DEPLOY_MODE:
    center                   centralized network mode
    p2p                      p2p network mode
    cxc                      center and center mode
    cxp                      center and p2p mode
    start                    Multi-machine mode

Common Options:
    -h,--help         Show this help text.
    -a                Whether to import secretflow image like '-a secretflow'. 'none' indicates that no image is imported.
    -c                The host path of kuscia configure file.  It will be mounted into the domain container.
    -d                The data directory used to store domain data. It will be mounted into the domain container.
                      You can set Env 'DOMAIN_DATA_DIR' instead.  Default is '{{ROOT}}/{{DOMAIN_CONTAINER_NAME}}/data'.
    -l                The data directory used to store domain logs. It will be mounted into the domain container.
                      You can set Env 'DOMAIN_LOG_DIR' instead.  Default is '{{ROOT}}/{{DOMAIN_CONTAINER_NAME}}/logs'.
    -m,--memory-limit Set an appropriate memory limit. For example, '-m 4GiB or --memory-limit=4GiB','-1' means no limit.Default master mode 2GiB,lite mode 4GiB,autonomy mode 6GiB.
    -p                The port exposed by domain. You can set Env 'DOMAIN_HOST_PORT' instead.
    -q                (Only used in autonomy or lite mode)The port exposed for internal use by domain. You can set Env 'DOMAIN_HOST_INTERNAL_PORT' instead.
    -t                Gateway routing forwarding capability. You can set Env 'transit=true' instead.
    -k                (Only used in autonomy or master mode)The http port exposed by KusciaAPI , default is 13082. You can set Env 'KUSCIAAPI_HTTP_PORT' instead.
    -g                (Only used in autonomy or master mode)The grpc port exposed by KusciaAPI, default is 13083. You can set Env 'KUSCIAAPI_GRPC_PORT' instead.
    --cluster         (Only used in Multi-machine mode) This parameter can be used when deploying a single node. In a multi-copy scenario, the cluster network will be used. For example: '--cluster'"
}

mode=
case "$1" in
center | p2p | cxc | cxp | start)
  mode=$1
  shift
  ;;
esac

NEW_ARGS=()

for arg in "$@"; do
    case "$arg" in
        --help)
            usage
            exit 0
            ;;
        --expose-ports)
            EXPOSE_PORTS=true
            ;;
        --memory-limit=*)
            MEMORY_LIMIT="${arg#*=}"
            ;;
        --cluster)
            CLUSTERED=true
            ;;
        *)
            NEW_ARGS+=("$arg")
            ;;
    esac
done

interconn_protocol=
transit=false
set -- "${NEW_ARGS[@]}"
while getopts 'P:a:c:d:l:m:p:q:tk:g:h' option; do
  case "$option" in
  P)
    interconn_protocol=$OPTARG
    [ "$interconn_protocol" == "bfia" -o "$interconn_protocol" == "kuscia" ] && continue
    printf "illegal value for -%s\n" "$option" >&2
    usage
    exit
    ;;
  a)
    IMPORT_SF_IMAGE=$OPTARG
    ;;
  c)
    KUSCIA_CONFIG_FILE=$(get_absolute_path "${OPTARG}")
    ;;
  d)
    DOMAIN_DATA_DIR=$OPTARG
    ;;
  l)
    DOMAIN_LOG_DIR=$OPTARG
    ;;
  m)
    MEMORY_LIMIT=$OPTARG
    ;;
  p)
    DOMAIN_HOST_PORT=$OPTARG
    ;;
  q)
    DOMAIN_HOST_INTERNAL_PORT=$OPTARG
    ;;
  t)
    transit=true
    ;;
  k)
    KUSCIAAPI_HTTP_PORT=$OPTARG
    ;;
  g)
    KUSCIAAPI_GRPC_PORT=$OPTARG
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
if [ "$mode" == "center" -a "$interconn_protocol" != "kuscia" ]; then
  printf "In current quickstart script, center mode just support 'kuscia'\n" >&2
  exit 1
fi

case "$mode" in
center)
  start_center_cluster
  ;;
p2p)
  start_p2p_cluster $interconn_protocol
  ;;
cxc)
  start_cxc_cluster $transit
  ;;
cxp)
  start_cxp_cluster $transit
  ;;
start)
  start_kuscia
  ;;
*)
  printf "unsupported network mode: %s\n" "$mode" >&2
  exit 1
  ;;
esac