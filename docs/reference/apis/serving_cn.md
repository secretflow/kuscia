# Serving

在 Kuscia 中，你可以使用 Serving 接口管理通用的常驻服务，例如：Secretflow Serving 等。
从 [这里](https://github.com/secretflow/kuscia/tree/main/proto/api/v1alpha1/kusciaapi/serving.proto) 可以找到对应的 protobuf 文件。

## 接口总览

| 方法名                                                    | 请求类型                           | 响应类型                            | 描述               |
|--------------------------------------------------------|--------------------------------|---------------------------------|------------------|
| [CreateServing](#create-serving)                       | CreateServingRequest           | CreateServingResponse           | 创建 Serving       |
| [QueryServing](#query-serving)                         | QueryServingRequest            | QueryServingResponse            | 查询 Serving       |
| [UpdateServing](#update-serving)                       | UpdateServingRequest           | UpdateServingResponse           | 更新 Serving       |
| [DeleteServing](#delete-serving)                       | DeleteServingRequest           | DeleteServingResponse           | 删除 Serving       |
| [BatchQueryServingStatus](#batch-query-serving-status) | BatchQueryServingStatusRequest | BatchQueryServingStatusResponse | 批量查询 Serving 状态  |

## 接口详情

{#create-serving}

### 创建 Serving

#### HTTP路径

/api/v1/serving/create

#### 请求（CreateServingRequest）

| 字段                   | 类型                                           | 选填 | 描述                                                                                                                                                      |
|----------------------|----------------------------------------------|----|---------------------------------------------------------------------------------------------------------------------------------------------------------|
| header               | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容                                                                                                                                                 |
| serving_id           | string                                       | 必填 | ServingID，满足 [RFC 1123 标签名规则要求](https://kubernetes.io/zh-cn/docs/concepts/overview/working-with-objects/names/#dns-label-names)                         |
| serving_input_config | string                                       | 可选 | 应用配置。[Secretflow Serving 预测应用配置参考](https://www.secretflow.org.cn/zh-CN/docs/serving/main/topics/deployment/serving_on_kuscia#configuration-description) |
| initiator            | string                                       | 必填 | 发起方节点 DomainID                                                                                                                                          |
| parties              | [ServingParty](#serving-party)[]             | 必填 | 参与方信息                                                                                                                                                   |

#### 响应（CreateServingResponse）

| 字段        | 类型                              | 描述                                                                               |
|-----------|---------------------------------|----------------------------------------------------------------------------------|
| status    | [Status](summary_cn.md#status)  | 状态信息                                                                             |

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
  "serving_id": "serving-1",
  "serving_input_config": "{\"partyConfigs\":{\"alice\":{\"serverConfig\":{\"featureMapping\":{\"v24\":\"x24\",\"v22\":\"x22\",\"v21\":\"x21\",\"v25\":\"x25\",\"v23\":\"x23\"}},\"modelConfig\":{\"modelId\":\"glm-test-1\",\"basePath\":\"/tmp/alice\",\"sourcePath\":\"examples/alice/glm-test.tar.gz\",\"sourceType\":\"ST_FILE\"},\"featureSourceConfig\":{\"mockOpts\":{}},\"channel_desc\":{\"protocol\":\"http\"}},\"bob\":{\"serverConfig\":{\"featureMapping\":{\"v6\":\"x6\",\"v7\":\"x7\",\"v8\":\"x8\",\"v9\":\"x9\",\"v10\":\"x10\"}},\"modelConfig\":{\"modelId\":\"glm-test-1\",\"basePath\":\"/tmp/bob\",\"sourcePath\":\"examples/bob/glm-test.tar.gz\",\"sourceType\":\"ST_FILE\"},\"featureSourceConfig\":{\"mockOpts\":{}},\"channel_desc\":{\"protocol\":\"http\"}}}}",
  "initiator": "alice",
  "parties": [
    {
      "domain_id": "alice",
      "app_image": "secretflow-serving-image",
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
          "max_cpu": "2",
          "min_memory": "100Mi",
          "max_memory": "2Gi"
        }
      ]
    },
    {
      "domain_id": "bob",
      "app_image": "secretflow-serving-image",
      "replicas": 1,
      "resources": [
        {
          "container_name": "",
          "min_cpu": "0.1",
          "max_cpu": "2",
          "min_memory": "100Mi",
          "max_memory": "2Gi"
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

{#query-serving}

### 查询 Serving

#### HTTP路径

/api/v1/serving/query

#### 请求（QueryServingRequest）

| 字段            | 类型                                           | 选填 | 描述                                                         |
|---------------|----------------------------------------------|----|------------------------------------------------------------|
| header        | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容                                                    |
| serving_id    | string                                       | 必填 | ServingID                                                  |

#### 响应（QueryServingResponse）

| 字段                        | 类型                                            | 描述                  |
|---------------------------|-----------------------------------------------|---------------------|
| status                    | [Status](summary_cn.md#status)                | 状态信息                |
| data                      | QueryServingResponseData                      |                     |
| data.serving_input_config | string                                        | 预测配置                |
| data.initiator            | string                                        | 发起方节点 ID            |
| data.parties              | [ServingParty](#serving-party)[]              | 参与方信息               |
| data.status               | [ServingStatusDetail](#serving-status-detail) | 状态信息                |

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
  "serving_id": "serving-1"
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
    "serving_input_config": "{\"partyConfigs\":{\"alice\":{\"serverConfig\":{\"featureMapping\":{\"v24\":\"x24\",\"v22\":\"x22\",\"v21\":\"x21\",\"v25\":\"x25\",\"v23\":\"x23\"}},\"modelConfig\":{\"modelId\":\"glm-test-1\",\"basePath\":\"/tmp/alice\",\"sourcePath\":\"examples/alice/glm-test.tar.gz\",\"sourceType\":\"ST_FILE\"},\"featureSourceConfig\":{\"mockOpts\":{}},\"channel_desc\":{\"protocol\":\"http\"}},\"bob\":{\"serverConfig\":{\"featureMapping\":{\"v6\":\"x6\",\"v7\":\"x7\",\"v8\":\"x8\",\"v9\":\"x9\",\"v10\":\"x10\"}},\"modelConfig\":{\"modelId\":\"glm-test-1\",\"basePath\":\"/tmp/bob\",\"sourcePath\":\"examples/bob/glm-test.tar.gz\",\"sourceType\":\"ST_FILE\"},\"featureSourceConfig\":{\"mockOpts\":{}},\"channel_desc\":{\"protocol\":\"http\"}}}}",
    "initiator": "alice",
    "parties": [
      {
        "domain_id": "alice",
        "role": "",
        "app_image": "secretflow-serving-image",
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
            "max_cpu": "3",
            "min_memory": "100Mi",
            "max_memory": "3Gi"
          }
        ]
      },
      {
        "domain_id": "bob",
        "role": "",
        "app_image": "secretflow-serving-image",
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
            "max_cpu": "3",
            "min_memory": "100Mi",
            "max_memory": "3Gi"
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
              "endpoint": "serving-1-brpc-builtin.alice.svc:22294"
            },
            {
              "port_name": "internal",
              "scope": "Domain",
              "endpoint": "serving-1-internal.alice.svc:22293"
            },
            {
              "port_name": "communication",
              "scope": "Cluster",
              "endpoint": "serving-1-communication.alice.svc"
            },
            {
              "port_name": "service",
              "scope": "Domain",
              "endpoint": "serving-1-service.alice.svc:22291"
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
              "port_name": "internal",
              "scope": "Domain",
              "endpoint": "serving-1-internal.bob.svc:30212"
            },
            {
              "port_name": "communication",
              "scope": "Cluster",
              "endpoint": "serving-1-communication.bob.svc"
            },
            {
              "port_name": "service",
              "scope": "Domain",
              "endpoint": "serving-1-service.bob.svc:30210"
            },
            {
              "port_name": "brpc-builtin",
              "scope": "Domain",
              "endpoint": "serving-1-brpc-builtin.bob.svc:30213"
            }
          ]
        }
      ]
    }
  }
}
```

{#update-serving}

### 更新 Serving

#### HTTP路径

/api/v1/serving/update

#### 请求（UpdateServingRequest）

| 字段                   | 类型                                              | 选填 | 描述                                                        |
|----------------------|-------------------------------------------------|----|-----------------------------------------------------------|
| header               | [RequestHeader](summary_cn.md#requestheader)    | 可选 | 自定义请求内容                                                   |
| serving_id           | string                                          | 必填 | ServingID                                                 |
| serving_input_config | string                                          | 可选 | 应用配置                                                      |
| parties              | [ServingParty](#serving-party)[]                | 可选 | 参与方信息                                                     |

#### 响应（UpdateServingResponse）

| 字段     | 类型                             | 描述   |
|--------|--------------------------------|------|
| status | [Status](summary_cn.md#status) | 状态信息 |

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
  "serving_id": "serving-1",
  "parties": [
    {
      "domain_id": "alice",
      "resources": [
        {
          "container_name": "",
          "min_cpu": "0.1",
          "max_cpu": "3",
          "min_memory": "100Mi",
          "max_memory": "3Gi"
        }
      ]
    },
    {
      "domain_id": "bob",
      "resources": [
        {
          "container_name": "",
          "min_cpu": "0.1",
          "max_cpu": "3",
          "min_memory": "100Mi",
          "max_memory": "3Gi"
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

| 字段           | 类型                                             | 选填 | 描述                                                         |
|--------------|------------------------------------------------|----|------------------------------------------------------------|
| header       | [RequestHeader](summary_cn.md#requestheader)   | 可选 | 自定义请求内容                                                    |
| serving_id   | string                                         | 必填 | ServingID                                                  |

#### 响应（DeleteServingResponse）

| 字段     | 类型                             | 描述   |
|--------|--------------------------------|------|
| status | [Status](summary_cn.md#status) | 状态信息 |

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
  "serving_id": "serving-1"
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

{#batch-query-serving-status}

### 批量查询 Serving 状态

#### HTTP路径

/api/v1/serving/status/batchQuery

#### 请求（BatchQueryServingStatusRequest）

| 字段            | 类型                                           | 选填 | 描述            |
|---------------|----------------------------------------------|----|---------------|
| header        | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容       |
| serving_ids   | string[]                                     | 必填 | ServingID列表   |

#### 响应（BatchQueryServingStatusResponse）

| 字段            | 类型                                  | 描述           |
|---------------|-------------------------------------|--------------|
| status        | [Status](summary_cn.md#status)      | 状态信息         |
| data          | BatchQueryServingStatusResponseData |              |
| data.servings | [ServingStatus](#serving-status)[]  | Serving 状态列表 |

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
     "serving-1"
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
        "serving_id": "serving-1",
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
                  "endpoint": "serving-1-brpc-builtin.alice.svc:22294"
                },
                {
                  "port_name": "internal",
                  "scope": "Domain",
                  "endpoint": "serving-1-internal.alice.svc:22293"
                },
                {
                  "port_name": "communication",
                  "scope": "Cluster",
                  "endpoint": "serving-1-communication.alice.svc"
                },
                {
                  "port_name": "service",
                  "scope": "Domain",
                  "endpoint": "serving-1-service.alice.svc:22291"
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
                  "port_name": "internal",
                  "scope": "Domain",
                  "endpoint": "serving-1-internal.bob.svc:30212"
                },
                {
                  "port_name": "communication",
                  "scope": "Cluster",
                  "endpoint": "serving-1-communication.bob.svc"
                },
                {
                  "port_name": "service",
                  "scope": "Domain",
                  "endpoint": "serving-1-service.bob.svc:30210"
                },
                {
                  "port_name": "brpc-builtin",
                  "scope": "Domain",
                  "endpoint": "serving-1-brpc-builtin.bob.svc:30213"
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

| 字段            | 类型                                             | 描述                 |
|---------------|------------------------------------------------|--------------------|
| serving_id    | string                                         | ServingID          |
| status        | [ServingStatusDetail](#serving-status-detail)  | 状态信息               |

{#serving-status-detail}

### ServingStatusDetail

| 字段              | 类型                                              | 描述                      |
| ------------------- |-------------------------------------------------|-------------------------|
| state             | string                                          | 状态信息，参考 [State](#state) |
| reason            | string                                          | 处于该状态的原因，一般用于描述失败的状态    |
| message           | string                                          | 处于该状态的详细信息，一般用于描述失败的状态  |
| total_parties     | int32                                           | 参与方总数                   |
| available_parties | int32                                           | 可用参与方数量                 |
| create_time       | string                                          | 创建时间                    |
| party_statuses    | [PartyServingStatus](#party-serving-status)[]   | 参与方状态                   |

{#party-serving-status}

### PartyServingStatus

| 字段                   | 类型                                                | 描述                                                                          |
|----------------------|---------------------------------------------------|-----------------------------------------------------------------------------|
| domain_id            | string                                            | 节点 DomainID                                                                 |
| role                 | string                                            | 角色，详细解释请参考 [AppImage](../concepts/appimage_cn.md) 中的 `deployTemplates.role` |
| state                | string                                            | 状态，参考 [State](#state)                                                       |
| replicas             | int32                                             | 应用副本总数                                                                      |
| available_replicas   | int32                                             | 应用可用副本数                                                                     |
| unavailable_replicas | int32                                             | 应用不可用副本数                                                                    |
| updatedReplicas      | int32                                             | 最新版本的应用副本数                                                                  |
| create_time          | string                                            | 创建时间，时间格式为 RFC3339。示例: "2024-01-17T10:18:02Z"                               |
| endpoints            | [ServingPartyEndpoint](#serving-party-endpoint)[] | 应用对外暴露的访问地址信息                                                               |

{#serving-party-endpoint}

### ServingPartyEndpoint

| 字段          | 类型   | 描述                                                                                                        |
|-------------| -------- |-----------------------------------------------------------------------------------------------------------|
| port_name   | string | 应用服务端口名称，详细解释请参考 [AppImage](../concepts/appimage_cn.md) 中的 `deployTemplates.spec.containers.ports.name`   |
| scope       | string | 应用服务使用范围, 详细解释请参考 [AppImage](../concepts/appimage_cn.md) 中的 `deployTemplates.spec.containers.ports.scope` |
| endpoint    | string | 应用服务暴露的访问地址                                                                                               |

{#serving-party}

### ServingParty

| 字段                  | 类型                                 | 选填 | 描述                                                                                                                                                                                                                                                                                                                                              |
|---------------------|------------------------------------| ------ |-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| domain_id           | string                             | 必填 | 节点 ID                                                                                                                                                                                                                                                                                                                                           |
| app_image           | string                             | 必填 | 应用镜像，对应 [AppImage](../concepts/appimage_cn.md) 资源名称。<br/>在调用更新接口时，如果更新该字段，当前仅会使用新的 [AppImage](../concepts/appimage_cn.md) 资源中的应用镜像 Name 和 Tag 信息，更新预测应用                                                                                                                                                                                         |
| role                | string                             | 可选 | 角色，详细解释请参考 [AppImage](../concepts/appimage_cn.md) 中的 `deployTemplates.role`                                                                                                                                                                                                                                                                     |
| replicas            | int32                              | 可选 | 应用总副本数，即启动的应用实例个数。默认为 1                                                                                                                                                                                                                                                                                                                         |
| update_strategy     | [UpdateStrategy](#update-strategy) | 可选 | 应用升级策略                                                                                                                                                                                                                                                                                                                                          |
| resources           | [Resource](#resource)[]            | 可选 | 应用运行资源。若不设时，那么不会限制应用运行过程中使用的资源大小                                                                                                                                                                                                                                                                                                                |
| service_name_prefix | string                             | 可选 | 自定义应用服务名称前缀。长度不超过 48 个字符，满足 [RFC 1123 标签名规则要求](https://kubernetes.io/zh-cn/docs/concepts/overview/working-with-objects/names/#dns-label-names)。 <br/> - 若配置，则应用服务名称拼接规则为 `{service_name_prefix}-{port_name}`，port_name 对应 [AppImage](../concepts/appimage_cn.md) 中的 `deployTemplates.spec.containers.ports.name` </br> - 若不配置，Kuscia 随机生成应用服务名称 |

{#update-strategy}

### UpdateStrategy

| 字段              | 类型     | 选填 | 描述                                                                                                                                                              |
|-----------------|--------|----|-----------------------------------------------------------------------------------------------------------------------------------------------------------------|
| type            | string | 必填 | 应用升级策略类型：支持"Recreate"和"RollingUpdate"两种类型<br/> "Recreate"：表示重建，在创建新的应用之前，所有现有应用都会被删除<br/> "RollingUpdate"：表示滚动升级，当应用升级时，结合"max_surge"和"max_unavailable"控制滚动升级过程 |
| max_surge       | string | 可选 | 用来指定可以创建的超出应用总副本数的应用数量。默认为总副本数的"25%"。max_unavailable为0，则此值不能为0                                                                                                  |
| max_unavailable | string | 可选 | 用来指定升级过程中不可用的应用副本个数上限。默认为总副本数的"25%"。max_surge为0，则此值不能为0                                                                                                         |

{#resource}

### Resource

| 字段           | 类型   | 选填 | 描述                                                                                                                            |
| ---------------- | -------- | ------ |-------------------------------------------------------------------------------------------------------------------------------|
| container_name | string | 可选 | 容器名称。若为空，则 "cpu" 和 "memory" 资源适用于所有容器。容器名称对应为 [AppImage](../concepts/appimage_cn.md) 中的`deployTemplates.spec.containers.name` |
| min_cpu        | string | 可选 | 最小cpu数量。例如："0.1"：表示100毫核；"1"：表示1核                                                                                             |
| max_cpu        | string | 可选 | 最大cpu数量。例如："0.1"：表示100毫核；"1"：表示1核                                                                                             |
| min_memory     | string | 可选 | 最小memory数量。单位："Mi"，"Gi"；例如："100Mi"：表示100兆字节                                                                                   |
| max_memory     | string | 可选 | 最大memory数量。单位："Mi"，"Gi"；例如："100Mi"：表示100兆字节                                                                                   |

{#state}

### State

| Name             | Number | 描述                     |
|------------------|--------|------------------------|
| Unknown          | 0      | 未知                     |
| Pending          | 1      | 还未被处理                  |
| Progressing      | 2      | 发布中，至少有一方不可用           |
| PartialAvailable | 3      | 发布完成，至少有一方的多应用实例不是全部可用 |
| Available        | 4      | 发布完成，所有方的所有应用实例全部可用    |
| Failed           | 5      | 发布失败                   |
