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

ROOT_DIR=$HOME/kuscia

GREEN='\033[0;32m'
NC='\033[0m'
RED='\033[31m'

if [[ ${KUSCIA_IMAGE} == "" ]]; then
  KUSCIA_IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia:latest
fi

if [ "$SECRETFLOW_IMAGE" != "" ]; then
  echo -e "SECRETFLOW_IMAGE=${SECRETFLOW_IMAGE}"
fi

CTR_PREFIX=${USER}-kuscia
CTR_ROOT=/home/kuscia
CTR_CERT_ROOT=${CTR_ROOT}/var/certs
MASTER_DOMAIN="kuscia-system"
ALICE_DOMAIN="alice"
BOB_DOMAIN="bob"
MASTER_CTR=${CTR_PREFIX}-master
FORCE_START=false
MASTER_MEMORY_LIMIT=2G
LITE_MEMORY_LIMIT=4G
AUTONOMY_MEMORY_LIMIT=6G
SF_IMAGE_NAME="secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/secretflow-lite-anolis8"
SF_IMAGE_TAG="1.3.0b0"
SF_IMAGE_REGISTRY=""
NETWORK_NAME="kuscia-exchange"
VOLUME_PATH="${ROOT_DIR}"

function log() {
  local log_content=$1
  echo -e "${GREEN}${log_content}${NC}"
}

