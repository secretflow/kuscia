#!/bin/bash -x
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
TMP_DIR=${KUSCIA_ROOT}/tmp-crds
CRD_VERSIONS=v1alpha1
CRD_OUTPUTS=${KUSCIA_ROOT}/crds/${CRD_VERSIONS}
_crdOptions="crd:generateEmbeddedObjectMeta=true,allowDangerousTypes=true"

function pre_install {
  # install controller-gen tool if not exist
  if [ "$(command -v controller-gen)" == "" ]; then
    echo "Start to install controller-gen tool"
    GO111MODULE=on go install -v sigs.k8s.io/controller-tools/cmd/controller-gen@v0.15.0
  fi
}

function gen_crds {
  # generate crds
  $(command -v controller-gen) paths="${KUSCIA_ROOT}/pkg/crd/..." ${_crdOptions} output:crd:artifacts:config="${TMP_DIR}"
}

function copy_to_destination {
  # rename files, copy files
  mkdir -p "${CRD_OUTPUTS}"
  cp -r "${TMP_DIR}"/*.yaml "${CRD_OUTPUTS}"
}

function cleanup {
  echo "Removing temporary files: ${TMP_DIR}"
  rm -rf "${TMP_DIR}"
}

function main() {
  pre_install
  gen_crds
  copy_to_destination
  cleanup
}

main