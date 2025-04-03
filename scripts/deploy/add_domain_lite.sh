#!/bin/bash
#
# Copyright 2025 Ant Group Co., Ltd.
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

usage="$(basename "$0") DOMAIN_ID [MASTER_DOMAIN_ID]"

DOMAIN_ID=$1

if [[ ${DOMAIN_ID} == "" ]]; then
  echo "missing argument: $usage"
  exit 1
fi

MASTER_DOMAIN_ID=$2

echo "
apiVersion: kuscia.secretflow/v1alpha1
kind: Domain
metadata:
  annotations:
    domain/${DOMAIN_ID}: kuscia.secretflow/domain-type=embedded
  name: ${DOMAIN_ID}
spec:
  cert:
  role:
  master: ${MASTER_DOMAIN_ID}
  authCenter:
    authenticationType: Token
    tokenGenMethod: RSA-GEN
" | kubectl apply -f - > /dev/null

function wait_csr_token() {
  local domain_id=$1
  local retry=0
  local max_retry=60
  while [ $retry -lt $max_retry ]; do
    csrToken=$(kubectl get domain "${domain_id}" -o=jsonpath='{.status.deployTokenStatuses[?(@.state=="unused")].token}')
    if [[ $csrToken != "" ]]; then
      return
    fi
    sleep 1
    retry=$((retry + 1))
  done
}

wait_csr_token "${DOMAIN_ID}"
echo "${csrToken}"
