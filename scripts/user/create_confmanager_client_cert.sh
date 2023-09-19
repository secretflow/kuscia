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

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
pushd "${ROOT}"/etc/certs || exit

common_name=$1
organizational_unit=$2
DAYS=1000

#create a PKCS#1 key for server, default is PKCS#1
openssl genrsa -out "${common_name}".key 2048

#generate the Certificate Signing Request
openssl req -new -key "${common_name}".key -days ${DAYS} -out "${common_name}".csr \
    -subj "/CN=${common_name}/OU=${organizational_unit}"

#sign it with Root CA
openssl x509  -req -in "${common_name}".csr \
    -CA ca.crt -CAkey ca.key  \
    -days ${DAYS} -sha256 -CAcreateserial \
    -out "${common_name}".crt
