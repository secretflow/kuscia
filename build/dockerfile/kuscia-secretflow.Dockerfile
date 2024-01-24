ARG KUSCIA_IMAGE="secretflow/kuscia:latest"

FROM secretflow/anolis8-python:3.8.15 as python

FROM ${KUSCIA_IMAGE}

COPY --from=python /root/miniconda3/envs/secretflow/bin/ /usr/local/bin/
COPY --from=python /root/miniconda3/envs/secretflow/lib/ /usr/local/lib/

RUN yum install -y protobuf libnl3 && \
    yum clean all && \
    grep -rl '#!/root/miniconda3/envs/secretflow/bin' /usr/local/bin/ | xargs sed -i -e 's/#!\/root\/miniconda3\/envs\/secretflow/#!\/usr\/local/g' && \
    rm /usr/local/bin/openssl

ARG SF_VERSION="1.3.0.dev20231120"
RUN pip install secretflow-lite==${SF_VERSION} --extra-index-url https://mirrors.aliyun.com/pypi/simple/ && rm -rf /root/.cache

RUN kuscia image builtin secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/secretflow-lite-anolis8:${SF_VERSION} --store /home/kuscia/var/images

WORKDIR /home/kuscia

ENTRYPOINT ["tini", "--"]