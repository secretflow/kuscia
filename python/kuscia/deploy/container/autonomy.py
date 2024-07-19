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


from abc import abstractmethod

from ..instance import KusciaInstanceMode
from .master import KusciaMaster
from .docker_container import KusciaContainer


class KusciaAutonomy(KusciaMaster):
    def __init__(self, domain_id):
        KusciaContainer.__init__(self, KusciaInstanceMode.Autonomy, domain_id)
        self.config["runtime"] = "runc"
        self.container_config["ports"]["8083/tcp"] = "master.kusciaapi"
        self.container_config["ports"]["6443/tcp"] = "master.k3s"
        self.container_config["volumes"][
            self.container_name + "-containerd"
        ] = "/home/kuscia/containerd"

    def join(self, other: KusciaContainer):
        """
        Bridge(ExchangeKey/DomainRoute) to another autonomy domain
        """
        if other.mode == KusciaInstanceMode.Autonomy:
            from .. import apihelper

            apihelper.create_p2p_domain(self, other)
            apihelper.create_p2p_domain(other, self)
            apihelper.make_route(self, self, other)
            apihelper.make_route(other, other, self)

        return Exception("not support now")
