ARG DEPS_IMAGE="secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia-deps:0.6.1b0"
ARG KUSCIA_ENVOY_IMAGE="secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia-envoy:0.6.1b0"
ARG PROM_NODE_EXPORTER="secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/node-exporter:v1.7.0"
ARG BASE_IMAGE="secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/anolisos:23"

FROM ${DEPS_IMAGE} as deps

FROM ${PROM_NODE_EXPORTER} as node_exporter
FROM ${KUSCIA_ENVOY_IMAGE} as kuscia_envoy

FROM ${BASE_IMAGE}

ENV TZ=Asia/Shanghai
ARG TARGETPLATFORM
ARG TARGETARCH
ARG HOME_DIR="/home/kuscia"
ENV HOME=${HOME_DIR}
RUN yum install -y openssl net-tools which jq logrotate iproute procps-ng libcap && \
    yum clean all && \
    mkdir -p ${HOME_DIR}/bin && \
    mkdir -p /bin/aux && \
    mkdir -p ${HOME_DIR}/scripts && \
    mkdir -p ${HOME_DIR}/etc/conf && \
    mkdir -p ${HOME_DIR}/etc/cni && \
    mkdir -p ${HOME_DIR}/crds && \
    mkdir -p ${HOME_DIR}/var/storage/data && \
    mkdir -p ${HOME_DIR}/var/k3s/server/db && \
    mkdir -p ${ROOT_DIR}/var/images && \
    mkdir -p ${HOME_DIR}/pause

# create non-root user kuscia and group
RUN useradd -ms /bin/bash kuscia && \
    usermod -aG kuscia kuscia && \
    chown -R kuscia:kuscia /home/kuscia && \
    chgrp kuscia /home/kuscia && \
    chmod -R g+rwxs /home/kuscia

COPY --chown=kuscia:kuscia --from=deps /image/home/kuscia/bin ${HOME_DIR}/bin
COPY --chown=kuscia:kuscia --from=deps /image/bin/aux /bin/aux
COPY --chown=kuscia:kuscia --from=node_exporter /bin/node_exporter ${HOME_DIR}/bin

RUN pushd ${HOME_DIR}/bin && \
    ln -s k3s crictl && \
    ln -s k3s ctr && \
    ln -s k3s kubectl && \
    ln -s cni bridge && \
    ln -s cni flannel && \
    ln -s cni host-local && \
    ln -s cni loopback && \
    ln -s cni portmap && \
    popd

COPY --chown=kuscia:kuscia build/${TARGETPLATFORM}/apps/kuscia/kuscia ${HOME_DIR}/bin
COPY --chown=kuscia:kuscia build/pause/pause-${TARGETARCH}.tar ${HOME_DIR}/pause/pause.tar
COPY --chown=kuscia:kuscia crds/v1alpha1 ${HOME_DIR}/crds/v1alpha1
COPY --chown=kuscia:kuscia etc/conf ${HOME_DIR}/etc/conf
COPY --chown=kuscia:kuscia etc/cni ${HOME_DIR}/etc/cni
COPY --chown=kuscia:kuscia testdata ${HOME_DIR}/var/storage/data
COPY --chown=kuscia:kuscia scripts ${HOME_DIR}/scripts
COPY --chown=kuscia:kuscia thirdparty/*/scripts ${HOME_DIR}/scripts
COPY --chown=kuscia:kuscia --from=kuscia_envoy /home/kuscia/bin/envoy ${HOME_DIR}/bin

ENV PATH="${PATH}:${HOME_DIR}/bin:/bin/aux"
WORKDIR ${HOME_DIR}

# non-root user bind low ports (0, 1024] permission
RUN setcap cap_net_bind_service=+ep /home/kuscia/bin/kuscia && \
    setcap cap_net_bind_service=+ep /home/kuscia/bin/envoy


ENTRYPOINT ["tini", "--"]
