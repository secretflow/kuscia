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

import kuscia.proto.api.v1alpha1.kusciaapi
import kuscia.proto.api.v1alpha1.kusciaapi.job_pb2 as job_pb2

from . import task


def build_mock_app_1party_task(alice: str, app_image: str, config=""):
    t = task.Task()
    t.app_image = app_image
    t.task_input_config = config
    t.parties.append(job_pb2.Party(domain_id=alice))
    return t


def build_mock_app_hello_job(
    alice: str, app_image: str = "secretflow/kuscia-python-image:latest"
):
    config = ""
    t = build_mock_app_1party_task(alice, app_image=app_image, config=config)
    return task.build_job_request(alice, [t])