function arch_check() {
  local arch=$(uname -a)
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
  ctr=$1

  if [[ ! "$(docker ps -a -q -f name=^/${ctr}$)" ]]; then
    # need start your container
    return 0
  fi

  if $FORCE_START; then
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
    echo -e "${RED}installation exit.${NC}"
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
    echo "[Error] Probe k3s in container '$domain_ctr' failed. Please check k3s log in container, path: /home/kuscia/var/logs/k3s.log" >&2
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
  echo "[Error] Probe gateway in namespace '$domain' failed. Please check envoy log in container, path: /home/kuscia/var/logs/envoy" >&2
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

function create_secretflow_app_image() {
  local ctr=$1
  docker exec -it ${ctr} scripts/deploy/create_sf_app_image.sh "${SF_IMAGE_NAME}" "${SF_IMAGE_TAG}"
  log "create secretflow app image done"
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
  local mount_flag="-v ${DOMAIN_DATA_DIR}:/home/kuscia/var/storage/data -v ${DOMAIN_LOG_DIR}:/home/kuscia/var/stdout"
  echo "$mount_flag"
}

function create_cluster_domain_route() {
  local src_domain=$1
  local dest_domain=$2
  log "Starting create cluster domain route from '${src_domain}' to '${dest_domain}'"

  docker exec -it ${MASTER_CTR} scripts/deploy/create_cluster_domain_route.sh ${src_domain} ${dest_domain} http://${CTR_PREFIX}-lite-${dest_domain}:1080
  log "Cluster domain route from '${src_domain}' to '${dest_domain}' created successfully dest_endpoint: '${CTR_PREFIX}'-lite-'${dest_domain}':1080"
}

function check_sf_image() {
  local domain_id=$1
  local domain_ctr=$2
  local volume_path=$3
  local env_file=${ROOT}/env.list
  local default_repo=${SF_IMAGE_REGISTRY}
  local repo
  if [ -e $env_file ]; then
    repo=$(awk -F "=" '/REGISTRY_ENDPOINT/ {print $2}' $env_file)
  fi
  local sf_image="${SF_IMAGE_NAME}:${SF_IMAGE_TAG}"
  if [ "$repo" != "" ]; then
    sf_image="${repo}/${SF_IMAGE_NAME##*/}:${SF_IMAGE_TAG}"
  elif [ "$default_repo" != "" ]; then
    sf_image="${default_repo}/${SF_IMAGE_NAME##*/}:${SF_IMAGE_TAG}"
  fi
  if [ "$SECRETFLOW_IMAGE" != "" ]; then
    sf_image=$SECRETFLOW_IMAGE
  fi

  if docker exec -it $domain_ctr crictl inspecti $sf_image >/dev/null 2>&1; then
    log "Image '${sf_image}' already exists in domain '${domain_id}'"
    return
  fi

  local has_sf_image=false
  if docker image inspect ${sf_image} >/dev/null 2>&1; then
    has_sf_image=true
  fi

  if [ "$has_sf_image" == true ]; then
    log "Found the secretflow image '${sf_image}' on host"
  else
    log "Not found the secretflow image '${sf_image}' on host"
    if [ "$repo" != "" ]; then
      docker login $repo
    fi
    log "Start pulling image '${sf_image}' ..."
    docker pull ${sf_image}
  fi

  log "Start importing image '${sf_image}' Please be patient..."
  local image_id
  image_id=$(docker images --filter="reference=${sf_image}" --format "{{.ID}}")
  local image_tar
  image_tar=/tmp/$(echo ${sf_image} | sed 's/\//_/g').${image_id}.tar
  if [ ! -e $image_tar ]; then
    docker save $sf_image -o $image_tar
  fi
  docker exec -it $domain_ctr ctr -a=${CTR_ROOT}/containerd/run/containerd.sock -n=k8s.io images import $image_tar
  log "Successfully imported image '${sf_image}' to container '${domain_ctr}' ..."
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
  local export_port="-p ${domain_host_internal_port}:80 \
    -p ${domain_host_port}:1080 \
    -p ${kusciaapi_http_port}:8082 \
    -p ${kusciaapi_grpc_port}:8083"
  
  if [[ ${mode} != "start" ]] && [[ "${EXPOSE_PORTS}" != true ]]; then
    export_port=""
  fi
  docker run -dit${privileged_flag} --name="${domain_ctr}" --hostname="${domain_ctr}" --restart=always --network=${NETWORK_NAME} -m $LITE_MEMORY_LIMIT \
    ${export_port} \
    --mount source=${domain_ctr}-containerd,target=${CTR_ROOT}/containerd \
    -v /tmp:/tmp \
    -v ${kuscia_conf_file}:/home/kuscia/etc/conf/kuscia.yaml \
    ${env_flag} ${mount_flag} \
    --env NAMESPACE=${domain_id} \
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
  local env_flag=$(generate_env_flag)
  build_kuscia_network

  if [[ ${init_kuscia_conf_file} = "true" ]]; then
    local kuscia_conf_file="${PWD}/${domain_ctr}/kuscia.yaml"
    init_kuscia_conf_file ${domain_type} ${domain_id} ${domain_ctr} ${kuscia_conf_file} ${master_endpoint}
  fi

  if need_start_docker_container "$domain_ctr"; then
    log "Starting container $domain_ctr ..."
    start_container "${domain_ctr}" "${domain_id}" "${env_flag}" "${kuscia_conf_file}" "${mount_flag}" "${domain_host_port}" "${kusciaapi_http_port}" "${kusciaapi_grpc_port}" "${domain_host_internal_port}"
    if [[ "$domain_type" != "lite" ]]; then
    probe_gateway_crd "${domain_ctr}" "${domain_id}" "${domain_ctr}" 60
    else
    probe_datamesh "${domain_ctr}"
    fi
  fi

  if [[ "$domain_type" != "master" ]] && [[ ${runtime} == "runc" ]]; then
    check_sf_image "${domain_id}" "${domain_ctr}"
  fi

  if [[ "$domain_type" != "lite" ]]; then
    create_secretflow_app_image "${domain_ctr}"
  fi
  log "$domain_type domain '${domain_id}' deployed successfully"
}

function get_config_value() {
  local config_file=$1
  local key=$2
  grep "$key:" "$config_file" | awk '{ print $2 }' | tr -d '\r\n'
}

function start_kuscia() {
  local kuscia_conf_file=${KUSCIA_CONFIG_FILE}
  local domain_id=$(get_config_value "$kuscia_conf_file" "domainID")
  local deploy_mode=$(get_config_value "$kuscia_conf_file" "mode")
  local master_endpoint=$(get_config_value "$kuscia_conf_file" "masterEndpoint")
  local runtime=$(get_runtime "$kuscia_conf_file")
  local privileged_flag
  local domain_host_internal_port=${DOMAIN_HOST_INTERNAL_PORT:-13081}
  local kusciaapi_http_port=${KUSCIAAPI_HTTP_PORT:-13082}
  local kusciaapi_grpc_port=${KUSCIAAPI_GRPC_PORT:-13083}

  wrap_kuscia_config_file ${kuscia_conf_file}
  local domain_ctr="${CTR_PREFIX}-${deploy_mode}-${domain_id}"
  if [[ "${deploy_mode}" == "master" ]]; then
    domain_ctr="${MASTER_CTR}"
  fi

  [[ ${runtime} == "runc" ]] && privileged_flag=" --privileged"  
  [[ ${DOMAIN_HOST_PORT} == "" ]] && { printf "empty domain host port\n" >&2; exit 1; }

  init ${domain_ctr}
  local mount_flag=$(generate_mount_flag)
  start_kuscia_container "${deploy_mode}" "${domain_id}" "$runtime" "$master_endpoint" "${domain_ctr}" "false" "${mount_flag}" "${DOMAIN_HOST_PORT}" "${kusciaapi_http_port}" "${kusciaapi_grpc_port}" "${domain_host_internal_port}"
}

function start_center_cluster() {
  local runtime="runc"
  local privileged_flag=" --privileged"
  local alice_ctr=${CTR_PREFIX}-lite-${ALICE_DOMAIN}
  local bob_ctr=${CTR_PREFIX}-lite-${BOB_DOMAIN}
  start_kuscia_container "master" "${MASTER_DOMAIN}" "" "" "${MASTER_CTR}" "true" ""
  start_kuscia_container "lite" "${ALICE_DOMAIN}" "${runtime}" "https://${MASTER_CTR}:1080" "${alice_ctr}" "true"
  start_kuscia_container "lite" "${BOB_DOMAIN}" "${runtime}" "https://${MASTER_CTR}:1080" "${bob_ctr}" "true"
  create_cluster_domain_route ${ALICE_DOMAIN} ${BOB_DOMAIN}
  create_cluster_domain_route ${BOB_DOMAIN} ${ALICE_DOMAIN}
  docker exec -it ${alice_ctr} scripts/deploy/init_example_data.sh ${ALICE_DOMAIN}
  docker exec -it ${bob_ctr} scripts/deploy/init_example_data.sh ${BOB_DOMAIN}
  log "Kuscia ${mode} cluster started successfully"
}

function start_p2p_cluster() {
  local runtime="runc"
  local p2p_protocol=$1
  local privileged_flag=" --privileged"
  local alice_ctr=${CTR_PREFIX}-autonomy-${ALICE_DOMAIN}
  local bob_ctr=${CTR_PREFIX}-autonomy-${BOB_DOMAIN}
  start_kuscia_container "autonomy" "${ALICE_DOMAIN}" "${runtime}" " " "${alice_ctr}" "true"
  start_kuscia_container "autonomy" "${BOB_DOMAIN}" "${runtime}" " " "${bob_ctr}" "true"
  build_interconn ${bob_ctr} ${alice_ctr} ${ALICE_DOMAIN} ${BOB_DOMAIN} ${p2p_protocol}
  build_interconn ${alice_ctr} ${bob_ctr} ${BOB_DOMAIN} ${ALICE_DOMAIN} ${p2p_protocol}
  docker exec -it ${alice_ctr} scripts/deploy/init_example_data.sh ${ALICE_DOMAIN}
  docker exec -it ${bob_ctr} scripts/deploy/init_example_data.sh ${BOB_DOMAIN}
  log "Kuscia ${mode} cluster started successfully"
}

function start_cxc_cluster() {
  local runtime="runc"
  local privileged_flag=" --privileged"
  local alice_ctr=${CTR_PREFIX}-lite-cxc-${ALICE_DOMAIN}
  local bob_ctr=${CTR_PREFIX}-lite-cxc-${BOB_DOMAIN}
  local alice_master_domain="master-cxc-alice"
  local bob_master_domain="master-cxc-bob"
  local alice_master_ctr=${CTR_PREFIX}-${alice_master_domain}
  local bob_master_ctr=${CTR_PREFIX}-${bob_master_domain}
  local p2p_protocol="kuscia"
  local transit=$1
  
  start_kuscia_container "master" "${alice_master_domain}" "" "" "${alice_master_ctr}" "true"
  start_kuscia_container "master" "${bob_master_domain}" "" "" "${bob_master_ctr}" "true"
  start_kuscia_container "lite" "${ALICE_DOMAIN}" "${runtime}" "https://${alice_master_ctr}:1080" "${alice_ctr}" "true"
  start_kuscia_container "lite" "${BOB_DOMAIN}" "${runtime}" "https://${bob_master_ctr}:1080" "${bob_ctr}" "true"

  build_interconn ${bob_master_ctr} ${alice_master_ctr} ${alice_master_domain} ${bob_master_domain} ${p2p_protocol}
  build_interconn ${alice_master_ctr} ${bob_master_ctr} ${bob_master_domain} ${alice_master_domain} ${p2p_protocol}
  copy_between_containers ${alice_ctr}:${CTR_CERT_ROOT}/domain.crt ${bob_master_ctr}:${CTR_CERT_ROOT}/${ALICE_DOMAIN}.domain.crt
  copy_between_containers ${bob_ctr}:${CTR_CERT_ROOT}/domain.crt ${alice_master_ctr}:${CTR_CERT_ROOT}/${BOB_DOMAIN}.domain.crt
  docker exec -it ${alice_master_ctr} scripts/deploy/add_domain.sh ${BOB_DOMAIN} p2p ${p2p_protocol} ${bob_master_domain}
  docker exec -it ${bob_master_ctr} scripts/deploy/add_domain.sh ${ALICE_DOMAIN} p2p ${p2p_protocol} ${alice_master_domain}
  if [[ $transit == false ]]; then
    docker exec -it ${alice_master_ctr} scripts/deploy/join_to_host.sh ${ALICE_DOMAIN} ${BOB_DOMAIN} http://${bob_ctr}:1080 -i false -p ${p2p_protocol}
    docker exec -it ${bob_master_ctr} scripts/deploy/join_to_host.sh ${BOB_DOMAIN} ${ALICE_DOMAIN} http://${alice_ctr}:1080 -i false -p ${p2p_protocol}
    docker exec -it ${bob_master_ctr} scripts/deploy/join_to_host.sh ${ALICE_DOMAIN} ${BOB_DOMAIN} http://${bob_ctr}:1080 -i false -p ${p2p_protocol}
    docker exec -it ${alice_master_ctr} scripts/deploy/join_to_host.sh ${BOB_DOMAIN} ${ALICE_DOMAIN} http://${alice_ctr}:1080 -i false -p ${p2p_protocol}
  else
    docker exec -it ${bob_master_ctr} scripts/deploy/join_to_host.sh ${bob_master_domain} ${BOB_DOMAIN} http://${bob_ctr}:1080 -i false -p ${p2p_protocol}
    docker exec -it ${alice_master_ctr} scripts/deploy/join_to_host.sh ${alice_master_domain} ${BOB_DOMAIN} http://${bob_ctr}:1080 -i false -x ${bob_master_domain} -p ${p2p_protocol}
    docker exec -it ${alice_master_ctr} scripts/deploy/join_to_host.sh ${ALICE_DOMAIN} ${BOB_DOMAIN} http://${bob_ctr}:1080 -i false -x ${alice_master_domain} -p ${p2p_protocol}
    docker exec -it ${alice_master_ctr} scripts/deploy/join_to_host.sh ${alice_master_domain} ${ALICE_DOMAIN} http://${alice_ctr}:1080 -i false -p ${p2p_protocol}
    docker exec -it ${bob_master_ctr} scripts/deploy/join_to_host.sh ${bob_master_domain} ${ALICE_DOMAIN} http://${alice_ctr}:1080 -i false -x ${alice_master_domain} -p ${p2p_protocol}
    docker exec -it ${bob_master_ctr} scripts/deploy/join_to_host.sh ${BOB_DOMAIN} ${ALICE_DOMAIN} http://${alice_ctr}:1080 -i false -x ${bob_master_domain} -p ${p2p_protocol}
  fi
  docker exec -it ${alice_ctr} scripts/deploy/init_example_data.sh ${ALICE_DOMAIN}
  docker exec -it ${bob_ctr} scripts/deploy/init_example_data.sh ${BOB_DOMAIN}
  log "Kuscia ${mode} cluster started successfully"
}

function start_cxp_cluster() {
  local runtime="runc"
  local privileged_flag=" --privileged"
  local alice_ctr=${CTR_PREFIX}-lite-cxp-${ALICE_DOMAIN}
  local bob_ctr=${CTR_PREFIX}-autonomy-cxp-${BOB_DOMAIN}
  local alice_master_domain="master-cxp-alice"
  local alice_master_ctr=${CTR_PREFIX}-${alice_master_domain}
  local p2p_protocol="kuscia"
  local transit=$1

  start_kuscia_container "master" "${alice_master_domain}" "" "" "${alice_master_ctr}" "true"
  start_kuscia_container "lite" "${ALICE_DOMAIN}" "${runtime}" "https://${alice_master_ctr}:1080" "${alice_ctr}" "true"
  start_kuscia_container "autonomy" "${BOB_DOMAIN}" "${runtime}" "https://${alice_master_ctr}:1080" "${bob_ctr}" "true"

  build_interconn ${bob_ctr} ${alice_master_ctr} ${alice_master_domain} ${BOB_DOMAIN} ${p2p_protocol}
  build_interconn ${alice_master_ctr} ${bob_ctr} ${BOB_DOMAIN} ${alice_master_domain} ${p2p_protocol}
  copy_between_containers ${alice_ctr}:${CTR_CERT_ROOT}/domain.crt ${bob_ctr}:${CTR_CERT_ROOT}/${ALICE_DOMAIN}.domain.crt
  docker exec -it ${bob_ctr} scripts/deploy/add_domain.sh ${ALICE_DOMAIN} p2p ${p2p_protocol} ${alice_master_domain}

  if [[ $transit == false ]]; then
    docker exec -it ${alice_master_ctr} scripts/deploy/join_to_host.sh ${ALICE_DOMAIN} ${BOB_DOMAIN} https://${bob_ctr}:1080 -i false -p ${p2p_protocol}
    docker exec -it ${bob_ctr} scripts/deploy/join_to_host.sh ${BOB_DOMAIN} ${ALICE_DOMAIN} http://${alice_ctr}:1080 -i false -p ${p2p_protocol}
    docker exec -it ${bob_ctr} scripts/deploy/join_to_host.sh ${ALICE_DOMAIN} ${BOB_DOMAIN} http://${bob_ctr}:1080 -i false -p ${p2p_protocol}
    docker exec -it ${alice_master_ctr} scripts/deploy/join_to_host.sh ${BOB_DOMAIN} ${ALICE_DOMAIN} http://${alice_ctr}:1080 -i false -p ${p2p_protocol}
  else
    docker exec -it ${alice_master_ctr} scripts/deploy/join_to_host.sh ${ALICE_DOMAIN} ${BOB_DOMAIN} https://${bob_ctr}:1080 -i false -x ${alice_master_domain} -p ${p2p_protocol}
    docker exec -it ${alice_master_ctr} scripts/deploy/join_to_host.sh $alice_master_domain ${ALICE_DOMAIN} http://${alice_ctr}:1080 -i false -p ${p2p_protocol}
    docker exec -it ${bob_ctr} scripts/deploy/join_to_host.sh ${BOB_DOMAIN} ${ALICE_DOMAIN} http://${alice_ctr}:1080 -i false -x ${alice_master_domain} -p ${p2p_protocol}
  fi

  docker exec -it ${alice_ctr} scripts/deploy/init_example_data.sh ${ALICE_DOMAIN}
  docker exec -it ${bob_ctr} scripts/deploy/init_example_data.sh ${BOB_DOMAIN}
  log "Kuscia ${mode} cluster started successfully"
}

function build_kuscia_network() {
  if [[ ! "$(docker network ls -q -f name=${NETWORK_NAME})" ]]; then
    docker network create ${NETWORK_NAME}
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
    -h,--help       Show this help text.
    -c              The host path of kuscia configure file.  It will be mounted into the domain container.
    -d              The data directory used to store domain data. It will be mounted into the domain container.
                    You can set Env 'DOMAIN_DATA_DIR' instead.  Default is '{{ROOT}}/{{DOMAIN_CONTAINER_NAME}}/data'.
    -l              The data directory used to store domain logs. It will be mounted into the domain container.
                    You can set Env 'DOMAIN_LOG_DIR' instead.  Default is '{{ROOT}}/{{DOMAIN_CONTAINER_NAME}}/logs'.
    -p              The port exposed by domain. You can set Env 'DOMAIN_HOST_PORT' instead.
    -q              (Only used in autonomy or lite mode)The port exposed for internal use by domain. You can set Env 'DOMAIN_HOST_INTERNAL_PORT' instead.
    -r              The install directory. You can set Env 'ROOT' instead. Default is $(pwd).
    -t              (Only used in lite mode) The deploy token. You can set Env 'DOMAIN_TOKEN' instead.
    -k              (Only used in autonomy or master mode)The http port exposed by KusciaAPI , default is 13082. You can set Env 'KUSCIAAPI_HTTP_PORT' instead.
    -g              (Only used in autonomy or master mode)The grpc port exposed by KusciaAPI, default is 13083. You can set Env 'KUSCIAAPI_GRPC_PORT' instead."
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
        *)
            NEW_ARGS+=("$arg")
            ;;
    esac
done

interconn_protocol=
transit=false
set -- "${NEW_ARGS[@]}"
while getopts 'P:c:d:l:p:q:t:r:k:g:h' option; do
  case "$option" in
  P)
    interconn_protocol=$OPTARG
    [ "$interconn_protocol" == "bfia" -o "$interconn_protocol" == "kuscia" ] && continue
    printf "illegal value for -%s\n" "$option" >&2
    usage
    exit
    ;;
  c)
    KUSCIA_CONFIG_FILE=$(get_absolute_path $OPTARG)
    ;;
  d)
    DOMAIN_DATA_DIR=$OPTARG
    ;;
  l)
    DOMAIN_LOG_DIR=$OPTARG
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
  r)
    ROOT_DIR=$OPTARG
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
[ "$mode" == "" ] && mode=$1
[ "$mode" == "" -o "$mode" == "centralized" ] && mode="center"
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