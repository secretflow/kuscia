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

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)

IP=$(ip -4 addr show eth0 | grep -oP '(?<=inet\s)\d+(\.\d+){3}')

pushd $ROOT/etc/certs || exit
openssl genrsa -out external_tls.key 2048
openssl req -new -nodes -key external_tls.key -subj "/CN=${IP}" -out external_tls.csr
openssl x509 -req -in external_tls.csr -CA ca.crt -CAkey ca.key -CAcreateserial -days 10000 -out external_tls.crt
popd || exit