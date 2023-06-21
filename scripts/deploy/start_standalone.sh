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

ROOT=$HOME/kuscia
mkdir -p $ROOT

GREEN='\033[0;32m'
NC='\033[0m'

IMAGE=secretflow/kuscia
if [ "${KUSCIA_IMAGE}" != "" ]; then
  IMAGE=${KUSCIA_IMAGE}
fi

echo -e "${GREEN}IMAGE=${IMAGE}${NC}"

CTR_PREFIX=${USER}-kuscia
CTR_ROOT=/home/kuscia
CTR_CERT_ROOT=${CTR_ROOT}/etc/certs
MASTER_DOMAIN="kuscia-system"
ALICE_DOMAIN="alice"
BOB_DOMAIN="bob"
MASTER_CTR=${CTR_PREFIX}-master
FORCE_START=false
MASTER_MEMORY_LIMIT=2G
LITE_MEMORY_LIMIT=4G
AUTONOMY_MEMORY_LIMIT=6G
SF_IMAGE_NAME="secretflow/secretflow-lite-anolis8"
SF_IMAGE_TAG="0.8.3b0"
NETWORK_NAME="kuscia-exchange"

function need_start_docker_container() {
  ctr=$1

  if [[ ! "$(docker ps -a -q -f name=^/${ctr}$)" ]]; then
    # need start your container
    return 0
  fi

  if $FORCE_START ; then
    echo -e "${GREEN}Remove container ${ctr} ...${NC}"
    docker rm -f $ctr
    # need start your container
    return 0
  fi

  read -rp "$(echo -e ${GREEN}The container \'${ctr}\' already exists. Do you need to recreate it? [y/n]: ${NC})" yn
  case $yn in
    [Yy]* )
      echo -e "${GREEN}Remove container ${ctr} ...${NC}"
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

function generate_env_flag() {
  local domain_id=$1
  local env_flag
  local env_file=${ROOT}/env.list
  if [ -e $env_file ] ; then
    env_flag="--env-file $env_file"
  fi
  echo $env_flag
}

function copy_between_containers() {
  local src_file=$1
  local dest_file=$2
  local temp_file
  temp_file=$(basename $dest_file)
  docker cp $src_file /tmp/${temp_file}
  docker cp /tmp/${temp_file} $dest_file
  rm /tmp/${temp_file}
}

function copy_container_file_to_volume() {
  local src_file=$1
  local dest_volume=$2
  local dest_file=$3
  docker run -d --rm --name ${CTR_PREFIX}-dummy --mount source=${dest_volume},target=/tmp/kuscia $IMAGE tail -f /dev/null >/dev/null 2>&1
  copy_between_containers ${src_file} ${CTR_PREFIX}-dummy:/tmp/kuscia/${dest_file}
  docker rm -f ${CTR_PREFIX}-dummy
}

function copy_volume_file_to_container() {
  local src_volume=$1
  local src_file=$2
  local dest_file=$3
  docker run -d --rm --name ${CTR_PREFIX}-dummy --mount source=${src_volume},target=/tmp/kuscia $IMAGE tail -f /dev/null >/dev/null 2>&1
  copy_between_containers ${CTR_PREFIX}-dummy:/tmp/kuscia/${src_file} ${dest_file}
  docker rm -f ${CTR_PREFIX}-dummy
}

