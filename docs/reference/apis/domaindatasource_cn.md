# DomainDataSource

DomainDataSource 表示 Kuscia 管理的数据源。请参考 [DomainDataSource](../concepts/domaindatasource_cn.md)。
您可以从 [这里](https://github.com/secretflow/kuscia/tree/main/proto/api/v1alpha1/kusciaapi/domaindatasource.proto) 找到对应的 protobuf 文件。

## 接口总览

| 方法名                                                           | 请求类型                              | 响应类型                               | 描述             |
|---------------------------------------------------------------|-----------------------------------|------------------------------------|----------------|
| [CreateDomainDataSource](#create-domain-data-source)          | CreateDomainDataSourceRequest     | CreateDomainDataSourceResponse     | 创建数据源          |
| [UpdateDomainDataSource](#update-domain-data-source)          | UpdateDomainDataSourceRequest     | UpdateDomainDataSourceResponse     | 更新数据源          |
| [DeleteDomainDataSource](#delete-domain-data-source)          | DeleteDomainDataSourceRequest     | DeleteDomainDataSourceResponse     | 删除数据源          |
| [QueryDomainDataSource](#query-domain-data-source)            | QueryDomainDataSourceRequest      | QueryDomainDataSourceResponse      | 查询数据源          |
| [BatchQueryDomainDataSource](#batch-query-domain-data-source) | BatchQueryDomainDataSourceRequest | BatchQueryDomainDataSourceResponse | 批量查询数据源        |
| [ListDomainDataSource](#list-domain-data-source)              | ListDomainDataSourceRequest       | ListDomainDataSourceResponse       | 列出Domain下全部数据源 |

## 接口详情

{#create-domain-data-source}

### 创建数据源

如果创建的数据源是为了DataMesh（DataProxy）使用，需要确保数据源的配置满足一定的权限要求。详情请见 [DataMesh数据读写-注意事项](./datamesh/datacrud_cn.md#注意事项)

#### HTTP 路径

/api/v1/domaindatasource/create

{#create-domain-data-source-request}

#### 请求（CreateDomainDataSourceRequest）

| 字段              | 类型                                           | 选填 | 描述                                                                                                                                                                                     |
|-----------------|----------------------------------------------|----|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| header          | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容                                                                                                                                                                                |
| domain_id       | string                                       | 必填 | 节点ID                                                                                                                                                                                   |
| datasource_id   | string                                       | 选填 | 数据源 ID，如果不填，则会由 kusciaapi 自动生成，并在 response 中返回。如果填写，则会使用填写的值，请注意需满足 [RFC 1123 标签名规则要求](https://kubernetes.io/zh-cn/docs/concepts/overview/working-with-objects/names/#dns-label-names) |
| type            | string                                       | 必填 | 数据源类型，支持 localfs, oss, mysql, odps                                                                                                                                                     |
| name            | string                                       | 可选 | 数据源名称（无需唯一）                                                                                                                                                                            |
| info            | [DataSourceInfo](#data-source-info)          | 必填 | 数据源信息，详情见 [DataSourceInfo](#data-source-info) ，当设置 info_key 时，此字段可不填。                                                                                                                  |
| info_key        | string                                       | 选填 | info 与 info_key 字段二者填一个即可，info_key 用于从 Kuscia ConfigManager 的加密后端中获取数据源的信息。                                                                                                            |
| access_directly | bool                                         | 可选 | 隐私计算应用（如 SecretFlow ）是否可直连访问数据源的标志位，true：应用直连访问数据源（不经过 DataProxy）， false: 应用可通过 DataProxy 访问数据源。当前建设设置为 true, 使用 odps 类型时目前必须经过 DataProxy                                              |

{#create-domain-data-source-response}

#### 响应（CreateDomainDataSourceResponse）

| 字段                 | 类型                             | 描述     |
|--------------------|--------------------------------|--------|
| status             | [Status](summary_cn.md#status) | 状态信息   |
| data.datasource_id | string                         | 数据源 ID |

#### 请求示例

发起请求：

##### 创建本地文件数据源示例

```sh
# Run the sample within the container
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/domaindatasource/create' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "domain_id": "alice",
  "datasource_id":"demo-local-datasource",
  "type":"localfs",
  "name": "DemoDataSource",
  "info": {
      "localfs": {
          "path": "/home/kuscia/var/storage/data"
      }
  },
  "access_directly": true
}'
```

##### 创建对象存储服务数据源示例

```sh
# Run the sample within the container
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/domaindatasource/create' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "domain_id": "alice",
  "datasource_id":"demo-oss-datasource",
  "type":"oss",
  "name": "DemoDataSource",
  "info": {
      "oss": {
          "endpoint": "https://oss.xxx.cn-xxx.com",
          "bucket": "secretflow",
          "prefix": "kuscia/",
          "access_key_id":"ak-xxxx",
          "access_key_secret" :"sk-xxxx"
#         "virtualhost": true (Required for Alibaba Cloud OSS configuration)
      }
  },
  "access_directly": true
}'
```

##### 创建 MySQL 数据库数据源示例

```sh
# Run the sample within the container
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/domaindatasource/create' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "domain_id": "alice",
  "datasource_id":"demo-mysql-datasource",
  "type":"mysql",
  "name": "DemoDataSource",
  "info": {
      "database": {
          "endpoint": "localhost:3306",
          "user": "xxxxx",
          "password": "xxxxx",
          "database":"kuscia"
      }
  },
  "access_directly": true
}'
```

##### 创建 ODPS 数据库数据源示例

> ODPS 服务 endpoint 具体可参考[云原生大数据计算服务 MaxCompute 官网](https://help.aliyun.com/zh/maxcompute/user-guide/endpoints)提供，
> Endpoint 支持 HTTP 和 HTTPS，若需要加密请求，请使用HTTPS。

```sh
# Run the sample within the container
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/domaindatasource/create' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "domain_id": "alice",
  "datasource_id":"demo-odps-datasource",
  "type":"odps",
  "name": "DemoDataSource",
  "info": {
      "odps": {
          "endpoint": "http://service.xxx.com/api",
          "access_key_id": "xxx",
          "access_key_secret": "xxx",
          "project":"kuscia"
      }
  },
  "access_directly": false
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
    "datasource_id": "demo-local-datasource"
  }
}
```

{#update-domain-data-source}

### 更新数据源

#### HTTP 路径

/api/v1/domaindatasource/update

#### 请求（UpdateDomainDataSourceRequest）

| 字段              | 类型                                           | 选填 | 描述                                                                                                                            |
|-----------------|----------------------------------------------|----|-------------------------------------------------------------------------------------------------------------------------------|
| header          | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容                                                                                                                       |
| domain_id       | string                                       | 必填 | 节点ID                                                                                                                          |
| datasource_id   | string                                       | 必填 | 要更新的数据源 ID                                                                                                                    |
| type            | string                                       | 必填 | 数据源类型，支持 localfs, oss, mysql                                                                                                  |
| name            | string                                       | 可选 | 数据源名称（无需唯一）                                                                                                                   |
| info            | [DataSourceInfo](#data-source-info)          | 必填 | 数据源信息，详情见 [DataSourceInfo](#data-source-info) ，当设置 info_key 时，此字段可不填。                                                         |
| info_key        | string                                       | 选填 | info 与 info_key 字段二者填一个即可，info_key 用于从 Kuscia ConfigManager 的加密后端中获取数据源的信息。                                                   |
| access_directly | bool                                         | 可选 | 隐私计算应用（如 SecretFlow ）是否可直连访问数据源的标志位，true：应用直连访问数据源（不经过 DataProxy）， false: 应用可通过 DataProxy 访问数据源（DataProxy暂未支持）。当前建设设置为 true 。 |

#### 响应（UpdateDomainDataSourceResponse）

| 字段     | 类型                             | 描述   |
|--------|--------------------------------|------|
| status | [Status](summary_cn.md#status) | 状态信息 |

#### 请求示例

发起请求：

```sh
# Run the sample within the container
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/domaindatasource/update' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "domain_id": "alice",
  "datasource_id":"demo-local-datasource",
  "type":"localfs",
  "name": "DemoDataSource",
  "info": {
      "localfs": {
          "path": "/home/kuscia/var/storage/data/alice"
      }
  },
  "access_directly": true
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

{#delete-domain-data-source}

### 删除数据源

#### HTTP 路径

/api/v1/domaindatasource/delete

#### 请求（DeleteDomainDataSourceRequest）

| 字段            | 类型                                           | 选填 | 描述         |
|---------------|----------------------------------------------|----|------------|
| header        | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容    |
| domain_id     | string                                       | 必填 | 节点 ID      |
| datasource_id | string                                       | 必填 | 要删除的数据源 ID |

#### 响应（DeleteDomainDataSourceResponse）

| 字段     | 类型                             | 描述   |
|--------|--------------------------------|------|
| status | [Status](summary_cn.md#status) | 状态信息 |

#### 请求示例

发起请求：

```sh
# Run the sample within the container
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/domaindatasource/delete' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "datasource_id":"demo-local-datasource",
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
  }
}
```

{#query-domain-data-source}

### 查询数据源

#### HTTP 路径

/api/v1/domaindatasource/query

#### 请求（QueryDomainDataSourceRequest）

| 字段            | 类型                                           | 选填 | 描述      |
|---------------|----------------------------------------------|----|---------|
| header        | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容 |
| domain_id     | string                                       | 必填 | 节点 ID   |
| datasource_id | string                                       | 必填 | 数据源 ID  |

#### 响应（QueryDomainDataSourceResponse）

| 字段     | 类型                                             | 描述    |
|--------|------------------------------------------------|-------|
| status | [Status](summary_cn.md#status)                 | 状态信息  |
| data   | [DomainDataSource](#domain-data-source-entity) | 数据源信息 |

#### 请求示例

发起请求：

```sh
# Run the sample within the container
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/domaindatasource/query' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "datasource_id":"demo-local-datasource",
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
    "domain_id": "alice",
    "datasource_id": "demo-local-datasource",
    "name": "DemoDataSource",
    "type": "localfs",
    "status": "",
    "info": {
      "localfs": {
        "path": "/home/kuscia/var/storage/data/alice"
      },
      "oss": null,
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
    "code": 11802,
    "message": "get domain alice kuscia data source demo-local-datasource-1 failed, domaindatasources.kuscia.secretflow \"demo-local-datasource-1\" not found",
    "details": []
  },
  "data": null
}
```

{#batch-query-domain-data-source}

### 批量查询数据源

#### HTTP 路径

/api/v1/domaindatasource/batchQuery

#### 请求（BatchQueryDomainDataSourceRequest）

| 字段     | 类型                                                                           | 选填 | 描述      |
|--------|------------------------------------------------------------------------------|----|---------|
| header | [RequestHeader](summary_cn.md#requestheader)                                 | 可选 | 自定义请求内容 |
| data   | [QueryDomainDataSourceRequestData](#query-domain-data-source-request-data)[] | 必填 | 查询内容    |

#### 响应（BatchQueryDomainDataSourceResponse）

| 字段                   | 类型                                               | 描述      |
|----------------------|--------------------------------------------------|---------|
| status               | [Status](summary_cn.md#status)                   | 状态信息    |
| data.datasource_list | [DomainDataSource](#domain-data-source-entity)[] | 数据源信息列表 |

#### 请求示例

发起请求：

```sh
# Run the sample within the container
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/domaindatasource/batchQuery' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "data": [
    {
      "domain_id": "alice",
      "datasource_id":"demo-local-datasource"
    },
    {
      "domain_id": "alice",
      "datasource_id":"demo-oss-datasource"
    },
    {
      "domain_id": "alice",
      "datasource_id":"demo-mysql-datasource"
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
    "datasource_list": [{
      "domain_id": "alice",
      "datasource_id": "demo-local-datasource",
      "name": "DemoDataSource",
      "type": "localfs",
      "status": "",
      "info": {
        "localfs": {
          "path": "/home/kuscia/var/storage/data/alice"
        },
        "oss": null,
        "database": null
      },
      "info_key": "",
      "access_directly": true
    }, {
      "domain_id": "alice",
      "datasource_id": "demo-oss-datasource",
      "name": "DemoDataSource",
      "type": "oss",
      "status": "",
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
    }, {
      "domain_id": "alice",
      "datasource_id": "demo-mysql-datasource",
      "name": "DemoDataSource",
      "type": "mysql",
      "status": "",
      "info": {
        "localfs": null,
        "oss": null,
        "database": {
          "endpoint": "localhost:3306",
          "user": "xxxxx",
          "password": "xxxxx",
          "database": "kuscia"
        }
      },
      "info_key": "",
      "access_directly": true
    }]
  }
}
```

{#list-domain-data-source}

### 列出数据源

#### HTTP 路径

/api/v1/domaindatasource/list

#### 请求（ListDomainDataSourceRequest）

| 字段        | 类型                                           | 选填 | 描述      |
|-----------|----------------------------------------------|----|---------|
| header    | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容 |
| domain_id | string                                       | 必填 | 节点 ID   |

#### 响应（ListDomainDataSourceResponse）

| 字段                   | 类型                                               | 描述      |
|----------------------|--------------------------------------------------|---------|
| status               | [Status](summary_cn.md#status)                   | 状态信息    |
| data.datasource_list | [DomainDataSource](#domain-data-source-entity)[] | 数据源信息列表 |

#### 请求示例

发起请求：

```sh
# Run the sample within the container
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/domaindatasource/list' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
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
    "datasource_list": [{
      "domain_id": "alice",
      "datasource_id": "demo-local-datasource",
      "name": "DemoDataSource",
      "type": "localfs",
      "status": "",
      "info": {
        "localfs": {
          "path": "/home/kuscia/var/storage/data/alice"
        },
        "oss": null,
        "database": null
      },
      "info_key": "",
      "access_directly": true
    }, {
      "domain_id": "alice",
      "datasource_id": "demo-oss-datasource",
      "name": "DemoDataSource",
      "type": "oss",
      "status": "",
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
    }, {
      "domain_id": "alice",
      "datasource_id": "demo-mysql-datasource",
      "name": "DemoDataSource",
      "type": "mysql",
      "status": "",
      "info": {
        "localfs": null,
        "oss": null,
        "database": {
          "endpoint": "localhost:3306",
          "user": "xxxxx",
          "password": "xxxxx",
          "database": "kuscia"
        }
      },
      "info_key": "",
      "access_directly": true
    }, {
      "domain_id": "alice",
      "datasource_id": "demo-odps-datasource",
      "name": "DemoDataSource",
      "type": "odps",
      "status": "",
      "info": {
        "localfs": null,
        "oss": null,
        "database": null,
        "odps": {
          "endpoint": "http://service.xxx.com/api",
          "project": "kuscia",
          "access_key_id": "xxx",
          "access_key_secret": "xxx"
        }
      },
      "info_key": "",
      "access_directly": false
    }]
  }
}
```

## 公共

{#data-source-info}

### DataSourceInfo

| 字段       | 类型                                                   | 选填 | 描述         |
|----------|------------------------------------------------------|----|------------|
| localfs  | [LocalDataSourceInfo](#local-data-source-info)       | 选填 | 本地文件系统信息   |
| oss      | [OssDataSourceInfo](#oss-data-source-info)           | 选填 | 对象存储系统相关信息 |
| database | [DatabaseDataSourceInfo](#database-data-source-info) | 选填 | 数据库相关信息    |
| odps     | [OdpsDataSourceInfo](#odps-data-source-info)         | 选填 | ODPS 相关信息  |

{#local-data-source-info}

### LocalDataSourceInfo

| 字段   | 类型     | 选填 | 描述                                                                              |
|------|--------|----|---------------------------------------------------------------------------------|
| path | string | 必填 | 本地文件系统绝对路径，建议为/home/kuscia/var/storage/data/或/home/kuscia/var/storage/data/的子目录 |

{#oss-data-source-info}

### OssDataSourceInfo

| 字段                | 类型     | 选填 | 描述                                                                               |
|-------------------|--------|----|----------------------------------------------------------------------------------|
| endpoint          | string | 必填 | 对象存储系统的链接地址，如: <https://oss.xxx.cn-xxx.com> 或 <http://127.0.0.1:9000>                |
| bucket            | string | 必填 | 对象存储系统桶 bucket 名称                                                                |
| prefix            | string | 选填 | 存储系统的路径前缀，可不填，当需要通过路径前缀prefix隔离区分不同数据文件时填写，如：data/traindata/ 或 data/predictdata/ |
| access_key_id     | string | 必填 | 访问 OSS 所需的 AK                                                                    |
| access_key_secret | string | 必填 | 访问 OSS 所需的 SK                                                                    |
| virtualhost       | bool   | 选填 | 若为阿里云 OSS 需要设置 virtualhost=true，[详见文档](https://help.aliyun.com/zh/oss/developer-reference/overview-24?scm=20140722.S_help%40%40%E6%96%87%E6%A1%A3%40%40375247.S_RQW%40ag0%2BBB2%40ag0%2BBB1%40ag0%2Bos0.ID_375247-RL_virtualhost-LOC_doc%7EUND%7Eab-OR_ser-V_4-P0_1&spm=a2c4g.11186623.0.i3) 。                                           |
| version           | string | 选填 | AWS S3 协议版本号，可不填                                                                 |

{#database-data-source-info}

### DatabaseDataSourceInfo

| 字段       | 类型     | 选填 | 描述                       |
|----------|--------|----|--------------------------|
| endpoint | string | 必填 | 数据库的地址，如: localhost:3306 |
| user     | string | 必填 | 数据库用户名                   |
| password | string | 必填 | 数据库密码                    |
| database | string | 必填 | 数据库名称                    |

{#odps-data-source-info}

### OdpsDataSourceInfo

| 字段                | 类型     | 选填 | 描述                                 |
|-------------------|--------|----|------------------------------------|
| endpoint          | string | 必填 | 服务地址，如: <http://service.xxx.com/api> |
| access_key_id     | string | 必填 | 访问 ODPS 所需的 AK                     |
| access_key_secret | string | 必填 | 访问 ODPS 所需的 SK                     |
| project           | string | 必填 | 访问 ODPS 的项目                        |

{#query-domain-data-source-request-data}

### QueryDomainDataSourceRequestData

| 字段            | 类型     | 选填 | 描述     |
|---------------|--------|----|--------|
| domain_id     | string | 必填 | 节点 ID  |
| datasource_id | string | 必填 | 数据源 ID |

{#domain-data-source-entity}

### DomainDataSource

| 字段              | 类型                                  | 描述                                                                                                                                        |
|-----------------|-------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------|
| domain_id       | string                              | 节点 ID                                                                                                                                     |
| datasource_id   | string                              | 数据源唯一标识                                                                                                                                   |
| name            | string                              | 数据源名称                                                                                                                                     |
| type            | string                              | 数据源类型，支持 localfs, oss, mysql, odps                                                                                                        |
| status          | string                              | 据源的状态，暂未支持校验数据源的状态，现为空字符串                                                                                                                 |
| info            | [DataSourceInfo](#data-source-info) | 数据源信息，详情见 [DataSourceInfo](#data-source-info) ，当设置 info_key 时，此字段可不填。                                                                     |
| info_key        | string                              | info 与 info_key 字段二者填一个即可，info_key 用于从 Kuscia ConfigManager 的加密后端中获取数据源的信息。                                                               |
| access_directly | bool                                | 隐私计算应用（如 SecretFlow ）是否可直连访问数据源的标志位，true：应用直连访问数据源（不经过 DataProxy）， false: 应用可通过 DataProxy 访问数据源。当前建设设置为 true, 使用 odps 类型时目前必须经过 DataProxy |
