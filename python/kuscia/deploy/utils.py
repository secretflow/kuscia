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
import rsa
import string
import random
import base64
import socket
import hashlib


def create_domain_key():
    """
    Create Domain Public/Private Key
    """
    (pubkey, privkey) = rsa.newkeys(2048)
    res = base64.b64encode(privkey.save_pkcs1())
    return res


def is_port_in_use(port):
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        return s.connect_ex(("localhost", port)) == 0


def get_random_port(minPort: int, maxPort: int, hint: str) -> int:
    """
    Select a random port to listen (no one is listening at now)
    Port range: [minPort, maxPort)
    Hint: Used to ensure that the first checked port is the same every time.
    """
    start = abs(hash(hint))
    for i in range(minPort, maxPort):
        port = (start + i) % (maxPort - minPort) + minPort
        if not is_port_in_use(port):
            return port
    raise Exception("not found random port")


def get_random_filename(hint: str = None) -> str:
    """
    Create a random filename
    """
    if hint == None:
        length = random.randint(20, 60)
        characters = string.ascii_letters + string.digits
        hint = "".join(random.choice(characters) for i in range(length))
    name = hashlib.md5(hint.encode("utf-8")).hexdigest()
    return name


def parse_image_tag(image_tag: str):
    """
    Parse image
    'docker.io/secretflow/secretflow:v1' --> ('docker.io/secretflow/secretflow', 'secretflow', 'v1')
    'secretflow' --> ('secretflow', 'secretflow', 'latest')
    """
    tmp = image_tag.split(":")
    if len(tmp) == 2:
        rep = tmp[0]
        ver = tmp[1]
    elif len(tmp) == 1:
        rep = tmp[0]
        ver = "latest"
    else:
        raise Exception("invalidate image tag: " + image_tag)
    if rep.rfind("/") != -1:
        name = rep[rep.rfind("/") + 1 :]
    else:
        name = rep
    return (rep, name, ver)


def running_in_container() -> bool:
    """
    Is running in a container
    """
    return os.path.exists("/.dockerenv")
