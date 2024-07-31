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
import json
import time
import tarfile
import logging
import getpass
from abc import abstractmethod

from .. import k8shelper
from .. import utils
from ..instance import KusciaInstance, KusciaInstanceMode


class KusciaContainer(KusciaInstance):
    """
    Kuscia Instance using docker
    every instance is a docker container
    self.ports: port mappings. it is a readonly value
    self.container_config: container input configs
        self.container_config['ports']: container need mapping ports to local
        self.container_config['volumes']: container need mapping volumes to local
    """

    def __init__(self, mode, domain_id):
        super().__init__(mode, domain_id)
        self.client = None
        self.container_name = "-".join(
            [getpass.getuser(), domain_id, mode.name.lower()]
        )
        self.ports = {}
        self.container_config = {}
        self.container_config["ports"] = {}
        self.container_config["volumes"] = {}
        logging.info("container[%s] init done", self.container_name)

    def get_public_host(self, use_docker_network=True):
        """
        get public host name
        current is only support container name
        """
        if use_docker_network and self.runtime.network != None:
            return self.container_name
        else:
            return "127.0.0.1"

    def get_gateway_uri(self, use_docker_network=True):
        host = ""
        port = "1080"
        if use_docker_network and self.runtime.network != None:
            host = self.container_name
        else:
            host = "127.0.0.1"
            if "1080/tcp" in self.ports:
                port = str(self.ports["1080/tcp"])
        if self.runtime.protocol != "NOTLS":
            return "https://" + host + ":" + port
        else:
            return "http://" + host + ":" + port

    @abstractmethod
    def is_started(self):
        """
        check container is started.
        return:
            True: started success
            False: started failed
            None: starting
        """
        pass

    def run(self, image=None):
        """
        docker run -d --name={self.container_name} [-v/-p] {image} bin/kuscia start -c etc/conf/kuscia.yaml
        """
        if not image:
            image = self.runtime.kuscia_image

        cfgfile = self.get_config_file()

        volumes = [os.path.abspath(cfgfile) + ":/home/kuscia/etc/conf/kuscia.yaml"]

        c = None
        containers = self.runtime.docker_client.containers
        try:
            c = containers.get(self.container_name)
        except:
            pass

        if c == None or not self.runtime.skip_exists_instance:
            if c != None:
                logging.info("try to remove container: %s", c.name)
                c.remove(force=True, v=(not self.runtime.keep_container_volume))

            if self.runtime.is_port_expose():
                for k, v in self.container_config["ports"].items():
                    self.ports[k] = utils.get_random_port(10000, 20000, v)
                logging.info(
                    "container %s bind ports: %s",
                    self.container_name,
                    json.dumps(self.ports),
                )

            for k, v in self.container_config["volumes"].items():
                if (
                    self.config.get("runtime") != "runc"
                    and k == self.domain_id + "-containerd"
                ):
                    logging.info("runtime is not runc, skip containerd volume")
                    continue
                try:
                    myv = self.runtime.docker_client.volumes.get(k)
                except:
                    # not exists
                    self.runtime.docker_client.volumes.create(name=k)
                    volumes.append(":".join([k, v]))

            logging.info("try to start new container %s", self.container_name)
            containers.run(
                image,
                "bin/kuscia start -c etc/conf/kuscia.yaml",
                name=self.container_name,
                detach=True,
                network=self.runtime.network,
                volumes=volumes,
                ports=self.ports,
                privileged=True,
            )
        else:  # get port from current container
            for k, p in c.ports.items():
                self.ports[k] = int(p[0]["HostPort"])
            logging.info("bind ports: %s", json.dumps(self.ports))
            pass

        for i in range(50):
            started = self.is_started()
            if started == True:
                break
            elif started == False:
                raise Exception("start failed")
            time.sleep(1)
        if i >= 49:
            raise Exception("check started failed")

        logging.info("docker %s started", self.container_name)

        return self

    def stop(self):
        """
        docker stop {self.container_name}
        """
        pass

    def remove(self):
        """
        docker rm -f {self.container_name}
        """
        pass

    def get_file_content(self, path_in_container):
        """
        Get a file content from current docker instance
        equal: docker exec {container_name} cat {path_in_container}
        """
        c = self.runtime.docker_client.containers.get(self.container_name)
        (code, output) = c.exec_run("cat " + path_in_container, stdout=True)
        if code == 0:
            return output
        raise Exception("get_file_content failed")

    def put_files(self, local_prefix, files_in_local, path_in_container):
        """
        Copy files into container
        local_prefix}/{file} ---> path_in_container/{file}
        """
        c = self.runtime.docker_client.containers.get(self.container_name)
        tarpath = os.path.join(
            self.runtime.cache_path(), utils.get_random_filename() + ".tar"
        )
        with tarfile.open(tarpath, "w") as tar:
            if isinstance(files_in_local, str):
                files_in_local = [files_in_local]
            for f in files_in_local:
                tar.add(os.path.join(local_prefix, f), arcname=f)
        with open(tarpath, "rb") as file:
            c.put_archive(path_in_container, file)

    def has_master(self):
        """
        Current Instance has k3s master?
        """
        return (
            self.mode == KusciaInstanceMode.Master
            or self.mode == KusciaInstanceMode.Autonomy
        )

    def import_image(self, app_image_tag: str):
        """
        Copy docker image (in local machine) into current container
        Used for lite/autonomy to import app image
        """
        from .. import dockerhelper

        dockerhelper.copy_image_to_container(self.runtime, app_image_tag, self)

    def exec(self, cmd):
        """
        Exec a command in current container, and return std/stderr output string
        """
        from docker.errors import NotFound

        c = self.runtime.docker_client.containers.get(self.container_name)
        code, output = c.exec_run(cmd=cmd)
        logging.info(
            "Run in container %s, command=%s, ret=%d", self.container_name, cmd, code
        )
        if code != 0:
            logging.info(output)
            raise Exception("run container command failed")

        return output
