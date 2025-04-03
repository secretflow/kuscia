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

DOMAIN_ID=$1
DOMAIN_KEY_DATA=$2

if [[ ${DOMAIN_ID} == "" ]]; then
  echo "missing argument: DOMAIN_ID"
  exit 1
fi

if [[ ${DOMAIN_KEY_DATA} == "" ]]; then
  echo "missing argument: DOMAIN_KEY_DATA"
  exit 1
fi

CERT_NAME=${DOMAIN_ID}.domain

echo "${DOMAIN_KEY_DATA}" | base64 -d > "${CERT_NAME}".key

openssl req -sha256 -new -key "${CERT_NAME}".key -out "${CERT_NAME}".csr -subj "/CN=${DOMAIN_ID}"
openssl x509 -req -sha256 -days 36500 -in "${CERT_NAME}".csr -signkey "${CERT_NAME}".key -out "${CERT_NAME}".crt

cat "${CERT_NAME}".crt