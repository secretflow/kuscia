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

SECRET_PAD_CTR_NAME=$1
DOMAIN_ID=$2

usage="$(basename "$0") SECRET_PAD_CTR_NAME DOMAIN_ID"

if [[ ${SECRET_PAD_CTR_NAME} == "" ]]; then
  echo "missing argument: $usage"
  exit 1
fi

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)

SECRET_PAD_SVC_TEMPLATE=$(sed "s/{{.SECRET_PAD_CTR_NAME}}/${SECRET_PAD_CTR_NAME}/g;
 s/{{.DOMAIN}}/${DOMAIN_ID}/g;" \
  < "${ROOT}/scripts/templates/secretpad_svc.yaml")

echo "${SECRET_PAD_SVC_TEMPLATE}" | kubectl apply -f -