# 概览

## Data Mesh API 使用场景

Data Mesh API 专为隐私计算应用（如 SecretFlow、TrustedFlow）设计，为其提供获取 DomainData 与 DomainDataSource 信息的能力。隐私计算平台（如 SecretPad）可以使用[KusciaApi](../summary_cn.md)接口实现更广泛的功能。

## Data Mesh API 约定

Data Mesh API 提供 HTTP 和 GRPC 两种访问方法。

当你使用 GRPC 时，你可以通过 protoc 生成出对应编程语言的客户端桩代码，你可以在
[这里](https://github.com/secretflow/kuscia/tree/main/proto/api/v1alpha1/kusciaapi)
找到 Kuscia API 的 protobuf 文件。

当你使用 HTTP 时，你可以访问对应的 HTTP 端点，Kuscia API 的接口通过 POST+JSON 或 POST+PROTOBUF 的形式提供 ，并且满足
protobuf
的 [JSON Mapping](https://protobuf.dev/programming-guides/proto3/#json) 。当请求的 `Content-Type`
为 `application/x-protobuf` 时，使用 PROTOBUF 编码，否则使用 JSON 编码。

### 请求和响应约定

请求总是携带会一个 header 字段，类型为 [RequestHeader](#request-header) ， 如 [CreateDomainDataRequest](domaindata_cn.md#请求createdomaindatarequest) ， 该字段可以携带你自定义的一些数据。

响应总是携带一个 status 字段，类型为 [Status](#status) ， 如 [CreateDomainDataResponse](domaindata_cn.md#响应createdomaindataresponse) ， 该字段描述了响应的状态信息。


{#request-header}

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


{#data-mesh-api}
## 如何使用 Data Mesh API

Data Mesh API 用于在 Domain 侧管理 DomainData，提供了 HTTP 和 GRPC 两种访问方法，在请求形式上和 Kuscia API 均相同。
和 Kuscia API 不同的是，Data Mesh API 位于 Domain 侧而不是 master 上。

### GRPC

为了使用 GRPC 连接上 Kuscia API，你需要：
1. 从 [这里](https://github.com/secretflow/kuscia/tree/main/proto/api/v1alpha1/datamesh) 下载 Kuscia 的 protobuf 文件，使用 protoc
   生成对应编程语言的客户端桩代码。 关于如何生成客户端桩代码，请参看 [Protobuf官方教程](https://protobuf.dev/getting-started/)。
2. 使用 GRPC 客户端发起请求。

GRPC 端口默认在：Domain 的 8071。

### HTTP

你也可以使用 HTTP 的客户端工具连接上 Kuscia API，如 curl，你需要替换 {} 中的内容：
```shell
curl -X POST 'http://{{USER-kuscia-lite-alice}:8070/api/v1/datamesh/domaindata/query' --header 'Content-Type: application/json' -d '{
  "domaindata_id": "alice"
}'
```

HTTP 端口默认在：Domain 的 8070。
