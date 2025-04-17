# Config

在 Kuscia 中，你可以使用 Config 接口管理节点应用的配置，并且 Kuscia 会通过节点私钥加密保存注册的配置信息。
从 [这里](https://github.com/secretflow/kuscia/tree/main/proto/api/v1alpha1/kusciaapi/config.proto) 可以找到对应的 protobuf 文件。

## 接口总览

| 方法名                                       | 请求类型                      | 响应类型                       | 描述     |
|-------------------------------------------|---------------------------|----------------------------|--------|
| [CreateConfig](#create-config)            | CreateConfigRequest       | CreateConfigResponse       | 注册配置   |
| [QueryConfig](#query-config)              | QueryConfigRequest        | QueryConfigResponse        | 查询配置   |
| [UpdateConfig](#update-config)            | UpdateConfigRequest       | UpdateConfigResponse       | 更新配置   |
| [DeleteConfig](#delete-config)            | DeleteConfigRequest       | DeleteConfigResponse       | 删除配置   |
| [BatchQueryConfig](#batch-query-config)   | BatchQueryConfigRequest   | BatchQueryConfigResponse   | 批量查询配置 |

## 接口详情

{#create-config}

### 注册配置

#### HTTP路径

/api/v1/config/create

#### 请求（CreateConfigRequest）

| 字段           | 类型                                           | 选填 | 描述      |
|--------------|----------------------------------------------|----|---------|
| header       | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容 |
| data         | ConfigData[]                                 |    |         |
| data[].key   | string                                       | 必填 | 配置的键    |
| data[].value | string                                       | 选填 | 配置的值    |

#### 响应（CreateConfigResponse）

| 字段        | 类型                              | 描述                                                                               |
|-----------|---------------------------------|----------------------------------------------------------------------------------|
| status    | [Status](summary_cn.md#status)  | 状态信息                                                                             |

#### 请求示例

发起请求：

```sh
# Execute the example within the container
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/config/create' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "data": [
   {
     "key": "SERVING_LOG_LEVEL",
     "value": "INFO_LOG_LEVEL"
   },
   {
     "key": "SPI_TLS_CONFIG",
     "value": "{\"certificate_path\":\"xxx\", \"private_key_path\":\"xxx\",\"ca_file_path\":\"xxx\"}"
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

{#query-config}

### 查询配置

#### HTTP路径

/api/v1/config/query

#### 请求（QueryConfigRequest）

| 字段       | 类型                                             | 选填 | 描述      |
|----------|------------------------------------------------|----|---------|
| header   | [RequestHeader](summary_cn.md#requestheader)   | 可选 | 自定义请求内容 |
| key      | string                                         | 必填 | 配置的键    |

#### 响应（QueryConfigResponse）

| 字段       | 类型                                             | 描述   |
|----------|------------------------------------------------|------|
| status   | [Status](summary_cn.md#status)                 | 状态信息 |
| key      | string                                         | 配置的键 |
| value    | string                                         | 配置的值 |

#### 请求示例

发起请求：

```sh
# Execute the example within the container
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/config/query' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "key": "SERVING_LOG_LEVEL"
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
  "key": "SERVING_LOG_LEVEL",
  "value": "INFO_LOG_LEVEL"
}
```

{#update-config}

### 更新配置

#### HTTP路径

/api/v1/config/update

#### 请求（UpdateConfigRequest）

| 字段             | 类型                                           | 选填 | 描述   |
|----------------|----------------------------------------------|----|------|
| header         | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容 |
| data           | ConfigData[]                    |    |      |
| data[].key     | string                                       | 必填 | 配置的键 |
| data[].value   | string                                       | 选填 | 配置的值 |

#### 响应（UpdateConfigResponse）

| 字段     | 类型                             | 描述   |
|--------|--------------------------------|------|
| status | [Status](summary_cn.md#status) | 状态信息 |

#### 请求示例

发起请求：

```sh
# Execute the example within the container
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/config/update' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "data": [
   {
     "key": "SERVING_LOG_LEVEL",
     "value": "DEBUG_LOG_LEVEL"
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

{#delete-config}

### 删除配置

#### HTTP路径

/api/v1/config/delete

#### 请求（DeleteConfigRequest）

| 字段     | 类型                                           | 选填 | 描述      |
|--------|----------------------------------------------|----|---------|
| header | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容 |
| keys   | string[]                                     | 必填 | 配置的键列表  |

#### 响应（DeleteConfigResponse）

| 字段     | 类型                             | 描述   |
|--------|--------------------------------|------|
| status | [Status](summary_cn.md#status) | 状态信息 |

#### 请求示例

发起请求：

```sh
# Execute the example within the container
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/config/delete' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "keys": ["SERVING_LOG_LEVEL"]
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

{#batch-query-config}

### 批量查询配置

#### HTTP路径

/api/v1/config/batchQuery

#### 请求（BatchQueryConfigRequest）

| 字段      | 类型                                             | 选填 | 描述            |
|---------|------------------------------------------------|----|---------------|
| header  | [RequestHeader](summary_cn.md#requestheader)   | 可选 | 自定义请求内容       |
| keys    | string[]                                       | 必填 | 配置的键列表  |

#### 响应（BatchQueryConfigResponse）

| 字段            | 类型                             | 描述   |
|---------------|--------------------------------|------|
| status        | [Status](summary_cn.md#status) | 状态信息 |
| data          | ConfigData[]                   |      |
| data[].key    | string                         | 配置的键 |
| data[].value  | string                         | 配置的值 |

#### 请求示例

发起请求：

```sh
# Execute the example within the container
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/config/batchQuery' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
   "keys": ["SPI_TLS_CONFIG", "SERVING_LOG_LEVEL"]
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
  "data": [
    {
      "key": "SPI_TLS_CONFIG",
      "value": "{\"certificate_path\":\"xxx\", \"private_key_path\":\"xxx\",\"ca_file_path\":\"xxx\"}"
    },
    {
      "key": "SERVING_LOG_LEVEL",
      "value": ""
    }
  ]
}
```
