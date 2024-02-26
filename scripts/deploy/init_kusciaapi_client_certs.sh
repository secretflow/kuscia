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

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)

pushd ${ROOT}/var/certs >/dev/null || exit

CLIENT=kusciaapi-client

#create a PKCS#8 key for client(JAVA native supported), default is PKCS#1
openssl genpkey -out ${CLIENT}.key -algorithm RSA -pkeyopt rsa_keygen_bits:2048 >/dev/null 2>&1

#generate the Certificate Signing Request for client
openssl req -new -key ${CLIENT}.key -out ${CLIENT}.csr -subj "/CN=KusciaAPIClient" >/dev/null 2>&1

#sign it with Root CA for client
openssl x509  -req -in ${CLIENT}.csr \
    -CA ca.crt -CAkey ca.key  \
    -days 1000 -sha256 -CAcreateserial \
    -out ${CLIENT}.crt >/dev/null 2>&1

popd >/dev/null || exit
