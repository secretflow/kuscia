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

GREEN='\033[0;32m'
NC='\033[0m'
SUB_HOST_REGEXP="^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$"

USAGE="$(basename "$0") [JOB_EXAMPLE] [JOB_NAME]
JOB_EXAMPLE:
    PSI                 run psi (default)
    NSJAIL_PSI          run psi via nsjail
"
JOB_EXAMPLE=$1
JOB_NAME=$2

if [[ ${JOB_EXAMPLE} == "" ]]; then
  JOB_EXAMPLE="PSI"
fi

if [[ ${JOB_EXAMPLE} != "PSI" && ${JOB_EXAMPLE} != "NSJAIL_PSI" ]]; then
  printf "invalid arguments: JOB_EXAMPLE=%s\n\n%s" "${JOB_EXAMPLE}" "${USAGE}" >&2
  exit 1
fi

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
pushd ${ROOT} || exit

SELF_DOMAIN_ID=${NAMESPACE}
if [[ $SELF_DOMAIN_ID == "" ]] ; then
  echo "can not get self domain id, please check NAMESPACE environment"
  exit 1
fi
INITIATOR=alice
if [[ $SELF_DOMAIN_ID == bob ]]; then
  INITIATOR=bob
fi

if [[ $JOB_NAME == "" ]]; then
  JOB_NAME=secretflow-task-$(date +"%Y%m%d%H%M%S")
fi
if [[ ! $JOB_NAME =~ ${SUB_HOST_REGEXP} ]]; then
  echo "job name should match ${SUB_HOST_REGEXP}"
  exit 1
fi
TASK_INPUT_CONFIG=$(jq -c . <"scripts/templates/task_input_config.2pc_balanced_psi.json")
ESCAPE_TASK_INPUT_CONFIG=$(echo $TASK_INPUT_CONFIG | sed "s~[\]~\\\&~g")

APP_IMAGE=""
case ${JOB_EXAMPLE} in
"PSI")
  APP_IMAGE="secretflow-image"
  ;;
"NSJAIL_PSI")
  APP_IMAGE="secretflow-nsjail-image"
  ;;
esac
echo -e "With JOB_EXAMPLE=${JOB_EXAMPLE}, job via APP_IMAGE=${APP_IMAGE} creating ..."

template=$(sed "s~{{.JOB_NAME}}~${JOB_NAME}~g;s~{{.TASK_INPUT_CONFIG}}~${ESCAPE_TASK_INPUT_CONFIG}~g;s~{{.Initiator}}~${INITIATOR}~g;s~{{.APP_IMAGE}}~${APP_IMAGE}~g" <"scripts/templates/job.2pc_balanced_psi.yaml")

echo "$template" | kubectl apply -f -

echo -e "${GREEN}Job '$JOB_NAME' created successfully. You can use the following command to display job status:
  kubectl get kj${NC}"

popd || exit
