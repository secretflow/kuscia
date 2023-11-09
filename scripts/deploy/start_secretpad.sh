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
NC='\033[0m'

ROOT=$(pwd)
CTR_ROOT=/home/kuscia
CTR_CERT_ROOT=${CTR_ROOT}/etc/certs
CTR_PREFIX=${USER}-kuscia
MASTER_CTR=${CTR_PREFIX}-master
FORCE_START=false
LITE_MEMORY_LIMIT=4G
NETWORK_NAME="kuscia-exchange"
SECRETPAD_USER_NAME=""
SECRETPAD_PASSWORD=""
VOLUME_PATH="${ROOT}"


function log() {
  local log_content=$1
  echo -e "${GREEN}${log_content}${NC}"
}

IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia
if [ "${KUSCIA_IMAGE}" != "" ]; then
  IMAGE=${KUSCIA_IMAGE}
fi

if [[ $SECRETPAD_IMAGE == "" ]]; then
  SECRETPAD_IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/secretpad:latest
fi
log "SECRETPAD_IMAGE=${SECRETPAD_IMAGE}"



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

function copy_secretpad_file_to_volume() {
  local dst_path=$1
  mkdir -p ${dst_path}/secretpad
  mkdir -p ${dst_path}/data
  # copy config file
  docker run --rm --entrypoint /bin/bash -v ${dst_path}/secretpad:/tmp/secretpad $SECRETPAD_IMAGE -c 'cp -R /app/config /tmp/secretpad/'
  # copy sqlite db file
  docker run --rm --entrypoint /bin/bash -v ${dst_path}/secretpad:/tmp/secretpad $SECRETPAD_IMAGE -c 'cp -R /app/db /tmp/secretpad/'
  # copy demo data file
  docker run --rm --entrypoint /bin/bash -v ${dst_path}:/tmp/secretpad $SECRETPAD_IMAGE -c 'cp -R /app/data /tmp/secretpad/'
  log "copy webserver config and database file done"
}

function generate_secretpad_serverkey() {
  local tmp_volume=$1
  local password=$2
  # generate server key in secretPad container
  docker run -it --rm --entrypoint /bin/bash --volume=${tmp_volume}/secretpad/config/:/tmp/temp ${SECRETPAD_IMAGE} -c "scripts/gen_secretpad_serverkey.sh ${password} /tmp/temp"
  rm -rf ${tmp_volume}/server.jks
  log "generate webserver server key done"
}

function init_secretpad_db() {
  # generate server key in secretPad container
  docker run -it --rm --entrypoint /bin/bash --volume=${volume_path}/secretpad/db:/app/db ${SECRETPAD_IMAGE} -c "scripts/update-sql.sh"
  log "initialize  webserver database done"
}

function create_secretpad_user_password() {
  local volume_path=$1
  local user_name=$2
  local password=$3
  # generate server key in secretPad container
  docker run -it --rm --entrypoint /bin/bash --volume=${volume_path}/secretpad/db:/app/db ${SECRETPAD_IMAGE} -c "scripts/register_account.sh -n '${user_name}' -p '${password}'"

  log "create webserver user and password done"
}

function copy_kuscia_api_client_certs() {
  local volume_path=$1
  local IMAGE=$SECRETPAD_IMAGE
  # copy result
  tmp_path=${volume_path}/temp/certs
  mkdir -p ${tmp_path}
  docker cp ${MASTER_CTR}:/${CTR_CERT_ROOT}/ca.crt ${tmp_path}/ca.crt
  docker cp ${MASTER_CTR}:/${CTR_CERT_ROOT}/kusciaapi-client.crt ${tmp_path}/client.crt
  docker cp ${MASTER_CTR}:/${CTR_CERT_ROOT}/kusciaapi-client.key ${tmp_path}/client.pem
  docker cp ${MASTER_CTR}:/${CTR_CERT_ROOT}/token ${tmp_path}/token
  docker run -d --rm --name ${CTR_PREFIX}-dummy --volume=${volume_path}/secretpad/config:/tmp/temp $IMAGE tail -f /dev/null >/dev/null 2>&1
  docker cp -a ${tmp_path} ${CTR_PREFIX}-dummy:/tmp/temp/
  docker rm -f ${CTR_PREFIX}-dummy >/dev/null 2>&1
  rm -rf ${volume_path}/temp
  log "copy kuscia api client certs to web server container done"
}

