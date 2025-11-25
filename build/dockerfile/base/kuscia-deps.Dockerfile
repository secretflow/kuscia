#
# Copyright 2025 Ant Group Co., Ltd.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#   https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

ARG K3S_VER=v1.33.5-k3s1
ARG K3S_IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/k3s:${K3S_VER}
ARG PROOT_IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/proot
FROM ${PROOT_IMAGE} as proot-image
FROM ${K3S_IMAGE} as k3s-image

FROM secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/anolisos:23
ARG TARGETARCH
ARG TARGETOS
RUN yum install -y git glibc-static wget gcc make && \
    yum clean all

RUN mkdir -p /image/home/kuscia/bin && \
    mkdir -p /image/home/kuscia/libexec/cni

WORKDIR /tmp

COPY --from=proot-image /root/proot/src/proot /image/home/kuscia/bin/
COPY --from=k3s-image /bin/k3s /bin/runc /bin/cni /image/home/kuscia/bin/
COPY --from=k3s-image /bin/aux /image/home/kuscia/bin/

RUN wget "https://github.com/krallin/tini/releases/download/v0.19.0/tini-${TARGETARCH}" -O /image/home/kuscia/bin/tini && \
    chmod +x /image/home/kuscia/bin/tini

ARG CONTAINERD_VERSION=1.7.28
RUN fname="containerd-${CONTAINERD_VERSION}-${TARGETOS:-linux}-${TARGETARCH:-amd64}.tar.gz" && \
   wget --progress=bar:force:noscroll "https://github.com/containerd/containerd/releases/download/v${CONTAINERD_VERSION}/${fname}" -O "${fname}" && \
   tar xzf "${fname}" -C /tmp && \
   cp -rf /tmp/bin/* /image/home/kuscia/bin && \
   rm -f "${fname}" && rm -rf /tmp/bin

RUN echo "${TARGETARCH:-amd64}" | sed -e s/amd64/x86_64/ -e s/arm64/aarch64/ | tee /target_uname_m
ARG CNI_PLUGINS_VERSION=v1.7.1
RUN fname="cni-plugins-${TARGETOS:-linux}-${TARGETARCH:-amd64}-${CNI_PLUGINS_VERSION}.tgz" && \
   wget --progress=bar:force:noscroll "https://github.com/containernetworking/plugins/releases/download/${CNI_PLUGINS_VERSION}/${fname}" -O "${fname}" && \
   tar xzf "${fname}" -C /image/home/kuscia/libexec/cni && \
   rm -f "${fname}"

ARG ROOTLESSKIT_VERSION=v2.3.5
RUN fname="rootlesskit-$(cat /target_uname_m).tar.gz" && \
  wget --progress=bar:force:noscroll "https://github.com/rootless-containers/rootlesskit/releases/download/${ROOTLESSKIT_VERSION}/${fname}" -O "${fname}" && \
  tar xzf "${fname}" -C /image/home/kuscia/bin && \
  rm -f "${fname}" /image/home/kuscia/bin/rootlesskit-docker-proxy

ARG SLIRP4NETNS_VERSION=v1.3.1
RUN fname="slirp4netns-$(cat /target_uname_m)" && \
  wget --progress=bar:force:noscroll "https://github.com/rootless-containers/slirp4netns/releases/download/${SLIRP4NETNS_VERSION}/${fname}" -O "${fname}" && \
  mv "${fname}" /image/home/kuscia/bin/slirp4netns && \
  chmod +x /image/home/kuscia/bin/slirp4netns