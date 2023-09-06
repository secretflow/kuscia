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

DOMAIN_ID=$1
CSR_TOKEN=$2
usage="$(basename "$0") DOMAIN_ID CSR_TOKEN"

if [[ ${DOMAIN_ID} == "" ]]; then
  echo "missing argument: $usage"
  exit 1
fi

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)

mkdir -p ${ROOT}/etc/certs
pushd ${ROOT}/etc/certs >/dev/null || exit

if [ ! -f domain.key ]; then
  openssl genrsa -out domain.key 2048
fi

openssl req -new -nodes -key domain.key -subj "/CN=${DOMAIN_ID}" -addext "1.2.3.4=ASN1:UTF8String:${CSR_TOKEN}" -out domain.csr

if [ ! -f ca.key ]; then
  openssl genrsa -out ca.key 2048
fi

openssl req -x509 -new -nodes -key ca.key -subj "/CN=Kuscia" -days 10000 -out ca.crt

popd >/dev/null || exit
