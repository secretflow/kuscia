# 概览

{#api-all}

## K8s API 和 Kuscia API

Kuscia 提供了两套 API 接口，分别是 K8s API 和 Kuscia API。

K8s API 是基于 K8s 接口，以 CRD 的形式提供，请参考 [概念](../concepts/index) 。
你可以通过 kubectl 或 类似于 [client-go](https://github.com/kubernetes/client-go) 这样的 Go 客户端 进行接入。
其他语言的接入方式，可以参考[这里](https://kubernetes.io/docs/reference/using-api/client-libraries/)。

Kuscia API 是在 K8s API 上的一层封装，提供了更加上层的逻辑，你可以通过 GRPC 或者 HTTPS 进行接入。

当前我们推荐基于 Kuscia API 接入隐语能力；仅当业务方对 K3s 非常熟悉，并且有很多 Kuscia API 无法满足的复杂功能需要实现，才推荐基于
K8s API 接入。

{#kuscia-api}

## Kuscia API 约定

Kuscia API 提供 HTTPS 和 GRPC 两种访问方法。

当你使用 GRPC 时，你可以通过 protoc 生成出对应编程语言的客户端桩代码，你可以在
[这里](https://github.com/secretflow/kuscia/tree/main/proto/api/v1alpha1/kusciaapi)
找到 Kuscia API 的 protobuf 文件。

当你使用 HTTPS 时，你可以访问对应的 HTTPS 端口，Kuscia API 的接口通过 POST+JSON 或 POST+PROTOBUF 的形式提供 ，并且满足
protobuf
的 [JSON Mapping](https://protobuf.dev/programming-guides/proto3/#json) 。当请求的 `Content-Type`
为 `application/x-protobuf` 时，使用 PROTOBUF 编码，否则使用 JSON 编码。

### 请求和响应约定

请求总是携带会一个 header 字段，类型为 [RequestHeader](#requestheader) ，
如 [CreateDomainDataRequest](domaindata_cn.md#请求createdomaindatarequest) ， 该字段可以携带你自定义的一些数据。

响应总是携带一个 status 字段，类型为 [Status](#status) ，
如 [CreateDomainDataResponse](domaindata_cn.md#响应createdomaindataresponse) ， 该字段描述了响应的状态信息。

{#requestheader}

#### RequestHeader

RequestHeader 可以携带自定义的信息。

| 字段             | 类型                  | 选填 | 描述      |
|----------------|---------------------|----|---------|
| custom_headers | map<string, string> | 可选 | 自定义的键值对 |

{#status}

#### Status

Status 携带请求响应的状态信息。

参考: [GRPC 的 Status 定义](https://github.com/grpc/grpc/blob/master/src/proto/grpc/status/status.proto)

| 字段      | 类型                                                                            | 选填 | 描述     |
|---------|-------------------------------------------------------------------------------|----|--------|
| code    | int32                                                                         | 必填 | 错误码    |
| message | string                                                                        | 必填 | 错误信息   |
| details | [google.protobuf.Any](https://protobuf.dev/programming-guides/proto3/#json)[] | 可选 | 错误详细描述 |

## 如何使用 Kuscia API

### 获取 Kuscia API server 证书和私钥

Kuscia master 部署完成之后，会默认生成一个 kuscia API server 证书，你可以通过以下命令获取（以中心化组网模式为例）：

```shell
docker cp ${USER}-kuscia-master:/home/kuscia/var/certs/kusciaapi-server.key .
docker cp ${USER}-kuscia-master:/home/kuscia/var/certs/kusciaapi-server.crt .
docker cp ${USER}-kuscia-master:/home/kuscia/var/certs/ca.crt .
docker cp ${USER}-kuscia-master:/home/kuscia/var/certs/token .
```

### GRPC

为了使用 GRPC 连接上 Kuscia API，你需要：

1. 从 [这里](https://github.com/secretflow/kuscia/tree/main/proto/api/v1alpha1/kusciaapi) 下载 Kuscia 的 protobuf 文件，使用
   protoc
   生成对应编程语言的客户端桩代码。
   关于如何生成客户端桩代码，请参看 [Protobuf 官方教程](https://protobuf.dev/getting-started/)。
2. 在初始化 GRPC 客户端时，设置 HTTPS 双向验证的配置。
3. 在初始化 GRPC 客户端时，读取 token 文件内容，设置 Metadata：Token={token}。
4. 使用 GRPC 客户端发起请求。

```python
# Python 示例 #

import os
import grpc
from kuscia.proto.api.v1alpha1.kusciaapi.domain_pb2_grpc import (
    DomainServiceStub,
)
from kuscia.proto.api.v1alpha1.kusciaapi.domain_pb2 import (
    QueryDomainRequest,
)


def query_domain():
    server_cert_file = "kusciaapi-server.crt"
    server_key_file = "kusciaapi-server.key"
    trusted_ca_file = "ca.crt"
    token_file = "token"
    address = "root-kuscia-master:8083"
    with open(server_cert_file, 'rb') as server_cert, open(
            server_key_file, 'rb'
    ) as server_key, open(trusted_ca_file, 'rb') as trusted_ca, open(token_file, 'rb') as token:
        credentials = grpc.ssl_channel_credentials(trusted_ca.read(), server_key.read(), server_cert.read())
        channel = grpc.secure_channel(address, credentials)
        domainStub = DomainServiceStub(channel)
        metadata = [('token', token.read())]
        ret = domainStub.QueryDomain(request=QueryDomainRequest(domain_id="alice"), metadata=metadata)
        print(ret)
```

你也可以使用 GRPC 的客户端工具连接上 Kuscia API，如 [grpcurl](https://github.com/fullstorydev/grpcurl/releases)，你需要替换 {} 中的内容：
> 如果 GRPC 的主机端口是 8083 ，则可以执行下面的命令，端口号不是 8083 ，可以先用 `docker inspect --format="{{json .NetworkSettings.Ports}}" ${容器名}` 命令检查下端口
```shell
grpcurl --cert /home/kuscia/var/certs/kusciaapi-server.crt \
        --key /home/kuscia/var/certs/kusciaapi-server.key \
        --cacert /home/kuscia/var/certs/ca.crt \
        -H 'Token: {token}' \
        -d '{"domain_id": "alice"}' \
        ${USER}-kuscia-master:8083 kuscia.proto.api.v1alpha1.kusciaapi.DomainService.QueryDomain
```

GRPC 容器内端口默认在：master 或者 autonomy 节点的 8083。
GRPC 主机上端口：master 或者 autonomy 可以通过 `docker inspect --format="{{json .NetworkSettings.Ports}}" ${容器名}` 获得 8083 端口的主机映射。

### HTTPS

为了使用 HTTPS 连接上 Kuscia API，你需要：

1. 使用编程语言的 HTTP 客户端库连接上 Kuscia API，注意：Kuscia API 使用 双向 HTTPS，所以你需要配置你的客户端库的双向 HTTPS
   配置。
2. 读取 Token 文件内容，设置 HTTP 请求的 Header，增加：TOKEN={token}。
3. 发送请求。

你也可以使用 HTTP 的客户端工具连接上 Kuscia API，如 curl，你需要替换 {} 中的内容：
> 如果 HTTPS 的主机端口是 8082 ，则可以执行下面的命令，端口号不是 8082 ，可以先用 `docker inspect --format="{{json .NetworkSettings.Ports}}" ${容器名}` 命令检查下端口
```shell
curl --cert /home/kuscia/var/certs/kusciaapi-server.crt \
     --key /home/kuscia/var/certs/kusciaapi-server.key \
     --cacert /home/kuscia/var/certs/ca.crt \
     --header 'Token: {token}' --header 'Content-Type: application/json' \
     'https://{{USER}-kuscia-master}:8082/api/v1/domain/query' \
     -d '{"domain_id": "alice"}'
```

HTTPS 容器内端口默认在：master 或者 autonomy 节点的 8082 。
HTTPS 主机上端口：master 或者 autonomy 可以通过 `docker inspect --format="{{json .NetworkSettings.Ports}}" ${容器名}` 获得 8082 端口的主机映射
