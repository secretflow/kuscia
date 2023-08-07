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

IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia
if [ "${KUSCIA_IMAGE}" != "" ]; then
  IMAGE=${KUSCIA_IMAGE}
fi

echo -e "IMAGE=${IMAGE}"

if [ "${SECRETPAD_IMAGE}" == "" ]; then
  SECRETPAD_IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/secretpad
fi
echo -e "SECRETPAD_IMAGE=${SECRETPAD_IMAGE}"

if [ "$SECRETFLOW_IMAGE" != "" ]; then
  echo -e "SECRETFLOW_IMAGE=${SECRETFLOW_IMAGE}"
fi

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
SF_IMAGE_TAG="latest"
SF_IMAGE_REGISTRY="secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow"
NETWORK_NAME="kuscia-exchange"
SECRETPAD_USER_NAME=""
SECRETPAD_PASSWORD=""
VOLUME_PATH="${ROOT}"

function log() {
  local log_content=$1
  echo -e "${GREEN}${log_content}${NC}"
}

function init_sf_image_info() {
    if [ "$SECRETFLOW_IMAGE" != "" ]; then
      SF_IMAGE_TAG=${SECRETFLOW_IMAGE##*:}
      path_separator_count="$(echo "$SECRETFLOW_IMAGE" | tr -cd "/" | wc -c )"
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
  ctr=$1

  if [[ ! "$(docker ps -a -q -f name=^/${ctr}$)" ]]; then
    # need start your container
    return 0
  fi

  if $FORCE_START ; then
    log "Remove container '${ctr}' ..."
    docker rm -f $ctr >/dev/null 2>&1
    # need start your container
    return 0
  fi

  read -rp "$(echo -e ${GREEN}The container \'${ctr}\' already exists. Do you need to recreate it? [y/n]: ${NC})" yn
  case $yn in
    [Yy]* )
      log "Remove container '${ctr}' ..."
      docker rm -f $ctr >/dev/null 2>&1
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
  docker cp $src_file /tmp/${temp_file} >/dev/null 2>&1
  docker cp /tmp/${temp_file} $dest_file >/dev/null 2>&1
  rm /tmp/${temp_file}
  echo "Copy file successfully src_file:'$src_file' to dest_file:'$dest_file'"
}

function copy_container_file_to_volume() {
  local src_file=$1
  local dest_volume=$2
  local dest_file=$3
  docker run -d --rm --name ${CTR_PREFIX}-dummy --mount source=${dest_volume},target=/tmp/kuscia $IMAGE tail -f /dev/null >/dev/null 2>&1
  copy_between_containers ${src_file} ${CTR_PREFIX}-dummy:/tmp/kuscia/${dest_file} >/dev/null 2>&1
  docker rm -f ${CTR_PREFIX}-dummy >/dev/null 2>&1
  echo "Copy file successfully src_file:'$src_file' to dest_file:'$dest_volume:$CTR_CERT_ROOT/$dest_file'"
}

function copy_volume_file_to_container() {
  local src_volume=$1
  local src_file=$2
  local dest_file=$3
  docker run -d --rm --name ${CTR_PREFIX}-dummy --mount source=${src_volume},target=/tmp/kuscia $IMAGE tail -f /dev/null >/dev/null 2>&1
  copy_between_containers ${CTR_PREFIX}-dummy:/tmp/kuscia/${src_file} ${dest_file} >/dev/null 2>&1
  docker rm -f ${CTR_PREFIX}-dummy >/dev/null 2>&1
  echo "Copy file successfully src_file:'$src_volume/$src_file' to dest_file:'$dest_file'"
}

