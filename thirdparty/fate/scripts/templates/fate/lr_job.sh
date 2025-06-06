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

JOB_EXAMPLE="FATE LR"

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd -P)
pushd ${ROOT} || exit

INITIATOR=bob

CLUSTER_IP=$(kubectl get pods -l app=fate-deploy-${INITIATOR} -n ${INITIATOR} -o jsonpath='{.items[*].status.podIP}')
APP_IMAGE_TEMPLATE=$(sed "s~{{.CLUSTER_IP}}~${CLUSTER_IP}~g" <"${ROOT}/scripts/templates/fate/app_image.fate.yaml")

echo "$APP_IMAGE_TEMPLATE" | kubectl apply -f -

JOB_NAME=fate-task-$(date +"%Y%m%d%H%M%S")

READER_CONFIG=$(sed "s~{{.JOB_NAME}}~${JOB_NAME}~g" <"${ROOT}/scripts/templates/fate/task_input_config.reader.json"|jq -c . )
TRANSFORM_CONFIG=$(sed "s~{{.JOB_NAME}}~${JOB_NAME}~g" <"${ROOT}/scripts/templates/fate/task_input_config.data_transform.json"|jq -c . )
INTERSECTION_CONFIG=$(sed "s~{{.JOB_NAME}}~${JOB_NAME}~g" <"${ROOT}/scripts/templates/fate/task_input_config.intersection.json"|jq -c . )
LR_CONFIG=$(sed "s~{{.JOB_NAME}}~${JOB_NAME}~g" <"${ROOT}/scripts/templates/fate/task_input_config.hetero_lr.json"|jq -c . )
EVALUATION_CONFIG=$(sed "s~{{.JOB_NAME}}~${JOB_NAME}~g" <"${ROOT}/scripts/templates/fate/task_input_config.evaluation.json"|jq -c . )


APP_IMAGE="fate-image"

echo -e "With JOB_EXAMPLE=${JOB_EXAMPLE}, job via APP_IMAGE=${APP_IMAGE} creating ..."

JOB_TEMPLATE=$(sed "s~{{.JOB_NAME}}~${JOB_NAME}~g;
  s~{{.READER_CONFIG}}~${READER_CONFIG}~g;
  s~{{.INTERSECTION_CONFIG}}~${INTERSECTION_CONFIG}~g;
  s~{{.TRANSFORM_CONFIG}}~${TRANSFORM_CONFIG}~g;
  s~{{.LR_CONFIG}}~${LR_CONFIG}~g;
  s~{{.EVALUATION_CONFIG}}~${EVALUATION_CONFIG}~g;
  s~{{.Initiator}}~${INITIATOR}~g;
  s~{{.APP_IMAGE}}~${APP_IMAGE}~g" \
  <"${ROOT}/scripts/templates/fate/fate_job.yaml")

echo "$JOB_TEMPLATE" | kubectl apply -f -

echo -e "${GREEN}Job '$JOB_NAME' created successfully. You can use the following command to display job status:
  kubectl get kj -n cross-domain${NC}"

popd || exit
