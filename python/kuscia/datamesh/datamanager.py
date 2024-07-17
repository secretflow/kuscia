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

import logging
import pyarrow.flight as flight

from kuscia.proto.api.v1alpha1.common_pb2 import FileFormat
from kuscia.proto.api.v1alpha1.datamesh.domaindata_pb2 import (
    CreateDomainDataRequest,
    CreateDomainDataResponse,
    DomainData,
)

from . import config

def create_domain_data_in_dp(
    dm_flight_client,
    domain_data: DomainData,
    file_format: FileFormat,
):
    create_domain_data_request = CreateDomainDataRequest(
        domaindata_id=domain_data.domaindata_id,
        name=domain_data.name,
        type=domain_data.type,
        datasource_id=domain_data.datasource_id,
        relative_uri=domain_data.relative_uri,
        attributes=domain_data.attributes,
        # partition=data.partition,
        columns=domain_data.columns,
        vendor=domain_data.vendor,
        file_format=file_format,
    )

    action = flight.Action(
        "ActionCreateDomainDataRequest",
        create_domain_data_request.SerializeToString(),
    )

    results = dm_flight_client.do_action(
        action=action, options=config.DEFAULT_FLIGHT_CALL_OPTIONS
    )

    for res in results:
        action_response = CreateDomainDataResponse()
        action_response.ParseFromString(res.body.to_pybytes())
        logging.info(f"create domain data status=%d", action_response.status.code)
        if action_response.status.code == 0:
            return action_response.data.domaindata_id

    return None
