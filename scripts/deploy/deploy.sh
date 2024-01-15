#!/bin/bash
#
# Copyright 2023 Ant Group Co., Ltd.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

set -e

GREEN='\033[0;32m'
NC='\033[0m'
RED='\033[31m'

function log() {
  local log_content=$1
  echo -e "${GREEN}${log_content}${NC}"
}

function arch_check() {
  local arch=$(uname -a)
  if [[ $arch == *"ARM"* ]] || [[ $arch == *"aarch64"* ]]; then
    echo -e "${RED}ARM architecture is not supported by kuscia currently${NC}"
    exit 1
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

if [[ ${KUSCIA_IMAGE} == "" ]]; then
  KUSCIA_IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia:latest
fi
log "KUSCIA_IMAGE=${KUSCIA_IMAGE}"

if [[ "$SECRETFLOW_IMAGE" == "" ]]; then
  SECRETFLOW_IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/secretflow-lite-anolis8:1.3.0.dev20231120
fi
log "SECRETFLOW_IMAGE=${SECRETFLOW_IMAGE}"

SF_IMAGE_REGISTRY=""
CTR_ROOT=/home/kuscia
CTR_CERT_ROOT=${CTR_ROOT}/var/certs
MASTER_MEMORY_LIMIT=2G
LITE_MEMORY_LIMIT=4G
AUTONOMY_MEMORY_LIMIT=6G
NETWORK_NAME="kuscia-exchange"

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

function need_start_docker_container() {
  local ctr=$1

  if [[ ! "$(docker ps -a -q -f name=^/${ctr}$)" ]]; then
    # need start your container
    return 0
  fi

  read -rp "$(log "The container '${ctr}' already exists. Do you need to recreate it? [y/n]:")" yn
  case $yn in
  [Yy]*)
    log "Remove container ${ctr} ..."
    docker rm -f "$ctr"
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
  local retry=0
  while [ $retry -lt "$max_retry" ]; do
    local status_code
    status_code=$(docker exec -it $ctr curl -k --write-out '%{http_code}' --silent --output /dev/null "${endpoint}")
    if [[ $status_code -eq 200 || $status_code -eq 404 || $status_code -eq 401 ]]; then
      return 0
    fi
    sleep 1
    retry=$((retry + 1))
  done

  return 1
}

function do_https_probe() {
  local ctr=$1
  local endpoint=$2
  local max_retry=$3
  local retry=0
  while [ $retry -lt "$max_retry" ]; do
    local status_code
    status_code=$(docker exec -it $ctr curl -k --write-out '%{http_code}' --silent --output /dev/null "${endpoint}"   --cacert ${CTR_CERT_ROOT}/ca.crt --cert ${CTR_CERT_ROOT}/ca.crt --key ${CTR_CERT_ROOT}/ca.key )
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

  if ! do_http_probe "$domain_ctr" "https://127.0.0.1:6443" 60; then
    echo "[Error] Probe k3s in container '$domain_ctr' failed. Please check k3s log in container, path: /home/kuscia/var/logs/k3s.log" >&2
    exit 1
  fi
}

function probe_gateway_crd() {
  local master=$1
  local domain=$2
  local gw_name=$3
  local max_retry=$4
  probe_k3s "$master"

  local retry=0
  while [ $retry -lt $max_retry ]; do
    local line_num
    line_num=$(docker exec -it $master kubectl get gateways -n $domain | grep -i $gw_name | wc -l | xargs)
    if [[ $line_num == "1" ]]; then
      return
    fi
    sleep 1
    retry=$((retry + 1))
  done
  echo "[Error] Probe gateway in namespace '$domain' failed. Please check envoy log in container, path: /home/kuscia/var/logs/envoy" >&2
  exit 1
}

function build_kuscia_network() {
  if [[ ! "$(docker network ls -q -f name=${NETWORK_NAME})" ]]; then
    docker network create ${NETWORK_NAME}
  fi
}