function start_lite() {
  local domain_id=$1
  local master_endpoint=$2
  local domain_ctr=${CTR_PREFIX}-lite-${domain_id}

  if need_start_docker_container $domain_ctr ; then
    echo -e "${GREEN}Starting container $domain_ctr ...${NC}"
    local certs_volume=${domain_ctr}-certs
    env_flag=$(generate_env_flag $domain_id)
    docker run -it --rm --mount source=${certs_volume},target=${CTR_CERT_ROOT} ${IMAGE} scripts/deploy/init_domain_certs.sh ${domain_id}
    copy_volume_file_to_container $certs_volume domain.csr ${MASTER_CTR}:${CTR_CERT_ROOT}/${domain_id}.domain.csr
    docker exec -it ${MASTER_CTR} kubectl create ns $domain_id
    docker exec -it ${MASTER_CTR} scripts/deploy/add_domain.sh $domain_id

    copy_container_file_to_volume ${MASTER_CTR}:${CTR_CERT_ROOT}/${domain_id}.domain.crt $certs_volume domain.crt
    copy_container_file_to_volume ${MASTER_CTR}:${CTR_CERT_ROOT}/ca.crt $certs_volume master.ca.crt
  
    docker run -dit --privileged --name=${domain_ctr} --hostname=${domain_ctr} --restart=always --network=${NETWORK_NAME} -m $LITE_MEMORY_LIMIT ${env_flag} \
      --env NAMESPACE=${domain_id} \
      --mount source=${domain_ctr}-containerd,target=${CTR_ROOT}/containerd \
      --mount source=${certs_volume},target=${CTR_CERT_ROOT} \
      -v /tmp:/tmp \
      --entrypoint bin/entrypoint.sh \
      ${IMAGE} tini -- scripts/deploy/start_lite.sh ${domain_id} ${MASTER_DOMAIN} ${master_endpoint} ${ALLOW_PRIVILEGED}
    probe_gateway_crd ${MASTER_CTR} ${domain_id} ${domain_ctr} 60
    echo -e "${GREEN}Lite domain '${domain_id}' started successfully${NC}"
  fi
}

function create_cluster_domain_route() {
  local src_domain=$1
  local dest_domain=$2
  local src_ctr=${CTR_PREFIX}-lite-${src_domain}
  local dest_ctr=${CTR_PREFIX}-lite-${dest_domain}
  local src_domain_csr=${CTR_CERT_ROOT}/${src_domain}.domain.csr
  local src_2_dest_cert=${CTR_CERT_ROOT}/${src_domain}-2-${dest_domain}.crt
  local dest_ca=${CTR_CERT_ROOT}/${dest_domain}.ca.crt

  copy_between_containers ${src_ctr}:${CTR_CERT_ROOT}/domain.csr ${dest_ctr}:${src_domain_csr}
  docker exec -it ${dest_ctr} openssl x509 -req -in $src_domain_csr -CA ${CTR_CERT_ROOT}/ca.crt -CAkey ${CTR_CERT_ROOT}/ca.key -CAcreateserial -days 10000 -out ${src_2_dest_cert}
  copy_between_containers ${dest_ctr}:${CTR_CERT_ROOT}/ca.crt ${MASTER_CTR}:${dest_ca}
  copy_between_containers ${dest_ctr}:${src_2_dest_cert} ${MASTER_CTR}:${src_2_dest_cert}

  docker exec -it ${MASTER_CTR} scripts/deploy/create_cluster_domain_route.sh ${src_domain} ${dest_domain} ${CTR_PREFIX}-lite-${dest_domain}:1080 ${dest_ca} ${src_2_dest_cert}
  echo -e "${GREEN}Cluster domain route from ${src_domain} to ${dest_domain} created successfully${NC}"
}

