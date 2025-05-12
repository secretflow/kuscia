# DomainRoute

DomainRoute 用于在中心化网络中配置 Lite 节点与 Master 之间的路由规则、Lite 节点之间的路由规则，以及点对点（P2P）网络中
Autonomy 节点之间的路由规则。请参考 [DomainRoute](../concepts/domainroute_cn.md) 。
您可以从 [这里](https://github.com/secretflow/kuscia/tree/main/proto/api/v1alpha1/kusciaapi/domain_route.proto) 找到对应的 protobuf 文件。

## 接口总览

| 方法名                                                             | 请求类型                               | 响应类型                                | 描述         |
|-----------------------------------------------------------------|------------------------------------|-------------------------------------|------------|
| [CreateDomainRoute](#create-domain-route)                       | CreateDomainRouteRequest           | CreateDomainRouteResponse           | 创建节点路由     |
| [DeleteDomainRoute](#delete-domain-route)                       | DeleteDomainRouteRequest           | DeleteDomainRouteResponse           | 删除节点路由     |
| [QueryDomainRoute](#query-domain-route)                         | QueryDomainRouteRequest            | QueryDomainRouteResponse            | 查询节点路由     |
| [BatchQueryDomainRouteStatus](#batch-query-domain-route-status) | BatchQueryDomainRouteStatusRequest | BatchQueryDomainRouteStatusResponse | 批量查询节点路由状态 |

## 接口详情

{#create-domain-route}

### 创建节点路由

#### HTTP 路径

/api/v1/route/create

#### 请求（CreateDomainRouteRequest）

| 字段                 | 类型                                          | 选填  | 描述                                                                                         |
|---------------------|----------------------------------------------|-----|--------------------------------------------------------------------------------------------|
| header              | [RequestHeader](summary_cn.md#requestheader) | 可选  | 请求头                                                                                    |
| authentication_type | string                                       | 必填  | 认证类型：\[Token，MTLS，None]，参考 [DomainRoute 概念](../concepts/domainroute_cn.md)                 |
| destination         | string                                       | 必填  | 目标节点ID                                                                                       |
| endpoint            | [RouteEndpoint](#route-endpoint)             | 可选  | 目标节点的地址（请填写域名，端口和协议不需要填写）；如果是[路由转发模式](../concepts/domainroute_cn.md#domain-route-advance)则不需要填该字段  |
| source              | string                                       | 必填  | 源节点ID |
| token_config        | [TokenConfig](#token-config)                 | 可选  | Token 相关配置；authenticationType 为`Token`，需要配置该字段。 |
| mtls_config         | [MtlsConfig](#mtls-config)                   | 可选  | MTLS 相关配置，authenticationType 为`MTLS`时，需要配置该字段。  |
| transit             | [Transit](#transit)                          | 可选  | 路由转发配置                                                                                     |
| body_encryption     | [BodyEncryption](#bodyencryption)            | 可选  | 加密配置；当路由为转发模式时需要配置该字段     |

#### 响应（CreateDomainRouteResponse）

| 字段        | 类型                             | 描述   |
|-----------|--------------------------------|------|
| status    | [Status](summary_cn.md#status) | 状态信息 |
| data      | CreateDomainRouteResponseData  |      |
| data.name | string                         | 名称   |

#### 请求示例

发起请求：

```sh
# Execute example within the container
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/route/create' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "authentication_type": "Token",
  "destination": "bob",
  "endpoint": {
    "host": "root-kuscia-autonomy-bob",
    "ports": [
      {
        "port": 1080,
        "protocol": "HTTP",
        "isTLS": true
      }
    ]
  },
  "source": "alice",
  "token_config": {
    "token_gen_method": "RSA-GEN"
  }
}'
```

请求响应成功结果：

```json
{
  "status": {
    "code": 0,
    "message": "success",
    "details": []
  },
  "data": null
}
```

请求响应异常结果：

```json
{
  "status": {
    "code": 11405,
    "message": "clusterdomainroutes.kuscia.secretflow \"alice-bob\" already exists",
    "details": []
  },
  "data": null
}
```

{#delete-domain-route}

### 删除节点路由

#### HTTP 路径

/api/v1/route/delete

#### 请求（DeleteDomainRequest）

| 字段          | 类型                                           | 选填 | 描述      |
|-------------|----------------------------------------------|----|---------|
| header      | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容 |
| destination | string                                       | 必填 | 目标节点    |
| source      | string                                       | 必填 | 源节点     |

# DomainRoute

DomainRoute 用于在中心化网络中配置 Lite 节点与 Master 之间的路由规则、Lite 节点之间的路由规则，以及点对点（P2P）网络中
Autonomy 节点之间的路由规则。请参考 [DomainRoute](../concepts/domainroute_cn.md) 。
您可以从 [这里](https://github.com/secretflow/kuscia/tree/main/proto/api/v1alpha1/kusciaapi/domain_route.proto) 找到对应的 protobuf 文件。

## 接口总览

| 方法名                                                             | 请求类型                               | 响应类型                                | 描述         |
|-----------------------------------------------------------------|------------------------------------|-------------------------------------|------------|
| [CreateDomainRoute](#create-domain-route)                       | CreateDomainRouteRequest           | CreateDomainRouteResponse           | 创建节点路由     |
| [DeleteDomainRoute](#delete-domain-route)                       | DeleteDomainRouteRequest           | DeleteDomainRouteResponse           | 删除节点路由     |
| [QueryDomainRoute](#query-domain-route)                         | QueryDomainRouteRequest            | QueryDomainRouteResponse            | 查询节点路由     |
| [BatchQueryDomainRouteStatus](#batch-query-domain-route-status) | BatchQueryDomainRouteStatusRequest | BatchQueryDomainRouteStatusResponse | 批量查询节点路由状态 |

## 接口详情

{#create-domain-route}

### 创建节点路由

#### HTTP 路径

/api/v1/route/create

#### 请求（CreateDomainRouteRequest）

| 字段                 | 类型                                          | 选填  | 描述                                                                                         |
|---------------------|----------------------------------------------|-----|--------------------------------------------------------------------------------------------|
| header              | [RequestHeader](summary_cn.md#requestheader) | 可选  | 请求头                                                                                    |
| authentication_type | string                                       | 必填  | 认证类型：\[Token，MTLS，None]，参考 [DomainRoute 概念](../concepts/domainroute_cn.md)                 |
| destination         | string                                       | 必填  | 目标节点ID                                                                                       |
| endpoint            | [RouteEndpoint](#route-endpoint)             | 可选  | 目标节点的地址（请填写域名，端口和协议不需要填写）；如果是[路由转发模式](../concepts/domainroute_cn.md#domain-route-advance)则不需要填该字段  |
| source              | string                                       | 必填  | 源节点ID |
| token_config        | [TokenConfig](#token-config)                 | 可选  | Token 相关配置；authenticationType 为`Token`，需要配置该字段。 |
| mtls_config         | [MtlsConfig](#mtls-config)                   | 可选  | MTLS 相关配置，authenticationType 为`MTLS`时，需要配置该字段。  |
| transit             | [Transit](#transit)                          | 可选  | 路由转发配置                                                                                     |
| body_encryption     | [BodyEncryption](#bodyencryption)            | 可选  | 加密配置；当路由为转发模式时需要配置该字段     |

#### 响应（CreateDomainRouteResponse）

| 字段        | 类型                             | 描述   |
|-----------|--------------------------------|------|
| status    | [Status](summary_cn.md#status) | 状态信息 |
| data      | CreateDomainRouteResponseData  |      |
| data.name | string                         | 名称   |

#### 请求示例

发起请求：

```sh
# Execute example within the container
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/route/create' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "authentication_type": "Token",
  "destination": "bob",
  "endpoint": {
    "host": "root-kuscia-autonomy-bob",
    "ports": [
      {
        "port": 1080,
        "protocol": "HTTP",
        "isTLS": true
      }
    ]
  },
  "source": "alice",
  "token_config": {
    "token_gen_method": "RSA-GEN"
  }
}'
```

请求响应成功结果：

```json
{
  "status": {
    "code": 0,
    "message": "success",
    "details": []
  },
  "data": null
}
```

请求响应异常结果：

```json
{
  "status": {
    "code": 11405,
    "message": "clusterdomainroutes.kuscia.secretflow \"alice-bob\" already exists",
    "details": []
  },
  "data": null
}
```

{#delete-domain-route}

### 删除节点路由

#### HTTP 路径

/api/v1/route/delete

#### 请求（DeleteDomainRequest）

| 字段          | 类型                                           | 选填 | 描述      |
|-------------|----------------------------------------------|----|---------|
| header      | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容 |
| destination | string                                       | 必填 | 目标节点    |
| source      | string                                       | 必填 | 源节点     |

#### 响应（DeleteDomainResponse）

| 字段     | 类型                             | 描述   |
|--------|--------------------------------|------|
| status | [Status](summary_cn.md#status) | 状态信息 |

#### 请求示例

发起请求：

```sh
# Execute example within the container
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/route/delete' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "destination": "bob",
  "source": "alice"
}'
```

请求响应成功结果：

```json
{
  "status": {
    "code": 0,
    "message": "success",
    "details": []
  }
}
```

请求响应异常结果：假设删除路由为 `delete-alice` -> `delete-bob` 且不存在

```json
{
  "status": {
    "code": 11404,
    "message": "clusterdomainroutes.kuscia.secretflow \"delete-alice-delete-bob\" not found",
    "details": []
  }
}
```

{#query-domain-route}

### 查询节点路由

#### HTTP 路径

/api/v1/route/query

#### 请求（QueryDomainRouteRequest）

| 字段          | 类型                                           | 选填 | 描述      |
|-------------|----------------------------------------------|----|---------|
| header      | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容 |
| destination | string                                       | 必填 | 目标地址    |
| source      | string                                       | 必填 | 源地址     |

#### 响应（QueryDomainRouteResponse）

| 字段                       | 类型                               | 描述                                                                         |
|--------------------------|----------------------------------|----------------------------------------------------------------------------|
| status                   | [Status](./summary_cn.md#status) | 状态信息                                                                       |
| data                     | QueryDomainRouteResponseData     |                                                                             |
| data.name                | string                           | 节点名称                                                                       |
| data.authentication_type | string                           | 认证类型：\[Token，MTLS，None]，参考 [DomainRoute 概念](../concepts/domainroute_cn.md) |
| data.destination         | string                           | 目标节点                                                                       |
| data.endpoint            | [RouteEndpoint](#route-endpoint) | 目标节点的地址，参考 [DomainRoute 概念](../concepts/domainroute_cn.md)                 |
| data.source              | string                           | 源节点                                                                        |
| data.token_config        | [TokenConfig](#token-config)     | Token 配置                                                                   |
| data.mtls_config         | [MTLSConfig](#mtls-config)       | MTLS 配置                                                                    |
| data.transit             | [Transit](#transit)              | 路由转发配置                                                                   |
| data.body_encryption     | [BodyEncryption](#bodyencryption) | 加密配置                                                                    |
| data.status              | [RouteStatus](#route-status)     | 状态                                                                         |

#### 请求示例

发起请求：

```sh
# Execute example within the container
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/route/query' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "destination": "bob",
  "source": "alice"
}'
```

请求响应成功结果：

```json
{
  "status": {
    "code": 0,
    "message": "success",
    "details": []
  },
  "data": {
    "name": "alice-bob",
    "authentication_type": "Token",
    "destination": "bob",
    "endpoint": {
      "host": "root-kuscia-autonomy-bob",
      "ports": [
        {
          "name": "",
          "port": 1080,
          "protocol": "HTTP"
        }
      ]
    },
    "source": "alice",
    "token_config": {
      "destination_public_key": "LS0tLS1CRUdJTiBSU0EgUFVCTElDIEt... base64 encoded str",
      "rolling_update_period": "0",
      "source_public_key": "LS0tLS1CRUdJTiBSU0EgUFVCTElDIEtFWS0t... base64 encoded str",
      "token_gen_method": "RSA-GEN"
    },
    "mtls_config": null,
    "status": {
      "status": "Succeeded",
      "reason": ""
    }
  }
}
```

请求响应异常结果：假设请求路由为 `query-alice` -> `query-bob` 且不存在

```json
{
  "status": {
    "code": 11404,
    "message": "clusterdomainroutes.kuscia.secretflow \"query-alice-query-bob\" not found",
    "details": []
  },
  "data": null
}
```

{#batch-query-domain-route-status}

### 批量查询节点路由状态

#### HTTP 路径

/api/v1/route/status/batchQuery

#### 请求（BatchQueryDomainRouteStatusRequest）

| 字段         | 类型                                           | 选填 | 描述      |
|------------|----------------------------------------------|----|---------|
| header     | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容 |
| route_keys | [DomainRouteKey](#domain-route-key)          | 必填 | 路由列表    |

#### 响应（BatchQueryDomainRouteStatusResponse）

| 字段          | 类型                                          | 描述     |
|-------------|---------------------------------------------|--------|
| status      | [Status](summary_cn.md#status)              | 状态信息   |
| data        | BatchQueryDomainRouteStatusResponseData     |        |
| data.routes | [DomainRouteStatus](#domain-route-status)[] | 授权路由列表 |

#### 请求示例

发起请求：

```sh
# Execute example within the container
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/route/status/batchQuery' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "route_keys": [
    {
      "source": "alice",
      "destination": "bob"
    },
    {
      "source": "bob",
      "destination": "alice"
    }
  ]
}'
```

请求响应成功结果：

```json
{
  "status": {
    "code": 0,
    "message": "success",
    "details": []
  },
  "data": {
    "routes": [
      {
        "name": "alice-bob",
        "destination": "bob",
        "source": "alice",
        "status": {
          "status": "Succeeded",
          "reason": ""
        }
      },
      {
        "name": "bob-alice",
        "destination": "alice",
        "source": "bob",
        "status": {
          "status": "Succeeded",
          "reason": ""
        }
      }
    ]
  }
}
```

请求响应异常结果：假设查询中包含不存在的路由 `status-alice` -> `status-bob`

```json
{
  "status": {
    "code": 11404,
    "message": "clusterdomainroutes.kuscia.secretflow \"status-alice-status-bob\" not found",
    "details": []
  },
  "data": null
}
```

## 公共

{#domain-route-key}

### DomainRouteKey

| 字段          | 类型     | 选填 | 描述      |
|-------------|--------|----|---------|
| source      | string | 必填 | 源节点 ID  |
| destination | string | 必填 | 目标节点 ID |

{#domain-route-status}

### DomainRouteStatus

| 字段          | 类型                           | 描述      |
|-------------|------------------------------|---------|
| name        | string                       | 名称      |
| destination | string                       | 目标节点 ID |
| source      | string                       | 源节点 ID  |
| status      | [RouteStatus](#route-status) | 状态      |

{#endpoint-port}

### EndpointPort

| 字段         | 类型    | 选填 | 描述                    |
|-------------|--------|------|------------------------|
| port        | int32  | 必填 | 端口号，有效值1-65535                   |
| protocol    | string | 必填 | 端口协议：\[HTTP, GRPC]   |
| isTLS       | bool   | 可选 | 是否开启 TLS，默认为 false |
| path_prefix | string | 可选 | 如果非空，网关会对发送的请求进行 path rewrite，在请求的path 前加上 path_prefix 的值      |

{#route-endpoint}

### RouteEndpoint

| 字段    | 类型                               | 选填 | 描述   |
|-------|----------------------------------|----|------|
| host  | string                           | 必填 | 目标主机 |
| ports | [EndpointPort](#endpoint-port)[] | 必填 | 目标端口 |

{#route-status}

### RouteStatus

| 字段     | 类型     | 描述                       |
|--------|--------|--------------------------|
| status | string | 是否成功：\[Succeeded,Failed] |
| reason | string | 原因                       |

{#mtls-config}

### MTLSConfig

详细参考 [DomainRoute 概念](../concepts/domainroute_cn.md) 。

| 字段                        | 类型     | 选填 | 描述       |
|---------------------------|--------|----|----------|
| tls_ca                    | string | 必填 | TLS 的 CA，BASE64 编码格式 |
| source_client_private_key | string | 必填 | 来源客户端的私钥，BASE64 编码格式 |
| source_client_cert        | string | 必填 | 来源客户端的证书，BASE64 编码格式 |

{#token-config}

### TokenConfig

详细参考 [DomainRoute 概念](../concepts/domainroute_cn.md) 。

| 字段                     | 类型     | 选填 | 描述                        |
|------------------------|--------|----|---------------------------|
| rolling_update_period  | int64  |选填 | 滚动更新间隔，单位：秒，默认值为 0                                                   |
| token_gen_method       | string | 必填 | 签名方式：`RSA-GEN`，表示双方各生成一半，拼成一个32长度的通信 Token，并且用对方的公钥加密，双方都会用自己的私钥验证 Token 有效性  |

{#transit}

### Transit

详细参考 [DomainRoute 概念](../concepts/domainroute_cn.md) 。

| 字段           | 类型                                  | 选填 | 描述                                                                                      |
| -------------- |-------------------------------------| ---- |-----------------------------------------------------------------------------------------|
| transit_method | string                              | 必填 | 路由转发类型：\[THIRD-DOMAIN，REVERSE-TUNNEL]。THIRD-DOMAIN 表示经由第三方节点转发，REVERSE-TUNNEL 表示经由反向隧道转发 |
| domain         | [Transit_Domain](#transit_domain)[] | 可选 | transit_method 为 THIRD-DOMAIN 时需要填写，否则不需要填写       |

{#transit_domain}

#### Transit_Domain

| 字段      | 类型   | 选填 | 描述            |
| --------- | ------ | ---- | --------------- |
| domain_id | string | 必填 | 第三方节点 ID |

### BodyEncryption

详细参考 [DomainRoute 概念](../concepts/domainroute_cn.md) 。

| 字段             | 类型   | 选填 | 描述                                                            |
| ---------------- | ------ | ---- | --------------------------------------------------------------- |
| algorithm        | string | 必填 | 加密方法，当前只支持 AES                                          |

发起请求：

```sh
# 在容器内执行示例
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/route/delete' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "destination": "bob",
  "source": "alice"
}'
```

请求响应成功结果：

```json
{
  "status": {
    "code": 0,
    "message": "success",
    "details": []
  }
}
```

请求响应异常结果：假设删除路由为 `delete-alice` -> `delete-bob` 且不存在

```json
{
  "status": {
    "code": 11404,
    "message": "clusterdomainroutes.kuscia.secretflow \"delete-alice-delete-bob\" not found",
    "details": []
  }
}
```

{#query-domain-route}

### 查询节点路由

#### HTTP 路径

/api/v1/route/query

#### 请求（QueryDomainRouteRequest）

| 字段          | 类型                                           | 选填 | 描述      |
|-------------|----------------------------------------------|----|---------|
| header      | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容 |
| destination | string                                       | 必填 | 目标地址    |
| source      | string                                       | 必填 | 源地址     |

#### 响应（QueryDomainRouteResponse）

| 字段                       | 类型                               | 描述                                                                         |
|--------------------------|----------------------------------|----------------------------------------------------------------------------|
| status                   | [Status](./summary_cn.md#status) | 状态信息                                                                       |
| data                     | QueryDomainRouteResponseData     |                                                                             |
| data.name                | string                           | 节点名称                                                                       |
| data.authentication_type | string                           | 认证类型：\[Token，MTLS，None]，参考 [DomainRoute 概念](../concepts/domainroute_cn.md) |
| data.destination         | string                           | 目标节点                                                                       |
| data.endpoint            | [RouteEndpoint](#route-endpoint) | 目标节点的地址，参考 [DomainRoute 概念](../concepts/domainroute_cn.md)                 |
| data.source              | string                           | 源节点                                                                        |
| data.token_config        | [TokenConfig](#token-config)     | Token 配置                                                                   |
| data.mtls_config         | [MTLSConfig](#mtls-config)       | MTLS 配置                                                                    |
| data.transit             | [Transit](#transit)              | 路由转发配置                                                                   |
| data.body_encryption     | [BodyEncryption](#bodyencryption) | 加密配置                                                                    |
| data.status              | [RouteStatus](#route-status)     | 状态                                                                         |

#### 请求示例

发起请求：

```sh
# 在容器内执行示例
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/route/query' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "destination": "bob",
  "source": "alice"
}'
```

请求响应成功结果：

```json
{
  "status": {
    "code": 0,
    "message": "success",
    "details": []
  },
  "data": {
    "name": "alice-bob",
    "authentication_type": "Token",
    "destination": "bob",
    "endpoint": {
      "host": "root-kuscia-autonomy-bob",
      "ports": [
        {
          "name": "",
          "port": 1080,
          "protocol": "HTTP"
        }
      ]
    },
    "source": "alice",
    "token_config": {
      "destination_public_key": "LS0tLS1CRUdJTiBSU0EgUFVCTElDIEt... base64 encoded str",
      "rolling_update_period": "0",
      "source_public_key": "LS0tLS1CRUdJTiBSU0EgUFVCTElDIEtFWS0t... base64 encoded str",
      "token_gen_method": "RSA-GEN"
    },
    "mtls_config": null,
    "status": {
      "status": "Succeeded",
      "reason": ""
    }
  }
}
```

请求响应异常结果：假设请求路由为 `query-alice` -> `query-bob` 且不存在

```json
{
  "status": {
    "code": 11404,
    "message": "clusterdomainroutes.kuscia.secretflow \"query-alice-query-bob\" not found",
    "details": []
  },
  "data": null
}
```

{#batch-query-domain-route-status}

### 批量查询节点路由状态

#### HTTP 路径

/api/v1/route/status/batchQuery

#### 请求（BatchQueryDomainRouteStatusRequest）

| 字段         | 类型                                           | 选填 | 描述      |
|------------|----------------------------------------------|----|---------|
| header     | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容 |
| route_keys | [DomainRouteKey](#domain-route-key)          | 必填 | 路由列表    |

#### 响应（BatchQueryDomainRouteStatusResponse）

| 字段          | 类型                                          | 描述     |
|-------------|---------------------------------------------|--------|
| status      | [Status](summary_cn.md#status)              | 状态信息   |
| data        | BatchQueryDomainRouteStatusResponseData     |        |
| data.routes | [DomainRouteStatus](#domain-route-status)[] | 授权路由列表 |

#### 请求示例

发起请求：

```sh
# 在容器内执行示例
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/route/status/batchQuery' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "route_keys": [
    {
      "source": "alice",
      "destination": "bob"
    },
    {
      "source": "bob",
      "destination": "alice"
    }
  ]
}'
```

请求响应成功结果：

```json
{
  "status": {
    "code": 0,
    "message": "success",
    "details": []
  },
  "data": {
    "routes": [
      {
        "name": "alice-bob",
        "destination": "bob",
        "source": "alice",
        "status": {
          "status": "Succeeded",
          "reason": ""
        }
      },
      {
        "name": "bob-alice",
        "destination": "alice",
        "source": "bob",
        "status": {
          "status": "Succeeded",
          "reason": ""
        }
      }
    ]
  }
}
```

请求响应异常结果：假设查询中包含不存在的路由 `status-alice` -> `status-bob`

```json
{
  "status": {
    "code": 11404,
    "message": "clusterdomainroutes.kuscia.secretflow \"status-alice-status-bob\" not found",
    "details": []
  },
  "data": null
}
```

## 公共

{#domain-route-key}

### DomainRouteKey

| 字段          | 类型     | 选填 | 描述      |
|-------------|--------|----|---------|
| source      | string | 必填 | 源节点 ID  |
| destination | string | 必填 | 目标节点 ID |

{#domain-route-status}

### DomainRouteStatus

| 字段          | 类型                           | 描述      |
|-------------|------------------------------|---------|
| name        | string                       | 名称      |
| destination | string                       | 目标节点 ID |
| source      | string                       | 源节点 ID  |
| status      | [RouteStatus](#route-status) | 状态      |

{#endpoint-port}

### EndpointPort

| 字段         | 类型    | 选填 | 描述                    |
|-------------|--------|------|------------------------|
| port        | int32  | 必填 | 端口号，有效值1-65535                   |
| protocol    | string | 必填 | 端口协议：\[HTTP, GRPC]   |
| isTLS       | bool   | 可选 | 是否开启 TLS，默认为 false |
| path_prefix | string | 可选 | 如果非空，网关会对发送的请求进行 path rewrite，在请求的path 前加上 path_prefix 的值      |

{#route-endpoint}

### RouteEndpoint

| 字段    | 类型                               | 选填 | 描述   |
|-------|----------------------------------|----|------|
| host  | string                           | 必填 | 目标主机 |
| ports | [EndpointPort](#endpoint-port)[] | 必填 | 目标端口 |

{#route-status}

### RouteStatus

| 字段     | 类型     | 描述                       |
|--------|--------|--------------------------|
| status | string | 是否成功：\[Succeeded,Failed] |
| reason | string | 原因                       |

{#mtls-config}

### MTLSConfig

详细参考 [DomainRoute 概念](../concepts/domainroute_cn.md) 。

| 字段                        | 类型     | 选填 | 描述       |
|---------------------------|--------|----|----------|
| tls_ca                    | string | 必填 | TLS 的 CA，BASE64 编码格式 |
| source_client_private_key | string | 必填 | 来源客户端的私钥，BASE64 编码格式 |
| source_client_cert        | string | 必填 | 来源客户端的证书，BASE64 编码格式 |

{#token-config}

### TokenConfig

详细参考 [DomainRoute 概念](../concepts/domainroute_cn.md) 。

| 字段                     | 类型     | 选填 | 描述                        |
|------------------------|--------|----|---------------------------|
| rolling_update_period  | int64  |选填 | 滚动更新间隔，单位：秒，默认值为 0                                                   |
| token_gen_method       | string | 必填 | 签名方式：`RSA-GEN`，表示双方各生成一半，拼成一个32长度的通信 Token，并且用对方的公钥加密，双方都会用自己的私钥验证 Token 有效性  |

{#transit}

### Transit

详细参考 [DomainRoute 概念](../concepts/domainroute_cn.md) 。

| 字段           | 类型                                  | 选填 | 描述                                                                                      |
| -------------- |-------------------------------------| ---- |-----------------------------------------------------------------------------------------|
| transit_method | string                              | 必填 | 路由转发类型：\[THIRD-DOMAIN，REVERSE-TUNNEL]。THIRD-DOMAIN 表示经由第三方节点转发，REVERSE-TUNNEL 表示经由反向隧道转发 |
| domain         | [Transit_Domain](#transit_domain)[] | 可选 | transit_method 为 THIRD-DOMAIN 时需要填写，否则不需要填写       |

{#transit_domain}

#### Transit_Domain

| 字段      | 类型   | 选填 | 描述            |
| --------- | ------ | ---- | --------------- |
| domain_id | string | 必填 | 第三方节点 ID |

### BodyEncryption

详细参考 [DomainRoute 概念](../concepts/domainroute_cn.md) 。

| 字段             | 类型   | 选填 | 描述                                                            |
| ---------------- | ------ | ---- | --------------------------------------------------------------- |
| algorithm        | string | 必填 | 加密方法，当前只支持 AES                                          |
