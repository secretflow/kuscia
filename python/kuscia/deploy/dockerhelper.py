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
import hashlib
import logging


def copy_image_to_container(runtime, image_tag, ct):
    """
    Copy an image into a container and import it to kuscia containerd
    flow: image --> docker save --> copy to container --> ctr import
    image temp tar will write to: {cache_path}/{image_id}.tar.gz
    """
    img = runtime.docker_client.images.get(image_tag)
    name = hashlib.md5(img.id.encode()).hexdigest() + ".tar.gz"
    tarpath = os.path.join(runtime.cache_path(), name)
    if not os.path.exists(tarpath):
        logging.info("save image to %s", tarpath)
        with open(tarpath + ".tmp", "wb") as file:
            for chunk in img.save(named=True):
                file.write(chunk)
            file.close()
        os.rename(tarpath + ".tmp", tarpath)

    # copy to container
    ct.put_files(runtime.cache_path(), [name], "/tmp")
    inpath = "/tmp/" + name
    c = runtime.docker_client.containers.get(ct.container_name)
    (code, log) = c.exec_run(
        "ctr -a=/home/kuscia/containerd/run/containerd.sock -n=k8s.io images import "
        + inpath
    )
    logging.info(
        "import image (%s) to container(%s) code=%d", image_tag, ct.container_name, code
    )
    logging.info("log=%s", log)

    c.exec_run("rm -f " + inpath)

    if code != 0:
        raise Exception(
            "import image (%s) to container(%s) failed" % (image_tag, ct.container_name)
        )