# secretpad
function copy_secretpad_file_to_volume() {
  local dst_path=$1
  mkdir -p ${dst_path}/secretpad
  mkdir -p ${dst_path}/data
  # copy config file
  docker run --rm --entrypoint /bin/bash -v  ${dst_path}/secretpad:/tmp/secretpad $SECRETPAD_IMAGE -c 'cp -R /app/config /tmp/secretpad/'
  # copy sqlite db file
  docker run --rm --entrypoint /bin/bash -v  ${dst_path}/secretpad:/tmp/secretpad $SECRETPAD_IMAGE -c 'cp -R /app/db /tmp/secretpad/'
  # copy demo data file
  docker run --rm --entrypoint /bin/bash -v  ${dst_path}:/tmp/secretpad $SECRETPAD_IMAGE -c 'cp -R /app/data /tmp/secretpad/'
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
  docker run -it --rm --entrypoint /bin/bash --volume=${volume_path}/secretpad/db:/app/db  ${SECRETPAD_IMAGE} -c "scripts/update-sql.sh"
  log "initialize  webserver database done"
}

function create_secretpad_user_password() {
  local volume_path=$1
  local user_name=$2
  local password=$3
  # generate server key in secretPad container
  docker run -it --rm --entrypoint /bin/bash --volume=${volume_path}/secretpad/db:/app/db  ${SECRETPAD_IMAGE} -c "scripts/register_account.sh -n '${user_name}' -p '${password}'"

  log "create webserver user and password done"
}

function copy_kuscia_api_client_certs() {
  local volume_path=$1
  # copy result
  tmp_path=${volume_path}/temp/certs
  mkdir -p ${tmp_path}
  docker cp ${MASTER_CTR}:/${CTR_CERT_ROOT}/ca.crt ${tmp_path}/ca.crt
  docker cp ${MASTER_CTR}:/${CTR_CERT_ROOT}/kusciaapi-client.crt  ${tmp_path}/client.crt
  docker cp ${MASTER_CTR}:/${CTR_CERT_ROOT}/kusciaapi-client.key  ${tmp_path}/client.pem
  docker cp ${MASTER_CTR}:/${CTR_CERT_ROOT}/token  ${tmp_path}/token
  docker run -d --rm --name ${CTR_PREFIX}-dummy --volume=${volume_path}/secretpad/config:/tmp/temp $IMAGE tail -f /dev/null >/dev/null 2>&1
  docker cp -a ${tmp_path} ${CTR_PREFIX}-dummy:/tmp/temp/
  docker rm -f ${CTR_PREFIX}-dummy >/dev/null 2>&1
  rm -rf ${volume_path}/temp
  log "copy kuscia api client certs to web server container done"
}

function render_secretpad_config() {
  local volume_path=$1
  local tmpl_path=${volume_path}/secretpad/config/template/application.yaml.tmpl
  local store_key_password=$2
  #local default_login_password
  # create data mesh service
  log "kuscia_master_ip: '${MASTER_CTR}'"
  # render kuscia api address
  sed "s/{{.KUSCIA_API_ADDRESS}}/${MASTER_CTR}/g" ${tmpl_path} > ${volume_path}/application_01.yaml
  # render store password
  sed "s/{{.PASSWORD}}/${store_key_password}/g" ${volume_path}/application_01.yaml > ${volume_path}/application.yaml
  # cp file to secretpad's config path
  docker run -d --rm --name ${CTR_PREFIX}-dummy --volume=${volume_path}/secretpad/config:/tmp/temp $IMAGE tail -f /dev/null >/dev/null 2>&1
  docker cp ${volume_path}/application.yaml ${CTR_PREFIX}-dummy:/tmp/temp/
  docker rm -f ${CTR_PREFIX}-dummy >/dev/null 2>&1
  # rm temp file
  rm -rf ${volume_path}/application_01.yaml ${volume_path}/application.yaml
  # render default_login_password
  log "render webserver config done"
}

