#!/bin/bash
#
# Copyright 2025 Ant Group Co., Ltd.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#   https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

set -e


TEST_IMAGE_NAME="secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/busybox:latest"
TEST_IMAGE_TAG_NAME="busybox:latest"
GET_IMAGE_TAG_NAME="docker.io/library/busybox:latest"
RUNTIME_RUNP="runp"
TEST_LOAD_IMAGE_TAG_NAME="docker.io/secretflow/pause:3.6"
TEST_LOAD_IMAGE_FILE="/home/kuscia/pause/pause.tar"
TEST_IMAGE_MOUNT_PATH="/home/kuscia/busybox/"
TEST_BUILTIN_IMAGE="secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/busybox"

function test_runc_kuscia_images() {
  local get_pull_image_tag
  local get_rm_image_tag
  local get_load_image_tag
  local get_tag_image_tag
  
  echo "start runc kuscia images test"
  # test pull image && list images
  docker exec -i "${LITE_ALICE_CONTAINER}" bash -c "kuscia images pull ${TEST_IMAGE_NAME}"
  get_pull_image_tag=$(get_images "${TEST_IMAGE_NAME}" "${LITE_ALICE_CONTAINER}" )
  assertEquals "kuscia pull images success" "${TEST_IMAGE_NAME}" "${get_pull_image_tag}"
  assertEquals "kuscia list images success" "${TEST_IMAGE_NAME}" "${get_pull_image_tag}"

  # test rm image
  docker exec -i "${LITE_ALICE_CONTAINER}" bash -c "kuscia images rm ${TEST_LOAD_IMAGE_TAG_NAME}"
  get_rm_image_tag=$(get_images "${TEST_LOAD_IMAGE_TAG_NAME}" "${LITE_ALICE_CONTAINER}")
  assertNotEquals "kuscia rm images success" "${TEST_LOAD_IMAGE_TAG_NAME}" "${get_rm_image_tag}"

  # test load image
  docker exec -i "${LITE_ALICE_CONTAINER}" bash -c "kuscia images load --input ${TEST_LOAD_IMAGE_FILE}"
  get_load_image_tag=$(get_images "${TEST_LOAD_IMAGE_TAG_NAME}" "${LITE_ALICE_CONTAINER}")
  assertEquals "kuscia load images success" "${TEST_LOAD_IMAGE_TAG_NAME}" "${get_load_image_tag}"

  # test tag image
  docker exec -i "${LITE_ALICE_CONTAINER}" bash -c "kuscia images tag ${TEST_IMAGE_NAME} ${TEST_IMAGE_TAG_NAME}"
  get_tag_image_tag=$(get_images "${GET_IMAGE_TAG_NAME}" "${LITE_ALICE_CONTAINER}")
  assertEquals "kuscia tag images success" "${GET_IMAGE_TAG_NAME}" "${get_tag_image_tag}"
  echo "end runc kuscia images test"
}

function test_runp_kuscia_images() {
  local get_pull_image_tag
  local uuid
  local get_load_image_tag
  local get_tag_image_tag
  local orign_image_ID
  local builtin_image_ID
  local get_rm_image_tag

  echo "start runp kuscia images test"
  # test pull image && list images
  docker exec -i "${LITE_ALICE_CONTAINER}" bash -c "kuscia images --runtime=runp pull ${TEST_IMAGE_NAME}"
  get_pull_image_tag=$(get_images "${TEST_IMAGE_NAME}" "${LITE_ALICE_CONTAINER}" "${RUNTIME_RUNP}")
  assertEquals "kuscia pull images runp success" "${TEST_IMAGE_NAME}" "${get_pull_image_tag}"
  assertEquals "kuscia list images runp success" "${TEST_IMAGE_NAME}" "${get_pull_image_tag}"

  # test mount image
  uuid=$(LC_ALL=C tr -dc 'A-Za-z0-9' </dev/urandom | head -c 6)
  docker exec -i "${LITE_ALICE_CONTAINER}" bash -c "kuscia images --runtime=runp mount ${uuid} ${TEST_IMAGE_NAME} ${TEST_IMAGE_MOUNT_PATH}"
  assertEquals "kuscia mount images runp success" "0" $?

  # test load image
  docker exec -i "${LITE_ALICE_CONTAINER}" bash -c "kuscia images --runtime=runp load --input ${TEST_LOAD_IMAGE_FILE}"
  get_load_image_tag=$(get_images "${TEST_LOAD_IMAGE_TAG_NAME}" "${LITE_ALICE_CONTAINER}" "${RUNTIME_RUNP}")
  assertEquals "kuscia load images runp success" "${TEST_LOAD_IMAGE_TAG_NAME}" "${get_load_image_tag}"

  #test tag image
  docker exec -i "${LITE_ALICE_CONTAINER}" bash -c "kuscia images --runtime=runp tag ${TEST_IMAGE_NAME} ${TEST_IMAGE_TAG_NAME}"
  get_tag_image_tag=$(get_images "${GET_IMAGE_TAG_NAME}" "${LITE_ALICE_CONTAINER}" "${RUNTIME_RUNP}")
  assertEquals "kuscia tag images runp success" "${GET_IMAGE_TAG_NAME}" "${get_tag_image_tag}"

  #test builtin image
  orign_image_ID=$(get_image_ID "${TEST_BUILTIN_IMAGE}" "${LITE_ALICE_CONTAINER}" "${RUNTIME_RUNP}")
  docker exec -i "${LITE_ALICE_CONTAINER}" bash -c "kuscia images --runtime=runp builtin ${TEST_IMAGE_NAME}"
  builtin_image_ID=$(get_image_ID "${TEST_BUILTIN_IMAGE}" "${LITE_ALICE_CONTAINER}" "${RUNTIME_RUNP}")
  assertNotEquals "kuscia pull builtin image runp success" "${orign_image_ID}" "${builtin_image_ID}"

  #test rm image
  docker exec -i "${LITE_ALICE_CONTAINER}" bash -c "kuscia images --runtime=runp rm ${TEST_LOAD_IMAGE_TAG_NAME}"
  get_rm_image_tag=$(get_images "${TEST_LOAD_IMAGE_TAG_NAME}" "${LITE_ALICE_CONTAINER}" "${RUNTIME_RUNP}")
  assertNotEquals "kuscia rm images runp success" "${TEST_LOAD_IMAGE_TAG_NAME}" "${get_rm_image_tag}"
  echo "end runp kuscia images test"
}
