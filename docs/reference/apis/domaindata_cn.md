# DomainData

DomainData 表示被 Kuscia 管理的数据。请参考 [DomainData](../concepts/domaindata_cn.md)。
你可以从 [这里](https://github.com/secretflow/kuscia/tree/main/proto/api/v1alpha1/kusciaapi/domaindata.proto) 找到对应的 protobuf 文件。

Data Mesh API 提供了从 Domain 侧的管理 DomainData 的能力，详细 API 请参考 [Data Mesh](./datamesh/domaindata_cn.md)。

## 接口总览

| 方法名                                              | 请求类型                        | 响应类型                         | 描述       |
|--------------------------------------------------|-----------------------------|------------------------------|----------|
| [CreateDomainData](#create-domain-data)          | CreateDomainDataRequest     | CreateDomainDataResponse     | 创建数据对象   |
| [UpdateDomainData](#update-domain-data)          | UpdateDomainDataRequest     | UpdateDomainDataResponse     | 更新数据对象   |
| [DeleteDomainData](#delete-domain-data)          | DeleteDomainDataRequest     | DeleteDomainDataResponse     | 删除数据对象   |
| [QueryDomainData](#query-domain-data)            | QueryDomainDataRequest      | QueryDomainDataResponse      | 查询数据对象   |
| [BatchQueryDomainData](#batch-query-domain-data) | BatchQueryDomainDataRequest | BatchQueryDomainDataResponse | 批量查询数据对象 |
| [ListDomainData](#list-domain-data)              | ListDomainDataRequest       | ListDomainDataResponse       | 列出数据对象   |

## 接口详情

{#create-domain-data}

### 创建数据对象

#### HTTP 路径

/api/v1/domaindata/create

{#create-domain-data-request}

#### 请求（CreateDomainDataRequest）

| 字段            | 类型                                           | 选填 | 描述                                                                                                                             |
|---------------|----------------------------------------------|----|--------------------------------------------------------------------------------------------------------------------------------|
| header        | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容                                                                                                                        |
| domaindata_id | string                                       | 可选 | 数据对象 ID，满足 [DNS 子域名规则要求](https://kubernetes.io/zh-cn/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names)   |
| name          | string                                       | 必填 | 名称                                                                                                                             |
| type          | string                                       | 必填 | 类型，\[table,model,rule,report,unknown]，大小写敏感                                                                                    |
| relative_uri  | string                                       | 必填 | 相对数据源所在位置的路径，参考 [DomainData 概念](../concepts/domaindata_cn.md)                                                                  |
| domain_id     | string                                       | 必填 | 节点 ID                                                                                                                          |
| datasource_id | string                                       | 可选 | 数据源 ID，不填写则使用默认数据源，参考 [DomainData 概念](../concepts/domaindata_cn.md)                                                            |
| attributes    | map<string,string>                           | 可选 | 自定义属性，用作用户或应用算法组件为数据对象添加扩展信息，参考 [DomainData 概念](../concepts/domaindata_cn.md)                                                  |
| partition     | [Partition](#partition)                      | 可选 | 暂不支持                                                                                                                           |
| columns       | [DataColumn](#data-column) array             | 必填 | 列信息                                                                                                                            |
| vendor        | string                                       | 可选 | 来源，用于查询接口筛选数据对象，参考 [ListDomainDataRequestData](#list-domain-data-request-data) 和 [DomainData 概念](../concepts/domaindata_cn.md) |

{#create-domain-data-response}

#### 响应（CreateDomainDataResponse）

| 字段                 | 类型                             | 选填 | 描述      |
|--------------------|--------------------------------|----|---------|
| status             | [Status](summary_cn.md#status) | 必填 | 状态信息    |
| data               | CreateDomainDataResponseData   |    |         |
| data.domaindata_id | string                         | 必填 | 数据对象 ID |

{#update-domain-data}

### 更新数据对象

#### HTTP 路径

/api/v1/domaindata/update

#### 请求（UpdateDomainDataRequest）

| 字段            | 类型                                           | 选填 | 描述                                                                                                                               |
|---------------|----------------------------------------------|----|----------------------------------------------------------------------------------------------------------------------------------|
| header        | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容                                                                                                                          |
| domaindata_id | string                                       | 必填 | 数据对象 ID                                                                                                                          |
| name          | string                                       | 必填 | 名称                                                                                                                               |
| type          | string                                       | 必填 | 类型，\[table,model,rule,report,unknown]，大小写敏感                                                                                      |
| relative_uri  | string                                       | 必填 | 相对数据源所在位置的路径，参考 [DomainData 概念](../concepts/domaindata_cn.md)                                                                    |
| domain_id     | string                                       | 必填 | 节点 ID                                                                                                                            |
| datasource_id | string                                       | 可选 | 数据源 ID，不填写则使用默认数据源，参考 [DomainData 概念](../concepts/domaindata_cn.md)                                                              |
| attributes    | map<string,string>                           | 可选 | 自定义属性，用作用户或应用算法组件为数据对象添加扩展信息，参考 [DomainData 概念](../concepts/domaindata_cn.md)                                                    |
| partition     | [Partition](#partition)                      | 可选 | 暂不支持                                                                                                                             |
| columns       | [DataColumn](#data-column)[]                 | 必填 | 列信息                                                                                                                              |
| vendor        | string                                       | 可选 | 来源，用于批量查询接口筛选数据对象，参考 [ListDomainDataRequestData](#list-domain-data-request-data) 和 [DomainData 概念](../concepts/domaindata_cn.md) |

#### 响应（UpdateDomainDataResponse）

| 字段     | 类型                             | 选填 | 描述   |
|--------|--------------------------------|----|------|
| status | [Status](summary_cn.md#status) | 必填 | 状态信息 |

{#delete-domain-data}

### 删除数据对象

#### HTTP 路径

/api/v1/domaindata/delete

#### 请求（DeleteDomainDataRequest）

| 字段            | 类型                                           | 选填 | 描述      |
|---------------|----------------------------------------------|----|---------|
| header        | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容 |
| domain_id     | string                                       | 必填 | 节点 ID   |
| domaindata_id | string                                       | 必填 | 数据对象 ID |

#### 响应（DeleteDomainDataResponse）

| 字段     | 类型                             | 选填 | 描述   |
|--------|--------------------------------|----|------|
| status | [Status](summary_cn.md#status) | 必填 | 状态信息 |

{#query-domain-data}

### 查询数据对象

#### HTTP 路径

/api/v1/domaindata/query

#### 请求（QueryDomainRequest）

| 字段     | 类型                                                            | 选填 | 描述      |
|--------|---------------------------------------------------------------|----|---------|
| header | [RequestHeader](summary_cn.md#requestheader)                  | 可选 | 自定义请求内容 |
| data   | [QueryDomainDataRequestData](#query-domain-data-request-data) | 必填 | 查询内容    |

#### 响应（QueryDomainResponse）

| 字段     | 类型                                | 选填 | 描述   |
|--------|-----------------------------------|----|------|
| status | [Status](summary_cn.md#status)    | 必填 | 状态信息 |
| data   | [DomainData](#domain-data-entity) |    |      |

{#batch-query-domain-data}

### 批量查询数据对象

#### HTTP 路径

/api/v1/domaindata/batchQuery

#### 请求（BatchQueryDomainDataRequest）

| 字段     | 类型                                                              | 选填 | 描述      |
|--------|-----------------------------------------------------------------|----|---------|
| header | [RequestHeader](summary_cn.md#requestheader)                    | 可选 | 自定义请求内容 |
| data   | [QueryDomainDataRequestData](#query-domain-data-request-data)[] | 必填 | 查询内容    |

#### 响应（BatchQueryDomainDataResponse）

| 字段     | 类型                                  | 选填 | 描述   |
|--------|-------------------------------------|----|------|
| status | [Status](summary_cn.md#status)      | 必填 | 状态信息 |
| data   | [DomainDataList](#domain-data-list) |    |      |

{#list-domain-data}

### 列出数据对象

#### HTTP 路径

/api/v1/domaindata/list

#### 请求（ListDomainDataRequest）

| 字段     | 类型                                                          | 选填 | 描述      |
|--------|-------------------------------------------------------------|----|---------|
| header | [RequestHeader](summary_cn.md#requestheader)                | 可选 | 自定义请求内容 |
| data   | [ListDomainDataRequestData](#list-domain-data-request-data) | 必填 | 查询请求    |

#### 响应（ListDomainDataResponse）

| 字段     | 类型                                  | 选填 | 描述   |
|--------|-------------------------------------|----|------|
| status | [Status](summary_cn.md#status)      | 必填 | 状态信息 |
| data   | [DomainDataList](#domain-data-list) |    |      |

## 公共

{#query-domain-data-request-data}

### QueryDomainDataRequestData

| 字段            | 类型     | 选填 | 描述      |
|---------------|--------|----|---------|
| domain_id     | string | 必填 | 节点 ID   |
| domaindata_id | string | 必填 | 数据对象 ID |

{#list-domain-data-request-data}

##### ListDomainDataRequestData

| 字段                | 类型     | 选填 | 描述    |
|-------------------|--------|----|-------|
| domain_id         | string | 必填 | 节点 ID |
| domaindata_type   | string | 可选 | 类型    |
| domaindata_vendor | string | 可选 | 来源    |

{#domain-data-list}

### DomainDataList

| 字段              | 类型                                  | 选填 | 描述     |
|-----------------|-------------------------------------|----|--------|
| domaindata_list | [DomainData](#domain-data-entity)[] | 必填 | 数据对象列表 |

{#domain-data-entity}

### DomainData

| 字段            | 类型                           | 选填 | 描述                                                                                                                               |
|---------------|------------------------------|----|----------------------------------------------------------------------------------------------------------------------------------|
| domaindata_id | string                       | 必填 | 数据对象 ID                                                                                                                          |
| name          | string                       | 必填 | 名称                                                                                                                               |
| type          | string                       | 必填 | 类型，\[table,model,rule,report,unknown]，大小写敏感                                                                                      |
| relative_uri  | string                       | 必填 | 相对数据源所在位置的路径，参考 [DomainData 概念](../concepts/domaindata_cn.md)                                                                    |
| domain_id     | string                       | 必填 | 节点 ID                                                                                                                            |
| datasource_id | string                       | 必填 | 数据源 ID，参考 [DomainData 概念](../concepts/domaindata_cn.md)                                                                          |
| attributes    | map<string,string>           | 可选 | 自定义属性，用作用户或应用算法组件为数据对象添加扩展信息，参考 [DomainData 概念](../concepts/domaindata_cn.md)                                                    |
| partition     | [Partition](#partition)      | 可选 | 暂不支持                                                                                                                             |
| columns       | [DataColumn](#data-column)[] | 必填 | 列信息                                                                                                                              |
| vendor        | string                       | 可选 | 来源，用于批量查询接口筛选数据对象，参考 [ListDomainDataRequestData](#list-domain-data-request-data) 和 [DomainData 概念](../concepts/domaindata_cn.md) |
| author        | string                       | 可选 | 表示 DomainData 的所属者的节点 ID ，用来标识这个 DomainData 是由哪个节点创建的 |

{#partition}

### Partition

| 字段     | 类型                           | 选填 | 描述  |
|--------|------------------------------|----|-----|
| type   | string                       | 必填 | 类型  |
| fields | [DataColumn](#data-column)[] | 必填 | 列信息 |

{#data-column}

### DataColumn

| 字段      | 类型     | 选填 | 描述                                                                   |
|---------|--------|----|----------------------------------------------------------------------|
| name    | string | 必填 | 列名称                                                                  |
| type    | string | 必填 | 类型，当前版本由应用算法组件定义和消费，参考 [DomainData 概念](../concepts/domaindata_cn.md) |
| comment | string | 可选 | 列注释                                                                  |
