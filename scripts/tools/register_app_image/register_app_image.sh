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

IMAGE=""
DEPLOY_MODE="p2p"
DEPLOY_USER="${USER}"
DOMAIN_IDS="alice,bob"
APP_IMAGE_NAME_IN_KUSCIA=""
APP_IMAGE_TEMPLATE_FILE=""

usage="$(basename "$0") [OPTIONS]

OPTIONS:
    -h    [optional] show this help text
    -i    [mandatory] app docker image info
    -m    [optional] kuscia deploy mode. support [center, p2p], default value: p2p
    -d    [optional] domain ids that makes up the kuscia container name. default value: alice,bob
    -u    [optional] user who deploy the kuscia container. default value: ${USER}
    -n    [optional] kuscia appImage name for app docker image. if not specified, the script will automatically generate a name
    -f    [optional] kuscia appImage template file full path. the recommended template file naming rule is {appImage name}.yaml under the same directory as tool script. otherwise the file full path must be specified

example:
 ./register_app_image.sh -u ${USER} -m center -d alice,bob -n secretflow-image -f ./secretflow-image.yaml -i secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/secretflow-lite-anolis8:latest
"

while getopts ':hi:u:d:m:n:f:' option; do
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
  d)
    DOMAIN_IDS=$OPTARG
    ;;
  m)
    DEPLOY_MODE=$OPTARG
    ;;
  n)
    APP_IMAGE_NAME_IN_KUSCIA=$OPTARG
    ;;
  f)
    APP_IMAGE_TEMPLATE_FILE=$OPTARG
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
KUSCIA_DOMAIN_CONTAINER_NAMES=()

DEFAULT_DOCKER_REPO="docker.io"
DEFAULT_DOCKER_REPO_BUCKET="docker.io/library"

APP_IMAGE_FILE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
DEFAULT_APP_IMAGE_FILE="${APP_IMAGE_FILE_DIR}/secretflow-image.yaml"
APP_IMAGE_FILE=""

function prepare_app_image() {
  import_appimage "$1" "$2"
  apply_appimage_crd "$1" "$2"
}

