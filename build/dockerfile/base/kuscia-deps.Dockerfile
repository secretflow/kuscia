ARG K3S_VER=v1.26.11-k3s2
ARG K3S_IMAGE=rancher/k3s:${K3S_VER}
FROM ${K3S_IMAGE} as k3s-image

FROM openanolis/anolisos:8.8
ARG TARGETPLATFORM
ARG TARGETARCH
RUN yum install -y git glibc-static wget gcc make && \
    yum clean all

RUN mkdir -p /image/home/kuscia/bin && \
    mkdir -p /image/bin/aux && \
    curl https://proot.gitlab.io/proot/bin/proot -o /image/home/kuscia/bin/proot && chmod u+x /image/home/kuscia/bin/proot

WORKDIR /tmp

COPY --from=k3s-image /bin/k3s /bin/containerd /bin/containerd-shim-runc-v2 /bin/runc /bin/cni /image/home/kuscia/bin/
COPY --from=k3s-image /bin/aux /image/bin/aux

COPY build/${TARGETPLATFORM}/k3s/bin/k3s /image/home/kuscia/bin/

RUN wget "https://github.com/krallin/tini/releases/download/v0.19.0/tini-${TARGETARCH}" -O /image/home/kuscia/bin/tini && \
    chmod +x /image/home/kuscia/bin/tini