function create_domain_datamesh_svc() {
  local ctr=$1
  local domain_id=$2
  local endpoint=$3
  #local domain_ip
  # create data mesh service
  #domain_ip=$(docker container inspect -f '{{ .NetworkSettings.IPAddress }}' ${CTR_PREFIX}-lite-${domain_id})
  #echo "domain : '${domain_id}' ip: '${domain_ip}'"
  docker exec -it ${ctr} scripts/deploy/create_datamesh_svc.sh ${domain_id} ${endpoint}
  log "create datamesh service done domain_id: '${domain_id}' endpoint: '${endpoint}'"
}

function create_secretflow_app_image() {
  local ctr=$1
  docker exec -it ${ctr} scripts/deploy/create_sf_app_image.sh "${SF_IMAGE_NAME}" "${SF_IMAGE_TAG}"
  log "create secretflow app image done"
}

function create_domaindata_alice_table() {
  local ctr=$1
  local domain_id=$2
  local data_path="/home/kuscia/var/storage/data"
  # create domain datasource
  docker exec -it ${ctr} scripts/deploy/create_domain_datasource.sh ${domain_id}
  # create domain data alice table
  docker exec -it ${ctr} scripts/deploy/create_domaindata_alice_table.sh ${domain_id}
  log "create domaindata alice's table done default stored path: '${data_path}'"
}

function create_domaindata_bob_table() {
  local ctr=$1
  local domain_id=$2
  local data_path="/home/kuscia/var/storage/data"
  # create domain datasource
  docker exec -it ${ctr} scripts/deploy/create_domain_datasource.sh ${domain_id}
  # create domain data bob table
  docker exec -it ${ctr} scripts/deploy/create_domaindata_bob_table.sh ${domain_id}
  log "create domaindata bob's table done default stored path: '${data_path}'"
}

function check_user_name(){
  local user_name=$1
  strlen=$(echo "${user_name}" | grep -E --color '^(.{4,}).*$')
  if [ -n "${strlen}" ]; then
    return 0
  else
    log "The username requires a length greater than 4"
    return 1
  fi
}

function check_user_passwd(){
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
  if [ -n "${str_len}" ] && [ -n "${str_low}" ] && [ -n "${str_upp}" ] && [ -n "${str_ts}" ]  && [ -n "${str_num}" ]
  then
      return 0
  else
      log "The password requires a length greater than 8, including uppercase and lowercase letters, numbers, and special characters."
      return 2
  fi
}

function account_settings(){
  local RET
  set +e
  log "Please set the username and the password used to login the KUSCIA-WEB.\n\
The username requires a length greater than 4, The password requires a length greater than 8,\n\
including uppercase and lowercase letters, numbers, and special characters."
  for (( i = 0; i < 1; i++ )); do
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
  for (( i = 0; i < 3; i++ )); do
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
  if need_start_docker_container $secretpad_ctr ; then
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
      # render secretpad config
      render_secretpad_config ${volume_path} ${secretpad_key_pass}
      # run secretpad
      docker run  -itd --init --name=${CTR_PREFIX}-secretpad --restart=always --network=${NETWORK_NAME} -m $LITE_MEMORY_LIMIT \
      	--volume=${volume_path}/data:/app/data \
      	--volume=${volume_path}/secretpad/config:/app/config \
      	--volume=${volume_path}/secretpad/db:/app/db \
      	--workdir=/app \
      	-p 8088:8080 \
      	${SECRETPAD_IMAGE}
      probe_secret_pad ${CTR_PREFIX}-secretpad
      log "web server started successfully"
      log "Please visit the website http://localhost:8088 (or http://{the IPAddress of this machine}:8088) to experience the Kuscia web's functions ."
      log "The login name:'${SECRETPAD_USER_NAME}' ,The login password:'${SECRETPAD_PASSWORD}' ."
      log "The demo data would be stored in the path: ${VOLUME_PATH} ."
  fi
}

