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

SRC_DOMAIN=$1
DEST_DOMAIN=$2
DEST_ENDPOINT=$3

usage="$(basename "$0") SRC_DOMAIN DEST_DOMAIN DEST_ENDPOINT(http(s)://ip:port) "

if [[ ${SRC_DOMAIN} == "" || ${DEST_DOMAIN} == "" || ${DEST_ENDPOINT} == "" ]]; then
  echo "missing argument: $usage"
  exit 1
fi

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
HOST=${DEST_ENDPOINT}
PORT=80
PROTOCOL_TLS=false

if [[ "${DEST_ENDPOINT}" == *"://"* ]]; then
  HOST=${DEST_ENDPOINT##*://}
  if [[ "${DEST_ENDPOINT}" == https://* ]]; then
    PROTOCOL_TLS=true
    PORT=443
  fi
fi

if [[ "${HOST}" == *":"* ]]; then
  PORT=${HOST##*:}
  HOST=${HOST%%:*}
fi

CLUSTER_DOMAIN_ROUTE_TEMPLATE=$(sed "s/{{.SRC_DOMAIN}}/${SRC_DOMAIN}/g;
  s/{{.DEST_DOMAIN}}/${DEST_DOMAIN}/g;
  s/{{.HOST}}/${HOST}/g;
  s/{{.ISTLS}}/${PROTOCOL_TLS}/g;
  s/{{.PORT}}/${PORT}/g" \
  <"${ROOT}/scripts/templates/cluster_domain_route.token.yaml")
echo "${CLUSTER_DOMAIN_ROUTE_TEMPLATE}" | kubectl apply -f -