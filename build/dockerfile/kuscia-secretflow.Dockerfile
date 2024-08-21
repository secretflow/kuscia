ARG KUSCIA_IMAGE="secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia:latest"
ARG PYTHON_IMAGE="secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/anolis8-python:3.10.13"

FROM ${PYTHON_IMAGE} as python
ARG SF_VERSION="1.7.0b0"
RUN cp -rf /root/miniconda3/envs/secretflow/bin/* /usr/local/bin/ && \
    cp -rf /root/miniconda3/envs/secretflow/lib/* /usr/local/lib/
RUN pip install secretflow-lite==${SF_VERSION} --extra-index-url https://mirrors.aliyun.com/pypi/simple/ && rm -rf /root/.cache

FROM ${KUSCIA_IMAGE}

COPY --from=python /root/miniconda3/envs/secretflow/bin/ /usr/local/bin/
COPY --from=python /root/miniconda3/envs/secretflow/lib/ /usr/local/lib/
COPY --from=python /usr/local/lib/python3.10/ /usr/local/lib/python3.10/
COPY --from=python /root/miniconda3 /root/miniconda3

RUN yum install -y protobuf libnl3 libgomp && \
    yum clean all && \
    grep -rl '#!/root/miniconda3/envs/secretflow/bin' /usr/local/bin/ | xargs sed -i -e 's/#!\/root\/miniconda3\/envs\/secretflow/#!\/usr\/local/g' && \
    rm /usr/local/bin/openssl

ARG SF_VERSION="1.7.0b0"
RUN kuscia image --store /home/kuscia/var/images --runtime runp builtin secretflow/secretflow-lite-anolis8:${SF_VERSION} && \
    kuscia image --store /home/kuscia/var/images --runtime runp builtin secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/secretflow-lite-anolis8:${SF_VERSION}

WORKDIR /home/kuscia

ENTRYPOINT ["tini", "--"]