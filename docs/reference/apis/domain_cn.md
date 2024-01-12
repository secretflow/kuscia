# Domain

在 Kuscia 中将隐私计算的节点称为 Domain，一个 Domain 中可以包含多个 K3s
的工作节点（Node）。详情请参考 [Domain](../concepts/domain_cn.md) 。
你可以从 [这里](https://github.com/secretflow/kuscia/tree/main/proto/api/v1alpha1/kusciaapi/domain.proto) 找到对应的 protobuf 文件。

## 接口总览

| 方法名                                     | 请求类型                    | 响应类型                     | 描述       |
|-----------------------------------------|-------------------------|--------------------------|----------|
| [CreateDomain](#create-domain)          | CreateDomainRequest     | CreateDomainResponse     | 创建节点     |
| [UpdateDomain](#update-domain)          | UpdateDomainRequest     | UpdateDomainResponse     | 更新节点     |
| [DeleteDomain](#delete-domain)          | DeleteDomainRequest     | DeleteDomainResponse     | 删除节点     |
| [QueryDomain](#query-domain)            | QueryDomainRequest      | QueryDomainResponse      | 查询节点     |
| [BatchQueryDomain](#batch-query-domain) | BatchQueryDomainRequest | BatchQueryDomainResponse | 批量查询节点状态 |

## 接口详情

{#create-domain}

### 创建节点

#### HTTP 路径

/api/v1/domain/create

#### 请求（CreateDomainRequest）

| 字段          | 类型                                           | 选填 | 描述                                                                                                                                            |
|-------------|----------------------------------------------|----|-----------------------------------------------------------------------------------------------------------------------------------------------|
| header      | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容                                                                                                                                       |
| domain_id   | string                                       | 必填 | 节点 ID 需要符合 DNS 子域名规则要求，参考 [DomainId 规则要求](https://kubernetes.io/zh-cn/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names) |
| role        | string                                       | 可选 | 角色：\["", "partner"]，参考 [Domain 概念](../concepts/domain_cn.md)                                                                                  |
| cert        | string                                       | 可选 | BASE64 的计算节点证书，参考 [Domain 概念](../concepts/domain_cn.md)                                                                                       |
| auth_center | [AuthCenter](#auth-center)                   | 可选 | 节点到中心的授权模式                                                                                                                                    |

#### 响应（CreateDomainResponse）

| 字段     | 类型                             | 选填 | 描述   |
|--------|--------------------------------|----|------|
| status | [Status](summary_cn.md#status) | 必填 | 状态信息 |

{#update-domain}

### 更新节点

#### HTTP 路径

/api/v1/domain/update

#### 请求（UpdateDomainRequest）

| 字段        | 类型                                           | 选填 | 描述                                                           |
|-----------|----------------------------------------------|----|--------------------------------------------------------------|
| header    | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容                                                      |
| domain_id | string                                       | 必填 | 节点 ID                                                        |
| role      | string                                       | 可选 | 角色：\["", "partner"]，参考 [Domain 概念](../concepts/domain_cn.md) |
| cert      | string                                       | 可选 | BASE64 的计算节点证书，参考 [Domain 概念](../concepts/domain_cn.md)      |

#### 响应（UpdateDomainResponse）

| 字段     | 类型                             | 选填 | 描述   |
|--------|--------------------------------|----|------|
| status | [Status](summary_cn.md#status) | 必填 | 状态信息 |

{#delete-domain}

### 删除节点

#### HTTP 路径

/api/v1/domain/delete

#### 请求（DeleteDomainRequest）

| 字段        | 类型                                           | 选填 | 描述      |
|-----------|----------------------------------------------|----|---------|
| header    | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容 |
| domain_id | string                                       | 必填 | 节点 ID   |

#### 响应（DeleteDomainResponse）

| 字段     | 类型                             | 选填 | 描述   |
|--------|--------------------------------|----|------|
| status | [Status](summary_cn.md#status) | 必填 | 状态信息 |

{#query-domain}

### 查询节点

#### HTTP 路径

/api/v1/domain/query

#### 请求（QueryDomainRequest）

| 字段        | 类型                                           | 选填 | 描述      |
|-----------|----------------------------------------------|----|---------|
| header    | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容 |
| domain_id | string                                       | 必填 | 节点 ID   |

#### 响应（QueryDomainResponse）

| 字段                         | 类型                                          | 选填 | 描述                                                           |
|----------------------------|---------------------------------------------|----|--------------------------------------------------------------|
| status                     | [Status](summary_cn.md#status)              | 必填 | 状态信息                                                         |
| data                       | QueryDomainResponseData                     |    |                                                              |
| data.domain_id             | string                                      | 必填 | 节点 ID                                                        |
| data.role                  | string                                      | 必填 | 角色：\["", "partner"]，参考 [Domain 概念](../concepts/domain_cn.md) |
| data.cert                  | string                                      | 可选 | BASE64 的计算节点证书，参考 [Domain 概念](../concepts/domain_cn.md)      |
| data.annotations           | map[string]string                           | 可选 | 节点的额外信息，比如是否是内置节点等                                           |
| data.auth_center           | [AuthCenter](#auth-center)                  | 可选 | 节点到中心的授权模式                                                   |
| data.node_statuses         | [NodeStatus](#node-status)[]                | 必填 | 物理节点状态                                                       |
| data.deploy_token_statuses | [DeployTokenStatus](#deploy-token-status)[] | 必填 | 部署令牌状态                                                       |

{#batch-query-domain}

### 批量查询节点状态

#### HTTP 路径

/api/v1/domain/batchQuery

#### 请求（BatchQueryDomainRequest）

| 字段         | 类型                                           | 选填 | 描述           |
|------------|----------------------------------------------|----|--------------|
| header     | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容      |
| domain_ids | string[]                                     | 必填 | 待查询的节点 ID 列表 |

#### 响应（ BatchQueryDomainResponse）

| 字段           | 类型                             | 选填 | 描述   |
|--------------|--------------------------------|----|------|
| status       | [Status](summary_cn.md#status) | 必填 | 状态信息 |
| data         | BatchQueryDomainResponseData   | 必填 |      |
| data.domains | [Domain](#domain-entity)[]     | 必填 | 节点列表 |

## 公共

{#domain-entity}

### Domain

| 字段                    | 类型                                          | 选填 | 描述                                                           |
|-----------------------|---------------------------------------------|----|--------------------------------------------------------------|
| domain_id             | string                                      | 必填 | 节点 ID                                                        |
| role                  | string                                      | 必填 | 角色：\["", "partner"]，参考 [Domain 概念](../concepts/domain_cn.md) |
| cert                  | string                                      | 可选 | BASE64 的计算节点证书，参考 [Domain 概念](../concepts/domain_cn.md)      |
| node_statuses         | [NodeStatus](#node-status)[]                | 必填 | 真实物理节点状态                                                     |
| deploy_token_statuses | [DeployTokenStatus](#deploy-token-status)[] | 必填 | 部署令牌状态                                                       |

{#node-status}

### NodeStatus

| 字段                   | 类型     | 选填 | 描述                                           |
|----------------------|--------|----|----------------------------------------------|
| name                 | string | 必填 | 节点名称                                         |
| status               | string | 必填 | 节点状态                                         |
| version              | string | 必填 | 节点 Agent 版本                                  |
| last_heartbeat_time  | string | 必填 | 最后心跳时间，RFC3339 格式（e.g. 2006-01-02T15:04:05Z） |
| last_transition_time | string | 必填 | 最后更新时间，RFC3339 格式（e.g. 2006-01-02T15:04:05Z） |

{#auth-center}

### AuthCenter

| 字段                  | 类型     | 选填 | 描述                                 |
|---------------------|--------|----|------------------------------------|
| authentication_type | string | 必填 | 节点到中心授权类型，目前仅支持 Token              |
| token_gen_method    | string | 必填 | 节点到中心 Token 生成类型，目前仅支持 UID-RSA-GEN |

{#deploy-token-status}

### DeployTokenStatus

| 字段                   | 类型     | 选填 | 描述                                           |
|----------------------|--------|----|----------------------------------------------|
| token                | string | 必填 | 部署令牌                                         |
| state                | string | 必填 | 部署令牌状态 used, unsed                           |
| last_transition_time | string | 必填 | 最后更新时间，RFC3339 格式（e.g. 2006-01-02T15:04:05Z） |
