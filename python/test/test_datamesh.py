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


import sys
sys.path.append('..') 

import os
import logging
import hashlib
from datetime import datetime
import kuscia.datamesh as datamesh
import kuscia.datamesh.api as dmapi

oss_datasource_id = "oss-data-source"
local_datasource_id = "default-data-source"

local_dir = "/tmp/datamesh/var/client"

def get_file_sha256(filepath):
    with open(filepath, "r") as file:
        content = file.read()
        return hashlib.sha256(content.encode()).hexdigest()

def new_file_name(ext):
    return 'test_' + datetime.now().strftime("%Y%m%d_%H%M%S") + ext

def new_column(name, type):
    col1 = dmapi.DataColumn()
    col1.name = name
    col1.type = type
    return col1

def init_binary_domaindata(datasource_id, filepath):
    new_domain_data = dmapi.DomainData()
    new_domain_data.name = "test"
    new_domain_data.type = "raw"
    new_domain_data.relative_uri = filepath
    new_domain_data.datasource_id = datasource_id
    new_domain_data.vendor = "test"
    return new_domain_data

def init_csv_domaindata(datasource_id, filepath):
    new_domain_data = dmapi.DomainData()
    new_domain_data.name = "test"
    new_domain_data.type = "table"
    new_domain_data.relative_uri = filepath
    new_domain_data.datasource_id = datasource_id
    new_domain_data.vendor = "test"
    new_domain_data.columns.append(new_column('ID', 'string'))
    new_domain_data.columns.append(new_column('credit_rank', 'int32'))
    new_domain_data.columns.append(new_column('income', 'int32'))
    new_domain_data.columns.append(new_column('age', 'int32'))
    return new_domain_data

def test_binary_file_upload(datasource_id, local_dir, filename):
    new_domain_data = init_binary_domaindata(datasource_id, new_file_name(".raw"))

    domaindata_id = datamesh.create_domaindata(new_domain_data, dmapi.FileFormat.BINARY)
    print("created domaindata: ", domaindata_id)

    datamesh.upload_file(domaindata_id, os.path.join(local_dir, filename), dmapi.FileFormat.BINARY)


def test_binary_file_download(datasource_id, local_dir):
    new_domain_data = init_binary_domaindata(datasource_id, "test.raw")

    domaindata_id = datamesh.create_domaindata(new_domain_data, dmapi.FileFormat.BINARY)
    print("created domaindata: ", domaindata_id)

    download_path = os.path.join(local_dir, "download-"+domaindata_id+".raw")
    datamesh.download_to_file(domaindata_id, download_path, dmapi.FileFormat.BINARY)
    print("download to file: ", download_path)
    print("[OUTPUT] file sha256=", get_file_sha256(download_path))

    with open(download_path, "r") as file:
        content = file.read()
        print("FILE-CONTENT: ")
        print(content)

def test_csv_file_download(datasource_id, local_dir):
    new_domain_data = init_csv_domaindata(datasource_id, "alice.csv")

    domaindata_id = datamesh.create_domaindata(new_domain_data, dmapi.FileFormat.CSV)
    print("created domaindata: ", domaindata_id)

    download_path = os.path.join(local_dir, "download-"+domaindata_id+".csv")
    datamesh.download_to_file(domaindata_id, download_path, dmapi.FileFormat.CSV)
    print("download to file: ", download_path)
    print("[OUTPUT] file sha256=", get_file_sha256(download_path))

    with open(download_path, "r") as file:
        content = file.read()
        print("FILE-CONTENT: ")
        print(content)

def test_csv_file_upload(datasource_id, local_dir, filename):
    new_domain_data = init_csv_domaindata(datasource_id, new_file_name(".csv"))

    domaindata_id = datamesh.create_domaindata(new_domain_data, dmapi.FileFormat.CSV)
    print("created domaindata: ", domaindata_id)

    datamesh.upload_file(domaindata_id, os.path.join(local_dir, filename), dmapi.FileFormat.CSV)

if not os.path.exists(local_dir):
    os.makedirs(local_dir)
with open(os.path.join(local_dir, "local-test.csv"), "w") as file:
    file.write("hello world!")

with open(os.path.join(local_dir, "large-test.csv"), "w") as file:
    for i in range(500 * 1024):
        file.write("hello world!\n")

logging.root.setLevel(logging.DEBUG)


datamesh.init("127.0.0.1:8071")


print("[INPUT] init file sha256=", get_file_sha256(os.path.join(local_dir, "local-test.csv")))

#test_binary_file_upload(local_datasource_id, local_dir)

#test_binary_file_download(local_datasource_id, local_dir)


#test_binary_file_upload(oss_datasource_id, local_dir, "local-test.csv")
#test_binary_file_upload(oss_datasource_id, local_dir, "large-test.csv")

#test_binary_file_download(local_datasource_id, local_dir)


#test_csv_file_download(local_datasource_id, local_dir)
#test_csv_file_upload(local_datasource_id, local_dir, 'alice.csv')