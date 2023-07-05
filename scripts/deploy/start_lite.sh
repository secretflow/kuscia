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

USAGE="$(basename "$0") NAMESPACE CENTRAL_NAMESPACE MASTER_ENDPOINT ALLOW_PRIVILEGED"
NAMESPACE=$1
CENTRAL_NAMESPACE=$2
MASTER_ENDPOINT=$3
ALLOW_PRIVILEGED=$4
if [[ ${NAMESPACE} == "" || ${CENTRAL_NAMESPACE} == "" || ${MASTER_ENDPOINT} == "" ]]; then
  echo "missing argument: ${USAGE}"
  exit 1
fi
if [[ ${ALLOW_PRIVILEGED} != "false"  && ${ALLOW_PRIVILEGED} != "true" ]]; then
  ALLOW_PRIVILEGED="false"
fi

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
echo "${ROOT}"

pushd ${ROOT} || exit

cp /etc/resolv.conf ${ROOT}/etc/resolv.conf
IP=$(ip -4 addr show eth0 | grep -oP '(?<=inet\s)\d+(\.\d+){3}')
echo "nameserver ${IP}" > /etc/resolv.conf

sh scripts/deploy/iptables_pre_detect.sh

sh scripts/deploy/init_external_tls_cert.sh ${NAMESPACE}

cp ${ROOT}/etc/conf/crictl.yaml /etc/crictl.yaml
echo "
rootDir: ${ROOT}
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
agent:
  allowPrivileged: ${ALLOW_PRIVILEGED}
  plugins:
  - name: config-render
  - name: env-import
    config:
      usePodLabels: false
      envList:
      - envs:
        - name: image
          value: secretflow-anolis8
        selectors:
        - key: maintainer
          value: secretflow-contact@service.alipay.com
" > etc/kuscia.yaml
bin/kuscia lite -c etc/kuscia.yaml --log.path var/logs/kuscia.log

popd || exit

tail -f /dev/null

