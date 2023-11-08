# DomainDataGrant

DomainDataGrant 表示被 Kuscia 管理的数据授权对象。请参考 [DomainDataGrant](../concepts/domaindatagrant_cn.md)。
你可以从 [这里](https://github.com/secretflow/kuscia/tree/main/proto/api/v1alpha1/kusciaapi/domaindatagrant.proto) 找到对应的 protobuf 文件。


## 接口总览

| 方法名                                              | 请求类型                        | 响应类型                         | 描述       |
|--------------------------------------------------|-----------------------------|------------------------------|----------|
| [CreateDomainDataGrant](#create-domain-data-grant) | CreateDomainDataGrantRequest | CreateDomainDataGrantResponse     | 创建数据对象授权   |
| [UpdateDomainDataGrant](#update-domain-data-grant) | UpdateDomainDataGrantRequest | UpdateDomainDataGrantResponse     | 更新数据对象授权   |
| [DeleteDomainDataGrant](#delete-domain-data-grant) | DeleteDomainDataGrantRequest | DeleteDomainDataGrantResponse     | 删除数据对象授权   |
| [QueryDomainDataGrant](#query-domain-data-grant) | QueryDomainDataGrantRequest | QueryDomainDataGrantResponse      | 查询数据对象授权   |
| [BatchQueryDomainDataGrant](#batch-query-domain-data-grant) | BatchQueryDomainDataGrantRequest | BatchQueryDomainDataGrantResponse | 批量查询数据对象授权 |

## 接口详情

{#create-domain-data-grant}

### 创建数据对象授权

#### HTTP 路径

/api/v1/domaindatagrant/create

{#create-domain-data-grant-request}

#### 请求（CreateDomainDataGrantRequest）

| 字段            | 类型                                           | 选填 | 描述                                                                                                                             |
|---------------|----------------------------------------------|----|--------------------------------------------------------------------------------------------------------------------------------|
| header        | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容 |
| domaindatagrant_id | string | 可选 | 数据对象授权 ID，如果不填，则会由 kusciaapi 自动生成，并在 response 中返回。如果填写，则会使用填写的值，请注意需满足 [DNS 子域名规则要求](https://kubernetes.io/zh-cn/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names) |
| domaindata_id | string | 必填 | 数据对象 ID   |
| grant_domain  | string | 必填 | 被授权节点ID       |
| limit         | [GrantLimit](#grant-limit-entity) | 选填 | 授权限制条件  |
| description   | map<string, string> | 必填 | 自定义描述 |
| domain_id   | string | 必填 | 授权信息所有者节点ID |

{#create-domain-data-grant-response}

#### 响应（CreateDomainDataGrantResponse）

| 字段                 | 类型                             | 选填 | 描述      |
|--------------------|--------------------------------|----|---------|
| status             | [Status](summary_cn.md#status) | 必填 | 状态信息    |
| data               | CreateDomainDataGrantResponseData   |  选填  | 授权信息结果        |
| data.domaindatagrant_id | string                         | 必填 | 数据对象授权 ID |

{#update-domain-data-grant}

### 更新数据对象授权

#### HTTP 路径

/api/v1/domaindatagrant/update

#### 请求（UpdateDomainDataGrantRequest）

| 字段            | 类型                                           | 选填 | 描述                                                                                                                               |
|---------------|----------------------------------------------|----|----------------------------------------------------------------------------------------------------------------------------------|
| header        | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容 |
| domaindatagrant_id | string | 必填 | 数据对象授权ID |
| domaindata_id | string | 必填 | 数据对象ID  |
| grant_domain  | string | 必填 | 被授权节点ID       |
| limit         | [GrantLimit](#grant-limit-entity) | 选填 | 授权限制条件  |
| description   | map<string, string> | 可选 | 自定义描述 |
| domain_id   | string | 必填 | 授权信息所有者节点ID |

#### 响应（UpdateDomainDataGrantResponse）

| 字段     | 类型                             | 选填 | 描述   |
|--------|--------------------------------|----|------|
| status | [Status](summary_cn.md#status) | 必填 | 状态信息 |

{#delete-domain-data-grant}

### 删除数据对象授权

#### HTTP 路径

/api/v1/domaindatagrant/delete

#### 请求（DeleteDomainDataGrantRequest）

| 字段            | 类型                                           | 选填 | 描述      |
|---------------|----------------------------------------------|----|---------|
| header        | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容 |
| domain_id     | string                                       | 必填 | 节点 ID   |
| domaindatagrant_id | string                                  | 必填 | 数据对象授权 ID |

#### 响应（DeleteDomainDataGrantResponse）

| 字段     | 类型                             | 选填 | 描述   |
|--------|--------------------------------|----|------|
| status | [Status](summary_cn.md#status) | 必填 | 状态信息 |

{#query-domain-data-grant}

### 查询数据对象授权

#### HTTP 路径

/api/v1/domaindatagrant/query

#### 请求（QueryDomainGrantRequest）

| 字段     | 类型                                                            | 选填 | 描述      |
|--------|---------------------------------------------------------------|----|---------|
| header | [RequestHeader](summary_cn.md#requestheader)                  | 可选 | 自定义请求内容 |
| domain_id     | string                                       | 必填 | 节点 ID   |
| domaindatagrant_id | string                                  | 必填 | 数据对象授权 ID |

#### 响应（QueryDomainGrantResponse）

| 字段     | 类型                                | 选填 | 描述   |
|--------|-----------------------------------|----|------|
| status | [Status](summary_cn.md#status)    | 必填 | 状态信息 |
| data   | [DomainDataGrant](#domain-data-grant-entity) |  可选  |   授权信息    |

{#batch-query-domain-data-grant}

### 批量查询数据对象授权

#### HTTP 路径

/api/v1/domaindatagrant/batchQuery

#### 请求（BatchQueryDomainDataGrantRequest）

| 字段     | 类型                                                              | 选填 | 描述      |
|--------|-----------------------------------------------------------------|----|---------|
| header | [RequestHeader](summary_cn.md#requestheader)                    | 可选 | 自定义请求内容 |
| data   | [QueryDomainDataGrantRequestData](#query-domain-data-grant-request-data)[] | 必填 | 查询内容    |

#### 响应（BatchQueryDomainDataGrantResponse）

| 字段     | 类型                                  | 选填 | 描述   |
|--------|-------------------------------------|----|------|
| status | [Status](summary_cn.md#status)      | 必填 | 状态信息 |
| data   | [DomainDataGrant](#domain-data-grant-entity)[] |  可选  |  授权信息列表    |

{#list-domain-data-grant}

## 公共

{#grant-limit-entity}

### GrantLimit

| 字段 | 类型 | 选填 | 描述 |
|---------------|------------------------------|----|------------------------------------------------------------------------------------------------------------------------------------|
| expiration_time | int64 |选填 | 授权过期时间，Unix 时间戳，精确到纳秒   |
| use_count | int32 |选填  | 授权使用次数  |
| flow_id | string |选填  | 授权对应的任务流ID  |
| components |  repeated string | 选填  | 授权可用的组件ID  |
| initiator |  string |选填  | 授权指定的发起方  |
| input_config |  string |选填  | 授权指定的算子输入参数 |

{#query-domain-data-grant-request-data}

### QueryDomainDataGrantRequestData

| 字段 | 类型 | 选填 | 描述 |
|---------------|------------------------------|----|------------------------------------------------------------------------------------------------------------------------------------|
| domain_id     | string        | 必填 | 节点 ID   |
| domaindatagrant_id | string   | 必填 | 数据对象授权 ID |

{#domain-data-grant-entity}

### DomainDataGrant

| 字段 | 类型 | 选填 | 描述 |
|---------------|------------------------------|----|------------------------------------------------------------------------------------------------------------------------------------|
| data | [DomainDataGrantData](#domain-data-grant-data-entity)       | 必填 | 数据对象ID  |
| status | [DomainDataGrantStatus](#domain-data-grant-status-entity)   | 必填 | 数据对象授权 ID |

{#domain-data-grant-data-entity}

### DomainDataGrantData

| 字段 | 类型 | 选填 | 描述 |
|---------------|------------------------------|----|------------------------------------------------------------------------------------------------------------------------------------|
| domaindatagrant_id | string | 必填 | 数据对象授权 ID |
| author | string | 必填 | 数据对象授权方节点 ID |
| domaindata_id | string | 必填 | 数据对象 ID  |
| grant_domain  | string | 必填 | 被授权节点 ID       |
| limit         | [GrantLimit](#grant-limit-entity) | 选填 | 授权限制条件  |
| description   | map<string, string> | 可选 | 自定义描述 |
| domain_id   | string | 必填 | 授权信息所有者节点 ID |

{#domain-data-grant-status-entity}

### DomainDataGrantStatus

| 字段 | 类型 | 选填 | 描述 |
|---------------|------------------------------|----|------------------------------------------------------------------------------------------------------------------------------------|
| phase   | string | 必填 | 状态 |
| message   | string | 可选 | 状态描述信息 |
| records   | UseRecord[] | 选填 | 授权使用记录 |
| records[].use_time   | int64 | 选填 | 读取授权信息时间，Unix 时间戳，精确到纳秒 |
| records[].grant_domain   | string | 选填 | 被授权节点 ID |
| records[].component   | string | 选填 | 使用授权的组件 |
| records[].output   | string | 选填 | 使用授权的任务输出 |
