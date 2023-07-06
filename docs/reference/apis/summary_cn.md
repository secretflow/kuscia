# 概览

{#api-all}

## K8S API 和 Kuscia API

Kuscia 提供了两套 API 接口，分别是 K8S API 和 Kuscia API。

K8S API 是基于 K8S 接口，以 CRD 的形式提供，请参考 [概念](../concepts/index) 。
你可以通过 kubectl 或 类似于 [client-go](https://github.com/kubernetes/client-go) 这样的 Go 客户端 进行接入。
其他语言的接入方式，可以参考其他 K8S SDK 开源项目。

Kuscia API 是在 K8S API 上的一层封装，提供了更加上层的逻辑，你可以通过 GRPC 或者 HTTPS 进行接入。

当前我们推荐基于 Kuscia API 接入隐语能力；仅当业务方对 K8s 非常熟悉，并且有很多 Kuscia API 无法满足的复杂功能需要实现，才推荐基于 K8s API 接入。

{#kuscia-api}

## Kuscia API 约定

Kuscia API 提供 HTTPS 和 GRPC 两种访问方法。

当你使用 GRPC 时，你可以通过 protoc 生成出对应编程语言的客户端桩代码，你可以在 
[这里](https://github.com/secretflow/kuscia/tree/main/proto/api/v1alpha1/kusciaapi)
找到 Kuscia API 的 protobuf 文件。

当你使用 HTTPS 时，你可以访问对应的 HTTPS 端点，Kuscia API 的接口通过 POST+JSON 或 POST+PROTOBUF 的形式提供 ，并且满足 protobuf
的 [JSON Mapping](https://protobuf.dev/programming-guides/proto3/#json) 。当请求的 `Content-Type` 为 `application/x-protobuf` 时，使用 PROTOBUF 编码，否则使用 JSON 编码。

### 请求和响应约定

请求总是携带会一个 header 字段，类型为 [RequestHeader](#request-header) ， 如 [CreateDomainDataRequest](domaindata_cn.md#create-domain-data-request) ， 该字段可以携带你自定义的一些数据。

响应总是携带一个 status 字段，类型为 [Status](#status) ， 如 [CreateDomainDataResponse](domaindata_cn.md#create-domain-data-response) ， 该字段描述了响应的状态信息。


{#request-header}

#### RequestHeader

RequestHeader 可以携带自定义的信息。

| 字段             | 类型                  | 可选 | 描述      |
|----------------|---------------------|----|---------|
| custom_headers | map<string, string> | 是  | 自定义的键值对 |

{#status}

#### Status

Status 携带请求响应的状态信息。

参考: [GRPC 的 Status 定义](https://github.com/grpc/grpc/blob/master/src/proto/grpc/status/status.proto)

| 字段      | 类型                                                                            | 可选 | 描述     |
|---------|-------------------------------------------------------------------------------|----|--------|
| code    | int32                                                                         |    | 错误码    |
| message | string                                                                        |    | 错误信息   |
| details | [google.protobuf.Any](https://protobuf.dev/programming-guides/proto3/#json)[] |    | 错误详细描述 |

## 如何使用 Kuscia API

### 获取 Kuscia API client 证书和私钥

Kuscia 部署完成之后，会默认生成一个 kuscia API client证书，你可以通过一下命令获取：
```shell
docker cp ${USER}-kuscia-master:/home/kuscia/etc/certs/kusciaapi-client.key .
docker cp ${USER}-kuscia-master:/home/kuscia/etc/certs/kusciaapi-client.crt .
docker cp ${USER}-kuscia-master:/home/kuscia/etc/certs/ca.crt .
docker cp ${USER}-kuscia-master:/home/kuscia/etc/certs/token .
```

### GRPC

为了使用 GRPC 连接上 Kuscia API，你需要：
1. 从 [这里](https://github.com/secretflow/kuscia/tree/main/proto/api/v1alpha1/kusciaapi) 下载 Kuscia 的 protobuf 文件，使用 protoc
生成对应编程语言的客户端桩代码。 关于如何生成客户端桩代码，请参看 [Protobuf官方教程](https://protobuf.dev/getting-started/)。
2. 在初始化 GRPC 客户端时，设置 HTTPS 双向验证的配置。
3. 在初始化 GRPC 客户端时，读取token文件内容，设置 Metadata：Token={token}。
4. 使用 GRPC 客户端发起请求。

GRPC 端点默认在：kuscia-master 的 8083 端口。

### HTTPS

为了使用 HTTPS 连接上 Kuscia API，你需要：
1. 使用编程语言的 HTTP 客户端库连接上 Kuscia API，注意：Kuscia API 使用 双向HTTPS，所以你需要配置你的客户端库的双向 HTTPS 配置。
2. 读取token文件内容，设置 HTTP 请求的 Header，增加：TOKEN={token}。
3. 发送请求。

你也可以使用 HTTP 的客户端工具连接上 Kuscia API，如curl，你需要替换 {} 中的内容：
```shell
curl --cert kusciaapi-client.crt --key kusciaapi-client.key --cacert ca.crt -X POST 'https://{root-kuscia-master}:8082/api/v1/domain/query' --header 'Token: {token}' --header 'Content-Type: application/json' -d '{
  "domain_id": "alice"
}'
```

HTTP 端点默认在：kuscia-master 的 8082 端口。
