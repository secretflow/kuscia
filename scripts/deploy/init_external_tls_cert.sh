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

usage="$(basename "$0") DOMAIN_ID [EXTRA_SUBJECT_ALT_NAME]"

DOMAIN_ID=$1
SUBJECT_ALT_NAME=$2

if [[ ${DOMAIN_ID} == "" ]]; then
  echo "missing argument: $usage"
  exit 1
fi

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)

IP=$(ip -4 addr show eth0 | grep -oP '(?<=inet\s)\d+(\.\d+){3}')

SUBJECT_ALT_NAME="IP:127.0.0.1,IP:${IP},DNS:localhost"

if [[ ${EXTRA_SUBJECT_ALT_NAME} != "" ]]; then
  SUBJECT_ALT_NAME="${SUBJECT_ALT_NAME},${EXTRA_SUBJECT_ALT_NAME}"
fi
echo "subjectAltName=${SUBJECT_ALT_NAME}" > /tmp/external_tls_openssh.conf

#create a PKCS#1 key for tls
pushd $ROOT/etc/certs >/dev/null || exit
openssl genrsa -out external_tls.key 2048

#generate the Certificate Signing Request
openssl req -new -key external_tls.key -out external_tls.csr -subj "/CN=${DOMAIN_ID}_ENVOY_EXTERNAL"

#sign it with Root CA
openssl x509  -req -in external_tls.csr \
    -extfile /tmp/external_tls_openssh.conf \
    -CA ca.crt -CAkey ca.key  \
    -days 10000 -sha256 -CAcreateserial \
    -out external_tls.crt

rm -rf /tmp/external_tls_openssh.conf

popd >/dev/null || exit