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

IMAGE=""
DEPLOY_USER="${USER}"
DEPLOY_MODE=""
APP_IMAGE_NAME_IN_KUSCIA=""

usage="$(basename "$0") [OPTIONS]

OPTIONS:
    -h    [optional] show this help text
    -i    [mandatory] app docker image info
    -m    [mandatory] kuscia deploy mode. support [center, p2p], default value: p2p
    -u    [optional] user who deploy the kuscia container. default value: ${USER}
    -n    [optional] kuscia appImage name for app docker image. if not specified, the script will automatically generate a name

example:
 ./register_app_image.sh -u ${USER} -m center -n secretflow-image -i secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/secretflow-lite-anolis8:latest
"

while getopts ':hi:u:m:n:' option; do
  case "$option" in
  h)
    echo "$usage"
    exit
    ;;
  i)
    IMAGE=$OPTARG
    ;;
  u)
    DEPLOY_USER=$OPTARG
    ;;
  m)
    DEPLOY_MODE=$OPTARG
    ;;
  n)
    APP_IMAGE_NAME_IN_KUSCIA=$OPTARG
    ;;
  \?)
    echo -e "invalid option: -$OPTARG" && echo "${usage}"
    exit 1
    ;;
  esac
done

if [[ $IMAGE = "" ]]; then
  echo "please use flag '-i' to provide image info"
  echo "$usage"
  exit 1
fi

if [[ $DEPLOY_MODE != "center" && $DEPLOY_MODE != "p2p" ]]; then
    echo "please use flag '-m' to provide correct kuscia deploy mode"
    echo "$usage"
    exit 1
fi

CTR_ROOT="/home/kuscia"
KUSCIA_MASTER_CONTAINER_NAME="${DEPLOY_USER}-kuscia-master"
KUSCIA_ALICE_LITE_CONTAINER_NAME="${DEPLOY_USER}-kuscia-lite-alice"
KUSCIA_BOB_LITE_CONTAINER_NAME="${DEPLOY_USER}-kuscia-lite-bob"
KUSCIA_ALICE_AUTONOMY_CONTAINER_NAME="${DEPLOY_USER}-kuscia-autonomy-alice"
KUSCIA_BOB_AUTONOMY_CONTAINER_NAME="${DEPLOY_USER}-kuscia-autonomy-bob"

IMAGE_TEMP_DIR="/tmp/kuscia-appimage-tmp"
APP_IMAGE_FILE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)/${APP_IMAGE_NAME_IN_KUSCIA}.yaml"
APP_IMAGE_TEMP_FILE="${IMAGE_TEMP_DIR}/appimage_tmp.yaml"

function prepare_app_image() {
  echo "=> start preparing app image..."
  image_name=$1
  image_tag=$2

  image_tar=${IMAGE_TEMP_DIR}/$(echo ${image_name} | sed 's/\//_/g' ).tar
  if [[ ! -e ${image_tar} ]] ; then
    docker save "${IMAGE}" -o "${image_tar}"
  fi

  alice_container_name=${KUSCIA_ALICE_LITE_CONTAINER_NAME}
  bob_container_name=${KUSCIA_BOB_LITE_CONTAINER_NAME}
  if [[ $DEPLOY_MODE = "p2p" ]]; then
    alice_container_name=${KUSCIA_ALICE_AUTONOMY_CONTAINER_NAME}
    bob_container_name=${KUSCIA_BOB_AUTONOMY_CONTAINER_NAME}
  fi

  if [[ -n $(docker ps -q -f "name=${alice_container_name}") ]]; then
    echo "=> => import app image into ${alice_container_name} container"
    docker exec -it "${alice_container_name}" ctr -a=${CTR_ROOT}/containerd/run/containerd.sock -n=k8s.io images import "${image_tar}" || exit 1
  fi

  if [[ -n $(docker ps -q -f "name=${bob_container_name}") ]]; then
    echo "=> => import app image into ${bob_container_name} container"
    docker exec -it "${bob_container_name}" ctr -a=${CTR_ROOT}/containerd/run/containerd.sock -n=k8s.io images import "${image_tar}" || exit 1
  fi

  if [[ $APP_IMAGE_NAME_IN_KUSCIA = "" ]]; then
    APP_IMAGE_NAME_IN_KUSCIA=$(echo ${image_name##*/}-${image_tag} | sed 's/_/-/g')
  fi

  app_image_content=$(sed "s!{{APP_IMAGE_NAME}}!${APP_IMAGE_NAME_IN_KUSCIA}!g;
    s!{{IMAGE_NAME}}!${image_name}!g; \
    s!{{IMAGE_TAG}}!${image_tag}!g" \
    < "${APP_IMAGE_FILE}")

  echo "${app_image_content}" > ${APP_IMAGE_TEMP_FILE}

  if [[ $DEPLOY_MODE = "p2p" ]]; then
      if [[ -n $(docker ps -q -f "name=${alice_container_name}") ]]; then
        docker exec -it "${alice_container_name}" kubectl apply -f "${APP_IMAGE_TEMP_FILE}" || exit 1
      fi
      if [[ -n $(docker ps -q -f "name=${bob_container_name}") ]]; then
        docker exec -it "${bob_container_name}" kubectl apply -f "${APP_IMAGE_TEMP_FILE}" || exit 1
      fi
  else
      docker exec -it "${KUSCIA_MASTER_CONTAINER_NAME}" kubectl apply -f "${APP_IMAGE_TEMP_FILE}" || exit 1
  fi

  echo "=> finish preparing app image"
}

function post_action() {
  echo "=> remove temporary directory ${IMAGE_TEMP_DIR}"
  rm -rf "${IMAGE_TEMP_DIR}"
}

function register_app_image() {
  echo "=> register app image: ${IMAGE}"
  image_name=$(echo "${IMAGE}" | awk -F ":" '{print $1}')
  image_tag=$(echo "${IMAGE}" | awk -F ":" '{print $2}')

  if [[ $image_name = "" ]]; then
    echo "=> => split image ${IMAGE} failed"
    exit 1
  fi

  if [[ $image_tag = "" ]]; then
    image_tag="latest"
  fi

  if [[ ! -d ${IMAGE_TEMP_DIR} ]]; then
    mkdir ${IMAGE_TEMP_DIR}
  fi

  prepare_app_image "${image_name}" "${image_tag}"
  post_action
  echo "=> finish registering app image: ${IMAGE}"
  echo "=> app_image_name: ${APP_IMAGE_NAME_IN_KUSCIA}"
}

register_app_image