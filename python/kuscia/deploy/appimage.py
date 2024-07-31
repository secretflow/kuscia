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
import os
import yaml
import time
import logging
from typing import Dict
from kubernetes.dynamic.exceptions import ResourceNotFoundError, NotFoundError


def regist_app_image(client, app_image):
    """
    regist appImage to kuscia
    client: K8sClient
    app_image: appImage config(dict or yaml file or string)
    """
    if isinstance(app_image, str):
        if os.path.exists(app_image):  # app_image is filename
            with open(app_image) as file:
                app_image = file.read()
        app_image = yaml.load(io.StringIO(app_image), Loader=yaml.FullLoader)

    app_image_api = client.resources.get(
        api_version="kuscia.secretflow/v1alpha1", kind="AppImage"
    )
    try:
        app_image_api.get(name=app_image["metadata"]["name"])
        # delete org appImage
        app_image_api.delete(name=app_image["metadata"]["name"])
    except (ResourceNotFoundError, NotFoundError):
        pass

    app_image_api.create(body=app_image)
    for i in range(50):
        # wait image is registed
        try:
            app_image_api.get(name=app_image["metadata"]["name"])
            break
        except (ResourceNotFoundError, NotFoundError):
            time.sleep(1)
    if i >= 49:
        raise Exception(
            "check create app_image ({}) failed" % (app_image["metadata"]["name"])
        )


def regist_app_image_from_docker_image(docker_client, k8s_client, image_tag):
    """
    Use Kuscia App Image meta file(/kuscia/app_image.yaml) to regist an image
    """
    logging.info("image_tag=%s", image_tag)
    c = docker_client.containers.run(
        image_tag,
        "cat /kuscia/app_image.yaml",
        detach=True,
        remove=True,
        stdout=True,
        stderr=True,
    )

    regist_app_image(k8s_client, c.logs().decode("utf-8"))


def get_app_image_from_template_file(input_yaml: str, replaces: Dict[str, str] = None):

    with open(input_yaml) as file:
        content = file.read()
        if content.find("---") != -1:
            content = content[: content.find("---")]

    if replaces:
        for k, v in replaces.items():
            content = content.replace(k, v)
    return content


###############################################################################################
# The following code is not used now
def get_app_image_template(name: str):
    app = {
        "kind": "AppImage",
        "apiVersion": "kuscia.secretflow/v1alpha1",
        "metadata": {
            "name": name,
        },
        "spec": {
            "configTemplates": [],
            "deployTemplates": [
                {
                    "name": name,
                    "replicas": 1,
                    "spec": {
                        "containers": [
                            {
                                "name": name,
                                "command": [],
                                "args": [],
                                "ports": [],
                                "workingDir": "/home/admin/kuscia",
                                "configVolumeMounts": [],
                            }
                        ],
                        "restartPolicy": "Never",
                    },
                }
            ],
            "image": {
                "id": "",
                "name": "",
                "tag": "",
            },
        },
    }
    return app


def app_image_append_port(app_image, name, port, protocol, scope):
    deployTemplates = app_image["spec"]["deployTemplates"]
    containers = deployTemplates[0].spec.containers
    containers[0].ports.append(
        {
            "name": name,
            "port": port,
            "protocol": protocol,
            "scope": scope,
        }
    )
    return app_image


def app_image_append_config(app_image, files):
    for path, content in files:
        name = os.path.basename(path)
        app_image.spec.configTemplates.append({name: content})
        configVolumeMounts = (
            app_image.spec.deployTemplates[0].spec.containers[0].configVolumeMounts
        )
        configVolumeMounts.append(
            {
                "mountPath": path,
                "subPath": name,
            }
        )
    return app_image


def app_image_set_image(app_image, image_tag):
    m = image_tag.split(":")

    if len(m) == 1:
        app_image["spec"]["image"]["name"] = image_tag[0]
        app_image["spec"]["image"]["tag"] = "latest"
    elif len(m) == 2:
        app_image["spec"]["image"]["name"] = image_tag[0]
        app_image["spec"]["image"]["tag"] = image_tag[1]
    else:
        raise Exception("input image_tag {} is invalidate" % (image_tag))
    return app_image