function import_appimage(){
  echo "=> start import containerd image..."
  image_name=$1
  image_tag=$2

  if [[ -z $(docker images -q -f "reference=${IMAGE}") ]]; then
    docker pull "${IMAGE}"
  fi

  ctr_num=${#KUSCIA_DOMAIN_CONTAINER_NAMES[@]}

  idx=0
  while ((idx<ctr_num)); do
    container_name=${KUSCIA_DOMAIN_CONTAINER_NAMES[$idx]}
    echo "=> => import app image into ${container_name} container"
    domain_image_work_dir=$(docker inspect -f '{{range .Mounts}}{{if eq .Destination "/home/kuscia/var/images"}}{{.Source}}{{end}}{{end}}' "$container_name")
    image_tar=${domain_image_work_dir}/${image_tag}.tar
    docker save "${image_name}:${image_tag}" -o "${image_tar}"
    docker exec -it "${container_name}" kuscia image load -i ${CTR_ROOT}/var/images/"${image_tag}".tar
    rm -rf "${image_tar}"
    idx+=1
  done

  wait

  echo "=> finish import containerd image"
}

function apply_appimage_crd(){
  echo "=> start apply kuscia AppImage crd..."
  image_name=$1
  image_tag=$2

  if [[ ${APP_IMAGE_NAME_IN_KUSCIA} = "" ]]; then
    APP_IMAGE_NAME_IN_KUSCIA=$(echo "${image_name##*/}"-"${image_tag}" | sed 's/_/-/g')
  fi

  if [[ ${APP_IMAGE_TEMPLATE_FILE} != "" ]]; then
    APP_IMAGE_FILE=${APP_IMAGE_TEMPLATE_FILE}
  else
    APP_IMAGE_FILE=${APP_IMAGE_FILE_DIR}/${APP_IMAGE_NAME_IN_KUSCIA}.yaml
  fi

  if [[ ! -f $APP_IMAGE_FILE ]]; then
      echo "=> => $APP_IMAGE_FILE is not exist, register fail"
      exit 1
  fi

  app_image_content=$(sed "s!{{APP_IMAGE_NAME}}!${APP_IMAGE_NAME_IN_KUSCIA}!g;
    s!{{IMAGE_NAME}}!${image_name}!g; \
    s!{{IMAGE_TAG}}!${image_tag}!g" \
    < "${APP_IMAGE_FILE}")

  if [[ $DEPLOY_MODE = "p2p" ]]; then
      for container_name in "${KUSCIA_DOMAIN_CONTAINER_NAMES[@]}"; do
        domain_image_work_dir=$(docker inspect -f '{{range .Mounts}}{{if eq .Destination "/home/kuscia/var/images"}}{{.Source}}{{end}}{{end}}' "$container_name")
        APP_IMAGE_TEMP_FILE="${domain_image_work_dir}/appimage_tmp.yaml"
        echo "${app_image_content}" > "${APP_IMAGE_TEMP_FILE}"
        docker exec -it "${container_name}" kubectl apply -f "${CTR_ROOT}/var/images/appimage_tmp.yaml" || exit 1
        rm -rf "${APP_IMAGE_TEMP_FILE}"
      done
  else
      domain_image_work_dir=$(docker inspect -f '{{range .Mounts}}{{if eq .Destination "/home/kuscia/var/images"}}{{.Source}}{{end}}{{end}}' "${KUSCIA_MASTER_CONTAINER_NAME}")
      APP_IMAGE_TEMP_FILE="${domain_image_work_dir}/appimage_tmp.yaml"
      echo "${app_image_content}" > "${APP_IMAGE_TEMP_FILE}"
      docker exec -it "${KUSCIA_MASTER_CONTAINER_NAME}" kubectl apply -f "${CTR_ROOT}/var/images/appimage_tmp.yaml" || exit 1
      rm -rf "${APP_IMAGE_TEMP_FILE}"
  fi
  echo "=> finish apply kuscia AppImage crd"
}

function gen_domain_container_names(){
  IFS=',' read -ra DOMAINS <<< "$DOMAIN_IDS"
  for DOMAIN in "${DOMAINS[@]}"; do
    container_name="${DEPLOY_USER}-kuscia-lite-${DOMAIN}"
    if [[ $DEPLOY_MODE = "p2p" ]]; then
      container_name="${DEPLOY_USER}-kuscia-autonomy-${DOMAIN}"
    fi

    if [[ -n $(docker ps -q -f "name=${container_name}") ]]; then
      KUSCIA_DOMAIN_CONTAINER_NAMES+=("${container_name}")
    else
      echo "=> ${container_name} container is not exist! will skip it."
    fi
  done
}

function register_app_image() {
  echo "=> register app image: ${IMAGE}"
  image_name=$(echo "${IMAGE}" | awk -F ":" '{print $1}')
  image_tag=$(echo "${IMAGE}" | awk -F ":" '{print $2}')

  if [[ ${image_name} = "" ]]; then
    echo "=> => split image ${IMAGE} failed"
    exit 1
  fi

  if [[ ${image_tag} = "" ]]; then
    image_tag="latest"
  fi

  count=$(echo "${image_name}" | grep -o "/" | wc -l)
  if [[ ${count} -eq 1 ]]; then
    image_name="${DEFAULT_DOCKER_REPO}/${image_name}"
  elif [[ ${count} -eq 0 ]]; then
    image_name="${DEFAULT_DOCKER_REPO_BUCKET}/${image_name}"
  fi

  gen_domain_container_names
  prepare_app_image "${image_name}" "${image_tag}"
  echo "=> finish registering app image: ${IMAGE}"
  echo "=> app_image_name: ${APP_IMAGE_NAME_IN_KUSCIA}"
}

register_app_image