function start_lite() {
  local domain_id=$1
  local master_endpoint=$2
  local domain_ctr=${CTR_PREFIX}-lite-${domain_id}
  local volume_path=$3

  if need_start_docker_container $domain_ctr ; then
    log "Starting container '$domain_ctr' ..."
    local certs_volume=${domain_ctr}-certs
    env_flag=$(generate_env_flag $domain_id)
    local mount_volume_param="-v /tmp:/tmp"
    if [ "$volume_path" != "" ] ; then
      mount_volume_param="-v /tmp:/tmp  -v ${volume_path}/data/${domain_id}:/home/kuscia/var/storage/data "
    fi

    docker run -it --rm --mount source=${certs_volume},target=${CTR_CERT_ROOT} ${IMAGE} scripts/deploy/init_domain_certs.sh ${domain_id}
    copy_volume_file_to_container $certs_volume domain.csr ${MASTER_CTR}:${CTR_CERT_ROOT}/${domain_id}.domain.csr
    docker exec -it ${MASTER_CTR} kubectl create ns $domain_id
    docker exec -it ${MASTER_CTR} scripts/deploy/add_domain.sh $domain_id ${MASTER_CTR}

    copy_container_file_to_volume ${MASTER_CTR}:${CTR_CERT_ROOT}/${domain_id}.domain.crt $certs_volume domain.crt
    copy_container_file_to_volume ${MASTER_CTR}:${CTR_CERT_ROOT}/ca.crt $certs_volume master.ca.crt

    docker run -dit --privileged --name=${domain_ctr} --hostname=${domain_ctr} --restart=always --network=${NETWORK_NAME} -m $LITE_MEMORY_LIMIT ${env_flag} \
      --env NAMESPACE=${domain_id} \
      --mount source=${domain_ctr}-containerd,target=${CTR_ROOT}/containerd \
      --mount source=${certs_volume},target=${CTR_CERT_ROOT} \
      ${mount_volume_param} \
      --entrypoint bin/entrypoint.sh \
      ${IMAGE} tini -- scripts/deploy/start_lite.sh ${domain_id} ${MASTER_DOMAIN} ${master_endpoint} ${ALLOW_PRIVILEGED}
    probe_gateway_crd ${MASTER_CTR} ${domain_id} ${domain_ctr} 60
    log "Lite domain '${domain_id}' started successfully docker container name:'${domain_ctr}', crt path: '${CTR_CERT_ROOT}'"
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

  log "Starting create cluster domain route from '${src_domain}' to '${dest_domain}'"
  copy_between_containers ${src_ctr}:${CTR_CERT_ROOT}/domain.csr ${dest_ctr}:${src_domain_csr}
  docker exec -it ${dest_ctr} openssl x509 -req -in $src_domain_csr -CA ${CTR_CERT_ROOT}/ca.crt -CAkey ${CTR_CERT_ROOT}/ca.key -CAcreateserial -days 10000 -out ${src_2_dest_cert}
  copy_between_containers ${dest_ctr}:${CTR_CERT_ROOT}/ca.crt ${MASTER_CTR}:${dest_ca}
  copy_between_containers ${dest_ctr}:${src_2_dest_cert} ${MASTER_CTR}:${src_2_dest_cert}

  docker exec -it ${MASTER_CTR} scripts/deploy/create_cluster_domain_route.sh ${src_domain} ${dest_domain} ${CTR_PREFIX}-lite-${dest_domain}:1080 ${dest_ca} ${src_2_dest_cert}
  log "Cluster domain route from '${src_domain}' to '${dest_domain}' created successfully dest_endpoint: '${CTR_PREFIX}'-lite-'${dest_domain}':1080"
}

