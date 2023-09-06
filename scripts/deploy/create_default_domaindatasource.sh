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

RESULT=$(curl -k -s -H "Content-Type: application/json" \
  http://localhost:8070/api/v1/datamesh/domaindatasource/create \
  -d '{
             "datasource_id": "default-data-source",
             "type": "localfs",
             "info": {
               "localfs": {
                  "path": "/home/kuscia/var/storage/data"
               }
             }
           }')

STATUS_CODE=$(echo ${RESULT} | jq '.status.code')
if [[ "$STATUS_CODE" != "0" && "$STATUS_CODE" != "1112" ]]; then
  ERR_MEG=$(echo ${RESULT} | jq '.status.message')
  echo "create datasource error: ${ERR_MEG}"
  exit 1
fi
