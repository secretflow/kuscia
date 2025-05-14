ARG K3S_VER=v1.26.11-k3s2
ARG K3S_IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/k3s:${K3S_VER}
ARG PROOT_IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/proot
FROM ${PROOT_IMAGE} as proot-image
FROM ${K3S_IMAGE} as k3s-image

FROM secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/anolisos:23
ARG TARGETPLATFORM
ARG TARGETARCH
RUN yum install -y git glibc-static wget gcc make && \
    yum clean all

RUN mkdir -p /image/home/kuscia/bin && \
    mkdir -p /image/bin/aux

WORKDIR /tmp

COPY --from=proot-image /root/proot/src/proot /image/home/kuscia/bin/
COPY --from=k3s-image /bin/k3s /bin/containerd /bin/containerd-shim-runc-v2 /bin/runc /bin/cni /image/home/kuscia/bin/
COPY --from=k3s-image /bin/aux /image/bin/aux

COPY build/${TARGETPLATFORM}/k3s/bin/k3s /image/home/kuscia/bin/

RUN wget "https://github.com/krallin/tini/releases/download/v0.19.0/tini-${TARGETARCH}" -O /image/home/kuscia/bin/tini && \
    chmod +x /image/home/kuscia/bin/tini
