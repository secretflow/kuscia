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

PNAEM=$1

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
echo "${ROOT}"

source ${ROOT}/${PNAEM}/bin/init_env.sh
echo "upload guest data"
flow data upload -c ${ROOT}/etc/flow/data/upload_guest.json

echo "sleep 30"
sleep 30

echo "upload host data"
flow data upload -c ${ROOT}/etc/flow/data/upload_host.json