function check_sf_image() {
  local domain_id=$1
  local domain_ctr=$2
  local env_file=${ROOT}/env.list
  local repo
  if [ -e $env_file ] ; then
    repo=$(awk -F "=" '/REGISTRY_ENDPOINT/ {print $2}' $env_file)
  fi
  local sf_image="${SF_IMAGE_NAME}:${SF_IMAGE_TAG}"
  if [ "$repo" != "" ] ; then
    sf_image="${repo}/${SF_IMAGE_NAME##*/}:${SF_IMAGE_TAG}"
  fi

  if docker exec -it $domain_ctr crictl inspecti $sf_image > /dev/null 2>&1 ; then
    echo -e "${GREEN}Image '${sf_image}' already exists in domain '${domain_id}'${NC}"
    return
  fi

  local has_sf_image=false
  if docker image inspect ${sf_image} >/dev/null 2>&1; then
    has_sf_image=true
  fi

  local ask_message
  if [ "$has_sf_image" == true ] ; then
    echo -e "Found the secretflow image '${sf_image}' on host"
    ask_message="Do you want to import the image into container '${domain_ctr}'? This may take some time. You can also manually import or pull the image later. [y/n]:"
  else
    echo -e "Not found the secretflow image '${sf_image}' on host"
    ask_message="Do you want to pull the image and import it into container '${domain_ctr}'? This may take some time. You can also manually import or pull the image later. [y/n]:"
  fi

  read -rp "$(echo -e ${GREEN}${ask_message}${NC})" yn
  case $yn in
    [Yy]* )
      if [ "$has_sf_image" != true ] ; then
        if [ "$repo" != "" ] ; then
          docker login $repo
        fi
        echo -e "${GREEN}Start pulling image '${sf_image}' ...${NC}"
        docker pull ${sf_image}
      fi

      echo -e "${GREEN}Start importing image '${sf_image}' ...${NC}"
      local image_tar=/tmp/${SF_IMAGE_NAME##*/}.${SF_IMAGE_TAG}.tar
      if [ ! -e $image_tar ] ; then
        docker save $sf_image -o $image_tar
      fi
      docker exec -it $domain_ctr ctr -a=${CTR_ROOT}/containerd/run/containerd.sock -n=k8s.io images import $image_tar
      echo -e "${GREEN}Successfully imported image '${sf_image}' to container '${domain_ctr}' ...${NC}"
      return 0 ;;
  esac

  local prompt_cmd="docker exec -it $domain_ctr crictl pull ${sf_image}"
  if [ "$repo" != "" ] ; then
    prompt_cmd="docker exec -it $domain_ctr crictl pull --creds {{your_username}}:{{your_password}} ${sf_image}"
  fi

  echo -e "Not found secretflow image '${sf_image}' for domain '${domain_id}'. You can use the following command to pull the image in advance (optional):
  ${GREEN}${prompt_cmd}${NC}"
}

function run_centralized() {
  build_kuscia_network

  if need_start_docker_container $MASTER_CTR; then
    echo -e "${GREEN}Starting container $MASTER_CTR ...${NC}"
    local certs_volume=${MASTER_CTR}-certs
    env_flag=$(generate_env_flag $MASTER_DOMAIN)
    docker run -it --rm --mount source=${certs_volume},target=${CTR_CERT_ROOT} ${IMAGE} scripts/deploy/init_domain_certs.sh ${MASTER_DOMAIN}

    docker run -dit --name=${MASTER_CTR} --hostname=${MASTER_CTR} --restart=always --network=${NETWORK_NAME} -m $MASTER_MEMORY_LIMIT ${env_flag} \
      --env NAMESPACE=${MASTER_DOMAIN} \
      --mount source=${certs_volume},target=${CTR_CERT_ROOT} \
      ${IMAGE} scripts/deploy/start_master.sh ${MASTER_DOMAIN}
    probe_gateway_crd ${MASTER_CTR} ${MASTER_DOMAIN} ${MASTER_CTR} 60
    echo -e "${GREEN}Master '${MASTER_DOMAIN}' started successfully${NC}"
    FORCE_START=true
  fi

  start_lite ${ALICE_DOMAIN} https://${MASTER_CTR}:1080
  start_lite ${BOB_DOMAIN} https://${MASTER_CTR}:1080

  create_cluster_domain_route ${ALICE_DOMAIN} ${BOB_DOMAIN}
  create_cluster_domain_route ${BOB_DOMAIN} ${ALICE_DOMAIN}

  check_sf_image $ALICE_DOMAIN ${CTR_PREFIX}-lite-${ALICE_DOMAIN}
  check_sf_image $BOB_DOMAIN ${CTR_PREFIX}-lite-${BOB_DOMAIN}

  echo -e "${GREEN}Kuscia centralized cluster started successfully${NC}"
}