function copy_kuscia_api_lite_client_certs() {
  local domain_id=$1
  local volume_path=$2
  local IMAGE=$SECRETPAD_IMAGE
  local domain_ctr=${CTR_PREFIX}-lite-${domain_id}
  # copy result
  tmp_path=${volume_path}/temp/certs/${domain_id}
  mkdir -p ${tmp_path}
  docker cp ${domain_ctr}:/${CTR_CERT_ROOT}/ca.crt ${tmp_path}/ca.crt
  docker cp ${domain_ctr}:/${CTR_CERT_ROOT}/kusciaapi-client.crt ${tmp_path}/client.crt
  docker cp ${domain_ctr}:/${CTR_CERT_ROOT}/kusciaapi-client.key ${tmp_path}/client.pem
  docker cp ${domain_ctr}:/${CTR_CERT_ROOT}/token ${tmp_path}/token
  docker run -d --rm --name ${CTR_PREFIX}-dummy --volume=${volume_path}/secretpad/config/certs:/tmp/temp $IMAGE tail -f /dev/null >/dev/null 2>&1
  docker cp -a ${tmp_path} ${CTR_PREFIX}-dummy:/tmp/temp/
  docker rm -f ${CTR_PREFIX}-dummy >/dev/null 2>&1
  rm -rf ${volume_path}/temp
  log "copy kuscia api client lite :${domain_id} certs to web server container done"
}

function render_secretpad_config() {
  local volume_path=$1
  local tmpl_path=${volume_path}/secretpad/config/template/application.yaml.tmpl
  local store_key_password=$2
  #local default_login_password
  # create data mesh service
  log "kuscia_master_ip: '${MASTER_CTR}'"
  # render kuscia api address
  sed "s/{{.KUSCIA_API_ADDRESS}}/${MASTER_CTR}/g;
  s/{{.KUSCIA_API_LITE_ALICE_ADDRESS}}/${CTR_PREFIX}-lite-${ALICE_DOMAIN}/g;
  s/{{.KUSCIA_API_LITE_BOB_ADDRESS}}/${CTR_PREFIX}-lite-${BOB_DOMAIN}/g" \
  ${tmpl_path} >${volume_path}/application_01.yaml
  # render store password
  sed "s/{{.PASSWORD}}/${store_key_password}/g" ${volume_path}/application_01.yaml >${volume_path}/application.yaml
  # cp file to secretpad's config path
  docker run -d --rm --name ${CTR_PREFIX}-dummy --volume=${volume_path}/secretpad/config:/tmp/temp $IMAGE tail -f /dev/null >/dev/null 2>&1
  docker cp ${volume_path}/application.yaml ${CTR_PREFIX}-dummy:/tmp/temp/
  docker rm -f ${CTR_PREFIX}-dummy >/dev/null 2>&1
  # rm temp file
  rm -rf ${volume_path}/application_01.yaml ${volume_path}/application.yaml
  # render default_login_password
  log "render webserver config done"
}



function check_user_name() {
  local user_name=$1
  strlen=$(echo "${user_name}" | grep -E --color '^(.{4,}).*$')
  if [ -n "${strlen}" ]; then
    return 0
  else
    log "The username requires a length greater than 4"
    return 1
  fi
}


function check_user_passwd() {
  local password=$1
  # length greater than 8
  str_len=$(echo "${password}" | grep -E --color '^(.{8,}).*$')
  # with lowercase letters
  str_low=$(echo "${password}" | grep -E --color '^(.*[a-z]+).*$')
  # with uppercase letters
  str_upp=$(echo "${password}" | grep -E --color '^(.*[A-Z]).*$')
  # with special characters
  str_ts=$(echo "${password}" | grep -E --color '^(.*\W).*$')
  # with numbers
  str_num=$(echo "${password}" | grep -E --color '^(.*[0-9]).*$')
  if [ -n "${str_len}" ] && [ -n "${str_low}" ] && [ -n "${str_upp}" ] && [ -n "${str_ts}" ] && [ -n "${str_num}" ]; then
    return 0
  else
    log "The password requires a length greater than 8, including uppercase and lowercase letters, numbers, and special characters."
    return 2
  fi
}

