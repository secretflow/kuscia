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

function log() {
  local log_content=$1
  echo -e "${GREEN}${log_content}${NC}"
}

if [[ ${KUSCIA_IMAGE} == "" ]]; then
  KUSCIA_IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia:latest
fi
log "KUSCIA_IMAGE=${KUSCIA_IMAGE}"

if [[ "$SECRETFLOW_IMAGE" == "" ]]; then
  SECRETFLOW_IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/secretflow-lite-anolis8:1.1.0.dev230811
fi
log "SECRETFLOW_IMAGE=${SECRETFLOW_IMAGE}"


CTR_ROOT=/home/kuscia
CTR_CERT_ROOT=${CTR_ROOT}/etc/certs
AUTONOMY_MEMORY_LIMIT=6G
NETWORK_NAME="kuscia-exchange"

function need_start_docker_container() {
  ctr=$1

  if [[ ! "$(docker ps -a -q -f name=^/${ctr}$)" ]]; then
    # need start your container
    return 0
  fi

  read -rp "$(echo -e ${GREEN}The container \'${ctr}\' already exists. Do you need to recreate it? [y/n]: ${NC})" yn
  case $yn in
    [Yy]* )
      log "Remove container ${ctr} ..."
      docker rm -f $ctr
      # need start your container
      return 0 ;;
    * )
      return 1 ;;
  esac

  return 1
}

function do_http_probe() {
  local ctr=$1
  local endpoint=$2
  local max_retry=$3
  local retry=0
  while [ $retry -lt $max_retry ]; do
    local status_code
    # TODO support MTLS
    status_code=$(docker exec -it $ctr curl -k --write-out '%{http_code}' --silent --output /dev/null ${endpoint})
    if [[ $status_code -eq 200 || $status_code -eq 404 || $status_code -eq 401 ]] ; then
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
    echo "[Error] Probe k3s in container '$domain_ctr' failed. Please check the log" >&2
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
    local line_num=$(docker exec -it $master kubectl get gateways -n $domain | grep $gw_name | wc -l | xargs)
    if [[ $line_num == "1" ]] ; then
      return
    fi
    sleep 1
    retry=$((retry + 1))
  done
  echo "[Error] Probe gateway in namespace '$domain' failed. Please check the log" >&2
  exit 1
}

function build_kuscia_network() {
  if [[ ! "$(docker network ls -q -f name=${NETWORK_NAME})" ]]; then
    docker network create ${NETWORK_NAME}
  fi
}

function check_sf_image() {
  local domain_id=$1
  local domain_ctr=$2
  local sf_image=$SECRETFLOW_IMAGE


  if docker exec -it $domain_ctr crictl inspecti $sf_image > /dev/null 2>&1 ; then
    log "Image '${sf_image}' already exists in domain '${domain_id}'"
    return
  fi

  local has_sf_image=false
  if docker image inspect ${sf_image} >/dev/null 2>&1; then
    has_sf_image=true
  fi

  if [ "$has_sf_image" == true ] ; then
    echo -e "Found the secretflow image '${sf_image}' on host"
  else
    echo -e "Not found the secretflow image '${sf_image}' on host"
    log "Start pulling image '${sf_image}' ..."
    docker pull ${sf_image}
  fi

  log "Start importing image '${sf_image}' ..."
  local image_id
  image_id=$(docker images --filter="reference=${sf_image}" --format "{{.ID}}")
  local image_tar
  image_tar=/tmp/$(echo ${sf_image} | sed 's/\//_/g' ).${image_id}.tar
  if [ ! -e $image_tar ] ; then
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

  docker exec -it ${ctr} scripts/deploy/create_sf_app_image.sh "${image_repo}" "${image_tag}"
  log "Create secretflow app image done"
}

function create_domain_datamesh_svc() {
  local ctr=$1
  local domain_id=$2
  local endpoint=$3
  docker exec -it ${ctr} scripts/deploy/create_datamesh_svc.sh ${domain_id} ${endpoint}
  log "Create datamesh service '${domain_id}' done"
}

function create_domaindata_table() {
  local ctr=$1
  local domain_id=$2
  # create domain datasource
  docker exec -it ${ctr} scripts/deploy/create_domain_datasource.sh ${domain_id}
  # create domain data table
  docker exec -it ${ctr} scripts/deploy/create_domaindata_alice_table.sh ${domain_id}
  docker exec -it ${ctr} scripts/deploy/create_domaindata_bob_table.sh ${domain_id}
  log "Create domain data table done"
}

