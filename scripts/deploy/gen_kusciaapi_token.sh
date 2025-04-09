#!/bin/bash

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

set -e
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'
DOMAIN_ID=$1
DOMAIN_KEY_DATA=$2

if [ "$#" -ne 2 ]; then
    echo -e "${RED}Please run the script in this format: $0 \${domainID} \${domainKeyData}${NC}"
    exit 1
fi

echo "$DOMAIN_KEY_DATA" | base64 -d > /tmp/domain_key
echo -n "$DOMAIN_ID" | openssl dgst -sha256 -sign /tmp/domain_key -out /tmp/signture_file
base64 < /tmp/signture_file > /tmp/signture_file_base64
TOKEN=$(head -c 32 /tmp/signture_file_base64)
echo -e "${GREEN}$TOKEN${NC}"
rm -rf /tmp/domain_key /tmp/signture_file /tmp/signture_file_base64