function check_sf_image() {
  local domain_id=$1
  local domain_ctr=$2
  local volume_path=$3
  local env_file=${ROOT}/env.list
  local default_repo=${SF_IMAGE_REGISTRY}
  local repo
  if [ -e $env_file ] ; then
    repo=$(awk -F "=" '/REGISTRY_ENDPOINT/ {print $2}' $env_file)
  fi
  local sf_image="${SF_IMAGE_NAME}:${SF_IMAGE_TAG}"
  if [ "$repo" != "" ] ; then
    sf_image="${repo}/${SF_IMAGE_NAME##*/}:${SF_IMAGE_TAG}"
  elif [ "$default_repo" != "" ]; then
    sf_image="${default_repo}/${SF_IMAGE_NAME##*/}:${SF_IMAGE_TAG}"
  fi
  if [ "$SECRETFLOW_IMAGE" != "" ] ; then
    sf_image=$SECRETFLOW_IMAGE
  fi

  if docker exec -it $domain_ctr crictl inspecti $sf_image > /dev/null 2>&1 ; then
    log "Image '${sf_image}' already exists in domain '${domain_id}'"
    return
  fi

  local has_sf_image=false
  if docker image inspect ${sf_image} >/dev/null 2>&1; then
    has_sf_image=true
  fi

  if [ "$has_sf_image" == true ] ; then
    log "Found the secretflow image '${sf_image}' on host"
  else
    log "Not found the secretflow image '${sf_image}' on host"
    if [ "$repo" != "" ] ; then
      docker login $repo
    fi
    log "Start pulling image '${sf_image}' ..."
    docker pull ${sf_image}
  fi

  log "Start importing image '${sf_image}' Please be patient..."
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

function run_centralized() {
  build_kuscia_network
  local volume_path=$1
  if need_start_docker_container $MASTER_CTR; then
    log "Starting container '$MASTER_CTR' ..."
    local certs_volume=${MASTER_CTR}-certs
    env_flag=$(generate_env_flag $MASTER_DOMAIN)
    docker run -it --rm --mount source=${certs_volume},target=${CTR_CERT_ROOT} ${IMAGE} scripts/deploy/init_domain_certs.sh ${MASTER_DOMAIN}

    docker run -dit --name=${MASTER_CTR} --hostname=${MASTER_CTR} --restart=always --network=${NETWORK_NAME} -m $MASTER_MEMORY_LIMIT ${env_flag} \
      --env NAMESPACE=${MASTER_DOMAIN} \
      --mount source=${certs_volume},target=${CTR_CERT_ROOT} \
      -v /tmp:/tmp \
      ${IMAGE} scripts/deploy/start_master.sh ${MASTER_DOMAIN} ${MASTER_CTR}
    probe_gateway_crd ${MASTER_CTR} ${MASTER_DOMAIN} ${MASTER_CTR} 60
    log "Master '${MASTER_DOMAIN}' started successfully"
    FORCE_START=true
  fi

  start_lite ${ALICE_DOMAIN} https://${MASTER_CTR}:1080 ${volume_path}
  start_lite ${BOB_DOMAIN} https://${MASTER_CTR}:1080 ${volume_path}

  create_cluster_domain_route ${ALICE_DOMAIN} ${BOB_DOMAIN}
  create_cluster_domain_route ${BOB_DOMAIN} ${ALICE_DOMAIN}

  check_sf_image $ALICE_DOMAIN ${CTR_PREFIX}-lite-${ALICE_DOMAIN} ${volume_path}
  check_sf_image $BOB_DOMAIN ${CTR_PREFIX}-lite-${BOB_DOMAIN} ${volume_path}

  # create datamesh svc
  create_domain_datamesh_svc ${MASTER_CTR} ${ALICE_DOMAIN} ${CTR_PREFIX}-lite-${ALICE_DOMAIN}
  create_domain_datamesh_svc ${MASTER_CTR} ${BOB_DOMAIN} ${CTR_PREFIX}-lite-${BOB_DOMAIN}

  # create demo data
  create_domaindata_alice_table ${MASTER_CTR} ${ALICE_DOMAIN}
  create_domaindata_bob_table ${MASTER_CTR} ${BOB_DOMAIN}

  # create secretflow app image
  create_secretflow_app_image ${MASTER_CTR}

  log "Kuscia centralized cluster started successfully"
}

function run_centralized_all() {
  ui_flag=$1
  if [ "${ui_flag}" == "cli" ]; then
    run_centralized
    exit 0
  fi
  read -rp "$(echo -e "${GREEN}Please specify a valid and existing path for storage the demo data,Or would use the default path('$VOLUME_PATH').\n\
Please input a valid path for demo data('$VOLUME_PATH'):${NC}")" volume_path
  if [ -d "${volume_path}" ]; then
    echo "use the '${volume_path}' as the data path."
    VOLUME_PATH=${volume_path}
  else
    echo -e "${RED}the input path is not valid or is not a existing directory, would use the '${VOLUME_PATH}' as the demo data path.${NC}"
  fi
  account_settings
  run_centralized $VOLUME_PATH
  start_secretpad $VOLUME_PATH $SECRETPAD_USER_NAME $SECRETPAD_PASSWORD
}

function start_autonomy() {
  local domain_id=$1
  local domain_ctr=${CTR_PREFIX}-autonomy-${domain_id}
  if need_start_docker_container $domain_ctr; then
    log "Starting container '$domain_ctr' ..."
    env_flag=$(generate_env_flag $domain_id)
    docker run -it --rm --mount source=${domain_ctr}-certs,target=${CTR_CERT_ROOT} ${IMAGE} scripts/deploy/init_domain_certs.sh ${domain_id}
    docker run -it --rm --mount source=${domain_ctr}-certs,target=${CTR_CERT_ROOT} ${IMAGE} scripts/deploy/init_external_tls_cert.sh ${domain_id}
    docker run -dit --privileged --name=${domain_ctr} --hostname=${domain_ctr} --restart=always --network=${NETWORK_NAME} -m $AUTONOMY_MEMORY_LIMIT ${env_flag} \
      --env NAMESPACE=${domain_id} \
      --mount source=${domain_ctr}-containerd,target=${CTR_ROOT}/containerd \
      --mount source=${domain_ctr}-certs,target=${CTR_CERT_ROOT} \
      -v /tmp:/tmp \
      --entrypoint bin/entrypoint.sh \
      ${IMAGE} tini -- scripts/deploy/start_autonomy.sh ${domain_id} ${ALLOW_PRIVILEGED}
    probe_gateway_crd ${domain_ctr} ${domain_id} ${domain_ctr} 60
    log "Autonomy domain '${domain_id}' started successfully docker container name: '${domain_ctr}' crt path:'${CTR_CERT_ROOT}'"
  fi
}

function build_interconn() {
  local member_domain=$1
  local host_domain=$2
  local interconn_protocol=$3
  local member_ctr=${CTR_PREFIX}-autonomy-${member_domain}
  local host_ctr=${CTR_PREFIX}-autonomy-${host_domain}

  log "Starting build internet connect from '${member_domain}' to '${host_domain}'"
  copy_between_containers ${member_ctr}:${CTR_CERT_ROOT}/domain.csr ${host_ctr}:${CTR_CERT_ROOT}/${member_domain}.domain.csr
  docker exec -it ${host_ctr} scripts/deploy/add_domain.sh $member_domain ${host_ctr} p2p ${interconn_protocol}
  copy_between_containers ${host_ctr}:${CTR_CERT_ROOT}/${member_domain}.domain.crt ${member_ctr}:${CTR_CERT_ROOT}/domain-2-${host_domain}.crt
  copy_between_containers ${host_ctr}:${CTR_CERT_ROOT}/ca.crt ${member_ctr}:${CTR_CERT_ROOT}/${host_domain}.host.ca.crt

  docker exec -it ${member_ctr} scripts/deploy/join_to_host.sh $member_domain $host_domain ${host_ctr}:1080 -p ${interconn_protocol}
  log "Build internet connect from '${member_domain}' to '${host_domain}' successfully protocol: '${interconn_protocol}' dest host: '${host_ctr}':1080"
}

function run_p2p() {
  local p2p_protocol=$1
  build_kuscia_network

  start_autonomy ${ALICE_DOMAIN}
  start_autonomy ${BOB_DOMAIN}

  build_interconn ${ALICE_DOMAIN} ${BOB_DOMAIN}  ${p2p_protocol}
  build_interconn ${BOB_DOMAIN} ${ALICE_DOMAIN}  ${p2p_protocol}

  check_sf_image $ALICE_DOMAIN ${CTR_PREFIX}-autonomy-${ALICE_DOMAIN}
  check_sf_image $BOB_DOMAIN ${CTR_PREFIX}-autonomy-${BOB_DOMAIN}

  create_secretflow_app_image ${CTR_PREFIX}-autonomy-${ALICE_DOMAIN}
  create_secretflow_app_image ${CTR_PREFIX}-autonomy-${BOB_DOMAIN}

  # create datamesh svc
  create_domain_datamesh_svc ${CTR_PREFIX}-autonomy-${ALICE_DOMAIN} ${ALICE_DOMAIN} ${CTR_PREFIX}-autonomy-${ALICE_DOMAIN}
  create_domain_datamesh_svc ${CTR_PREFIX}-autonomy-${BOB_DOMAIN} ${BOB_DOMAIN} ${CTR_PREFIX}-autonomy-${BOB_DOMAIN}

  # create demo data
  create_domaindata_alice_table ${CTR_PREFIX}-autonomy-${ALICE_DOMAIN} ${ALICE_DOMAIN}
  create_domaindata_bob_table ${CTR_PREFIX}-autonomy-${BOB_DOMAIN} ${BOB_DOMAIN}

  log "Kuscia p2p cluster started successfully"
}

function build_kuscia_network() {
  if [[ ! "$(docker network ls -q -f name=${NETWORK_NAME})" ]]; then
    docker network create ${NETWORK_NAME}
  fi
}

usage() {
  echo "$(basename "$0") NETWORK_MODE [OPTIONS]
NETWORK_MODE:
    center,centralized       centralized network mode (default)
    p2p                      p2p network mode

Common Options:
    -h                  show this help text
    -p                  interconnection protocol, must be 'kuscia' or 'bfia', default is 'kuscia'. In current quickstart script, center mode just support 'bfia' protocol.
    -u                  user interface, must be 'web' or 'cli', default is 'cli',webui only support for centralized network mode.
                        'web' means install kuscia with web UI, user could access the website (http://127.0.0.1:8088) to experience the web features via the webui,
                        'cli' means experience the features of kuscia via command line interface. "
}

mode=
case  "$1" in
center | centralized | p2p)
  mode=$1
  shift
  ;;
esac

interconn_protocol=
ui=
while getopts 'p:u:h' option; do
  case "$option" in
  p)
    interconn_protocol=$OPTARG
    [ "$interconn_protocol" == "bfia" -o "$interconn_protocol" == "kuscia" ] && continue
    printf "illegal value for -%s\n" "$option" >&2
    usage
    exit
    ;;
  u)
    ui=$OPTARG
    [ "$ui" == "web" -o "$ui" == "cli" ] && continue
    printf "illegal value for -%s\n" "$option" >&2
    usage
    exit
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
[ "$ui" == "" ] && ui="cli"
[ "$mode" == "" ] && mode=$1
[ "$mode" == "" -o "$mode" == "centralized" ] && mode="center"
if [ "$mode" == "center"  -a "$interconn_protocol" != "kuscia" ]; then
    printf "In current quickstart script, center mode just support 'kuscia'\n"  >&2
    exit 1
fi
if [ "$mode" != "center" ] && [ $ui == "web" ]
then
  echo "webui only support for centralized network mode."
  exit 1
fi


case  "$mode" in
center)
  run_centralized_all $ui
  ;;
p2p)
  run_p2p $interconn_protocol
  ;;
*)
  printf "unsupported network mode: %s\n" "$mode" >&2
  exit 1
;;
esac
