ARG ARCH=amd64
ARG K3S_VER=v1.26.5
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

ARG GO_VER=1.19.7
RUN wget https://go.dev/dl/go${GO_VER}.linux-amd64.tar.gz && \
    tar -C /usr/local -zxvf go${GO_VER}.linux-amd64.tar.gz && \
    rm -rf go${GO_VER}.linux-amd64.tar.gz
ENV GO111MODULE=on
ENV GOPATH=/root/gopath
ENV PATH=${PATH}:${GOPATH}/bin:/usr/local/go/bin

ARG FLANNEL_VER=v0.21.5
RUN git clone https://github.com/flannel-io/flannel.git && \
    pushd flannel && \
    git checkout -b stable ${FLANNEL_VER} && \
    CGO_ENABLED=1 make dist/flanneld && \
    cp dist/flanneld /image/home/kuscia/bin && \
    popd && \
    rm -rf flannel && \
    go clean -modcache && \
    rm -rf /usr/local/go/


