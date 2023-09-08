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

usage="$(basename "$0") DOMAIN_ID [HOST] [ROLE] [INTERCONN_PROTOCOL]"

DOMAIN_ID=$1
if [[ ${DOMAIN_ID} == "" ]]; then
  echo "missing argument: $usage"
  exit 1
fi

HOST=$2
if [[ ${HOST} == "" ]]; then
  HOST=$(hostname)
fi

ROLE=
if [[ $3 == p2p ]] ; then
  ROLE=partner
fi

INTERCONN_PROTOCOL=$4
[ "${INTERCONN_PROTOCOL}" != "" ] || INTERCONN_PROTOCOL="kuscia"

SELF_DOMAIN_ID=${NAMESPACE}
if [[ $SELF_DOMAIN_ID == "" ]] ; then
  echo "can not get self domain id, please check NAMESPACE environment"
  exit 1
fi 

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)

pushd $ROOT/etc/certs >/dev/null || exit
DOMAIN_CERT_FILE=${DOMAIN_ID}.domain.crt
openssl x509 -req -in ${DOMAIN_ID}.domain.csr -CA ca.crt -CAkey ca.key -CAcreateserial -days 10000 -out ${DOMAIN_CERT_FILE}
CERT=$(base64 ${DOMAIN_CERT_FILE} | tr -d "\n")
popd >/dev/null || exit

DOMAIN_TEMPLATE=$(sed "s/{{.DOMAIN_ID}}/${DOMAIN_ID}/g;
  s/{{.CERT}}/${CERT}/g;
  s/{{.ROLE}}/${ROLE}/g" \
  < "${ROOT}/scripts/templates/domain.yaml")
if [ "$3" == "p2p" ]; then
  APPEND_LINE=$(printf "\n%*sinterConnProtocols: [ '${INTERCONN_PROTOCOL}' ]" "2")
  DOMAIN_TEMPLATE="${DOMAIN_TEMPLATE}${APPEND_LINE}"
fi
echo "${DOMAIN_TEMPLATE}" | kubectl apply -f -

kubectl annotate domain/${DOMAIN_ID} "kuscia.secretflow/domain-type=embedded"

TOKEN=$(${ROOT}/scripts/deploy/create_token.sh ${DOMAIN_ID} | tail -n 1)

DOMAIN_ROUTE_TEMPLATE=$(sed "s/{{.SELF_DOMAIN}}/${SELF_DOMAIN_ID}/g;
  s/{{.SRC_DOMAIN}}/${DOMAIN_ID}/g;
  s/{{.DEST_DOMAIN}}/${SELF_DOMAIN_ID}/g;
  s/{{.INTERCONN_PROTOCOL}}/${INTERCONN_PROTOCOL}/g;
  s/{{.HOST}}/${HOST}/g;
  s/{{.PORT}}/1080/g;
  s/{{.TLS_CA}}//g;
  s/{{.SRC_CERT}}//g;
  s/{{.SRC_KEY}}//g;
  s/{{.TOKEN}}/${TOKEN}/g" \
  < "${ROOT}/scripts/templates/domain_route.mtls.yaml")
echo "${DOMAIN_ROUTE_TEMPLATE}" | kubectl apply -f -

