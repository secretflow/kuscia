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

ARG KUSCIA_IMAGE="secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia:latest"
ARG PYTHON_IMAGE="secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/anolis8-python:3.10.13"

FROM ${PYTHON_IMAGE} as python
FROM ${KUSCIA_IMAGE}

COPY --from=python /root/miniconda3/envs/secretflow/bin/ /usr/local/bin/
COPY --from=python /root/miniconda3/envs/secretflow/lib/ /usr/local/lib/

RUN yum install -y protobuf libnl3 libgomp && \
    yum clean all && \
    grep -rl '#!/root/miniconda3/envs/secretflow/bin' /usr/local/bin/ | xargs sed -i -e 's/#!\/root\/miniconda3\/envs\/secretflow/#!\/usr\/local/g' && \
    rm /usr/local/bin/openssl

ARG SF_VERSION="1.11.0b1"
RUN pip install secretflow-lite==${SF_VERSION} --extra-index-url https://mirrors.aliyun.com/pypi/simple/ && rm -rf /root/.cache && \
    kuscia image --store /home/kuscia/var/images --runtime runp builtin secretflow/secretflow-lite-anolis8:${SF_VERSION} && \
    kuscia image --store /home/kuscia/var/images --runtime runp builtin secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/secretflow-lite-anolis8:${SF_VERSION}

WORKDIR /home/kuscia

ENTRYPOINT ["tini", "--"]