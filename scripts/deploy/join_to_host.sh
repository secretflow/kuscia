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

usage="$(basename "$0") HOST_DOMAIN_ID HOST_ENDPOINT(ip:port)"

HOST_DOMAIN_ID=$1
HOST_ENDPOINT=$2
INTERCONN_PROTOCOL=$3

[ "${INTERCONN_PROTOCOL}" != "" ] || INTERCONN_PROTOCOL="kuscia"

if [[ ${HOST_DOMAIN_ID} == "" || ${HOST_ENDPOINT} == "" ]]; then
  echo "missing argument: $usage"
  exit 1
fi

SELF_DOMAIN_ID=${NAMESPACE}
if [[ $SELF_DOMAIN_ID == "" ]] ; then
  echo "can not get self domain id, please check NAMESPACE environment"
  exit 1
fi

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
HOST=${HOST_ENDPOINT}
PORT=80

if [[ "${HOST_ENDPOINT}" == *":"* ]]; then
  HOST=${HOST_ENDPOINT%%:*}
  PORT=${HOST_ENDPOINT##*:}
fi

pushd $ROOT/etc/certs || exit
TLS_CA_FILE=${HOST_DOMAIN_ID}.host.ca.crt
TLS_CA=$(base64 ${TLS_CA_FILE} | tr -d "\n")
SRC_CERT_FILE=domain-2-${HOST_DOMAIN_ID}.crt
SRC_CERT=$(base64 ${SRC_CERT_FILE} | tr -d "\n")
SRC_KEY_FILE=domain.key
SRC_KEY=$(base64 ${SRC_KEY_FILE} | tr -d "\n")

popd || exit

DOMAIN_ROUTE_TEMPLATE=$(sed "s/{{.SELF_DOMAIN}}/${SELF_DOMAIN_ID}/g;
  s/{{.SRC_DOMAIN}}/${SELF_DOMAIN_ID}/g;
  s/{{.DEST_DOMAIN}}/${HOST_DOMAIN_ID}/g;
  s/{{.INTERCONN_PROTOCOL}}/${INTERCONN_PROTOCOL}/g;
  s/{{.HOST}}/${HOST}/g;
  s/{{.PORT}}/${PORT}/g;
  s/{{.TLS_CA}}/${TLS_CA}/g;
  s/{{.SRC_CERT}}/${SRC_CERT}/g;
  s/{{.SRC_KEY}}/${SRC_KEY}/g" \
  < "${ROOT}/scripts/templates/domain_route.mtls.yaml")
echo "${DOMAIN_ROUTE_TEMPLATE}" | kubectl apply -f -

if [[ ${INTERCONN_PROTOCOL} == "kuscia" ]]; then
   INTEROP_CONFIG_TEMPLATE=$(sed "s/{{.MEMBER_DOMAIN_ID}}/${SELF_DOMAIN_ID}/g;
     s/{{.HOST_DOMAIN_ID}}/${HOST_DOMAIN_ID}/g" \
     < "${ROOT}/scripts/templates/interop_config.yaml")
   echo "${INTEROP_CONFIG_TEMPLATE}" | kubectl apply -f -
fi

