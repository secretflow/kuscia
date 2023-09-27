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

usage="$(basename "$0") NAMESPACE"
NAMESPACE=$1
if [[ ${NAMESPACE} == "" ]]; then
  echo "missing argument: $usage"
  exit 1
fi

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
echo "${ROOT}"

pushd ${ROOT} >/dev/null || exit

bin/kuscia master -c etc/kuscia.yaml -d ${NAMESPACE} --log.path var/logs/kuscia.log

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
