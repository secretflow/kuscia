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
import logging

import pyarrow as pa
import pyarrow.csv as csv
import pyarrow.flight as flight
from google.protobuf.any_pb2 import Any
from kuscia.proto.api.v1alpha1.common_pb2 import FileFormat
from kuscia.proto.api.v1alpha1.datamesh.flightdm_pb2 import (
    CommandDomainDataQuery,
    CommandDomainDataUpdate,
    ContentType,
    CSVWriteOptions,
    FileWriteOptions,
)

from . import config

def get_file_from_dp(
    dm_flight_client,
    domain_data_id: str,
    output_file_path: str,
    file_format: FileFormat,
):
    if file_format == FileFormat.CSV:
        domain_data_query = CommandDomainDataQuery(
            domaindata_id=domain_data_id,
            content_type=ContentType.Table,
        )
    elif file_format == FileFormat.BINARY:
        domain_data_query = CommandDomainDataQuery(
            domaindata_id=domain_data_id,
            content_type=ContentType.RAW,
        )
    else:
        raise AttributeError(f"unknown file_format {file_format}")

    any = Any()
    any.Pack(domain_data_query)

    descriptor = flight.FlightDescriptor.for_command(any.SerializeToString())

    flight_info = dm_flight_client.get_flight_info(
        descriptor=descriptor, options=config.DEFAULT_FLIGHT_CALL_OPTIONS
    )

    location = flight_info.endpoints[0].locations[0]
    ticket = flight_info.endpoints[0].ticket
    dp_uri = location.uri.decode('utf-8')

    if dp_uri.startswith("kuscia://"):
        dp_flight_client = dm_flight_client
    else:
        dp_flight_client = flight.connect(dp_uri, )
    flight_reader = dp_flight_client.do_get(ticket=ticket).to_reader()
    if file_format == FileFormat.CSV:
        # NOTE(junfeng): use pandas to write csv since pyarrow will add quotes in headers.
        logging.info(f"write csv file=%s", output_file_path)
        os.remove(output_file_path)
        for batch in flight_reader:
            batch_pd = batch.to_pandas()
            batch_pd.to_csv(
                output_file_path,
                index=False,
                mode='a',
                header=not os.path.exists(output_file_path),
            )
    elif file_format == FileFormat.BINARY:
        with open(output_file_path, "wb") as f:
            for batch in flight_reader:
                assert batch.num_columns == 1
                array = batch.column(0)
                assert array.type == pa.binary()
                for r in array:
                    f.write(r.as_py())
    else:
        raise AttributeError(f"unknown file_format {file_format}")


def put_file_to_dp(
    dm_flight_client,
    domaindata_id: str,
    file_local_path: str,
    file_format: FileFormat,
):
    if file_format == FileFormat.CSV:
        command_domain_data_update = CommandDomainDataUpdate(
            domaindata_id=domaindata_id,
            file_write_options=FileWriteOptions(
                csv_options=CSVWriteOptions(field_delimiter=",")
            ),
        )
        reader = csv.open_csv(file_local_path)
        schema = reader.schema
    elif file_format == FileFormat.BINARY:
        command_domain_data_update = CommandDomainDataUpdate(
            domaindata_id=domaindata_id,
            content_type=ContentType.RAW,
        )
        bin_col_name = "binary_data"

        def _bin_reader():
            # 1MB
            read_chunks = 8
            chunks = []
            with open(file_local_path, "rb") as f:
                for chunk in iter(lambda: f.read(1024 * 128), b''):
                    chunks.append(chunk)
                    if len(chunks) >= read_chunks:
                        yield pa.record_batch([pa.array(chunks)], names=[bin_col_name])
                        chunks = []
                if len(chunks):
                    yield pa.record_batch([pa.array(chunks)], names=[bin_col_name])

        reader = _bin_reader()
        schema = pa.schema([(bin_col_name, pa.binary())])
    else:
        raise AttributeError(f"unknown file_format {file_format}")

    any = Any()
    any.Pack(command_domain_data_update)

    descriptor = flight.FlightDescriptor.for_command(any.SerializeToString())

    flight_info = dm_flight_client.get_flight_info(
        descriptor=descriptor, options=config.DEFAULT_FLIGHT_CALL_OPTIONS
    )

    location = flight_info.endpoints[0].locations[0]
    ticket = flight_info.endpoints[0].ticket
    dp_uri = location.uri.decode('utf-8')
    if dp_uri.startswith("kuscia://"):
        dp_flight_client = dm_flight_client
    else:
        dp_flight_client = flight.connect(dp_uri, )

    descriptor = flight.FlightDescriptor.for_command(ticket.ticket)
    flight_writer, _ = dp_flight_client.do_put(descriptor=descriptor, schema=schema)

    for batch in reader:
        flight_writer.write(batch)

    reader.close()
    flight_writer.close()