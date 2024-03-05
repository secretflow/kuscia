#!/bin/bash
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

set -e
GREEN='\033[0;32m'
NC='\033[0m'
KEY=$(openssl genrsa 2048 2>/dev/null | base64 | tr -d "\n" && echo)
echo -e "${GREEN}Generate domain private key configuration:\n\n$KEY\n${NC}"