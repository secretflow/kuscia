# DomainData

DomainData 表示被 Kuscia 管理的数据，Data Mesh API 提供了从 Domain 侧的管理 DomainData
的能力。请参考 [DomainData](../../concepts/domaindata_cn.md)。
你可以从 [这里](https://github.com/secretflow/kuscia/blob/main/proto/api/v1alpha1/datamesh/domaindata.proto) 找到对应的 protobuf 文件。

## 接口总览

| 方法名                                              | 请求类型                        | 响应类型                         | 描述       |
|--------------------------------------------------|-----------------------------|------------------------------|----------|
| [CreateDomainData](#create-domain-data)          | CreateDomainDataRequest     | CreateDomainDataResponse     | 创建数据对象   |
| [UpdateDomainData](#update-domain-data)          | UpdateDomainDataRequest     | UpdateDomainDataResponse     | 更新数据对象   |
| [QueryDomainData](#query-domain-data)            | QueryDomainDataRequest      | QueryDomainDataResponse      | 查询数据对象   |

## 接口详情

{#create-domain-data}

### 创建数据对象

#### HTTP路径
/api/v1/datamesh/domaindata/create

{#create-domain-data-request}

#### 请求（CreateDomainDataRequest）

| 字段            | 类型                                           | 选填 | 描述                                                                                                                               |
|---------------|----------------------------------------------|----|----------------------------------------------------------------------------------------------------------------------------------|
| header        | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容                                                                                                                          |
| domaindata_id | string                                       | 可选 | 数据对象ID，满足 [DNS 子域名规则要求](https://kubernetes.io/zh-cn/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names)      |
| name          | string                                       | 必填 | 名称                                                                                                                               |
| type          | string                                       | 必填 | 类型，\[table,model,rule,report,unknown]，大小写敏感                                                                                      |
| relative_uri  | string                                       | 必填 | 相对数据源所在位置的路径，参考 [DomainData概念](../../concepts/domaindata_cn.md)                                                                  |
| datasource_id | string                                       | 可选 | 数据源ID，不填写则使用默认数据源，参考 [DomainData概念](../../concepts/domaindata_cn.md)                                                             |
| attributes    | map<string,string>                           | 可选 | 自定义属性，用作用户或应用算法组件为数据对象添加扩展信息，参考 [DomainData概念](../../concepts/domaindata_cn.md)                                                  |
| partition     | [Partition](#partition)                      | 可选 | 暂不支持                                                                                                                             |
| columns       | [DataColumn](#data-column) array             | 必填 | 列信息                                                                                                                              |
| vendor        | string                                       | 可选 | 来源，用于查询接口筛选数据对象，参考 [ListDomainDataRequestData](#list-domain-data-request-data) 和 [DomainData概念](../../concepts/domaindata_cn.md) |

{#create-domain-data-response}

#### 响应（CreateDomainDataResponse）

| 字段                 | 类型                             | 选填 | 描述     |
|--------------------|--------------------------------|----|--------|
| status             | [Status](summary_cn.md#status) | 必填 | 状态信息   |
| data               | CreateDomainDataResponseData   |    |        |
| data.domaindata_id | string                         | 必填 | 数据对象ID |

#### 请求示例
发起请求：
```bash
# 在容器内执行示例
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl https://127.0.0.1:8070/api/v1/datamesh/domaindata/create \
-X POST -H 'content-type: application/json' \
--cacert ${CTR_CERTS_ROOT}/ca.crt \
--cert ${CTR_CERTS_ROOT}/ca.crt \
--key ${CTR_CERTS_ROOT}/ca.key \
-d '{
 "domain_id": "alice",
 "domaindata_id": "alice-001",
 "datasource_id": "demo-oss-datasource",
 "name": "alice001",
 "type": "table",
 "relative_uri": "alice.csv",
 "columns": [
   {
     "name": "id1",
     "type": "str",
     "comment": ""
   },
   {
     "name": "marital_single",
     "type": "float",
     "comment": ""
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
    "domaindata_id": "alice-001"
  }
}
```

{#update-domain-data}

### 更新数据对象

#### HTTP路径
/api/v1/datamesh/domaindata/update

#### 请求（UpdateDomainDataRequest）

| 字段            | 类型                                     | 选填 | 描述                                                                                                                                 |
|---------------|----------------------------------------|----|------------------------------------------------------------------------------------------------------------------------------------|
| header        | [RequestHeader](summary_cn.md#请求和响应约定) | 可选 | 自定义请求内容                                                                                                                            |
| domaindata_id | string                                 | 必填 | 数据对象ID                                                                                                                             |
| name          | string                                 | 必填 | 名称                                                                                                                                 |
| type          | string                                 | 必填 | 类型，\[table,model,rule,report,unknown]，大小写敏感                                                                                        |
| relative_uri  | string                                 | 必填 | 相对数据源所在位置的路径，参考 [DomainData概念](../../concepts/domaindata_cn.md)                                                                    |
| datasource_id | string                                 | 可选 | 数据源ID，不填写则使用默认数据源，参考 [DomainData概念](../../concepts/domaindata_cn.md)                                                               |
| attributes    | map<string,string>                     | 可选 | 自定义属性，用作用户或应用算法组件为数据对象添加扩展信息，参考 [DomainData概念](../../concepts/domaindata_cn.md)                                                    |
| partition     | [Partition](#partition)                | 可选 | 暂不支持                                                                                                                               |
| columns       | [DataColumn](#data-column)[]           | 必填 | 列信息                                                                                                                                |
| vendor        | string                                 | 可选 | 来源，用于批量查询接口筛选数据对象，参考 [ListDomainDataRequestData](#list-domain-data-request-data) 和 [DomainData概念](../../concepts/domaindata_cn.md) |

#### 响应（UpdateDomainDataResponse）

| 字段     | 类型                             | 选填 | 描述   |
|--------|--------------------------------|----|------|
| status | [Status](summary_cn.md#status) | 必填 | 状态信息 |

#### 请求示例
发起请求：
```bash
# 在容器内执行示例
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl https://127.0.0.1:8070/api/v1/datamesh/domaindata/update \
-X POST -H 'content-type: application/json' \
--cacert ${CTR_CERTS_ROOT}/ca.crt \
--cert ${CTR_CERTS_ROOT}/ca.crt \
--key ${CTR_CERTS_ROOT}/ca.key \
 -d '{
  "domain_id": "alice",
  "domaindata_id": "alice-001",
  "datasource_id": "demo-oss-datasource",
  "name": "alice0010",
  "type": "table",
  "relative_uri": "alice.csv",
  "columns": [
    {
      "name": "id1",
      "type": "str",
      "comment": ""
    },
    {
      "name": "marital_single",
      "type": "float",
      "comment": ""
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
  }
}
```

### 查询数据对象

#### HTTP路径
/api/v1/datamesh/domaindata/query

#### 请求（QueryDomainDataRequest）

| 字段            | 类型                                           | 选填 | 描述      |
|---------------|----------------------------------------------|----|---------|
| header        | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容 |
| domaindata_id | string                                       | 必填 | 查询内容    |

#### 响应（QueryDomainDataResponse）

| 字段     | 类型                                | 选填 | 描述   |
|--------|-----------------------------------|----|------|
| status | [Status](summary_cn.md#status)    | 必填 | 状态信息 |
| data   | [DomainData](#domain-data-entity) |    |      |

#### 请求示例
发起请求：
```bash
# 在容器内执行示例
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl https://127.0.0.1:8070/api/v1/datamesh/domaindata/query \
-X POST -H 'content-type: application/json' \
--cacert ${CTR_CERTS_ROOT}/ca.crt \
--cert ${CTR_CERTS_ROOT}/ca.crt \
--key ${CTR_CERTS_ROOT}/ca.key \
 -d '{
  "domaindata_id": "alice-001"
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
    "domaindata_id": "alice-001",
    "name": "alice0010",
    "type": "table",
    "relative_uri": "alice.csv",
    "datasource_id": "demo-oss-datasource",
    "attributes": {},
    "partition": null,
    "columns": [
      {
        "name": "id1",
        "type": "str",
        "comment": "",
        "not_nullable": false
      },
      {
        "name": "marital_single",
        "type": "float",
        "comment": "",
        "not_nullable": false
      }
    ],
    "vendor": "manual",
    "file_format": "UNKNOWN",
    "author": "alice"
  }
}
```

## 公共

{#list-domain-data-request-data}

##### ListDomainDataRequestData

| 字段                | 类型     | 选填 | 描述   |
|-------------------|--------|----|------|
| domain_id         | string | 必填 | 节点ID |
| domaindata_type   | string | 可选 | 类型   |
| domaindata_vendor | string | 可选 | 来源   |

{#domain-data-entity}

### DomainData

| 字段            | 类型                           | 选填 | 描述                                                                                                                                 |
|---------------|------------------------------|----|------------------------------------------------------------------------------------------------------------------------------------|
| domaindata_id | string                       | 必填 | 数据对象ID                                                                                                                             |
| name          | string                       | 必填 | 名称                                                                                                                                 |
| type          | string                       | 必填 | 类型，\[table,model,rule,report,unknown]，大小写敏感                                                                                        |
| relative_uri  | string                       | 必填 | 相对数据源所在位置的路径，参考 [DomainData概念](../../concepts/domaindata_cn.md)                                                                    |
| domain_id     | string                       | 必填 | 节点ID                                                                                                                               |
| datasource_id | string                       | 必填 | 数据源ID，参考 [DomainData概念](../../concepts/domaindata_cn.md)                                                                           |
| attributes    | map<string,string>           | 可选 | 自定义属性，用作用户或应用算法组件为数据对象添加扩展信息，参考 [DomainData概念](../../concepts/domaindata_cn.md)                                                    |
| partition     | [Partition](#partition)      | 可选 | 暂不支持                                                                                                                               |
| columns       | [DataColumn](#data-column)[] | 必填 | 列信息                                                                                                                                |
| vendor        | string                       | 可选 | 来源，用于批量查询接口筛选数据对象，参考 [ListDomainDataRequestData](#list-domain-data-request-data) 和 [DomainData概念](../../concepts/domaindata_cn.md) |


{#partition}

### Partition

| 字段     | 类型                           | 选填 | 描述  |
|--------|------------------------------|----|-----|
| type   | string                       | 必填 | 类型  |
| fields | [DataColumn](#data-column)[] | 必填 | 列信息 |


{#data-column}

### DataColumn

| 字段      | 类型     | 选填 | 描述                                                                     |
|---------|--------|----|------------------------------------------------------------------------|
| name    | string | 必填 | 列名称                                                                    |
| type    | string | 必填 | 类型，当前版本由应用算法组件定义和消费，参考 [DomainData概念](../../concepts/domaindata_cn.md) |
| comment | string | 可选 | 列注释                                                                    |