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

usage="$(basename "$0") DOMAIN_ID [ROLE] [INTERCONN_PROTOCOL]"

DOMAIN_ID=$1
if [[ ${DOMAIN_ID} == "" ]]; then
  echo "missing argument: $usage"
  exit 1
fi

ROLE=
if [[ $2 == p2p ]]; then
  ROLE=partner
fi

INTERCONN_PROTOCOL=$3
[ "${INTERCONN_PROTOCOL}" != "" ] || INTERCONN_PROTOCOL="kuscia"

SELF_DOMAIN_ID=${NAMESPACE}
if [[ $SELF_DOMAIN_ID == "" ]]; then
  echo "can not get self domain id, please check NAMESPACE environment"
  exit 1
fi

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)

pushd $ROOT/etc/certs >/dev/null || exit
DOMAIN_CERT_FILE=${DOMAIN_ID}.domain.crt
openssl x509 -req -in ${DOMAIN_ID}.domain.csr -CA ca.crt -CAkey ca.key -CAcreateserial -days 10000 -out ${DOMAIN_CERT_FILE}
CERT=$(base64 ${DOMAIN_CERT_FILE} | tr -d "\n")
popd >/dev/null || exit


DOMAIN_TEMPLATE="
apiVersion: kuscia.secretflow/v1alpha1
kind: Domain
metadata:
  annotations:
    domain/${DOMAIN_ID}: kuscia.secretflow/domain-type=embedded
  name: ${DOMAIN_ID}
spec:
  cert: ${CERT}
  role: ${ROLE}
  authCenter:
    authenticationType: Token
    tokenGenMethod: RSA-GEN
"
if [ "$2" == "p2p" ]; then
  APPEND_LINE=$(printf "\n%*sinterConnProtocols: [ '${INTERCONN_PROTOCOL}' ]" "2")
  DOMAIN_TEMPLATE="${DOMAIN_TEMPLATE}${APPEND_LINE}"
fi
echo "${DOMAIN_TEMPLATE}" | kubectl apply -f -
