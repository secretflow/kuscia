# DomainRoute

DomainRoute 用于在中心化网络中配置 Lite 节点与 Master 之间的路由规则、Lite 节点之间的路由规则，以及点对点（P2P）网络中
Autonomy 节点之间的路由规则。请参考 [DomainRoute](../../concepts/domainroute_cn.md) 。
你可以从 [这里](https://github.com/secretflow/kuscia/tree/main/proto/api/v1alpha1/kusciaapi/domainroute.proto) 找到对应的
protobuf 文件。

## 接口总览

| 方法名                                                             | 请求类型                               | 响应类型                                | 描述         |
|-----------------------------------------------------------------|------------------------------------|-------------------------------------|------------|
| [CreateDomainRoute](#create-domain-route)                       | CreateDomainRouteRequest           | CreateDomainRouteResponse           | 创建节点路由     |
| [DeleteDomainRoute](#delete-domain-route)                       | DeleteDomainRouteRequest           | DeleteDomainRouteResponse           | 查询节点路由     |
| [QueryDomainRoute](#query-domain-route)                         | QueryDomainRouteRequest            | QueryDomainRouteResponse            | 查询节点路由     |
| [BatchQueryDomainRouteStatus](#batch-query-domain-route-status) | BatchQueryDomainRouteStatusRequest | BatchQueryDomainRouteStatusResponse | 批量查询节点路由状态 |

## 接口详情

{#create-domain-route}

### 创建节点路由

#### HTTP路径
/api/v1/route/create

#### 请求（CreateDomainRouteRequest）

| 字段                  | 类型                                            | 可选 | 描述                                                                           |
|---------------------|-----------------------------------------------|----|------------------------------------------------------------------------------|
| header              | [RequestHeader](summary_cn.md#request-header) | 是  | 自定义请求内容                                                                      |
| authentication_type | string                                        | 否  | 认证类型：\[Token，MTLS，None]，参考 [DomainRoute概念](../../concepts/domainroute_cn.md) |
| destination         | string                                        | 否  | 目标节点                                                                         |
| endpoint            | [RouteEndpoint](#route-endpoint)              | 否  | 目标节点的地址                                                                      |
| source              | string                                        | 否  | 源节点                                                                          |

#### 响应（CreateDomainRouteResponse）

| 字段        | 类型                             | 可选 | 描述   |
|-----------|--------------------------------|----|------|
| status    | [Status](summary_cn.md#status) | 否  | 状态信息 |
| data      | CreateDomainRouteResponseData  | 否  |      |
| data.name | string                         | 否  | 名称   |

{#delete-domain-route}

### 删除节点路由

#### HTTP路径
/api/v1/route/delete

#### 请求（DeleteDomainRequest）

| 字段          | 类型                                            | 可选 | 描述      |
|-------------|-----------------------------------------------|----|---------|
| header      | [RequestHeader](summary_cn.md#request-header) | 是  | 自定义请求内容 |
| destination | string                                        | 否  | 目标节点    |
| source      | string                                        | 否  | 源节点     |

#### 响应（DeleteDomainResponse）

| 字段     | 类型                             | 可选 | 描述   |
|--------|--------------------------------|----|------|
| status | [Status](summary_cn.md#status) | 否  | 状态信息 |


{#query-domain-route}

### 查询节点路由

#### HTTP路径
/api/v1/route/query

#### 请求（QueryDomainRouteRequest）

| 字段          | 类型                                            | 可选 | 描述      |
|-------------|-----------------------------------------------|----|---------|
| header      | [RequestHeader](summary_cn.md#request-header) | 是  | 自定义请求内容 |
| destination | string                                        | 否  | 目标地址    |
| source      | string                                        | 否  | 源地址     |

#### 响应（QueryDomainRouteResponse）

| 字段                       | 类型                               | 可选 | 描述                                                                           |
|--------------------------|----------------------------------|----|------------------------------------------------------------------------------|
| status                   | [Status](./summary.html#status)  | 否  | 状态信息                                                                         |
| data                     | QueryDomainRouteResponseData     | 否  |                                                                              |
| data.name                | string                           | 否  | 节点名称                                                                         |
| data.authentication_type | string                           | 否  | 认证类型：\[Token，MTLS，None]，参考 [DomainRoute概念](../../concepts/domainroute_cn.md) |
| data.destination         | string                           | 否  | 目标节点                                                                         |
| data.endpoint            | [RouteEndpoint](#route-endpoint) | 否  | 目标节点的地址，参考 [DomainRoute概念](../../concepts/domainroute_cn.md)                 |
| data.source              | string                           | 否  | 源节点                                                                          |
| data.token_config        | [TokenConfig](#token-config)     | 是  | Token配置                                                                      |
| data.mtls_config         | [MTLSConfig](#mtls-config)       | 是  | MTLS配置                                                                       |
| data.status              | [RouteStatus](#route-status)     | 否  | 状态                                                                           |

{#batch-query-domain-route-status}

### 批量查询节点路由状态

#### HTTP路径
/api/v1/route/status/batchQuery

#### 请求（BatchQueryDomainRouteStatusRequest）

| 字段         | 类型                                            | 可选 | 描述      |
|------------|-----------------------------------------------|----|---------|
| header     | [RequestHeader](summary_cn.md#request-header) | 是  | 自定义请求内容 |
| route_keys | [DomainRouteKey](#domain-route-key)           | 否  | 路由列表    |

#### 响应（BatchQueryDomainRouteStatusResponse）

| 字段          | 类型                                          | 可选 | 描述     |
|-------------|---------------------------------------------|----|--------|
| status      | [Status](summary_cn.md#status)              | 否  | 状态信息   |
| data        | BatchQueryDomainRouteStatusResponseData     |    |        |
| data.routes | [DomainRouteStatus](#domain-route-status)[] | 否  | 授权路由列表 |

## 公共

{#domain-route-key}

### DomainRouteKey

| 字段          | 类型     | 可选 | 描述     |
|-------------|--------|----|--------|
| source      | string | 否  | 源节点ID  |
| destination | string | 否  | 目标节点ID |

{#domain-route-status}

### DomainRouteStatus

| 字段          | 类型                           | 可选 | 描述     |
|-------------|------------------------------|----|--------|
| name        | string                       | 否  | 名称     |
| destination | string                       | 否  | 目标节点ID |
| source      | string                       | 否  | 源节点ID  |
| status      | [RouteStatus](#route-status) | 否  | 状态     |

{#endpoint-port}

### EndpointPort

| 字段       | 类型     | 可选 | 描述                 |
|----------|--------|----|--------------------|
| port     | int32  | 否  | 端口号                |
| protocol | string | 否  | 端口协议：\[HTTP, GRPC] |

{#route-endpoint}

### RouteEndpoint

| 字段    | 类型                               | 可选 | 描述   |
|-------|----------------------------------|----|------|
| host  | string                           | 否  | 目标主机 |
| ports | [EndpointPort](#endpoint-port)[] | 否  | 目标端口 |

{#route-status}

### RouteStatus

| 字段     | 类型     | 可选 | 描述                       |
|--------|--------|----|--------------------------|
| status | string | 否  | 是否成功：\[Succeeded,Failed] |
| reason | string | 是  | 原因                       |

{#mtls-config}

### MTLSConfig

详细参考 [DomainRoute概念](../../concepts/domainroute_cn.md) 。

| 字段                        | 类型     | 可选 | 描述       |
|---------------------------|--------|----|----------|
| tls_ca                    | string | 否  | TLS的CA   |
| source_client_private_key | string | 否  | 来源客户端的私钥 |
| source_client_cert        | string | 否  | 来源客户端的证书 |

{#token-config}

### TokenConfig

详细参考 [DomainRoute概念](../../concepts/domainroute_cn.md) 。

| 字段                     | 类型     | 可选 | 描述                        |
|------------------------|--------|----|---------------------------|
| destination_public_key | string | 否  | 目标节点的公钥                   |
| rolling_update_period  | int64  | 否  | 滚动更新间隔，单位 秒               |
| source_public_key      | string | 否  | 源节点的公钥                    |
| token_gen_method       | string | 否  | 签名方式：\[RSA-GEN, RAND-GEN] |