function start_autonomy() {
  local domain_id=$1
  local domain_ctr=${CTR_PREFIX}-autonomy-${domain_id}
  if need_start_docker_container $domain_ctr; then
    echo -e "${GREEN}Starting container $domain_ctr ...${NC}"
    env_flag=$(generate_env_flag $domain_id)
    docker run -it --rm --mount source=${domain_ctr}-certs,target=${CTR_CERT_ROOT} ${IMAGE} scripts/deploy/init_domain_certs.sh ${domain_id}

    docker run -dit --privileged --name=${domain_ctr} --hostname=${domain_ctr} --restart=always --network=${NETWORK_NAME} -m $AUTONOMY_MEMORY_LIMIT ${env_flag} \
      --env NAMESPACE=${domain_id} \
      --mount source=${domain_ctr}-containerd,target=${CTR_ROOT}/containerd \
      --mount source=${domain_ctr}-certs,target=${CTR_CERT_ROOT} \
      -v /tmp:/tmp \
      --entrypoint bin/entrypoint.sh \
      ${IMAGE} tini -- scripts/deploy/start_autonomy.sh ${domain_id} ${ALLOW_PRIVILEGED}
    probe_gateway_crd ${domain_ctr} ${domain_id} ${domain_ctr} 60
    echo -e "${GREEN}Autonomy domain '${domain_id}' started successfully${NC}"
  fi
}

function build_interop() {
  local member_domain=$1
  local host_domain=$2
  local member_ctr=${CTR_PREFIX}-autonomy-${member_domain}
  local host_ctr=${CTR_PREFIX}-autonomy-${host_domain}

  copy_between_containers ${member_ctr}:${CTR_CERT_ROOT}/domain.csr ${host_ctr}:${CTR_CERT_ROOT}/${member_domain}.domain.csr
  docker exec -it ${host_ctr} scripts/deploy/add_domain.sh $member_domain p2p
  copy_between_containers ${host_ctr}:${CTR_CERT_ROOT}/${member_domain}.domain.crt ${member_ctr}:${CTR_CERT_ROOT}/domain-2-${host_domain}.crt
  copy_between_containers ${host_ctr}:${CTR_CERT_ROOT}/ca.crt ${member_ctr}:${CTR_CERT_ROOT}/${host_domain}.host.ca.crt

  docker exec -it ${member_ctr} scripts/deploy/join_to_host.sh $host_domain ${host_ctr}:1080
}

function run_p2p() {
  build_kuscia_network

  start_autonomy ${ALICE_DOMAIN}
  start_autonomy ${BOB_DOMAIN}

  build_interop ${ALICE_DOMAIN} ${BOB_DOMAIN}
  build_interop ${BOB_DOMAIN} ${ALICE_DOMAIN}

  check_sf_image $ALICE_DOMAIN ${CTR_PREFIX}-autonomy-${ALICE_DOMAIN}
  check_sf_image $BOB_DOMAIN ${CTR_PREFIX}-autonomy-${BOB_DOMAIN}

  echo -e "${GREEN}Kuscia p2p cluster started successfully${NC}"
}

function build_kuscia_network() {
  if [[ ! "$(docker network ls -q -f name=${NETWORK_NAME})" ]]; then
    docker network create ${NETWORK_NAME}
  fi
}

usage="$(basename "$0") [NETWORK_MODE] [OPTIONS]
NETWORK_MODE:
    center,centralized  centralized network mode (default)
    p2p                 p2p network mode

OPTIONS:
    -h                  show this help text"

while getopts ':h' option; do
  case "$option" in
  h)
    echo "$usage"
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

case $1 in
center | centralized)
  run_centralized
  ;;
p2p)
  run_p2p
  ;;
"")
  run_centralized
  ;;
*)
  printf "unsupported network mode: %s\n" "$1" >&2
  exit 1
;;
esac
