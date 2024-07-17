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
from kuscia.proto.api.v1alpha1.kusciaapi.job_pb2 import CreateJobRequest
from kuscia.proto.api.v1alpha1.kusciaapi.job_pb2_grpc import JobServiceStub


def commit_kuscia_job(channel, meta, job: CreateJobRequest):
    stub = JobServiceStub(channel)
    res = stub.CreateJob(job, metadata=meta)
    return res
