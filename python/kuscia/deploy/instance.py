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
from abc import ABC, abstractmethod
from enum import Enum

from . import utils
from . import runtime


class KusciaInstanceMode(Enum):
    Master = 1
    Lite = 2
    Autonomy = 3


class KusciaInstance(ABC):
    """
    KusciaInstance maybe is docker or pod...
    A Kuscia domain is comprised of at least one KusciaInstance.
    KusciaInstance used to manager kuscia config
    """

    def __init__(self, mode, domain_id):
        """
        self.mode: Master/Lite/Autonomy
        self.private_key: domain's private key, will auto create when call self.get_config_file
        self.config: kuscia configs (content will write to kuscia.yaml)
                     self.config['domainKeyData'] will auto overwrite by self.private_key
        """
        self.mode = mode
        self.domain_id = domain_id
        self.runtime: runtime.Runtime = None
        self.private_key = ""
        self.config = self.create_common_config()

    def create_common_config(self):
        config = {}
        config["mode"] = str(self.mode.name.lower())
        config["domainID"] = self.domain_id
        config["logLevel"] = "INFO"

        return config

    def get_config_file(self) -> str:
        """
        create a config file using self.config
        return config file path: {install_path}/{domain_id}.yaml
        """
        cfgfile = os.path.join(self.runtime.install_path, self.domain_id + ".yaml")
        if self.runtime.protocol:
            self.config["protocol"] = self.runtime.protocol

        self.config["domainKeyData"] = self.get_private_key()

        with open(cfgfile, "w") as file:
            yaml.dump(self.config, file)
            logging.info("[%s] config file wirte to %s", self.domain_id, cfgfile)
        return cfgfile

    def get_private_key(self):
        if not self.private_key:
            self.private_key = utils.create_domain_key()
        return self.private_key

    @abstractmethod
    def run(self, image=None):
        pass

    @abstractmethod
    def stop(self):
        pass

    @abstractmethod
    def remove(self):
        pass
