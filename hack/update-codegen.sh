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

set -o errexit
set -o nounset
set -o pipefail

KUSCIA_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
echo "${KUSCIA_ROOT}"
TMP_DIR=${KUSCIA_ROOT}/tmp-crd-code

mkdir "${TMP_DIR}"

"${KUSCIA_ROOT}"/hack/generate-groups.sh all \
  github.com/secretflow/kuscia/pkg/crd github.com/secretflow/kuscia/pkg/crd/apis \
  "kuscia:v1alpha1" \
  --output-base "${TMP_DIR}" \
  --go-header-file "${KUSCIA_ROOT}/hack/boilerplate.go.txt"

cp -r "${TMP_DIR}"/github.com/secretflow/kuscia/pkg/crd/* "${KUSCIA_ROOT}"/pkg/crd
rm -r "${TMP_DIR}"
