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

FATE_CLUSTER_NAME=$1
FATE_IMAGE=$2
KUSCIA_CLUSTER_ID=$3
FATA_CLUSTER_ID=$4
KUSCIA_CLUSTER_IP=$5

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)

KUSCIA_FATE_TEMPLATE=$(sed "s!{{.PARTY}}!${FATE_CLUSTER_NAME}!g;
  s!{{.MEMORY}}!8G!g;
  s!{{.FATE_IMAGE}}!${FATE_IMAGE}!g;
  s!{{.PARTY_ID}}!${KUSCIA_CLUSTER_ID}!g;
  s!{{.OTHER_PARTY_ID}}!${FATA_CLUSTER_ID}!g;
  s!{{.OTHER_PARTY_IP}}!${KUSCIA_CLUSTER_IP}!g" \
  < "${ROOT}/scripts/templates/fate_deploy.yaml")


echo "${KUSCIA_FATE_TEMPLATE}" | kubectl apply -f -
