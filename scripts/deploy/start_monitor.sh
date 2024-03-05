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
MODE=$1
NETWORK=$2
if [ "${NETWORK}" == "" ]; then
  NETWORK_NAME="kuscia-exchange"
else
  NETWORK_NAME=${NETWORK}
fi
ROOT=$HOME/kuscia
CTR_PREFIX=${USER}-kuscia
MASTER_CTR=${CTR_PREFIX}-master
ALICE_DOMAIN=alice
BOB_DOMAIN=bob
FORCE_START=false
CONFIG_DATA="
global:
  scrape_interval:     5s
  external_labels:
    monitor: 'kuscia-monitor'
scrape_configs:
  - job_name: 'prometheus'
    scrape_interval: 5s
    #scrape_timeout: 10s
    static_configs:
      - targets: ['localhost:9090']
"
GREEN='\033[0;32m'
NC='\033[0m'
RED='\033[31m'

function log() {
  local log_content=$1
  echo -e "${GREEN}${log_content}${NC}"
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
    return 1
    ;;
  esac

  return 1
}

function generate_config_block(){
    local config_data=$1
    local job_name=$2
    local scrape_interval=$3
    local ip_addr=$4
    local port=$5
    local scheme=$6
    echo -e "${config_data}""
  - job_name: '${job_name}'
    scrape_interval: ${scrape_interval}s
    static_configs:
      - targets: ['$ip_addr:$port']
    metrics_path: /metrics
    scheme: $scheme
"
}
function generate_center_config(){
    local config_data=$1
    local alice_ctr=${CTR_PREFIX}"-lite-alice"
    local bob_ctr=${CTR_PREFIX}"-lite-bob"
    local master_ip=$(get_container_ipaddr ${MASTER_CTR})
    local alice_ip=$(get_container_ipaddr ${alice_ctr})
    local bob_ip=$(get_container_ipaddr ${bob_ctr})
    config_data=$(generate_config_block "${config_data}" master-node 5 ${master_ip} 9091 http)
    config_data=$(generate_config_block "${config_data}" alice-node 5 ${alice_ip} 9091 http)
    config_data=$(generate_config_block "${config_data}" bob-node 5 ${bob_ip} 9091 http)
    echo "${config_data}"
}
function generate_p2p_config(){
    local config_data=$1
    local domain_id=$2
    local domain_ctr=${CTR_PREFIX}"-autonomy-"${domain_id}
    local domain_ip=$(get_container_ipaddr ${domain_ctr})
    config_data=$(generate_config_block "${config_data}" "${domain_id}-network" 5 "${domain_ip}" 9091 http)
    echo "${config_data}"
}
function get_container_ipaddr(){
    local container_name=$1
    container_entries=$(docker network inspect kuscia-exchange | jq -r '.[]|select(.Name == "kuscia-exchange")|.Containers|to_entries[]')
    container_entry=$(echo -e ${container_entries} | jq -r "select(.value.Name==\"${container_name}\") | .value.IPv4Address" | sed 's/\/.*//')
    echo $container_entry
}

function init_monitor_config(){
    local mode=$1
    local conf_dir=$2
    local domain_id=$3
    local config_data=$4
    mkdir -p ${conf_dir}
    if [[ $mode == "center" ]]; then
	config_data=$(generate_center_config "${config_data}")
        echo "${config_data}" > ${conf_dir}/prometheus.yml
    elif [[ $mode == "p2p" ]]; then
        config_data=$(generate_p2p_config "${config_data}" "${domain_id}")
        echo "${config_data}" > ${conf_dir}/prometheus.yml
    else
    echo "Unsupported mode: $mode"
    exit 1
    fi
}


function start_kuscia_monitor() {
  local domain_id=$1
  local prometheus_port=$2
  local grafana_port=$3
  local image_name=$4
  local conf_dir=$5
  local name=${CTR_PREFIX}-monitor-${domain_id}
  echo ${name}
  if need_start_docker_container ${name}; then
    docker run -dit --name=${name} --hostname=${name} --restart=always --network=${NETWORK_NAME} -v ${conf_dir}:/home/config/ -p "${prometheus_port}":9090 -p "${grafana_port}":3000 ${image_name}
    log "kuscia-monitor started successfully docker container name:'${name}'"
  fi
}
if [ ${MODE} == "center" ]; then
  init_monitor_config center ${ROOT}/${MASTER_CTR} center "${CONFIG_DATA}"
  start_kuscia_monitor center 9090 3000 docker.io/secretflow/kusica-monitor "${ROOT}/${MASTER_CTR}/"
elif [ ${MODE} == "p2p" ]; then
  init_monitor_config p2p ${ROOT}/${CTR_PREFIX}-autonomy-${ALICE_DOMAIN} ${ALICE_DOMAIN} "${CONFIG_DATA}"
  start_kuscia_monitor ${ALICE_DOMAIN} 9089 3000 docker.io/secretflow/kusica-monitor "${ROOT}/${CTR_PREFIX}-autonomy-${ALICE_DOMAIN}" ${ALICE_DOMAIN}
  init_monitor_config p2p ${ROOT}/${CTR_PREFIX}-autonomy-${BOB_DOMAIN} ${BOB_DOMAIN} "${CONFIG_DATA}"
  start_kuscia_monitor ${BOB_DOMAIN} 9090 3001 docker.io/secretflow/kusica-monitor "${ROOT}/${CTR_PREFIX}-autonomy-${BOB_DOMAIN}"  ${BOB_DOMAIN}
fi
