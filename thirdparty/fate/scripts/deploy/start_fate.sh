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

DOMAIN_CTR=$1
KUSCIA_FATE_CTR=$2
KUSCIA_FATE_CLUSTER_NAME=$3
FATE_CLUSTER_NAME=$4

CTR_ROOT=/home/kuscia
FATE_IMAGE_NAME="secretflow/fate-deploy-basic"
FATE_IMAGE_TAG="0.0.1"
FATE_IMAGE="${FATE_IMAGE_NAME}:${FATE_IMAGE_TAG}"
FATE_ADAPTER_IMAGE_NAME="secretflow/fate-adapter"
FATE_ADAPTER_IMAGE_TAG="0.0.1"
FATE_ADAPTER_IMAGE="${FATE_ADAPTER_IMAGE_NAME}:${FATE_ADAPTER_IMAGE_TAG}"
ALIYUN_IMAGE_PREFXI="secretflow-registry.cn-hangzhou.cr.aliyuncs.com"
FATA_CLUSTER_ID=9999
KUSCIA_CLUSTER_ID=10000

function copy_fate_deploy_image() {
  if docker exec -it ${KUSCIA_FATE_CTR} crictl inspecti ${FATE_IMAGE} > /dev/null 2>&1 ; then
    echo "has image"
    return 
  fi

  local image_tar=/tmp/fate-deploy.${FATE_IMAGE_TAG}.tar

  echo -e "Start importing image '${FATE_IMAGE}'"
  
  docker save ${FATE_IMAGE} -o $image_tar
  docker exec -it ${KUSCIA_FATE_CTR} ctr -a=${CTR_ROOT}/containerd/run/containerd.sock -n=k8s.io images import $image_tar
  docker exec -it ${KUSCIA_FATE_CTR} ctr -a=${CTR_ROOT}/containerd/run/containerd.sock images tag docker.io/$FATE_IMAGE $ALIYUN_IMAGE_PREFXI/$FATE_IMAGE

  echo -e "Successfully imported image '${FATE_IMAGE}' to container '${KUSCIA_FATE_CTR}'"
}

function copy_fate_adapter_image {
  if docker exec -it ${KUSCIA_FATE_CTR} crictl inspecti ${FATE_ADAPTER_IMAGE} > /dev/null 2>&1 ; then
    echo "has image"
    return 
  fi

  local image_tar=/tmp/fate-deploy.${FATE_ADAPTER_IMAGE_TAG}.tar

  echo -e "Start importing image '${FATE_ADAPTER_IMAGE}'"

  docker save ${FATE_ADAPTER_IMAGE} -o $image_tar
  docker exec -it ${KUSCIA_FATE_CTR} ctr -a=${CTR_ROOT}/containerd/run/containerd.sock -n=k8s.io images import $image_tar
  docker exec -it ${KUSCIA_FATE_CTR} ctr -a=${CTR_ROOT}/containerd/run/containerd.sock -n=k8s.io images tag docker.io/$FATE_ADAPTER_IMAGE $ALIYUN_IMAGE_PREFXI/$FATE_ADAPTER_IMAGE

  echo -e "Successfully imported image '${FATE_ADAPTER_IMAGE}' to container '${KUSCIA_FATE_CTR}'"
}


function deploy_fate_cluster() {
    local fate_cluster_name="fate-${FATE_CLUSTER_NAME}"
    local kuscia_fate_cluster_ip=$(docker container inspect -f '{{ $network := index .NetworkSettings.Networks "kuscia-exchange" }}{{ $network.IPAddress}}' ${KUSCIA_FATE_CTR})
    docker run -itd -p 32141:8080 --name=${fate_cluster_name} --privileged ${FATE_IMAGE} scripts/start_third_party.sh fate ${FATA_CLUSTER_ID} ${KUSCIA_CLUSTER_ID} ${kuscia_fate_cluster_ip}
    docker network connect kuscia-exchange ${fate_cluster_name}

    local fate_cluster_ip=$(docker container inspect -f '{{ $network := index .NetworkSettings.Networks "kuscia-exchange" }}{{ $network.IPAddress}}' ${fate_cluster_name})
    local fate_image="secretflow/fate-deploy-basic:${FATE_IMAGE_TAG}"
    docker exec -it ${DOMAIN_CTR} scripts/deploy/deploy_kuscia_fate.sh ${KUSCIA_FATE_CLUSTER_NAME} ${fate_image} ${KUSCIA_CLUSTER_ID} ${FATA_CLUSTER_ID} ${fate_cluster_ip}

}

copy_fate_deploy_image
copy_fate_adapter_image
deploy_fate_cluster
