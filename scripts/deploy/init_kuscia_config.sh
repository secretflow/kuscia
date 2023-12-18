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

#TODO replace with kuscia init

MODE=$1
DOMAIN_ID=$2
MASTER_ENDPOINT=$3
DEPLOY_TOKEN=$4
ALLOW_PRIVILEGED=$5

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)

CONFIG_DATA=""

PRIVILEGED_CONFIG="
agent:
  allowPrivileged: true
"

DOMAIN_KEY_FILE="/home/kuscia/var/certs/domain.key"
if [[ -e ${DOMAIN_KEY_FILE} ]]; then
  echo -e "Domain key file already exists"
  DOMAIN_KEY_DATA=$(base64 -i ${DOMAIN_KEY_FILE} | tr -d "\n")
else
  echo -e "Generate key data"
  DOMAIN_KEY_DATA=$(openssl genrsa 2048 | base64 | tr -d "\n")
fi

if [[ $MODE == "lite" ]]; then
  CONFIG_DATA=$(sed -e "s!{{.DOMAIN_ID}}!${DOMAIN_ID}!g;
        s!{{.MASTER_ENDPOINT}}!${MASTER_ENDPOINT}!g;
        s!{{.DEPLOY_TOKEN}}!${DEPLOY_TOKEN}!g;
        s!{{.DOMAIN_KEY_DATA}}!${DOMAIN_KEY_DATA}!g" \
    <"${ROOT}/scripts/templates/kuscia-lite.yaml")
  if [[ $ALLOW_PRIVILEGED == "true" ]]; then
    CONFIG_DATA=$(echo -e "$CONFIG_DATA$PRIVILEGED_CONFIG")
  fi
elif [[ $MODE == "master" ]]; then
  CONFIG_DATA=$(sed "s!{{.DOMAIN_ID}}!${DOMAIN_ID}!g;
        s!{{.DOMAIN_KEY_DATA}}!${DOMAIN_KEY_DATA}!g" \
    <"${ROOT}/scripts/templates/kuscia-master.yaml")
elif [[ $MODE == "autonomy" ]]; then
  CONFIG_DATA=$(sed -e "s!{{.DOMAIN_ID}}!${DOMAIN_ID}!g;
          s!{{.DOMAIN_KEY_DATA}}!${DOMAIN_KEY_DATA}!g" \
    <"${ROOT}/scripts/templates/kuscia-autonomy.yaml")
  if [[ $ALLOW_PRIVILEGED == "true" ]]; then
    CONFIG_DATA=$(echo -e "$CONFIG_DATA$PRIVILEGED_CONFIG")
  fi
else
  echo "Unsupported mode: $MODE"
  exit 1
fi

echo "${CONFIG_DATA}" >/tmp/kuscia.yaml