function check_sf_image() {
  local domain_ctr=$1
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
    log "Image '${sf_image}' already exists in domain '${DOMAIN_ID}'"
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

function create_secretflow_app_image() {
  local ctr=$1
  local image_repo=$SECRETFLOW_IMAGE
  local image_tag=latest

  if [[ "${SECRETFLOW_IMAGE}" == *":"* ]]; then
    image_repo=${SECRETFLOW_IMAGE%%:*}
    image_tag=${SECRETFLOW_IMAGE##*:}
  fi

  app_type=$(echo "${image_repo}" | awk -F'/' '{print $NF}'  |awk -F'-' '{print $1}')
  if [[ ${app_type} == "" ]]; then
    app_type="secretflow"
  fi

  docker exec -it "${ctr}" scripts/deploy/create_sf_app_image.sh "${image_repo}" "${image_tag}"  "${app_type}"
  log "Create secretflow app image done"
}

function probe_datamesh() {
  local domain_ctr=$1
  if ! do_https_probe "$domain_ctr" "https://127.0.0.1:8070/healthZ" 30; then
    echo "[Error] Probe datamesh in container '$domain_ctr' failed." >&2
    echo "You cloud run command that 'docker logs $domain_ctr' to check the log" >&2
  fi
  log "Probe datamesh successfully"
}

function create_domaindata_table() {
  local ctr=$1

  # create domain data table
  docker exec -it "${ctr}" scripts/deploy/create_domaindata_alice_table.sh "${DOMAIN_ID}"
  docker exec -it "${ctr}" scripts/deploy/create_domaindata_bob_table.sh "${DOMAIN_ID}"
  log "Create domain data table done"
}

function generate_env_flag() {
  local env_flag
  local env_file=${ROOT}/env.list
  if [ -e "$env_file" ]; then
    env_flag="--env-file $env_file"
  else
    env_flag="--env REGISTRY_ENDPOINT=${SF_IMAGE_REGISTRY}"
  fi
  echo "$env_flag"
}

function getIPV4Address() {
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
  echo $ipv4
}

function generate_mount_flag() {
  local mount_flag="-v /tmp:/tmp -v ${DOMAIN_CERTS_DIR}:${CTR_CERT_ROOT} -v ${DOMAIN_DATA_DIR}:/home/kuscia/var/storage/data -v ${DOMAIN_LOG_DIR}:/home/kuscia/var/stdout"
  echo "$mount_flag"
}

function get_runtime() {
  local conf_file=$1
  local runtime
  runtime=$(grep '^runtime:' ${conf_file} | cut -d':' -f2 | awk '{$1=$1};1')
  if [[ $runtime == "" ]]; then
    runtime=runc
  fi
  echo "$runtime"
}

function deploy_autonomy() {
  local domain_ctr=${USER}-kuscia-autonomy-${DOMAIN_ID}
  local conf_dir=${ROOT}/${domain_ctr}
  local kuscia_conf_file=${conf_dir}/kuscia.yaml
  if [[ ${KUSCIA_CONFIG_FILE} != "" ]]; then
    kuscia_conf_file=${KUSCIA_CONFIG_FILE}
  fi
  local runtime
  runtime=$(get_runtime ${kuscia_conf_file})
  arch_check
  if need_start_docker_container "$domain_ctr"; then
    log "Starting container $domain_ctr ..."
    env_flag=$(generate_env_flag)
    mount_flag=$(generate_mount_flag)
    host_ip=$(getIPV4Address)

    if [[ ${KUSCIA_CONFIG_FILE} == "" ]]; then
      # TODO: to be remove
      docker run -it --rm -v ${conf_dir}:/tmp -v ${DOMAIN_CERTS_DIR}:${CTR_CERT_ROOT} ${KUSCIA_IMAGE} scripts/deploy/init_kuscia_config.sh autonomy ${DOMAIN_ID} "" "" "${ALLOW_PRIVILEGED}"
    fi

    local privileged_flag
    if [[ ${runtime} == "runc" ]]; then
      privileged_flag=" --privileged"
    fi

    docker run -dit${privileged_flag} --name="${domain_ctr}" --hostname="${domain_ctr}" --restart=always --network=${NETWORK_NAME} -m ${AUTONOMY_MEMORY_LIMIT} \
      -p "${DOMAIN_HOST_PORT}":1080 \
      -p "${KUSCIAAPI_HTTP_PORT}":8082 \
      -p "${KUSCIAAPI_GRPC_PORT}":8083 \
      --mount source="${domain_ctr}"-containerd,target=/home/kuscia/containerd \
      -v ${kuscia_conf_file}:/home/kuscia/etc/conf/kuscia.yaml \
      ${env_flag} ${mount_flag} \
      --env NAMESPACE=${DOMAIN_ID} \
      "${KUSCIA_IMAGE}" bin/kuscia start -c etc/conf/kuscia.yaml

    docker exec -it "${domain_ctr}" sh scripts/deploy/init_kusciaapi_client_certs.sh
    probe_gateway_crd "${domain_ctr}" "${DOMAIN_ID}" "${domain_ctr}" 60

    log "Container ${domain_ctr} started successfully"
  fi

  if [[ ${runtime} == "runc" ]]; then
    check_sf_image "${domain_ctr}"
  fi

  create_secretflow_app_image "${domain_ctr}"

  # create demo data
  create_domaindata_table "${domain_ctr}"

  log "Autonomy domain '${DOMAIN_ID}' deployed successfully"
}

function deploy_lite() {
  local domain_ctr=${USER}-kuscia-lite-${DOMAIN_ID}
  local conf_dir=${ROOT}/${domain_ctr}
  local kuscia_conf_file=${conf_dir}/kuscia.yaml
  if [[ ${KUSCIA_CONFIG_FILE} != "" ]]; then
    kuscia_conf_file=${KUSCIA_CONFIG_FILE}
  fi
  local runtime
  runtime=$(get_runtime ${kuscia_conf_file})
  local HttpResponseCode=$(docker run -it --rm  --network=${NETWORK_NAME} ${KUSCIA_IMAGE} curl -k -s -o /dev/null -w "%{http_code}" ${MASTER_ENDPOINT})
  arch_check

  if need_start_docker_container "$domain_ctr"; then
    log "Starting container $domain_ctr ..."

    env_flag=$(generate_env_flag)
    mount_flag=$(generate_mount_flag)

    if [[ $HttpResponseCode = "401" ]]; then
      echo -e "${GREEN}Communication with master is normal, response code is 401${NC}"
    else
      echo -e "${RED}Failed to connect to the master. Please check if the network link to the master is normal. Please refer to the kuscia documentation (https://www.secretflow.org.cn/docs/kuscia/latest/zh-Hans/deployment/deploy_master_lite_cn) for the correct return results${NC}"
      docker run -it --rm  --network=${NETWORK_NAME} ${KUSCIA_IMAGE} curl -kvvv ${MASTER_ENDPOINT}
      exit 1
    fi

    host_ip=$(getIPV4Address)
    if [[ ${KUSCIA_CONFIG_FILE} == "" ]]; then
      # TODO: to be remove
      docker run -it --rm -v ${conf_dir}:/tmp -v ${DOMAIN_CERTS_DIR}:${CTR_CERT_ROOT} ${KUSCIA_IMAGE} scripts/deploy/init_kuscia_config.sh lite ${DOMAIN_ID} ${MASTER_ENDPOINT} ${DOMAIN_TOKEN} "${ALLOW_PRIVILEGED}"
      # TODO end
    fi

    local privileged_flag
    if [[ ${runtime} == "runc" ]]; then
      privileged_flag=" --privileged"
    fi

    docker run -dit${privileged_flag} --name="${domain_ctr}" --hostname="${domain_ctr}" --restart=always --network=${NETWORK_NAME} -m $LITE_MEMORY_LIMIT \
      -p "${DOMAIN_HOST_PORT}":1080 \
      -p "${KUSCIAAPI_HTTP_PORT}":8082 \
      -p "${KUSCIAAPI_GRPC_PORT}":8083 \
      --mount source=${domain_ctr}-containerd,target=${CTR_ROOT}/containerd \
      -v ${kuscia_conf_file}:/home/kuscia/etc/conf/kuscia.yaml \
      ${env_flag} ${mount_flag} \
      --env NAMESPACE=${DOMAIN_ID} \
      "${KUSCIA_IMAGE}" bin/kuscia start -c etc/conf/kuscia.yaml

    docker exec -it "${domain_ctr}" sh scripts/deploy/init_kusciaapi_client_certs.sh
    probe_datamesh "$domain_ctr"

    log "Lite domain '${DOMAIN_ID}' started successfully"
  fi

  if [[ ${runtime} == "runc" ]]; then
    check_sf_image "${domain_ctr}"
  fi

  log "Lite domain '${DOMAIN_ID}' deployed successfully"
}

function deploy_master() {
  local domain_ctr=${USER}-kuscia-master
  local master_domain_id=kuscia-system
  local deploy_secretpad=${DEPLOY_SECRETPAD}
  local conf_dir=${ROOT}/${domain_ctr}
  local kuscia_conf_file=${conf_dir}/kuscia.yaml
  if [[ ${KUSCIA_CONFIG_FILE} != "" ]]; then
    kuscia_conf_file=${KUSCIA_CONFIG_FILE}
  fi
  arch_check

  if need_start_docker_container "${domain_ctr}"; then
    log "Starting container ${domain_ctr} ..."

    env_flag=$(generate_env_flag)
    mount_flag=$(generate_mount_flag)

    host_ip=$(getIPV4Address)

    if [[ ${KUSCIA_CONFIG_FILE} == "" ]]; then
      # TODO: to be remove
      docker run -it --rm -v ${conf_dir}:/tmp -v ${DOMAIN_CERTS_DIR}:${CTR_CERT_ROOT} ${KUSCIA_IMAGE} scripts/deploy/init_kuscia_config.sh master $master_domain_id
    fi

    docker run -dit --name="${domain_ctr}" --hostname="${domain_ctr}" --restart=always --network=${NETWORK_NAME} -m ${MASTER_MEMORY_LIMIT} \
      --env NAMESPACE=${master_domain_id} \
      -p "${DOMAIN_HOST_PORT}":1080 \
      -p "${KUSCIAAPI_HTTP_PORT}":8082 \
      -p "${KUSCIAAPI_GRPC_PORT}":8083 \
      -v ${kuscia_conf_file}:/home/kuscia/etc/conf/kuscia.yaml \
      ${env_flag} ${mount_flag} \
      "${KUSCIA_IMAGE}" bin/kuscia start -c etc/conf/kuscia.yaml

    docker exec -it "${domain_ctr}" sh scripts/deploy/init_kusciaapi_client_certs.sh
    probe_gateway_crd "${domain_ctr}" ${master_domain_id} "${domain_ctr}" 60
    log "Master '${master_domain_id}' started successfully"
  fi
  create_secretflow_app_image "${domain_ctr}"
  log "Master deployed successfully"

  if [[ ${deploy_secretpad} = "web" ]]; then
    source ./start_secretpad.sh
  fi
}

function deploy_secretpad() {
  local deploy_secretpad=${DEPLOY_SECRETPAD}
  if [[ ${deploy_secretpad} = "web" ]]; then
    source ./start_secretpad.sh
  fi
}

function get_absolute_path() {
  echo "$(cd "$(dirname -- "$1")" >/dev/null; pwd -P)/$(basename -- "$1")"
}

usage() {
  echo "$(basename "$0") DEPLOY_MODE [OPTIONS]
DEPLOY_MODE:
    autonomy        Deploy a autonomy domain.
    lite            Deploy a lite domain.
    master          Deploy a master.
    secretpad       Deploy a webui.

OPTIONS:
    -c              The host path of kuscia configure file.  It will be mounted into the domain container.
    -s              The host directory used to store domain certificates. It will be mounted into the domain container.
                    You can set Env 'DOMAIN_CERTS_DIR' instead. Default is '{{ROOT}}/kuscia-{{DEPLOY_MODE}}-{{DOMAIN_ID}}-certs'.
    -d              The data directory used to store domain data. It will be mounted into the domain container.
                    You can set Env 'DOMAIN_DATA_DIR' instead.  Default is '{{ROOT}}/kuscia-{{DEPLOY_MODE}}-{{DOMAIN_ID}}-data'.
    -h              Show this help text.
    -i              The IP address exposed by the domain. You can set Env 'DOMAIN_HOST_IP' instead.
                    Usually the host IP, default is the IP address of interface eth0.
    -l              The data directory used to store domain logs. It will be mounted into the domain container.
                    You can set Env 'DOMAIN_LOG_DIR' instead.  Default is '{{ROOT}}/kuscia-{{DEPLOY_MODE}}-{{DOMAIN_ID}}-log'.
    -m              (Only used in lite mode) The master endpoint. You can set Env 'MASTER_ENDPOINT' instead.
    -n              Domain id to be deployed. You can set Env 'DOMAIN_ID' instead.
    -p              The port exposed by domain. You can set Env 'DOMAIN_HOST_PORT' instead.
    -r              The install directory. You can set Env 'ROOT' instead. Default is $(pwd).
    -t              (Only used in lite mode) The deploy token. You can set Env 'DOMAIN_TOKEN' instead.
    -k              (Only used in autonomy or master mode)The http port exposed by KusciaAPI , default is 13082. You can set Env 'KUSCIAAPI_HTTP_PORT' instead.
    -g              (Only used in autonomy or master mode)The grpc port exposed by KusciaAPI, default is 13083. You can set Env 'KUSCIAAPI_GRPC_PORT' instead.
    -e              (Only used in autonomy or master mode)The extra subjectAltName for KusciaAPI server cert.
    -u              'web' means install kuscia with web UI, user could access the website (http://127.0.0.1:8088) to experience the web features via the webui.
    "
}

deploy_mode=
case "$1" in
autonomy | lite | master | secretpad)
  deploy_mode=$1
  shift
  ;;
-h)
  usage
  exit
  ;;
*)
  echo "deploy_mode is invalid, must be autonomy, lite, secretpad or master"
  usage
  exit 1
  ;;
