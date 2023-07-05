FROM openanolis/anolisos:8.8

ARG ROOT_DIR="/home/kuscia"

RUN yum install -y wget && \
    yum clean all && \
    mkdir -p ${ROOT_DIR}/bin

RUN wget https://github.com/krallin/tini/releases/download/v0.19.0/tini -O ${ROOT_DIR}/bin/tini && \
    chmod +x ${ROOT_DIR}/bin/tini

COPY build/apps/fate ${ROOT_DIR}/bin

ENV PATH=${PATH}:${ROOT_DIR}/bin

WORKDIR ${ROOT_DIR}

ENTRYPOINT ["tini", "--"]