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

import uuid
import logging
from typing import List

import kuscia.proto.api.v1alpha1.kusciaapi as kusciaapi
import kuscia.proto.api.v1alpha1.kusciaapi.job_pb2_grpc as job_pb2_grpc


class Task:
    def __init__(self, name: str = ""):
        super().__init__()
        self.__dict__["T"] = kusciaapi.job_pb2.Task()
        self.__dict__["task_dependencies"] = []

    def __getattr__(self, key):
        return getattr(self.T, key)

    def __setattr__(self, key, value):
        logging.info("set %s", key)
        return setattr(self.T, key, value)

    def after(self, task: "Task"):
        self.task_dependencies.append(task)


def build_job_request(initiator: str, tasks: List[Task], job_id: str = ""):
    idx = 1
    # update task names
    for task in tasks:
        if task.alias == "":
            task.alias = "task%d" % idx
            idx = idx + 1
    # update dependencies
    for task in tasks:
        for dt in task.task_dependencies:
            task.dependencies.append(dt.alias)

    if not job_id:
        job_id = "job-" + str(uuid.uuid4())
    req = kusciaapi.job_pb2.CreateJobRequest()
    req.job_id = job_id
    req.initiator = initiator
    req.max_parallelism = 1
    for task in tasks:
        req.tasks.append(task.T)
    return req