esac

while getopts 'c:s:i:l:n:p:m:t:r:d:k:g:e:u:h' option; do
  case "$option" in
  c)
    KUSCIA_CONFIG_FILE=$(get_absolute_path $OPTARG)
    ;;
  s)
    DOMAIN_CERTS_DIR=$OPTARG
    ;;
  i)
    DOMAIN_HOST_IP=$OPTARG
    ;;
  l)
    DOMAIN_LOG_DIR=$OPTARG
    ;;
  n)
    DOMAIN_ID=$OPTARG
    ;;
  p)
    DOMAIN_HOST_PORT=$OPTARG
    ;;
  m)
    MASTER_ENDPOINT=$OPTARG
    ;;
  t)
    DOMAIN_TOKEN=$OPTARG
    ;;
  r)
    ROOT=$OPTARG
    ;;
  d)
    DOMAIN_DATA_DIR=$OPTARG
    ;;
  k)
    KUSCIAAPI_HTTP_PORT=$OPTARG
    ;;
  g)
    KUSCIAAPI_GRPC_PORT=$OPTARG
    ;;
  e)
    KUSCIAAPI_EXTRA_SUBJECT_ALTNAME=$OPTARG
    ;;
  u)
    DEPLOY_SECRETPAD=$OPTARG
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

