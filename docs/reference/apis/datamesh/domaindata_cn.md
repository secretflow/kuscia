# DomainData

DomainData 表示被 Kuscia 管理的数据，Data Mesh API 提供了从 Domain 侧的管理 DomainData
的能力。请参考 [DomainData](../../concepts/domaindata_cn.md)。
你可以从 [这里](https://github.com/secretflow/kuscia/blob/main/proto/api/v1alpha1/datamesh/domaindata.proto) 找到对应的
protobuf 文件。

## 接口总览

| 方法名                                              | 请求类型                        | 响应类型                         | 描述       |
|--------------------------------------------------|-----------------------------|------------------------------|----------|
| [CreateDomainData](#create-domain-data)          | CreateDomainDataRequest     | CreateDomainDataResponse     | 创建数据对象   |
| [UpdateDomainData](#update-domain-data)          | UpdateDomainDataRequest     | UpdateDomainDataResponse     | 更新数据对象   |
| [DeleteDomainData](#delete-domain-data)          | DeleteDomainDataRequest     | DeleteDomainDataResponse     | 删除数据对象   |
| [QueryDomainData](#query-domain-data)            | QueryDomainDataRequest      | QueryDomainDataResponse      | 查询数据对象   |

## 接口详情

{#create-domain-data}

### 创建数据对象

#### HTTP路径
/api/v1/datamesh/domaindata/create

{#create-domain-data-request}

#### 请求（CreateDomainDataRequest）

| 字段            | 类型                                            | 可选 | 描述                                                                                                                               |
|---------------|-----------------------------------------------|----|----------------------------------------------------------------------------------------------------------------------------------|
| header        | [RequestHeader](summary_cn.md#request-header) | 是  | 自定义请求内容                                                                                                                          |
| domaindata_id | string                                        | 是  | 数据对象ID，满足 [DNS 子域名规则要求](https://kubernetes.io/zh-cn/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names)      |
| name          | string                                        | 否  | 名称                                                                                                                               |
| type          | string                                        | 否  | 类型，\[table,model,rule,report,unknown]，大小写敏感                                                                                      |
| relative_uri  | string                                        | 否  | 相对数据源所在位置的路径，参考 [DomainData概念](../../concepts/domaindata_cn.md)                                                                  |
| datasource_id | string                                        | 是  | 数据源ID，不填写则使用默认数据源，参考 [DomainData概念](../../concepts/domaindata_cn.md)                                                             |
| attributes    | map<string,string>                            | 是  | 自定义属性，用作用户或应用算法组件为数据对象添加扩展信息，参考 [DomainData概念](../../concepts/domaindata_cn.md)                                                  |
| partition     | [Partition](#partition)                       | 是  | 暂不支持                                                                                                                             |
| columns       | [DataColumn](#data-column) array              | 否  | 列信息                                                                                                                              |
| vendor        | string                                        | 是  | 来源，用于查询接口筛选数据对象，参考 [ListDomainDataRequestData](#list-domain-data-request-data) 和 [DomainData概念](../../concepts/domaindata_cn.md) |

{#create-domain-data-response}

#### 响应（CreateDomainDataResponse）

| 字段                 | 类型                             | 可选 | 描述     |
|--------------------|--------------------------------|----|--------|
| status             | [Status](summary_cn.md#status) | 否  | 状态信息   |
| data               | CreateDomainDataResponseData   |    |        |
| data.domaindata_id | string                         | 否  | 数据对象ID |

{#update-domain-data}

### 更新数据对象

#### HTTP路径
/api/v1/datamesh/domaindata/update

#### 请求（UpdateDomainDataRequest）

| 字段            | 类型                                            | 可选 | 描述                                                                                                                                 |
|---------------|-----------------------------------------------|----|------------------------------------------------------------------------------------------------------------------------------------|
| header        | [RequestHeader](summary_cn.md#request-header) | 是  | 自定义请求内容                                                                                                                            |
| domaindata_id | string                                        | 否  | 数据对象ID                                                                                                                             |
| name          | string                                        | 否  | 名称                                                                                                                                 |
| type          | string                                        | 否  | 类型，\[table,model,rule,report,unknown]，大小写敏感                                                                                        |
| relative_uri  | string                                        | 否  | 相对数据源所在位置的路径，参考 [DomainData概念](../../concepts/domaindata_cn.md)                                                                    |
| datasource_id | string                                        | 是  | 数据源ID，不填写则使用默认数据源，参考 [DomainData概念](../../concepts/domaindata_cn.md)                                                               |
| attributes    | map<string,string>                            | 是  | 自定义属性，用作用户或应用算法组件为数据对象添加扩展信息，参考 [DomainData概念](../../concepts/domaindata_cn.md)                                                    |
| partition     | [Partition](#partition)                       | 是  | 暂不支持                                                                                                                               |
| columns       | [DataColumn](#data-column)[]                  | 否  | 列信息                                                                                                                                |
| vendor        | string                                        | 是  | 来源，用于批量查询接口筛选数据对象，参考 [ListDomainDataRequestData](#list-domain-data-request-data) 和 [DomainData概念](../../concepts/domaindata_cn.md) |

#### 响应（UpdateDomainDataResponse）

| 字段     | 类型                             | 可选 | 描述   |
|--------|--------------------------------|----|------|
| status | [Status](summary_cn.md#status) | 否  | 状态信息 |

{#delete-domain-data}

### 删除数据对象

#### HTTP路径
/api/v1/datamesh/domaindata/delete

#### 请求（DeleteDomainDataRequest）

| 字段            | 类型                                            | 可选 | 描述      |
|---------------|-----------------------------------------------|----|---------|
| header        | [RequestHeader](summary_cn.md#request-header) | 是  | 自定义请求内容 |
| domaindata_id | string                                        | 否  | 数据对象ID  |

#### 响应（DeleteDomainDataResponse）

| 字段     | 类型                             | 可选 | 描述   |
|--------|--------------------------------|----|------|
| status | [Status](summary_cn.md#status) | 否  | 状态信息 |

{#query-domain-data}

### 查询数据对象

#### HTTP路径
/api/v1/datamesh/domaindata/query

#### 请求（QueryDomainRequest）

| 字段            | 类型                                            | 可选 | 描述      |
|---------------|-----------------------------------------------|----|---------|
| header        | [RequestHeader](summary_cn.md#request-header) | 是  | 自定义请求内容 |
| domaindata_id | string                                        | 否  | 查询内容    |

#### 响应（QueryDomainResponse）

| 字段     | 类型                                | 可选 | 描述   |
|--------|-----------------------------------|----|------|
| status | [Status](summary_cn.md#status)    | 否  | 状态信息 |
| data   | [DomainData](#domain-data-entity) |    |      |

## 公共

{#list-domain-data-request-data}

##### ListDomainDataRequestData

| 字段                | 类型     | 可选 | 描述   |
|-------------------|--------|----|------|
| domain_id         | string | 否  | 节点ID |
| domaindata_type   | string | 是  | 类型   |
| domaindata_vendor | string | 是  | 来源   |

{#domain-data-entity}

### DomainData

| 字段            | 类型                           | 可选 | 描述                                                                                                                                 |
|---------------|------------------------------|----|------------------------------------------------------------------------------------------------------------------------------------|
| domaindata_id | string                       | 否  | 数据对象ID                                                                                                                             |
| name          | string                       | 否  | 名称                                                                                                                                 |
| type          | string                       | 否  | 类型，\[table,model,rule,report,unknown]，大小写敏感                                                                                        |
| relative_uri  | string                       | 否  | 相对数据源所在位置的路径，参考 [DomainData概念](../../concepts/domaindata_cn.md)                                                                    |
| domain_id     | string                       | 否  | 节点ID                                                                                                                               |
| datasource_id | string                       | 否  | 数据源ID，参考 [DomainData概念](../../concepts/domaindata_cn.md)                                                                           |
| attributes    | map<string,string>           | 是  | 自定义属性，用作用户或应用算法组件为数据对象添加扩展信息，参考 [DomainData概念](../../concepts/domaindata_cn.md)                                                    |
| partition     | [Partition](#partition)      | 是  | 暂不支持                                                                                                                               |
| columns       | [DataColumn](#data-column)[] | 否  | 列信息                                                                                                                                |
| vendor        | string                       | 是  | 来源，用于批量查询接口筛选数据对象，参考 [ListDomainDataRequestData](#list-domain-data-request-data) 和 [DomainData概念](../../concepts/domaindata_cn.md) |


{#partition}

### Partition

| 字段     | 类型                           | 可选 | 描述  |
|--------|------------------------------|----|-----|
| type   | string                       | 否  | 类型  |
| fields | [DataColumn](#data-column)[] | 否  | 列信息 |


{#data-column}

### DataColumn

| 字段      | 类型     | 可选 | 描述                                                                     |
|---------|--------|----|------------------------------------------------------------------------|
| name    | string | 否  | 列名称                                                                    |
| type    | string | 否  | 类型，当前版本由应用算法组件定义和消费，参考 [DomainData概念](../../concepts/domaindata_cn.md) |
| comment | string | 是  | 列注释                                                                    |