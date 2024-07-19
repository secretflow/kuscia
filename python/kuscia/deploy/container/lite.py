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


import logging
from abc import abstractmethod

from ..instance import KusciaInstanceMode

from .docker_container import KusciaContainer


class KusciaLite(KusciaContainer):
    def __init__(self, domain_id):
        super().__init__(KusciaInstanceMode.Lite, domain_id)
        self.config["runtime"] = "runc"
        self.container_config["volumes"][
            self.container_name + "-containerd"
        ] = "/home/kuscia/containerd"
        self.master = None

    def is_started(self):
        return True

    def join(self, master):
        """
        Connect to Master
        """
        if master.mode == KusciaInstanceMode.Master:
            if self.master != None:
                if self.master == master:
                    return self
                raise Exception("can't change master")
            from .. import apihelper

            token = apihelper.create_token(master, self.domain_id)
            logging.info("get deploytoken=%s", token)
            self.master = master
            self.config["liteDeployToken"] = token
            self.config["masterEndpoint"] = self.master.get_gateway_uri(True)
            return self
        raise Exception("not support now")
