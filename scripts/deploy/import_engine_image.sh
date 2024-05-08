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

kuscia_container_name=$1
engine_image=$2

if docker exec -it ${kuscia_container_name} crictl inspecti ${engine_image} >/dev/null 2>&1; then
   echo -e "${GREEN}Image '${engine_image}' already exists in domain ${kuscia_container_name}${NC}"
else
  if docker image inspect ${engine_image} >/dev/null 2>&1; then
    echo -e "${GREEN}Found the engine image '${engine_image}' on host${NC}"
  else
    echo -e "${GREEN}Not found the engine image '${engine_image}' on host${NC}"
    echo -e "${GREEN}Start pulling image '${engine_image}' ...${NC}"
    docker pull ${engine_image}
  fi
  image_tag=$(echo ${engine_image} | cut -d ':' -f 2)
  echo -e "${GREEN}Start importing image '${engine_image}' Please be patient...${NC}"

  image_tar=/tmp/${image_tag}.tar
  docker save ${engine_image} -o ${image_tar}
  docker exec -it ${kuscia_container_name} ctr -a=/home/kuscia/containerd/run/containerd.sock -n=k8s.io images import ${image_tar}

  if docker exec -it ${kuscia_container_name} crictl inspecti ${engine_image} >/dev/null 2>&1; then
    rm -rf ${image_tar}
    echo -e "${GREEN}image ${engine_image} import successfully${NC}"
  else
    echo -e "${RED}error: ${engine_image} import failed${NC}"
  fi
fi