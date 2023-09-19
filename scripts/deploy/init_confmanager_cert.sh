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
IP=$(ip -4 addr show eth0 | grep -oP '(?<=inet\s)\d+(\.\d+){3}')
pushd ${ROOT}/etc/certs || exit

DAYS=1000
SERVER=confmanager-server
subjectAltName="IP:127.0.0.1,IP:${IP},DNS:localhost"
echo "subjectAltName=${subjectAltName}" > /tmp/confmanager-openssl.conf

#create a PKCS#1 key for server, default is PKCS#1
openssl genrsa -out ${SERVER}.key 2048

#generate the Certificate Signing Request
openssl req -new -key ${SERVER}.key -days ${DAYS} -out ${SERVER}.csr \
    -subj "/CN=ConfManager"

#sign it with Root CA
openssl x509  -req -in ${SERVER}.csr \
    -extfile /tmp/confmanager-openssl.conf \
    -CA ca.crt -CAkey ca.key  \
    -days ${DAYS} -sha256 -CAcreateserial \
    -out ${SERVER}.crt

rm -rf /tmp/confmanager-openssl.conf