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
import pyarrow.flight as flight

from kuscia.proto.api.v1alpha1.datamesh.domaindata_pb2 import (
    DomainData,
)
from kuscia.proto.api.v1alpha1.common_pb2 import (
    FileFormat,
    DataColumn
)


from . import dataproxy
from . import datamanager

DEFAULT_GENERIC_OPTIONS = [("GRPC_ARG_KEEPALIVE_TIME_MS", 60000)]

class datamesh_client_config:
    def __init__(self):
        self.address = None
        self.client_cert = None
        self.client_key = None
        self.trusted_ca = None

_dm_flight_client_config = None

def is_address_has_scheme(address: str):
    return address.startswith("grpc://") or address.startswith("grpcs://") or address.startswith("grpc+tls://")

def new_datamesh_client():
    global _dm_flight_client_config
    if _dm_flight_client_config is None:
        raise "datamesh client config is not inited"

    address = _dm_flight_client_config.address
    if not is_address_has_scheme(address):
        if _dm_flight_client_config.client_cert != None:
            address = "grpc+tls://" + address,
        else:
            address = "grpc://" + address

    if _dm_flight_client_config.client_cert != None:
        dm_flight_client = flight.connect(address,)
    else:
        dm_flight_client = flight.connect(
            address,
            tls_root_certs=_dm_flight_client_config.trusted_ca,
            cert_chain=_dm_flight_client_config.client_cert,
            private_key=_dm_flight_client_config.client_key,
            generic_options=DEFAULT_GENERIC_OPTIONS,
        )

    return dm_flight_client

def init(address: str):
    global _dm_flight_client_config
    if _dm_flight_client_config is not None:
        raise "datamesh had inited, can't init again"

    _dm_flight_client_config = datamesh_client_config()
    _dm_flight_client_config.address = address

    # load key/cert/ca from env
    if os.environ.get("CLIENT_CERT_FILE", '') != '':
        with open(os.environ.get("CLIENT_CERT_FILE", ''), 'rb') as file:
            _dm_flight_client_config.client_cert = file.read()
    if os.environ.get("CLIENT_PRIVATE_KEY_FILE", '') != '':
        with open(os.environ.get("CLIENT_PRIVATE_KEY_FILE", ''), 'rb') as file:
            _dm_flight_client_config.client_key = file.read()
    if os.environ.get("TRUSTED_CA_FILE", '') != '':
        with open(os.environ.get("TRUSTED_CA_FILE", ''), 'rb') as file:
            _dm_flight_client_config.trusted_ca = file.read()



def create_domaindata(domain_data: DomainData, file_format: FileFormat):
    client = new_datamesh_client()
    res = datamanager.create_domain_data_in_dp(client, domain_data, file_format)
    client.close()
    return res


def download_to_file(domain_data_id: str, output_file_path: str, file_format: FileFormat = FileFormat.BINARY):
    client = new_datamesh_client()
    res = dataproxy.get_file_from_dp(client, domain_data_id, output_file_path, file_format)
    client.close()
    return res


def upload_file(domain_data_id: str, input_file_path: str, file_format: FileFormat = FileFormat.BINARY):
    client = new_datamesh_client()
    res = dataproxy.put_file_to_dp(client, domain_data_id, input_file_path, file_format)
    client.close()
    return res