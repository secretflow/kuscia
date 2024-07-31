# Copyright 2024 Ant Group Co., Ltd.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import io
import yaml
import logging
from kubernetes import dynamic, config


def get_k8s_client(master):
    """
    Create K8s client from kuscia.kueconfig file
    """
    kc = master.get_file_content("/home/kuscia/etc/kuscia.kubeconfig")
    m = yaml.load(io.StringIO(kc.decode("utf-8")), Loader=yaml.FullLoader)

    m["clusters"][0]["cluster"]["server"] = master.get_k8s_uri()

    api_client = config.new_client_from_config_dict(m)
    client = dynamic.DynamicClient(api_client)
    return client
