# DomainDataGrant

DomainDataGrant 表示被 Kuscia 管理的数据对象的授权，Data Mesh API 提供了从 Domain 侧的管理 DomainDataGrant
的能力。
请参考 [DomainDataGrant](../../concepts/domaindatagrant_cn.md)。
你可以从 [这里](https://github.com/secretflow/kuscia/blob/main/proto/api/v1alpha1/datamesh/domaindatagrant.proto) 找到对应的 protobuf 文件。

## 接口总览

| 方法名                                              | 请求类型                        | 响应类型                         | 描述       |
|--------------------------------------------------|-----------------------------|------------------------------|----------|
| [CreateDomainDataGrant](#create-domain-data-grant)          | CreateDomainDataGrantRequest     | CreateDomainDataGrantResponse     | 创建数据对象授权   |
| [UpdateDomainDataGrant](#update-domain-data-grant)          | UpdateDomainDataGrantRequest     | UpdateDomainDataGrantResponse     | 更新数据对象授权   |
| [DeleteDomainDataGrant](#delete-domain-data-grant)          | DeleteDomainDataGrantRequest     | DeleteDomainDataGrantResponse     | 删除数据对象授权   |
| [QueryDomainDataGrant](#query-domain-data-grant)            | QueryDomainDataGrantRequest      | QueryDomainDataGrantResponse      | 查询数据对象授权   |

## 接口详情

{#create-domain-data-grant}

### 创建数据对象授权

#### HTTP路径
/api/v1/datamesh/domaindatagrant/create

{#create-domain-data-grant-request}

#### 请求（CreateDomainDataGrantRequest）

| 字段            | 类型                                           | 选填 | 描述                                                                                                                               |
|---------------|----------------------------------------------|----|----------------------------------------------------------------------------------------------------------------------------------|
| header        | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容 |
| domaindatagrant_id | string                                       | 可选 | 数据对象授权ID，如果不填，则会由 datamesh 自动生成，并在 response 中返回。如果填写，则会使用填写的值，请注意需满足 [DNS 子域名规则要求](https://kubernetes.io/zh-cn/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names)     |
| domaindata_id | string | 必填 | 数据对象ID  |
| grant_domain  | string | 必填 | 被授权节点ID       |
| limit         | [GrantLimit](#grant-limit-entity) | 选填 | 授权限制条件  |
| description   | map<string, string> | 可选 | 自定义描述 |

{#create-domain-data-grant-response}

#### 响应（CreateDomainDataGrantResponse）

| 字段                 | 类型                             | 选填 | 描述     |
|--------------------|--------------------------------|----|--------|
| status             | [Status](summary_cn.md#status) | 必填 | 状态信息   |
| data               | CreateDomainDataGrantResponseData   |    |        |
| data.domaindatagrant_id | string                         | 必填 | 数据对象授权ID |

{#update-domain-data-grant}

### 更新数据对象授权

#### HTTP路径
/api/v1/datamesh/domaindatagrant/update

#### 请求（UpdateDomainDataGrantRequest）

| 字段            | 类型                                     | 选填 | 描述                                                                                                                                 |
|---------------|----------------------------------------|----|------------------------------------------------------------------------------------------------------------------------------------|
| header        | [RequestHeader](summary_cn.md#请求和响应约定) | 可选 | 自定义请求内容 |
| domaindatagrant_id | string | 必填 | 数据对象授权ID |
| domaindata_id | string | 必填 | 数据对象ID  |
| grant_domain  | string | 必填 | 被授权节点ID       |
| limit         | [GrantLimit](#grant-limit-entity) | 选填 | 授权限制条件  |
| description   | map<string, string> | 可选 | 自定义描述 |

#### 响应（UpdateDomainDataGrantResponse）

| 字段     | 类型                             | 选填 | 描述   |
|--------|--------------------------------|----|------|
| status | [Status](summary_cn.md#status) | 必填 | 状态信息 |

{#delete-domain-data-grant}

### 删除数据对象授权

#### HTTP路径
/api/v1/datamesh/domaindatagrant/delete

#### 请求（DeleteDomainDataGrantRequest）

| 字段            | 类型                                           | 选填 | 描述      |
|---------------|----------------------------------------------|----|---------|
| header        | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容 |
| domaindatagrant_id | string | 必填 | 数据对象授权ID |

#### 响应（DeleteDomainDataGrantResponse）

| 字段     | 类型                             | 选填 | 描述   |
|--------|--------------------------------|----|------|
| status | [Status](summary_cn.md#status) | 必填 | 状态信息 |

{#query-domain-data-grant}

### 查询数据对象授权

#### HTTP路径
/api/v1/datamesh/domaindatagrant/query

#### 请求（QueryDomainGrantRequest）

| 字段            | 类型                                           | 选填 | 描述      |
|---------------|----------------------------------------------|----|---------|
| header        | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容 |
| domaindatagrant_id | string | 必填 | 数据对象授权ID    |

#### 响应（QueryDomainDataGrantResponse）

| 字段     | 类型                                | 选填 | 描述   |
|--------|-----------------------------------|----|------|
| status | [Status](summary_cn.md#status)    | 必填 | 状态信息 |
| data   | DomainDataGrantData | 选填   |   查询信息   |
| data.domaindatagrant_id | string | 必填 | 数据对象授权ID |
| data.domaindata_id | string | 必填 | 数据对象ID  |
| data.grant_domain  | string | 必填 | 被授权节点ID       |
| data.limit         | [GrantLimit](#grant-limit-entity) | 选填 | 授权限制条件  |
| data.description   | map<string, string> | 必填 | 自定义描述 |
| data.author   | string | 必填 | 数据授权方节点ID |
| data.signature   | string | 选填 | 数据授权信息签名（由授权方私钥签名） |

## 公共

{#grant-limit-entity}

### GrantLimit

| 字段 | 类型 | 选填 | 描述 |
|---------------|------------------------------|----|------------------------------------------------------------------------------------------------------------------------------------|
| expiration_time | int64 |选填 | 授权过期时间，Unix 时间戳，精确到纳秒  |
| use_count | int32 |选填  | 授权使用次数  |
| flow_id | string |选填  | 授权对应的任务流ID  |
| components |  string[] | 选填  | 授权可用的组件ID  |
| initiator |  string |选填  | 授权指定的发起方  |
| input_config |  string |选填  | 授权指定的算子输入参数 |
