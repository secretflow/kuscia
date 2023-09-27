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

USAGE="$(basename "$0") NAMESPACE MASTER_ENDPOINT [ALLOW_PRIVILEGED] [MASTER_CA]"
NAMESPACE=$1
MASTER_ENDPOINT=$2
ALLOW_PRIVILEGED=$3
MASTER_CA=$4
if [[ ${NAMESPACE} == "" || ${MASTER_ENDPOINT} == "" ]]; then
  echo "missing argument: ${USAGE}"
  exit 1
fi
if [[ ${ALLOW_PRIVILEGED} != "false" && ${ALLOW_PRIVILEGED} != "true" ]]; then
  ALLOW_PRIVILEGED="false"
fi

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
echo "${ROOT}"

pushd ${ROOT} >/dev/null || exit

sh scripts/deploy/iptables_pre_detect.sh

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
    caFile: ${MASTER_CA}
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
" >etc/kuscia.yaml
bin/kuscia lite -c etc/kuscia.yaml --log.path var/logs/kuscia.log

popd >/dev/null || exit

tail -f /dev/null