function deploy_autonomy() {
  local domain_id=$1
  local domain_host_ip=$2
  local domain_host_port=$3
  local domain_certs_dir=$4
  local domain_ctr=${USER}-kuscia-autonomy-${domain_id}

  log "DOMAIN_ID=${domain_id}"
  log "DOMAIN_HOST_IP=${domain_host_ip}"
  log "DOMAIN_HOST_PORT=${domain_host_port}"
  log "DOMAIN_CERTS_DIR=${domain_certs_dir}"
  log "Starting container $domain_ctr ..."

  if need_start_docker_container $domain_ctr ; then
    log "Starting container $domain_ctr ..."

    mkdir -p ${domain_certs_dir}

    docker run -it --rm -v ${domain_certs_dir}:/home/kuscia/etc/certs ${KUSCIA_IMAGE} scripts/deploy/init_domain_certs.sh ${domain_id}

    docker run -it --rm -v ${domain_certs_dir}:/home/kuscia/etc/certs ${KUSCIA_IMAGE} scripts/deploy/init_external_tls_cert.sh ${domain_id} IP:${domain_host_ip}

    build_kuscia_network

    docker run -dit --privileged --name=${domain_ctr} --hostname=${domain_ctr} --restart=always --network=${NETWORK_NAME} -m ${AUTONOMY_MEMORY_LIMIT} \
          -p ${domain_host_port}:1080 \
          --env NAMESPACE=${domain_id} \
          --mount source=${domain_ctr}-containerd,target=/home/kuscia/containerd \
          -v ${domain_certs_dir}:${CTR_CERT_ROOT} \
          -v /tmp:/tmp \
          --entrypoint bin/entrypoint.sh \
          ${KUSCIA_IMAGE} tini -- scripts/deploy/start_autonomy.sh ${domain_id}

    probe_gateway_crd ${domain_ctr} ${domain_id} ${domain_ctr} 60

    log "Container ${domain_ctr} started successfully"
  fi

  check_sf_image ${domain_id} ${domain_ctr}

  create_secretflow_app_image ${domain_ctr}

  # create datamesh svc
  create_domain_datamesh_svc ${domain_ctr} ${domain_id} ${domain_ctr}

  # create demo data
  create_domaindata_table ${domain_ctr} ${domain_id}

  log "Autonomy domain '${domain_id}' deployed successfully"
}

usage() {
  echo "$(basename "$0") DEPLOY_MODE [OPTIONS]
DEPLOY_MODE:
    autonomy        Deploy a autonomy domain.

OPTIONS:
    -i              The IP address exposed by the domain. Usually the host IP, default is the IP address of interface eth0.
    -c              The host directory used to store domain certificates, default is 'kuscia-{{DEPLOY_MODE}}-{{DOMAIN_ID}}-certs'. It will be mounted into the domain container.
    -h              Show this help text.
    -p              The port exposed by domain.
    -n              Domain id to be deployed."
}

deploy_mode=
case  "$1" in
autonomy)
  deploy_mode=$1
  shift
  ;;
esac

domain_id=
domain_host_ip=$(ifconfig eth0 | grep -Eo 'inet (addr:)?([0-9]*\.){3}[0-9]*' | grep -Eo '([0-9]*\.){3}[0-9]*' || true )
domain_host_port=
domain_certs_dir=

while getopts 'c:i:n:p:h' option; do
  case "$option" in
  c)
    domain_certs_dir=$OPTARG
    ;;
  i)
    domain_host_ip=$OPTARG
    ;;
  n)
    domain_id=$OPTARG
    ;;
  p)
    domain_host_port=$OPTARG
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


if [[ ${domain_id} == "" ]]; then
    printf "empty domain id\n" >&2
    exit 1
fi

if [[ ${domain_host_ip} == "" ]]; then
    printf "empty domain host IP address\n" >&2
    exit 1
fi

if [[ ${domain_host_port} == "" ]]; then
    printf "empty domain host port\n" >&2
    exit 1
fi

[[ ${domain_certs_dir} == "" ]] && domain_certs_dir="${PWD}/kuscia-${deploy_mode}-${domain_id}-certs"

case ${deploy_mode} in
autonomy)
  deploy_autonomy $domain_id $domain_host_ip $domain_host_port $domain_certs_dir
  ;;
*)
  printf "unsupported network mode: %s\n" "$mode" >&2
  exit 1
;;
esac
