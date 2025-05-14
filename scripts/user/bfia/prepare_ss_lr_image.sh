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

GREEN='\033[0;32m'
NC='\033[0m'

KUSCIA_CONTAINER_NAME="${USER}-kuscia-autonomy-alice"
OPERATOR_IMAGE="secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/ss-lr"

usage="$(basename "$0") [OPTIONS]

OPTIONS:
    -h    show this help text
    -i    (optional) operator image info, default value: secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/ss-lr
    -k    (optional) kuscia container name, default value: ${USER}-kuscia-autonomy-alice

example:
 ./bfia/prepare_ss_lr_image.sh -k ${USER}-kuscia-autonomy-alice -i secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/ss-lr
"

while getopts ':hi:k:' option; do
  case "$option" in
  h)
    echo "$usage"
    exit
    ;;
  i)
    OPERATOR_IMAGE=$OPTARG
    ;;
  k)
    KUSCIA_CONTAINER_NAME=$OPTARG
    ;;
  \?)
    echo -e "${GREEN}Invalid option: -$OPTARG${NC}" && echo "${usage}"
    exit 1
    ;;
  esac
done

echo -e "${GREEN}Operator image: ${OPERATOR_IMAGE}${NC}"
echo -e "${GREEN}Kuscia container name: ${KUSCIA_CONTAINER_NAME}${NC}"

CTR_ROOT=/home/kuscia
function prepare_operator_image() {
  local image_name=$1
  local image_tag=$2

  if [ "${OPERATOR_IMAGE}" == "" ]; then
    echo -e "${GREEN}Invalid operator image '${OPERATOR_IMAGE}'${NC}"
    exit 1
  fi

  local has_image=false
  if docker image inspect "${OPERATOR_IMAGE}" >/dev/null 2>&1; then
    has_image=true
  fi

  if docker exec -it "${KUSCIA_CONTAINER_NAME}" crictl inspecti "${OPERATOR_IMAGE}" > /dev/null 2>&1 ; then
    echo -e "${GREEN}Operator image '${OPERATOR_IMAGE}' already exists in container '${KUSCIA_CONTAINER_NAME}'${NC}"
    return
  fi

  if [ "$has_image" == true ] ; then
    echo -e "${GREEN}Found the operator image '${OPERATOR_IMAGE}' on host${NC}"
  else
    echo -e "${GREEN}Not found the operator image '${OPERATOR_IMAGE}' on host and pulling...${NC}"
    docker pull "${OPERATOR_IMAGE}"
  fi

  echo -e "${GREEN}Start importing operator image '${OPERATOR_IMAGE}'...${NC}"
  local image_tar
  image_tar="/tmp/$(echo "${image_name}" | sed 's/\//_/g' ).tar"
  if [ ! -e "${image_tar}" ] ; then
    docker save "${OPERATOR_IMAGE}" -o "${image_tar}"
  fi

  docker exec -it "${KUSCIA_CONTAINER_NAME}" ctr -a="${CTR_ROOT}/containerd/run/containerd.sock" -n=k8s.io images import "${image_tar}"
  rm -rf "${image_tar}"
  echo -e "${GREEN}Finish preparing operator image '${OPERATOR_IMAGE}' to container '${KUSCIA_CONTAINER_NAME}'...${NC}"
}

function prepare_kuscia_app_image() {
  local image_name=$1
  local image_tag=$2
  app_image_name="/tmp/ss-lr-appImage.yaml"

echo "
apiVersion: kuscia.secretflow/v1alpha1
kind: AppImage
metadata:
  name: ss-lr
spec:
  deployTemplates:
  - name: ss-lr
    replicas: 1
    spec:
      containers:
      - name: ss-lr
  image:
    id: 880258fc0d3b
    name: ${image_name}
    sign: 880258fc0d3b
    tag: ${image_tag}
" > "${app_image_name}"

docker exec -it "${KUSCIA_CONTAINER_NAME}" kubectl apply -f "${app_image_name}"
docker exec -it "${KUSCIA_CONTAINER_NAME}" kubectl annotate appimage ss-lr kuscia.secretflow/component-spec='{"component.1.description":"ss-lr","component.1.input.1.categories":"dataset","component.1.input.1.description":"train data","component.1.input.1.name":"train_data","component.1.name":"ss_lr","component.1.output.1.categories":"dataset","component.1.output.1.description":"train data","component.1.output.1.name":"train_data"}' --overwrite

rm -rf "${app_image_name}"
echo -e "${GREEN}Finished preparing appImage '${OPERATOR_IMAGE}' in container '${KUSCIA_CONTAINER_NAME}'${NC}"
}

function prepare_bfia_ss_lr_image() {
  operator_image_name=$(echo "${OPERATOR_IMAGE}" | awk -F ":" '{print $1}')
  operator_image_tag=$(echo "${OPERATOR_IMAGE}" | awk -F ":" '{print $2}')
  if [ "${operator_image_tag}" = "" ]; then
    operator_image_tag="latest"
  fi

  prepare_operator_image "$operator_image_name" "$operator_image_tag"
  prepare_kuscia_app_image "$operator_image_name" "$operator_image_tag"
}

prepare_bfia_ss_lr_image
