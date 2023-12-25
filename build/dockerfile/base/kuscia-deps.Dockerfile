ARG ARCH=amd64
ARG K3S_VER=v1.26.6
ARG K3S_IMAGE=rancher/k3s:${K3S_VER}-k3s1-${ARCH}
FROM ${K3S_IMAGE} as k3s-image

FROM openanolis/anolisos:8.8

RUN yum install -y git glibc-static wget gcc make && \
    yum clean all

RUN mkdir -p /image/home/kuscia/bin && \
    mkdir -p /image/bin/aux

WORKDIR /tmp

COPY --from=k3s-image /bin/k3s /bin/containerd /bin/containerd-shim-runc-v2 /bin/runc /bin/cni /image/home/kuscia/bin/
COPY --from=k3s-image /bin/aux /image/bin/aux


RUN wget https://github.com/krallin/tini/releases/download/v0.19.0/tini -O /image/home/kuscia/bin/tini && \
    chmod +x /image/home/kuscia/bin/tini