if [[ ${DOMAIN_HOST_PORT} == "" ]]; then
  printf "empty domain host port\n" >&2
  exit 1
fi

if [[ ${KUSCIAAPI_HTTP_PORT} == "" ]]; then
  KUSCIAAPI_HTTP_PORT=13082
fi

if [[ ${KUSCIAAPI_GRPC_PORT} == "" ]]; then
  KUSCIAAPI_GRPC_PORT=13083
fi

function init() {
  local deploy_mode=$1
  [[ ${ROOT} == "" ]] && ROOT=${PWD}
  [[ ${DOMAIN_CERTS_DIR} == "" ]] && DOMAIN_CERTS_DIR="${ROOT}/kuscia-${deploy_mode}-${DOMAIN_ID}-certs"
  [[ ${DOMAIN_DATA_DIR} == "" ]] && DOMAIN_DATA_DIR="${ROOT}/kuscia-${deploy_mode}-${DOMAIN_ID}-data"
  [[ ${DOMAIN_LOG_DIR} == "" ]] && DOMAIN_LOG_DIR="${ROOT}/kuscia-${deploy_mode}-${DOMAIN_ID}-log"
  [[ ${DOMAIN_HOST_IP} == "" ]] && DOMAIN_HOST_IP=$(getIPV4Address)

  pre_check "${DOMAIN_CERTS_DIR}"
  pre_check "${DOMAIN_DATA_DIR}"
  pre_check "${DOMAIN_LOG_DIR}"

  log "ROOT=${ROOT}"
  log "DOMAIN_ID=${DOMAIN_ID}"
  log "DOMAIN_HOST_IP=${DOMAIN_HOST_IP}"
  log "DOMAIN_HOST_PORT=${DOMAIN_HOST_PORT}"
  log "DOMAIN_DATA_DIR=${DOMAIN_DATA_DIR}"
  log "DOMAIN_LOG_DIR=${DOMAIN_LOG_DIR}"
  log "DOMAIN_CERTS_DIR=${DOMAIN_CERTS_DIR}"
  log "KUSCIA_IMAGE=${KUSCIA_IMAGE}"
  log "KUSCIAAPI_HTTP_PORT=${KUSCIAAPI_HTTP_PORT}"
  log "KUSCIAAPI_GRPC_PORT=${KUSCIAAPI_GRPC_PORT}"

  build_kuscia_network
}

case ${deploy_mode} in
autonomy)
  if [[ ${DOMAIN_ID} == "" ]]; then
    printf "empty domain id\n" >&2
    exit 1
  fi
  init ${deploy_mode}
  deploy_autonomy
  ;;
lite)
  if [[ ${DOMAIN_ID} == "" ]]; then
    printf "empty domain id\n" >&2
    exit 1
  fi
  if [[ ${DOMAIN_TOKEN} == "" ]]; then
    printf "Empty domain token\n" >&2
    exit 1
  fi
  if [[ ${MASTER_ENDPOINT} == "" ]]; then
    printf "Empty master endpoint\n" >&2
    exit 1
  fi

  init ${deploy_mode}
  deploy_lite
  ;;
master)
  DOMAIN_ID=kuscia-system
  init ${deploy_mode}
  deploy_master
  ;;
secretpad)
  deploy_secretpad
  ;;
*)
  printf "unsupported network mode: %s\n" "$deploy_mode" >&2
  exit 1
  ;;
esac
