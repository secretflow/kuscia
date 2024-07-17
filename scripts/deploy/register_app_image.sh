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
RED='\033[31m'
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
usage="$(basename "$0") [OPTIONS]

OPTIONS:
    -h    [optional] show this help text
    -c    [mandatory] kuscia container name
    -i    [mandatory] app docker image info
    -f    [optional] kuscia appImage template file full path. the recommended template file naming rule is {appImage name}.yaml under the same directory as tool script. otherwise the file full path must be specified
example:
 ./register_app_image.sh -c root-kuscia-autonomy-alice -f ./secretflow-image.yaml -i secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/secretflow-lite-anolis8:latest --import
"
NEW_ARGS=()
IMPORT=false
for arg in "$@"; do
  case $arg in
    --import)
      IMPORT=true
      ;;
    *)
       NEW_ARGS+=("$arg")
      ;;
  esac
done
set -- "${NEW_ARGS[@]}"
while getopts 'hc:i:mf:' option; do
  case "$option" in
  h)
    echo "$usage"
    exit 0
    ;;
  c)
    KUSCIA_CONTAINER_NAME=$OPTARG
    ;;
  i)
    IMAGE=$OPTARG
    ;;
  m)
    DEPLOY=ture
    ;;
  f)
    APP_IMAGE_FILE=$OPTARG
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

function import_engine_image() {
  if docker exec -it ${KUSCIA_CONTAINER_NAME} crictl inspecti ${IMAGE} >/dev/null 2>&1; then
     echo -e "${GREEN}Image '${IMAGE}' already exists in container ${KUSCIA_CONTAINER_NAME}${NC}"
  else
     if docker image inspect ${IMAGE} >/dev/null 2>&1; then
        echo -e "${GREEN}Found the engine image '${IMAGE}' on host${NC}"
     else
        echo -e "${GREEN}Not found the engine image '${IMAGE}' on host${NC}"
        echo -e "${GREEN}Start pulling image '${IMAGE}' ...${NC}"
        docker pull ${IMAGE}
     fi
     local image_tag=$(echo ${IMAGE} | cut -d ':' -f 2)
     echo -e "${GREEN}Start importing image '${IMAGE}' Please be patient...${NC}"

     local image_tar=/tmp/${image_tag}.tar
     docker save ${IMAGE} -o ${image_tar}
     docker cp ${image_tar} ${KUSCIA_CONTAINER_NAME}:/tmp
     docker exec -it ${KUSCIA_CONTAINER_NAME} ctr -a=/home/kuscia/containerd/run/containerd.sock -n=k8s.io images import ${image_tar}
     if docker exec -it ${KUSCIA_CONTAINER_NAME} crictl inspecti ${IMAGE} >/dev/null 2>&1; then
        echo -e "${GREEN}image ${IMAGE} import successfully${NC}"
     else
        echo -e "${RED}error: ${IMAGE} import failed${NC}"
     fi
     rm -rf ${image_tar}
     docker exec -it ${KUSCIA_CONTAINER_NAME} rm -rf ${image_tar}
  fi
}

function apply_appimage_crd() {
  local image_repo
  local image_tag
  if [[ "${IMAGE}" == *":"* ]]; then
     image_repo=${IMAGE%%:*}
     image_tag=${IMAGE##*:}
  fi
  if [[ ! -f $APP_IMAGE_FILE ]]; then
    echo -e "${RED}${APP_IMAGE_FILE} does not exist, register fail${NC}"
  else
    image_line=$(awk '/^  image:/{print NR; exit}' $APP_IMAGE_FILE)
    head -n "$((image_line - 1))" $APP_IMAGE_FILE > /tmp/engine_appimage.yaml
    echo -e "  image:\n    name: ${image_repo}\n    tag: ${image_tag}" >> /tmp/engine_appimage.yaml
    docker exec -it ${KUSCIA_CONTAINER_NAME} kubectl apply -f /tmp/engine_appimage.yaml
    rm -rf /tmp/engine_appimage.yaml
  fi
}

function register_default_app_image() {
  local image_repo
  local image_tag
  if [[ "${IMAGE}" == *":"* ]]; then
     image_repo=${IMAGE%%:*}
     image_tag=${IMAGE##*:}
  fi
  local app_type=$(echo "${image_repo}" | awk -F'/' '{print $NF}' | awk -F'-' '{print $1}')
  if [[ ${app_type} != "psi" ]] && [[ ${app_type} != "dataproxy" ]]; then
     app_type="secretflow"
  fi
  if [[ ${app_type} == "secretflow" ]] || [[ ${app_type} == "psi" ]]; then
    app_image_template=$(sed "s!{{.SF_IMAGE_NAME}}!'${image_repo}'!g;
    s!{{.SF_IMAGE_TAG}}!'${image_tag}'!g" \
    < "${ROOT}/scripts/templates/app_image.${app_type}.yaml")
  else
    app_image_template=$(sed "s!{{.IMAGE_NAME}}!'${image_repo}'!g;
    s!{{.IMAGE_TAG}}!'${image_tag}'!g" \
    < "${ROOT}/scripts/templates/app_image.${app_type}.yaml")
  fi
  echo "${app_image_template}" | kubectl apply -f -
}

function register_app_image() {
  if [[ -z "${KUSCIA_CONTAINER_NAME}" ]] || [[ -z "${IMAGE}" ]]; then
     echo -e "${RED}KUSCIA_CONTAINER_NAME and IMAGE must not be empty.${NC}"
     echo -e "${RED}$usage${NC}"
  else
    if ! docker ps -a --format '{{.Names}}' | grep -q "^${KUSCIA_CONTAINER_NAME}$"; then
      echo -e "${RED}Container ${KUSCIA_CONTAINER_NAME} does not exist.${NC}"
    else
      if [[ "${IMPORT}" == "true" ]]; then
         import_engine_image
      fi
      if [[ "${APP_IMAGE_FILE}" != "" ]]; then
         apply_appimage_crd
      fi
    fi
  fi
}

if [[ "${DEPLOY}" = "ture" ]]; then
  register_default_app_image
else
  register_app_image
fi