# Copyright 2023 Ant Group Co., Ltd.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


from setuptools import find_packages, setup
from generate_py_protobufs import generate_py_protobufs
import os
import re


def find_version(*filepath):
    with open(os.path.join(".", *filepath)) as fp:
        version_match = re.search(
            r"^__version__ = ['\"]([^'\"]*)['\"]", fp.read(), re.M
        )
        if version_match:
            return version_match.group(1)
        print("Unable to find version string.")
        exit(-1)


LICENSE = """
# Copyright 2023 Ant Group Co., Ltd.
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
"""


def add_init_files(path):
    if not os.path.exists(path):
        print(f"Unable to find path: {path}.")
        return

    init_file = os.path.join(path, "__init__.py")
    if not os.path.exists(init_file):
        with open(init_file, "w") as f:
            f.write(LICENSE)

    for item in os.listdir(path):
        subpath = os.path.join(path, item)
        if os.path.isdir(subpath):
            init_file = os.path.join(subpath, "__init__.py")
            if not os.path.exists(init_file):
                with open(init_file, "w") as f:
                    f.write(LICENSE)
            add_init_files(subpath)


add_init_files("kuscia/")

setup(
    name="kuscia",
    version=find_version("version.py"),
    license="Apache 2.0",
    description="Protocols for kuscia",
    author="secretflow authors",
    author_email="secretflow-contact@service.alipay.com",
    packages=find_packages(),
    cmdclass=dict(generate_py_protobufs=generate_py_protobufs),
    options={
        "generate_py_protobufs": {
            "source_dir": "../../kuscia/proto/",
            "proto_root_path": "../..",
            "output_dir": "./",
            "protoc": "python -m grpc_tools.protoc",
            "grpc": True,
        },
    },
)
