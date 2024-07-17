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

import os
import docker
import platform
from abc import ABC

KUSCIA_NETWORK = "kuscia-exchange"


class Runtime(ABC):
    """
    Kuscia Deploy Runtime Env.
    Used to hold some configs and init deploy env
    Every kuscia-instance(docker/pod) must bind to runtime
    """

    def __init__(self):
        """
        init runtime
        self.install_path: kuscia install path
        self.docker_client: docker client
        self.kuscia_image: default kuscia image
        self.app_image: default app image
        self.network: docker network
        self.protocol: NOTLS/MTLS/TLS
        self.expose_port: docker container expose port?
        self.skip_exists_instance: if container exists skip it
        self.keep_container_volume: remove container don't remove volume
        self.agent_runtime: runc/runp/runk
        self.instances: all instances bind with this runtime
        """
        self.install_path = "/tmp/var"
        self.docker_client = docker.DockerClient(base_url="unix://var/run/docker.sock")
        self.kuscia_image = None
        self.app_image: str = None
        self.network = KUSCIA_NETWORK
        self.protocol = "NOTLS"
        self.expose_port = None
        self.instances = []
        self.skip_exists_instance = False
        self.keep_container_volume = True
        self.agent_runtime = ""

    def cache_path(self, make_sure_exists: bool = True):
        """
        get cache path
        make sure self.install_path is setted before.
        make_sure_exists: if cache path is not exists, will auto create
        """
        cache = os.path.join(self.install_path, "cache")
        if make_sure_exists:
            os.makedirs(cache, exist_ok=True)
        return cache

    def setup_env(self):
        networks = self.docker_client.networks
        if self.network and not networks.list(names=[self.network]):
            networks.create(self.network, driver="bridge")

    def bind(self, *instances):
        """
        bind instances with this runtime.
        instance can use runtime config and canbe managered by runtime(run/stop/remove)
        """
        for instance in instances:
            if instance and instance not in self.instances:
                instance.runtime = self
                self.instances.append(instance)
        return self

    def is_port_expose(self):
        if self.expose_port == None:
            from . import utils

            return platform.system() == "Darwin" or utils.running_in_container()
        return self.expose_port

    def stop_all_instances(self):
        for c in self.instances:
            c.stop()

    def shutdown(self):
        self.stop_all_containers()

    def get_docker_apiclient(self):
        return docker.APIClient(base_url="unix://var/run/docker.sock")
