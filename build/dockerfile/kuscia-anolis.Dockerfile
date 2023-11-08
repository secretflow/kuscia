ARG DEPS_IMAGE="secretflow/kuscia-deps:0.2.0b0"
ARG KUSCIA_ENVOY_IMAGE="secretflow/kuscia-envoy:0.2.0b0"

FROM ${DEPS_IMAGE} as deps

FROM ${KUSCIA_ENVOY_IMAGE} as kuscia_envoy

FROM openanolis/anolisos:8.8

ENV TZ=Asia/Shanghai

ARG ROOT_DIR="/home/kuscia"
RUN yum install -y openssl net-tools which jq logrotate && \
    yum clean all && \
    mkdir -p ${ROOT_DIR}/bin && \
    mkdir -p /bin/aux && \
    mkdir -p ${ROOT_DIR}/scripts && \
    mkdir -p ${ROOT_DIR}/var/storage && \
    mkdir -p ${ROOT_DIR}/pause

COPY --from=deps /image/home/kuscia/bin ${ROOT_DIR}/bin
COPY --from=deps /image/bin/aux /bin/aux

RUN pushd ${ROOT_DIR}/bin && \
    ln -s k3s crictl && \
    ln -s k3s ctr && \
    ln -s k3s kubectl && \
    ln -s cni bridge && \
    ln -s cni flannel && \
    ln -s cni host-local && \
    ln -s cni loopback && \
    ln -s cni portmap && \
    popd

COPY build/apps/kuscia/kuscia ${ROOT_DIR}/bin
COPY build/pause/pause.tar ${ROOT_DIR}/pause
COPY crds/v1alpha1 ${ROOT_DIR}/crds/v1alpha1
COPY etc ${ROOT_DIR}/etc
COPY hack/entrypoint.sh ${ROOT_DIR}/bin
COPY testdata ${ROOT_DIR}/var/storage/data
COPY scripts ${ROOT_DIR}/scripts

COPY thirdparty/*/scripts ${ROOT_DIR}/scripts

COPY --from=kuscia_envoy /home/kuscia/bin/envoy ${ROOT_DIR}/bin

ENV PATH="${PATH}:${ROOT_DIR}/bin:/bin/aux"

WORKDIR ${ROOT_DIR}

ENTRYPOINT ["tini", "--"]
