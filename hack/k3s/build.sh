#!/bin/bash

set -ex

k3s_version="v1.26.11+k3s2"

function build_k3s() {
  echo "build k3s binary..."
  if [[ $(ls -A "build/k3s") = "" ]];then
    git clone -b $k3s_version --depth 1 https://github.com/k3s-io/k3s.git build/k3s
    KINE_VERSION="github.com/k3s-io/kine@v0.11.3"
    pushd build/k3s
    go mod edit -replace github.com/k3s-io/kine="${KINE_VERSION}"
    pushd
  fi

  cp -rf hack/k3s/build-kuscia-k3s-binary.sh build/k3s/scripts
  chmod +x build/k3s/scripts/build-kuscia-k3s-binary.sh
  cp -rf hack/k3s/Makefile.rebuild_k3s build/k3s/Makefile

  pushd build/k3s
  go mod tidy
  go mod vendor
  make build-kuscia-k3s
  popd
}

build_k3s