function account_settings() {
  local RET
  set +e
  log "Please set the username and the password used to login the KUSCIA-WEB.\n\
The username requires a length greater than 4, The password requires a length greater than 8,\n\
including uppercase and lowercase letters, numbers, and special characters."
  for ((i = 0; i < 1; i++)); do
    read -r -p "Enter username(admin):" SECRETPAD_USER_NAME
    check_user_name "${SECRETPAD_USER_NAME}"
    RET=$?
    if [ "${RET}" -eq 0 ]; then
      break
    elif [ "${RET}" -ne 0 ] && [ "${i}" == 0 ]; then
      log "would use default user: admin"
      SECRETPAD_USER_NAME="admin"
    fi
  done
  stty -echo # disable display
  for ((i = 0; i < 3; i++)); do
    read -r -p "Enter password: " SECRETPAD_PASSWORD
    echo ""
    check_user_passwd "${SECRETPAD_PASSWORD}"
    RET=$?
    if [ "${RET}" -eq 0 ]; then
      local CONFIRM_PASSWD
      read -r -p "Confirm password again: " CONFIRM_PASSWD
      echo ""
      if [ "${CONFIRM_PASSWD}" == "${SECRETPAD_PASSWORD}" ]; then
        break
      else
        log "Password not match! please reset"
      fi
    elif [ "${RET}" -ne 0 ] && [ "${i}" == 2 ]; then
      log "would use default password: 12#\$qwER"
      SECRETPAD_PASSWORD="12#\$qwER"
    fi
  done
  set -e
  stty echo # enable display
  log "The user and password have been set up successfully."
}

function create_secretpad_svc() {
  local ctr=$1
  local secretpad_ctr=$2
  # create secret pad service
  docker exec -it ${ctr} scripts/deploy/create_secretpad_svc.sh ${secretpad_ctr}
}

function probe_secret_pad() {
  local secretpad_ctr=$1
  if ! do_http_probe $secretpad_ctr "http://127.0.0.1:8080" 60; then
    echo "[Error] Probe secret pad in container '$secretpad_ctr' failed. Please check the log" >&2
    exit 1
  fi
}

function start_secretpad() {
  # volume_path
  # ├── data
  # │   ├── alice
  # │   │   └── alice.csv
  # │   └── bob
  # │       └── bob.csv
  # └── secretpad
  #     ├── config
  #     └── db
  #
  local volume_path=$1
  local user_name=$2
  local password=$3
  local secretpad_ctr=${CTR_PREFIX}-secretpad
  if need_start_docker_container $secretpad_ctr; then
    log "Starting container '$secretpad_ctr' ..."
    secretpad_key_pass="secretpad"
    # copy db,config,demodata from secretpad image
    copy_secretpad_file_to_volume ${volume_path}
    # generate server key
    generate_secretpad_serverkey ${volume_path} ${secretpad_key_pass}
    # initialize secretpad db
    init_secretpad_db
    # create secretpad user and password
    create_secretpad_user_password ${volume_path} ${user_name} ${password}
    # copy kuscia api client certs
    copy_kuscia_api_client_certs ${volume_path}
    # copy kuscia api lite:alice client certs
    copy_kuscia_api_lite_client_certs ${ALICE_DOMAIN} ${volume_path}
    # copy kuscia api lite:bob client certs
    copy_kuscia_api_lite_client_certs ${BOB_DOMAIN} ${volume_path}
    # render secretpad config
    render_secretpad_config ${volume_path} ${secretpad_key_pass}
    # run secretpad
    docker run -itd --init --name=${CTR_PREFIX}-secretpad --restart=always --network=${NETWORK_NAME} -m $LITE_MEMORY_LIMIT \
      --volume=${volume_path}/data:/app/data \
      --volume=${volume_path}/secretpad/config:/app/config \
      --volume=${volume_path}/secretpad/db:/app/db \
      --workdir=/app \
      -p 8088:8080 \
      ${SECRETPAD_IMAGE}
    create_secretpad_svc ${MASTER_CTR} ${CTR_PREFIX}-secretpad
    probe_secret_pad ${CTR_PREFIX}-secretpad
    log "web server started successfully"
    log "Please visit the website http://localhost:8088 (or http://{the IPAddress of this machine}:8088) to experience the Kuscia web's functions ."
    log "The login name:'${SECRETPAD_USER_NAME}' ,The login password:'${SECRETPAD_PASSWORD}' ."
    log "The demo data would be stored in the path: ${VOLUME_PATH} ."
  fi
}

function run_centralized_all() {
  read -rp "$(echo -e "${GREEN}Please specify a valid and existing path for storage the demo data,Or would use the default path('$VOLUME_PATH').\n\
Please input a valid path for demo data('$VOLUME_PATH'):${NC}")" volume_path
  if [ -d "${volume_path}" ]; then
    echo "use the '${volume_path}' as the data path."
    VOLUME_PATH=${volume_path}
  else
    echo -e "${RED}the input path is not valid or is not a existing directory, would use the '${VOLUME_PATH}' as the demo data path.${NC}"
  fi
  account_settings
  start_secretpad $VOLUME_PATH $SECRETPAD_USER_NAME $SECRETPAD_PASSWORD
}

run_centralized_all