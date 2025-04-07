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

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

CTR_PREFIX="${USER}-kuscia"
P2P_PREFIX="${USER}-kuscia-autonomy"

function print_usage() {
  echo "$(basename "$0") NETWORK_MODE [OPTIONS]
  NETWORK_MODE:
  center                  stop centralized network mode containers
  p2p                     stop p2p network mode containers
  all                     stop all containers (default)

  Common Options:
  -h                      show this help text"
}

function log() {
  local log_content=$1
  echo -e "${GREEN}${log_content}${NC}"
}

function log_error() {
  local log_content=$1
  echo -e "${RED}${log_content}${NC}" >&2
}

function log_hint(){
  local log_content=$1
  echo -e "${YELLOW}${log_content}${NC}"
}

function get_running_container_list() {
  local container_type=$1

  case "$container_type" in
      "p2p")
          container_list=$(docker ps --format '{{.Names}}' -f name=^"${P2P_PREFIX}")
          ;;
      "center")
          container_list=$(docker ps --format '{{.Names}}' -f name=^"${CTR_PREFIX}" | grep -v ^"${P2P_PREFIX}")
          ;;
      "all")
          container_list=$(docker ps --format '{{.Names}}' -f name=^"${CTR_PREFIX}")
          ;;
  esac

  echo "$container_list"
}

function stop_container() {
  local container_type=$1

  container_list=$(get_running_container_list "$container_type")

  if [ -z "$container_list" ]; then
      case "$container_type" in
          "p2p")
              log_hint "No Kuscia p2p containers running!"
              ;;
          "center")
              log_hint "No Kuscia center containers running!"
              ;;
          "all")
              log_hint "No Kuscia containers running!"
              ;;
      esac
  else
      log "Stopping Kuscia $container_type containers ..."
      docker stop "$container_list"
      log "Kuscia $container_type containers stopped successfully!"
  fi
}

if [ $# -eq 0 ]; then
  stop_container "all"
elif [ "$1" == "-h" ]; then
  print_usage
  exit 0
else
  case "$1" in
  "p2p")
      stop_container "p2p"
      ;;
  "center")
      stop_container "center"
      ;;
  "all")
      stop_container "all"
      ;;
  *)
      log_error "Invalid network mode: $1"
      print_usage
      exit 1
      ;;
  esac
fi
