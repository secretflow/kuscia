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

import grpc
import time
import logging
import base64
from google.protobuf.json_format import MessageToJson

from . import container

import kuscia.proto.api.v1alpha1.kusciaapi as kusciaapi
import kuscia.proto.api.v1alpha1.kusciaapi.domain_pb2_grpc
import kuscia.proto.api.v1alpha1.kusciaapi.domain_route_pb2_grpc
import kuscia.proto.api.v1alpha1.kusciaapi.job_pb2_grpc


def get_grpc_client(runtime, uri, keys):
    """
    Create grpc client
    runtime: runtime env
    uri: kuscia-api uri
    keys: mtls keys
    """
    if runtime.protocol == "MTLS" or runtime.protocol == None:
        (server_cert, server_key, trusted_ca, token) = keys
        credentials = grpc.ssl_channel_credentials(trusted_ca, server_key, server_cert)
        channel = grpc.secure_channel(uri, credentials)
        metadata = [("token", token)]
        return (channel, metadata)
    else:
        channel = grpc.insecure_channel(uri)
        return (channel, [])


def create_token(master: container.KusciaMaster, lite_domain_id: str):
    """
    center mode: call kuscia-api to create a new domain, and query liteDeployToken
    """
    (channel, meta) = master.get_grpc_client()
    stub = kusciaapi.domain_pb2_grpc.DomainServiceStub(channel)
    req = kusciaapi.domain_pb2.CreateDomainRequest()
    req.domain_id = lite_domain_id
    req.auth_center.authentication_type = "Token"
    req.auth_center.token_gen_method = "UID-RSA-GEN"
    req.master_domain_id = master.domain_id
    res = stub.CreateDomain(req, metadata=meta)
    if res.status.code != 0:
        raise Exception("create domain failed" + str(res))
    logging.info("create domain(%s) success", lite_domain_id)
    for i in range(50):
        req = kusciaapi.domain_pb2.QueryDomainRequest()
        req.domain_id = lite_domain_id
        res = stub.QueryDomain(req, metadata=meta)
        if res.status.code == 0 and res.data.deploy_token_statuses:
            return res.data.deploy_token_statuses[0].token
        time.sleep(1)
    raise Exception("create domain token is not ok after 60s")


def make_route(
    master: container.KusciaMaster,
    src: container.KusciaContainer,
    dest: container.KusciaContainer,
):
    """
    center mode: create domain-route (src --> dest), and wait status ok
    """
    (channel, meta) = master.get_grpc_client()
    stub = kusciaapi.domain_route_pb2_grpc.DomainRouteServiceStub(channel)
    req = kusciaapi.domain_route_pb2.CreateDomainRouteRequest()
    req.source = src.domain_id
    req.destination = dest.domain_id
    req.authentication_type = "Token"
    if src.runtime.protocol == "MTLS":
        req.authentication_type = "MTLS"
        # TODO: config req.mtls_config
        req.mtls_config = None

    port = kusciaapi.domain_route_pb2.EndpointPort()
    port.port = 1080
    port.protocol = "HTTP"
    port.isTLS = master.runtime.protocol != "NOTLS"
    req.endpoint.ports.append(port)
    req.endpoint.host = dest.get_public_host()
    res = stub.CreateDomainRoute(req, metadata=meta)
    if res.status.code != 0:
        logging.info("create domain route failed err=%s", res.status.message)
        raise Exception("create domain route failed" + MessageToJson(res))

    for i in range(60):
        req = kusciaapi.domain_route_pb2.QueryDomainRouteRequest()
        req.source = src.domain_id
        req.destination = dest.domain_id
        res = stub.QueryDomainRoute(req, metadata=meta)
        if res.status.code != 0:
            logging.info("query domain route failed err=%s", res.status.message)
            raise Exception("query domain route failed" + MessageToJson(res))
        if res.data.status.status == "Succeeded":
            logging.info(
                "create domainroute from (%s) to (%s) success",
                src.domain_id,
                dest.domain_id,
            )
            return
        time.sleep(1)
    raise Exception("domain-route is not ok after 60s")


def create_p2p_domain(alice: container.KusciaAutonomy, bob: container.KusciaAutonomy):
    """
    p2p mode: create alice domain in bob
    """
    (channel, meta) = bob.get_grpc_client()
    stub = kusciaapi.domain_pb2_grpc.DomainServiceStub(channel)
    req = kusciaapi.domain_pb2.CreateDomainRequest()
    req.domain_id = alice.domain_id
    req.role = "partner"
    req.cert = alice.get_file_content("/home/kuscia/var/certs/domain.crt")
    req.auth_center.authentication_type = "Token"
    req.auth_center.token_gen_method = "RSA-GEN"
    res = stub.CreateDomain(req, metadata=meta)
    if res.status.code != 0:
        raise Exception("create domain failed" + MessageToJson(res))
    for i in range(50):
        req = kusciaapi.domain_pb2.QueryDomainRequest()
        req.domain_id = alice.domain_id
        res = stub.QueryDomain(req, metadata=meta)
        if res.status.code == 0:
            logging.info(
                "create domain(%s) in domain(%s) success",
                alice.domain_id,
                bob.domain_id,
            )
            return
        time.sleep(1)
    raise Exception("create p2p domain failed after 60s")
