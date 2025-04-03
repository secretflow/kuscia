#!/bin/bash
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

set -e -x

cd "$(dirname "$0")"/..

. ./scripts/version.sh

mkdir -p build/data
mkdir -p build/static
mkdir -p build/out

GO=${GO-go}

echo Running: go generate
GOOS=linux CC=gcc CXX=g++ ${GO} generate

PKG="github.com/k3s-io/k3s"
PKG_CONTAINERD="github.com/containerd/containerd"
PKG_CRICTL="github.com/kubernetes-sigs/cri-tools/pkg"
PKG_K8S_BASE="k8s.io/component-base"
PKG_K8S_CLIENT="k8s.io/client-go/pkg"
PKG_CNI_PLUGINS="github.com/containernetworking/plugins"
PKG_KUBE_ROUTER="github.com/cloudnativelabs/kube-router/v2"
PKG_CRI_DOCKERD="github.com/Mirantis/cri-dockerd"
PKG_ETCD="go.etcd.io/etcd"

buildDate=$(date -u '+%Y-%m-%dT%H:%M:%SZ')

VERSIONFLAGS="
    -X ${PKG}/pkg/version.Version=${VERSION}
    -X ${PKG}/pkg/version.GitCommit=${COMMIT:0:8}

    -X ${PKG_K8S_CLIENT}/version.gitVersion=${VERSION}
    -X ${PKG_K8S_CLIENT}/version.gitCommit=${COMMIT}
    -X ${PKG_K8S_CLIENT}/version.gitTreeState=${TREE_STATE}
    -X ${PKG_K8S_CLIENT}/version.buildDate=${buildDate}

    -X ${PKG_K8S_BASE}/version.gitVersion=${VERSION}
    -X ${PKG_K8S_BASE}/version.gitCommit=${COMMIT}
    -X ${PKG_K8S_BASE}/version.gitTreeState=${TREE_STATE}
    -X ${PKG_K8S_BASE}/version.buildDate=${buildDate}

    -X ${PKG_CRICTL}/version.Version=${VERSION_CRICTL}

    -X ${PKG_CONTAINERD}/version.Version=${VERSION_CONTAINERD}
    -X ${PKG_CONTAINERD}/version.Package=${PKG_CONTAINERD_K3S}

    -X ${PKG_CNI_PLUGINS}/pkg/utils/buildversion.BuildVersion=${VERSION_CNIPLUGINS}
    -X ${PKG_CNI_PLUGINS}/plugins/meta/flannel.Program=flannel
    -X ${PKG_CNI_PLUGINS}/plugins/meta/flannel.Version=${VERSION_FLANNEL}
    -X ${PKG_CNI_PLUGINS}/plugins/meta/flannel.Commit=HEAD
    -X ${PKG_CNI_PLUGINS}/plugins/meta/flannel.buildDate=${buildDate}

    -X ${PKG_KUBE_ROUTER}/pkg/version.Version=${VERSION_KUBE_ROUTER}
    -X ${PKG_KUBE_ROUTER}/pkg/version.BuildDate=${buildDate}

    -X ${PKG_CRI_DOCKERD}/cmd/version.Version=${VERSION_CRI_DOCKERD}
    -X ${PKG_CRI_DOCKERD}/cmd/version.GitCommit=HEAD
    -X ${PKG_CRI_DOCKERD}/cmd/version.BuildTime=${buildDate}

    -X ${PKG_ETCD}/api/version.GitSHA=HEAD
"

if [ -n "${DEBUG}" ]; then
  GCFLAGS="-N -l"
else
  LDFLAGS="-w -s"
fi

STATIC="
    -extldflags '-static -lm -ldl -lz -lpthread'
"
TAGS="static_build libsqlite3 ctrd apparmor seccomp netcgo osusergo providerless urfave_cli_no_docs"

mkdir -p bin

if [ "${ARCH}" = armv7l ] || [ "${ARCH}" = arm ]; then
    export GOARCH="arm"
    export GOARM="7"
    # Context: https://github.com/golang/go/issues/58425#issuecomment-1426415912
    export GOEXPERIMENT=nounified
fi

if [ "${ARCH}" = s390x ]; then
    export GOARCH="s390x"
fi

echo Building k3s
CGO_ENABLED=1 "${GO}" build "$LDFLAGS" -tags "$TAGS" -buildvcs=false -gcflags="all=${GCFLAGS}" -ldflags "$VERSIONFLAGS $LDFLAGS $STATIC" -o bin/k3s ./cmd/server

stat bin/k3s
exit 0
