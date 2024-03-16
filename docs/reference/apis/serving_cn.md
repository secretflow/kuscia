# Serving

在 Kuscia 中，你可以使用 Serving 接口管理联合预测服务，
从 [这里](https://github.com/secretflow/kuscia/tree/main/proto/api/v1alpha1/kusciaapi/serving.proto) 可以找到对应的 protobuf 文件。

## 接口总览

| 方法名                                                    | 请求类型                           | 响应类型                            | 描述            |
|--------------------------------------------------------|--------------------------------|---------------------------------|---------------|
| [CreateServing](#create-serving)                       | CreateServingRequest           | CreateServingResponse           | 创建Serving     |
| [UpdateServing](#update-serving)                       | UpdateServingRequest           | UpdateServingResponse           | 更新Serving     |
| [DeleteServing](#delete-serving)                       | DeleteServingRequest           | DeleteServingResponse           | 删除Serving     |
| [QueryServing](#query-serving)                         | QueryServingRequest            | QueryServingResponse            | 查询Serving     |
| [BatchQueryServingStatus](#batch-query-serving-status) | BatchQueryServingStatusRequest | BatchQueryServingStatusResponse | 批量查询Serving状态 |

## 接口详情

{#create-serving}

### 创建 Serving

#### HTTP路径

/api/v1/serving/create

#### 请求（CreateServingRequest）

| 字段                   | 类型                                           | 选填 | 描述                                                                                                                            |
|----------------------|----------------------------------------------|----|-------------------------------------------------------------------------------------------------------------------------------|
| header               | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容                                                                                                                       |
| serving_id           | string                                       | 必填 | ServingID，满足[DNS 子域名规则要求](https://kubernetes.io/zh-cn/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names) |
| serving_input_config | string                                       | 必填 | 预测配置                                                                                                                          |
| initiator            | string                                       | 必填 | 发起方节点ID                                                                                                                       |
| parties              | [ServingParty](#serving-party)[]             | 必填 | 参与方信息                                                                                                                         |


#### 响应（CreateServingResponse）

| 字段     | 类型                             | 选填 | 描述   |
|--------|--------------------------------|----|------|
| status | [Status](summary_cn.md#status) | 必填 | 状态信息 |

#### 请求示例

发起请求：

```sh
# 在容器内执行示例
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/serving/create' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "serving_id": "serving-01",
  "serving_input_config": "{\"partyConfigs\":{\"alice\":{\"serverConfig\":{\"featureMapping\":{\"v24\":\"x24\",\"v22\":\"x22\",\"v21\":\"x21\",\"v25\":\"x25\",\"v23\":\"x23\"}},\"modelConfig\":{\"modelId\":\"glm-test-1\",\"basePath\":\"/tmp/alice\",\"sourceMd5\":\"4216c62acba4a630d5039f917612780b\",\"sourcePath\":\"examples/alice/glm-test.tar.gz\",\"sourceType\":\"ST_FILE\"},\"featureSourceConfig\":{\"mockOpts\":{}},\"channel_desc\":{\"protocol\":\"http\"}},\"bob\":{\"serverConfig\":{\"featureMapping\":{\"v6\":\"x6\",\"v7\":\"x7\",\"v8\":\"x8\",\"v9\":\"x9\",\"v10\":\"x10\"}},\"modelConfig\":{\"modelId\":\"glm-test-1\",\"basePath\":\"/tmp/bob\",\"sourceMd5\":\"1ded1513dab8734e23152ef906c180fc\",\"sourcePath\":\"examples/bob/glm-test.tar.gz\",\"sourceType\":\"ST_FILE\"},\"featureSourceConfig\":{\"mockOpts\":{}},\"channel_desc\":{\"protocol\":\"http\"}}}}",
  "initiator": "alice",
  "parties": [
    {
      "domain_id": "alice",
      "app_image": "sf-serving-image",
      "role": "",
      "replicas": 1,
      "update_strategy": {
        "type": "RollingUpdate",
        "max_surge": "25%",
        "max_unavailable": "25%"
      },
      "resources": [
        {
          "container_name": "",
          "min_cpu": "0.1",
          "max_cpu": "0.1",
          "min_memory": "100Mi",
          "max_memory": "100Mi"
        }
      ]
    },
    {
      "domain_id": "bob",
      "app_image": "sf-serving-image",
      "role": "",
      "replicas": 1,
      "update_strategy": {
        "type": "RollingUpdate",
        "max_surge": "25%",
        "max_unavailable": "25%"
      },
      "resources": [
        {
          "container_name": "",
          "min_cpu": "0.1",
          "max_cpu": "0.1",
          "min_memory": "100Mi",
          "max_memory": "100Mi"
        }
      ]
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

{#update-serving}

### 更新 Serving

#### HTTP路径
/api/v1/serving/update

#### 请求（UpdateServingRequest）

| 字段                   | 类型                                           | 选填 | 描述        |
|----------------------|----------------------------------------------|----|-----------|
| header               | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容   |
| serving_id           | string                                       | 必填 | ServingID |
| serving_input_config | string                                       | 可选 | 预测配置      |
| parties              | [ServingParty](#serving-party)[]             | 可选 | 参与方信息     |


#### 响应（UpdateServingResponse）

| 字段     | 类型                             | 选填 | 描述   |
|--------|--------------------------------|----|------|
| status | [Status](summary_cn.md#status) | 必填 | 状态信息 |

#### 请求示例

发起请求：

```sh
# 在容器内执行示例
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/serving/update' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "serving_id": "serving-01",
  "serving_input_config": "{\"partyConfigs\":{\"alice\":{\"serverConfig\":{\"featureMapping\":{\"v24\":\"x24\",\"v22\":\"x22\",\"v21\":\"x21\",\"v25\":\"x25\",\"v23\":\"x23\"}},\"modelConfig\":{\"modelId\":\"glm-test-1\",\"basePath\":\"/tmp/alice\",\"sourceMd5\":\"4216c62acba4a630d5039f917612780b\",\"sourcePath\":\"examples/alice/glm-test.tar.gz\",\"sourceType\":\"ST_FILE\"},\"featureSourceConfig\":{\"mockOpts\":{}},\"channel_desc\":{\"protocol\":\"http\"}},\"bob\":{\"serverConfig\":{\"featureMapping\":{\"v6\":\"x6\",\"v7\":\"x7\",\"v8\":\"x8\",\"v9\":\"x9\",\"v10\":\"x10\"}},\"modelConfig\":{\"modelId\":\"glm-test-1\",\"basePath\":\"/tmp/bob\",\"sourceMd5\":\"1ded1513dab8734e23152ef906c180fc\",\"sourcePath\":\"examples/bob/glm-test.tar.gz\",\"sourceType\":\"ST_FILE\"},\"featureSourceConfig\":{\"mockOpts\":{}},\"channel_desc\":{\"protocol\":\"http\"}}}}",
  "parties": [
    {
      "domain_id": "alice",
      "app_image": "sf-serving-image",
      "role": "",
      "replicas": 1,
      "update_strategy": {
        "type": "RollingUpdate",
        "max_surge": "25%",
        "max_unavailable": "25%"
      },
      "resources": [
        {
          "container_name": "",
          "min_cpu": "0.1",
          "max_cpu": "0.1",
          "min_memory": "100Mi",
          "max_memory": "100Mi"
        }
      ]
    },
    {
      "domain_id": "bob",
      "app_image": "sf-serving-image",
      "role": "",
      "replicas": 1,
      "update_strategy": {
        "type": "RollingUpdate",
        "max_surge": "25%",
        "max_unavailable": "25%"
      },
      "resources": [
        {
          "container_name": "",
          "min_cpu": "0.1",
          "max_cpu": "0.1",
          "min_memory": "100Mi",
          "max_memory": "200Mi"
        }
      ]
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

{#delete-serving}

### 删除 Serving

#### HTTP路径
/api/v1/serving/delete

#### 请求（DeleteServingRequest）

| 字段         | 类型                                           | 选填 | 描述        |
|------------|----------------------------------------------|----|-----------|
| header     | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容   |
| serving_id | string                                       | 必填 | ServingID |

#### 响应（DeleteServingResponse）

| 字段     | 类型                             | 选填 | 描述   |
|--------|--------------------------------|----|------|
| status | [Status](summary_cn.md#status) | 必填 | 状态信息 |


#### 请求示例

发起请求：

```sh
# 在容器内执行示例
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/serving/delete' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "serving_id": "serving-01"
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

{#query-serving}

### 查询 Serving

#### HTTP路径

/api/v1/serving/query

#### 请求（QueryServingRequest）

| 字段         | 类型                                           | 选填 | 描述        |
|------------|----------------------------------------------|----|-----------|
| header     | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容   |
| serving_id | string                                       | 必填 | ServingID |

#### 响应（QueryServingResponse）

| 字段                        | 类型                                            | 选填 | 描述        |
|---------------------------|-----------------------------------------------|----|-----------|
| status                    | [Status](summary_cn.md#status)                | 必填 | 状态信息      |
| data                      | QueryServingResponseData                      |    |           |
| data.serving_input_config | string                                        | 必填 | 预测配置      |
| data.initiator            | string                                        | 必填 | 发起方节点ID   |
| data.parties              | [ServingParty](#serving-party)[]              | 必填 | 参与方信息     |
| data.status               | [ServingStatusDetail](#serving-status-detail) | 必填 | Serving状态 |

#### 请求示例

发起请求：

```sh
# 在容器内执行示例
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/serving/query' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "serving_id": "serving-01"
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
    "serving_input_config": "{\"partyConfigs\":{\"alice\":{\"serverConfig\":{\"featureMapping\":{\"v24\":\"x24\",\"v22\":\"x22\",\"v21\":\"x21\",\"v25\":\"x25\",\"v23\":\"x23\"}},\"modelConfig\":{\"modelId\":\"glm-test-1\",\"basePath\":\"/tmp/alice\",\"sourceMd5\":\"4216c62acba4a630d5039f917612780b\",\"sourcePath\":\"examples/alice/glm-test.tar.gz\",\"sourceType\":\"ST_FILE\"},\"featureSourceConfig\":{\"mockOpts\":{}},\"channel_desc\":{\"protocol\":\"http\"}},\"bob\":{\"serverConfig\":{\"featureMapping\":{\"v6\":\"x6\",\"v7\":\"x7\",\"v8\":\"x8\",\"v9\":\"x9\",\"v10\":\"x10\"}},\"modelConfig\":{\"modelId\":\"glm-test-1\",\"basePath\":\"/tmp/bob\",\"sourceMd5\":\"1ded1513dab8734e23152ef906c180fc\",\"sourcePath\":\"examples/bob/glm-test.tar.gz\",\"sourceType\":\"ST_FILE\"},\"featureSourceConfig\":{\"mockOpts\":{}},\"channel_desc\":{\"protocol\":\"http\"}}}}",
    "initiator": "alice",
    "parties": [
      {
        "domain_id": "alice",
        "role": "",
        "app_image": "sf-serving-image",
        "replicas": 1,
        "update_strategy": {
          "type": "RollingUpdate",
          "max_surge": "25%",
          "max_unavailable": "25%"
        },
        "resources": [
          {
            "container_name": "secretflow",
            "min_cpu": "100m",
            "max_cpu": "100m",
            "min_memory": "100Mi",
            "max_memory": "100Mi"
          }
        ]
      },
      {
        "domain_id": "bob",
        "role": "",
        "app_image": "sf-serving-image",
        "replicas": 1,
        "update_strategy": {
          "type": "RollingUpdate",
          "max_surge": "25%",
          "max_unavailable": "25%"
        },
        "resources": [
          {
            "container_name": "secretflow",
            "min_cpu": "100m",
            "max_cpu": "100m",
            "min_memory": "100Mi",
            "max_memory": "100Mi"
          }
        ]
      }
    ],
    "status": {
      "state": "Progressing",
      "reason": "",
      "message": "",
      "total_parties": 2,
      "available_parties": 0,
      "create_time": "2024-01-17T10:18:02Z",
      "party_statuses": [
        {
          "domain_id": "alice",
          "role": "",
          "state": "Progressing",
          "replicas": 1,
          "available_replicas": 0,
          "unavailable_replicas": 1,
          "updatedReplicas": 1,
          "create_time": "2024-01-17T10:18:02Z",
          "endpoints": [
            {
              "port_name": "brpc-builtin",
              "scope": "Domain",
              "endpoint": "serving-01-brpc-builtin.alice.svc:53511"
            },
            {
              "port_name": "service",
              "scope": "Cluster",
              "endpoint": "serving-01-service.alice.svc"
            },
            {
              "port_name": "internal",
              "scope": "Domain",
              "endpoint": "serving-01-internal.alice.svc:53510"
            }
          ]
        },
        {
          "domain_id": "bob",
          "role": "",
          "state": "Progressing",
          "replicas": 1,
          "available_replicas": 0,
          "unavailable_replicas": 1,
          "updatedReplicas": 1,
          "create_time": "2024-01-17T10:18:02Z",
          "endpoints": [
            {
              "port_name": "service",
              "scope": "Cluster",
              "endpoint": "serving-01-service.bob.svc"
            },
            {
              "port_name": "internal",
              "scope": "Domain",
              "endpoint": "serving-01-internal.bob.svc:53510"
            },
            {
              "port_name": "brpc-builtin",
              "scope": "Domain",
              "endpoint": "serving-01-brpc-builtin.bob.svc:53511"
            }
          ]
        }
      ]
    }
  }
}
```

{#batch-query-serving-status}

### 批量查询 Serving 状态

#### HTTP路径

/api/v1/serving/status/batchQuery

#### 请求（BatchQueryServingStatusRequest）

| 字段          | 类型                                           | 选填 | 描述          |
|-------------|----------------------------------------------|----|-------------|
| header      | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容     |
| serving_ids | string[]                                     | 必填 | ServingID列表 |


#### 响应（BatchQueryServingStatusResponse）

| 字段            | 类型                                  | 选填 | 描述          |
|---------------|-------------------------------------|----|-------------|
| status        | [Status](summary_cn.md#status)      | 必填 | 状态信息        |
| data          | BatchQueryServingStatusResponseData |    |             |
| data.servings | [ServingStatus](#serving-status)[]  | 必填 | Serving状态列表 |

#### 请求示例

发起请求：

```sh
# 在容器内执行示例
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/serving/status/batchQuery' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "serving_ids": [
    "serving-01"
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
    "servings": [
      {
        "serving_id": "serving-01",
        "status": {
          "state": "Progressing",
          "reason": "",
          "message": "",
          "total_parties": 2,
          "available_parties": 0,
          "create_time": "2024-01-17T10:18:02Z",
          "party_statuses": [
            {
              "domain_id": "alice",
              "role": "",
              "state": "Progressing",
              "replicas": 1,
              "available_replicas": 0,
              "unavailable_replicas": 1,
              "updatedReplicas": 1,
              "create_time": "2024-01-17T10:18:02Z",
              "endpoints": [
                {
                  "port_name": "brpc-builtin",
                  "scope": "Domain",
                  "endpoint": "serving-01-brpc-builtin.alice.svc:53511"
                },
                {
                  "port_name": "service",
                  "scope": "Cluster",
                  "endpoint": "serving-01-service.alice.svc"
                },
                {
                  "port_name": "internal",
                  "scope": "Domain",
                  "endpoint": "serving-01-internal.alice.svc:53510"
                }
              ]
            },
            {
              "domain_id": "bob",
              "role": "",
              "state": "Progressing",
              "replicas": 1,
              "available_replicas": 0,
              "unavailable_replicas": 1,
              "updatedReplicas": 1,
              "create_time": "2024-01-17T10:18:02Z",
              "endpoints": [
                {
                  "port_name": "service",
                  "scope": "Cluster",
                  "endpoint": "serving-01-service.bob.svc"
                },
                {
                  "port_name": "internal",
                  "scope": "Domain",
                  "endpoint": "serving-01-internal.bob.svc:53510"
                },
                {
                  "port_name": "brpc-builtin",
                  "scope": "Domain",
                  "endpoint": "serving-01-brpc-builtin.bob.svc:53511"
                }
              ]
            }
          ]
        }
      }
    ]
  }
}
```

## 公共

{#serving-status}

### ServingStatus

| 字段         | 类型                                            | 选填 | 描述          |
|------------|-----------------------------------------------|----|-------------|
| serving_id | string                                        | 必填 | ServingID   |
| status     | [ServingStatusDetail](#serving-status-detail) | 必填 | Serving状态详情 |


{#serving-status-detail}

### ServingStatusDetail

| 字段                | 类型                                            | 选填 | 描述                           |
|-------------------|-----------------------------------------------|----|------------------------------|
| state             | string                                        | 必填 | Serving状态，参考 [State](#state) |
| reason            | string                                        | 可选 | Serving处于该状态的原因，一般用于描述失败的状态  |
| message           | string                                        | 可选 | Serving处于该状态的详细信息，一般用于描述失败的状态 |
| total_parties     | int32                                         | 必填 | 参与方总数                        |
| available_parties | int32                                         | 必填 | 可用参与方数量                      |
| create_time       | string                                        | 必填 | 创建时间                         |
| party_statuses    | [PartyServingStatus](#party-serving-status)[] | 必填 | 参与方状态                        |


{#party-serving-status}

### PartyServingStatus

| 字段                   | 类型                                                | 选填 | 描述                     |
|----------------------|---------------------------------------------------|----|------------------------|
| domain_id            | string                                            | 必填 | 节点ID                   |
| role                 | string                                            | 可选 | 角色                     |
| state                | string                                            | 必填 | 状态，参考 [State](#state)  |
| replicas             | int32                                             | 必填 | 应用副本总数                 |
| available_replicas   | int32                                             | 必填 | 应用可用副本数                |
| unavailable_replicas | int32                                             | 必填 | 应用不可用副本数               |
| updatedReplicas      | int32                                             | 必填 | 最新版本的应用副本数             |
| create_time          | string                                            | 必填 | 创建时间                   |
| endpoints            | [ServingPartyEndpoint](#serving-party-endpoint)[] | 必填 | 应用对外暴露的访问地址信息          |


{#serving-party-endpoint}

### ServingPartyEndpoint

| 字段        | 类型     | 选填 | 描述                                                                                                    |
|-----------|--------|---|-------------------------------------------------------------------------------------------------------|
| port_name | string | 必填 | 应用服务端口名称，详细解释请参考[AppImage](../concepts/appimage_cn.md) `deployTemplates.spec.containers.ports.name`   |
| scope     | string | 必填 | 应用服务使用范围, 详细解释请参考[AppImage](../concepts/appimage_cn.md) `deployTemplates.spec.containers.ports.scope` |
| endpoint  | string | 必填 | 应用服务访问地址                                                                                              |


{#serving-party}

### ServingParty

| 字段              | 类型                                 | 选填 | 描述                                                                                                                                               |
|-----------------|------------------------------------|----|--------------------------------------------------------------------------------------------------------------------------------------------------|
| domain_id       | string                             | 必填 | 节点ID                                                                                                                                             |
| app_image       | string                             | 必填 | 应用镜像，对应[AppImage](../concepts/appimage_cn.md)资源名称。<br/>在调用更新接口时，如果更新该字段，当前仅会使用新的[AppImage](../concepts/appimage_cn.md)资源中的应用镜像Name和Tag信息，更新预测应用。 |
| role            | string                             | 可选 | 角色                                                                                                                                               |
| replicas        | int32                              | 可选 | 应用总副本数                                                                                                                                           |
| update_strategy | [UpdateStrategy](#update-strategy) | 可选 | 应用更新策略                                                                                                                                           |
| resources       | [Resource](#resource)[]            | 可选 | 应用运行资源                                                                                                                                           |


{#update-strategy}

### UpdateStrategy

| 字段              | 类型     | 选填 | 描述                                                                                                                                                              |
|-----------------|--------|----|-----------------------------------------------------------------------------------------------------------------------------------------------------------------|
| type            | string | 必填 | 应用更新策略类型：支持"Recreate"和"RollingUpdate"两种类型<br/> "Recreate"：表示重建，在创建新的应用之前，所有现有应用都会被删除<br/> "RollingUpdate"：表示滚动更新，当应用更新时，结合"max_surge"和"max_unavailable"控制滚动更新过程 |
| max_surge       | string | 可选 | 用来指定可以创建的超出应用总副本数的应用数量。默认为总副本数的"25%"。max_unavailable为0，则此值不能为0                                                                                                  |
| max_unavailable | string | 可选 | 用来指定更新过程中不可用的应用副本个数上限。默认为总副本数的"25%"。max_surge为0，则此值不能为0                                                                                                         |


{#resource}

### Resource

| 字段             | 类型     | 选填 | 描述                                          |
|----------------|--------|----|---------------------------------------------|
| container_name | string | 可选 | 容器名称。若为空，则"cpu"和"memory"资源适用于所有容器           |
| min_cpu        | string | 可选 | 最小cpu数量。例如："0.1"：表示100毫核；"1"：表示1核           |
| max_cpu        | string | 可选 | 最大cpu数量。例如："0.1"：表示100毫核；"1"：表示1核           |
| min_memory     | string | 可选 | 最小memory数量。单位："Mi"，"Gi"；例如："100Mi"：表示100兆字节 |
| max_memory     | string | 可选 | 最大memory数量。单位："Mi"，"Gi"；例如："100Mi"：表示100兆字节 |


{#state}

### State

| Name               | Number | 描述                |
|-----------         |--------|-------------------|
| Unknown            | 0      | 未知                |
| Progressing        | 1      | 发布中，至少有一方不可用      |
| PartialAvailable   | 2      | 发布完成，至少有一方的多实例不是全部可用 |
| Available          | 3      | 发布完成，所有方的所有实例都可用  |
| Failed             | 4      | 发布失败              |