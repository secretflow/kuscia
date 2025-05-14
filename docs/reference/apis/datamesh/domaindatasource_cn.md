# DomainDataSource

DomainDataSource 表示 Kuscia 管理的数据源。请参考 [DomainDataSource](../concepts/domaindatasource_cn.md)。
您可以从 [这里](https://github.com/secretflow/kuscia/tree/main/proto/api/v1alpha1/kusciaapi/domaindatasource.proto) 找到对应的 protobuf 文件。

## 接口总览

| 方法名                                              | 请求类型                        | 响应类型                         | 描述 |
|--------------------------------------------------|-----------------------------|------------------------------|----------|
| [QueryDomainDataSource](#query-domain-data-source) | QueryDomainDataSourceRequest | QueryDomainDataSourceResponse      | 查询数据 |

## 接口详情

{#query-domain-data-source}

### 查询数据源

#### HTTP 路径

/api/v1/domaindatasource/query

#### 请求（QueryDomainGrantRequest）

| 字段     | 类型                                                            | 选填 | 描述      |
|--------|---------------------------------------------------------------|-----|--------------|
| header                | [RequestHeader](summary_cn.md#requestheader)   | 可选 | 自定义请求内容 |
| domain_id             | string                                         | 必填 | 节点 ID   |
| datasource_id    | string                                         | 必填 | 数据源 ID |

#### 响应（QueryDomainGrantResponse）

| 字段     | 类型                                | 选填 | 描述   |
|--------|--------------------------------------|----|------|
| status | [Status](summary_cn.md#status)               | 必填 | 状态信息 |
| data   | [DomainDataSource](#domain-data-source-entity) |  可选  |   数据源信息    |

#### 请求示例

发起请求：

```sh
# Execute example in container
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl https://127.0.0.1:8070/api/v1/datamesh/domaindatasource/query \
-X POST -H 'content-type: application/json' \
--cacert ${CTR_CERTS_ROOT}/ca.crt \
--cert ${CTR_CERTS_ROOT}/ca.crt \
--key ${CTR_CERTS_ROOT}/ca.key \
 -d '{
  "datasource_id":"demo-oss-datasource",
  "domain_id": "alice"
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
    "datasource_id": "demo-oss-datasource",
    "name": "DemoDataSource",
    "type": "oss",
    "status": "Available",
    "info": {
      "localfs": null,
      "oss": {
        "endpoint": "https://oss.xxx.cn-xxx.com",
        "bucket": "secretflow",
        "prefix": "kuscia/",
        "access_key_id": "ak-xxxx",
        "access_key_secret": "sk-xxxx",
        "virtualhost": true,
        "version": ""
      },
      "database": null
    },
    "info_key": "",
    "access_directly": true
  }
}
```

请求响应异常结果：假设传入`datasource_id`不存在

```json
{
  "status": {
    "code": 12302,
    "message": "domaindatasources.kuscia.secretflow \"demo-oss-datasource\" not found",
    "details": []
  },
  "data": null
}
```
