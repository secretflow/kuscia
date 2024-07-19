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


import yaml
import grpc
from abc import abstractmethod

from .. import k8shelper
from .. import utils
from ..instance import KusciaInstance, KusciaInstanceMode
from .docker_container import KusciaContainer

import kuscia.proto.api.v1alpha1.kusciaapi as kusciaapi
import kuscia.proto.api.v1alpha1.kusciaapi.health_pb2_grpc


class KusciaMaster(KusciaContainer):
    """
    KusciaMaster Instance
    """

    def __init__(self, domain_id):
        super().__init__(KusciaInstanceMode.Master, domain_id)
        # append kuscia-api/k3s port
        # k3s port: used to regist appImage or other kubectl command
        self.container_config["ports"]["8083/tcp"] = "master.kusciaapi"
        self.container_config["ports"]["6443/tcp"] = "master.k3s"

    def get_kusciaapi_uri(self):
        """
        Get KusciaApi Uri
        """
        if "8083/tcp" in self.ports:
            return "127.0.0.1:" + str(self.ports["8083/tcp"])

        return self.get_public_host() + ":8083"

    def get_k8s_uri(self):
        """
        Get K3s Uri
        """
        if "6443/tcp" in self.ports:
            return "https://127.0.0.1:" + str(self.ports["6443/tcp"])

        return self.get_public_host() + ":6443"

    def get_grpc_client(self):
        """
        Create KusciaApi grpc client
        """
        from .. import apihelper

        files = []
        if self.runtime.protocol == "MTLS" or self.runtime.protocol == None:
            files.append(
                self.get_file_content("/home/kuscia/var/certs/kusciaapi-server.crt")
            )
            files.append(
                self.get_file_content("/home/kuscia/var/certs/kusciaapi-server.key")
            )
            files.append(self.get_file_content("/home/kuscia/var/certs/ca.crt"))
            files.append(self.get_file_content("/home/kuscia/var/certs/token"))

        return apihelper.get_grpc_client(self.runtime, self.get_kusciaapi_uri(), files)

    def get_k8s_client(self):
        """
        Create K8s client
        """
        return k8shelper.get_k8s_client(self)

    def regist_app_image(
        self, image_tag: str = "", app_image_yaml_file: str = "", load_image=False
    ):
        """
        Regist AppImage use a kuscia-app image or app_image yaml file
        load_image: auto import image into container
        """
        from .. import appimage

        if image_tag:
            appimage.regist_app_image_from_docker_image(
                self.runtime.docker_client, self.get_k8s_client(), image_tag
            )
        elif app_image_yaml_file:
            with open(app_image_yaml_file) as file:
                m = yaml.load(file, Loader=yaml.FullLoader)
                appimage.regist_app_image(self.get_k8s_client(), m)

        if load_image and image_tag:
            self.import_image(image_tag)

    def is_started(self):
        try:
            (channel, meta) = self.get_grpc_client()
            stub = kusciaapi.health_pb2_grpc.HealthServiceStub(channel)
            req = kusciaapi.health_pb2.HealthRequest()
            res = stub.healthZ(req, metadata=meta)
            return res.status.code == 0
        except Exception as e:
            if e.code() != grpc.StatusCode.UNAVAILABLE:
                raise e
            return None

    def bridge(self, alice: KusciaInstance, bob: KusciaInstance, both=True):
        """
        Create DomainRoute between two domain(alice/bob)
        """
        from .. import apihelper

        apihelper.make_route(self, alice, bob)
        if both:
            apihelper.make_route(self, bob, alice)

        return self
