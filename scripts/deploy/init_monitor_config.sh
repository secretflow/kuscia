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
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
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

function generate_config_block(){
    local config_data=$1
    local job_name=$2
    local scrape_interval=$3
    local ip_addr=$4
    local port=$5
    local metric_path=$6
    local scheme=$7
    echo -e "${config_data}""
  - job_name: '${job_name}'
    scrape_interval: ${scrape_interval}s
    static_configs:
      - targets: ['$ip_addr:$port']
    metrics_path: $metric_path 
    scheme: $scheme
"
}
function generate_center_config(){
    local config_data=$1
    config_data=$(generate_config_block "${config_data}" master-network 5 192.168.224.1 9091 http)
    config_data=$(generate_config_block "${config_data}" alice-network 5 192.168.224.2 9091 http)
    config_data=$(generate_config_block "${config_data}" bob-network 5 192.168.224.3 9091 http)
    config_data=$(generate_config_block "${config_data}" alice-node 5 192.168.224.2 9100 http)
    config_data=$(generate_config_block "${config_data}" alice-node 5 192.168.224.3 9100 http)
    echo "${config_data}"
}

function generate_p2p_config(){
    local config_data=$1
    local domain_id=$2
    config_data=$(generate_config_block "${config_data}" "${domain_id}-network" 5 192.168.224.2 9091 http)
    config_data=$(generate_config_block "${config_data}" "${domain_id}-node" 5 192.168.224.3 9100 http)
    echo "${config_data}"
}


if [[ $MODE == "center" ]]; then
  CONFIG_DATA=$(generate_center_config "${CONFIG_DATA}")
  echo "${CONFIG_DATA}" > ${ROOT}/monitor.yaml
elif [[ $MODE == "p2p" ]]; then
  ORIGIN_CONF=${CONFIG_DATA}
  CONFIG_DATA=$(generate_p2p_config "${CONFIG_DATA}" "alice")
  echo "${CONFIG_DATA}" > ${ROOT}/alice-monitor.yaml
  CONFIG_DATA=$(generate_p2p_config "${ORIGIN_CONF}" "bob")
  echo "${CONFIG_DATA}" > ${ROOT}/bob-monitor.yaml
else
  echo "Unsupported mode: $MODE"
  exit 1
fi

