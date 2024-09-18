FROM secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/gcc:7.4.0

RUN git clone https://github.com/proot-me/proot.git /root/proot
COPY hack/proot/patch /tmp
RUN cd /root/proot && \
    git apply /tmp/*.patch && \
    LDFLAGS="${LDFLAGS} -static" make -C src proot