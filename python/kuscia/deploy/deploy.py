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
import logging
from . import runtime
from . import container


class KusciaSimpleCenterMode(runtime.Runtime):
    def __init__(self, kuscia_image="secretflow/kuscia", app_image=""):
        super().__init__()
        self.kuscia_image = kuscia_image
        self.app_image = app_image
        self.install_path = os.path.expanduser("~/kuscia")
        self.setup_env()
        self.master = None
        self.alice = None
        self.bob = None

    def init_instances(self):
        self.master = container.KusciaMaster("center")
        self.alice = container.KusciaLite("alice")
        self.bob = container.KusciaLite("bob")

        self.bind(self.alice, self.bob, self.master)

        if self.agent_runtime != "":
            self.alice.config["runtime"] = self.agent_runtime
            self.bob.config["runtime"] = self.agent_runtime

    def startup(self):
        if not self.instances:
            self.init_instances()
        self.master.run()

        self.alice.join(self.master).run()
        self.bob.join(self.master).run()

        self.master.bridge(self.alice, self.bob, True)

    def regist_app_image(self):
        if self.app_image:
            self.master.regist_app_image(image_tag=self.app_image, load_image=False)
            self.alice.import_image(self.app_image)
            self.bob.import_image(self.app_image)


class KusciaSimpleP2PMode(runtime.Runtime):
    def __init__(self, kuscia_image="secretflow/kuscia", app_image=""):
        super().__init__()
        self.kuscia_image = kuscia_image
        self.app_image = app_image
        self.install_path = os.path.expanduser("~/kuscia")
        self.setup_env()

    def init_instances(self):
        self.alice = container.KusciaAutonomy("alice")
        self.bob = container.KusciaAutonomy("bob")

        self.bind(self.alice, self.bob)

        if self.agent_runtime != "":
            self.alice.config["runtime"] = self.agent_runtime
            self.bob.config["runtime"] = self.agent_runtime

    def startup(self):
        if not self.instances:
            self.init_instances()
        self.alice.run()
        self.bob.run()

        self.alice.join(self.bob)

    def regist_app_image(self):
        if self.app_image:
            self.alice.regist_app_image(app_image=self.app_image, load_image=True)
            self.alice.regist_app_image(app_image=self.app_image, load_image=True)


if __name__ == "__main__":
    logging.root.setLevel(logging.INFO)

    env = KusciaSimpleCenterMode()
    env.startup()
