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

usage="$(basename "$0") NAMESPACE [ALLOW_PRIVILEGED] [AUTH_TYPE](default MTLS)"
NAMESPACE=$1
ALLOW_PRIVILEGED=$2
AUTH_TYPE=$3

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
echo "${ROOT}"

pushd ${ROOT} >/dev/null || exit

EXTERNAL_TLS_CA_FILE=${ROOT}/etc/certs/ca.crt
EXTERNAL_TLS_CERT_FILE=${ROOT}/etc/certs/external_tls.crt
EXTERNAL_TLS_KEY_FILE=${ROOT}/etc/certs/external_tls.key

if [[ ${NAMESPACE} == "" ]]; then
  echo "missing argument: $usage"
  exit 1
fi
if [[ ${ALLOW_PRIVILEGED} != "false" && ${ALLOW_PRIVILEGED} != "true" ]]; then
  ALLOW_PRIVILEGED="false"
fi

if [[ ${AUTH_TYPE} == "None" ]]; then
  EXTERNAL_TLS_CA_FILE=""
elif [[ ${AUTH_TYPE} == "Token" ]]; then
  EXTERNAL_TLS_CA_FILE=""
fi

if [[ ! -f ${EXTERNAL_TLS_CERT_FILE} || ! -f ${EXTERNAL_TLS_KEY_FILE} ]]; then
  EXTERNAL_TLS_CERT_FILE=""
  EXTERNAL_TLS_KEY_FILE=""
fi

cp /etc/resolv.conf etc/resolv.conf
IP=$(ip -4 addr show eth0 | grep -oP '(?<=inet\s)\d+(\.\d+){3}')
echo "nameserver ${IP}" >/etc/resolv.conf

sh scripts/deploy/iptables_pre_detect.sh

cp ${ROOT}/etc/conf/crictl.yaml /etc/crictl.yaml
echo "rootDir: ${ROOT}
domainID: ${NAMESPACE}
caKeyFile: etc/certs/ca.key
caFile: etc/certs/ca.crt
domainKeyFile: etc/certs/domain.key
master:
  endpoint: ${MASTER_ENDPOINT}
  tls:
    certFile: ${ROOT}/etc/certs/domain.crt
    keyFile: ${ROOT}/etc/certs/domain.key
    caFile: ${ROOT}/etc/certs/master.ca.crt
externalTLS:
  certFile: ${EXTERNAL_TLS_CERT_FILE}
  keyFile: ${EXTERNAL_TLS_KEY_FILE}
  caFile: ${EXTERNAL_TLS_CA_FILE}
agent:
  allowPrivileged: true
  plugins:
  - name: env-import
    config:
      usePodLabels: false
      envList:
      - envs:
        - name: system.transport
          value: transport.${NAMESPACE}.svc
        - name: system.storage
          value: file:///home/kuscia/var/storage
        selectors:
        - key: maintainer
          value: secretflow-contact@service.alipay.com
  - name: config-render
" >etc/kuscia.yaml
bin/kuscia autonomy -c etc/kuscia.yaml -d ${NAMESPACE} --log.path var/logs/kuscia.log

echo "
apiVersion: kuscia.secretflow/v1alpha1
kind: Domain
metadata:
  name: ${NAMESPACE}
spec:
  cert:
" | kubectl apply -f -

popd >/dev/null || exit

tail -f /dev/null
