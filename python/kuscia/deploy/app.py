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
import yaml
import logging
from abc import ABC
from kubernetes import client, config

import runtime
from instance import KusciaInstanceMode


class AppImage(ABC):
    """
    Not Used Now
    """

    def __init__(self, app_image_tag):
        self.app_image_tag = app_image_tag
        self.app_config = None

    def bind(self, runtime: runtime.Runtime):
        runtime.docker_client.images.get(self.app_image_tag)
        app_config_str = runtime.docker_client.containers.run(
            self.app_image_tag, "cat /kuscia/app.yaml", stdout=True, remove=True
        )
        self.app_config = yaml.load(app_config_str)

    def regist(self, runtime: runtime.Runtime):
        image_tag = self.app_config["spec"]["image"]["name"]
        if self.app_config["spec"]["image"]["tag"]:
            image_tag = image_tag + ":" + self.app_config["spec"]["image"]["tag"]

        appfile = os.path.join(
            runtime.install_path, self.app_config["metadata"]["name"]
        )
        with open(appfile, "w") as file:
            yaml.dump(self.app_config, file)
            logging.info(
                "[%s] app image file wirte to %s",
                self.app_config["metadata"]["name"],
                appfile,
            )
        for c in runtime.instances:
            if (
                c.mode == KusciaInstanceMode.Autonomy
                or c.mode == KusciaInstanceMode.Master
            ):
                self.master_regist_app(c, appfile)
            if (
                c.mode == KusciaInstanceMode.Autonomy
                or c.mode == KusciaInstanceMode.Lite
            ):
                self.lite_import_image(c, image_tag)

    def master_regist_app(self, master, appfile):
        kuscia_config = master.get_file_content("/home/kuscia/etc/kuscia.kubeconfig")
        kc_file = os.path.join(
            self.master.runtime.install_path,
            "kuscia." + self.master.domain_id + ".kubeconfig",
        )
        with open(kc_file, "w") as file:
            file.write(kuscia_config)
        config.kube_config.load_kube_config(config_file=kc_file)
        v1 = client.CoreV1Api()

    def lite_import_image(self, lite, image_tag):
        pass
