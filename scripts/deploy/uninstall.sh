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
NETWORK_NAME="kuscia-exchange"

function print_usage() {
  echo "$(basename "$0") NETWORK_MODE [OPTIONS]
  NETWORK_MODE:
  center                  uninstall centralized network mode containers & volumes & network (if no p2p containers)
  p2p                     uninstall p2p network mode containers & volumes & network (if no center containers)
  all                     uninstall all containers & volumes & network (default)

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

function get_all_container_list() {
  local container_type=$1

  case "$container_type" in
      "p2p")
          container_list=$(docker ps -a --format '{{.Names}}' -f name=^"${P2P_PREFIX}")
          ;;
      "center")
          container_list=$(docker ps -a --format '{{.Names}}' -f name=^"${CTR_PREFIX}" | grep -v ^"${P2P_PREFIX}")
          ;;
      "all")
          container_list=$(docker ps -a --format '{{.Names}}' -f name=^"${CTR_PREFIX}")
          ;;
  esac

  echo "$container_list"
}

function get_volume_list() {
  local volume_type=$1

  case "$volume_type" in
      "p2p")
          volume_list=$(docker volume ls --format '{{.Name}}' -f name=^"${P2P_PREFIX}")
          ;;
      "center")
          volume_list=$(docker volume ls --format '{{.Name}}' -f name=^"${CTR_PREFIX}" | grep -v ^"${P2P_PREFIX}")
          ;;
      "all")
          volume_list=$(docker volume ls --format '{{.Name}}' -f name=^"${CTR_PREFIX}")
          ;;
  esac

  echo "$volume_list"
}

function check_running_containers() {
  local container_type=$1

  container_list=$(get_running_container_list "$container_type")

  if [ -n "$container_list" ]; then
    log_hint "There are still running $container_type containers:"
    echo "$container_list"
    read -rp "$(echo -e "${GREEN}" Do you want to stop them now? [y/N]: "${NC}")" choice
    case "$choice" in
      y|Y )
        log "Stopping $container_type containers ..."
        docker stop "$container_list"
        log "Kuscia $container_type containers stopped successfully!"
        ;;
      n|N )
        log_error "Please manually stop them before uninstalling!"
        exit 1
        ;;
      * )
        log_error "Invalid choice"
        exit 1
        ;;
    esac
  fi

  return 0
}

function remove_containers() {
  local container_type=$1

  check_running_containers "$container_type"

  container_list=$(get_all_container_list "$container_type")

  if [ -z "$container_list" ]; then
      case "$container_type" in
          "p2p")
              log_hint "No Kuscia p2p containers found!"
              ;;
          "center")
              log_hint "No Kuscia center containers found!"
              ;;
          "all")
              log_hint "No Kuscia containers found!"
              ;;
      esac
  else
      log "Removing $container_type containers ..."
      docker rm "$container_list"
      log "Kuscia $container_type containers removed successfully!"
  fi

  return 0
}

function remove_volumes() {
  local volume_type=$1

  volume_list=$(get_volume_list "$volume_type")

  if [ -z "$volume_list" ]; then
      case "$volume_type" in
          "p2p")
              log_hint "No Kuscia p2p volumes found!"
              ;;
          "center")
              log_hint "No Kuscia center volumes found!"
              ;;
          "all")
              log_hint "No Kuscia volumes found!"
              ;;
      esac
  else
      log "Removing $volume_type volumes ..."
      docker volume rm "$volume_list"
      log "Kuscia $volume_type volumes removed successfully!"
  fi

  return 0
}

function remove_network() {

  if [ -z "$(docker network ls --format '{{.Name}}' -f name=${NETWORK_NAME})" ]; then
      log_hint "No Kuscia $NETWORK_NAME network found!"
      return 0
  fi

  CONTAINERS_USING_NETWORK=$(docker ps -a --filter "network=${NETWORK_NAME}" --format "{{.Names}}")

  if [ -n "$CONTAINERS_USING_NETWORK" ]; then
    log_hint "There are containers still using the $NETWORK_NAME network:"
    echo "$CONTAINERS_USING_NETWORK"
    log_hint "Please remove them before removing the $NETWORK_NAME network!"
    return 1
  fi

  log "Removing $NETWORK_NAME network ..."
  if ! docker network rm $NETWORK_NAME; then
      log_hint "Kuscia $NETWORK_NAME network removed failed!"
      return 1
  else
      log "Kuscia $NETWORK_NAME network removed successfully!"
  fi

  return 0
}

function uninstall() {
  local network_mode=$1

  remove_containers "$network_mode"
  remove_volumes "$network_mode"
  remove_network
}

if [ $# -eq 0 ]; then
  uninstall "all"
elif [ "$1" == "-h" ]; then
  print_usage
  exit 0
else
  case "$1" in
  "p2p")
      uninstall "p2p"
      ;;
  "center")
      uninstall "center"
      ;;
  "all")
      uninstall "all"
      ;;
  *)
      log_error "Invalid network mode: $1"
      print_usage
      exit 1
      ;;
  esac
fi
