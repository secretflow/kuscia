#
# Copyright 2025 Ant Group Co., Ltd.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#   https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

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
