# AppImage

在 Kuscia 中，你可以使用 AppImage 存储注册应用镜像模版信息。详情请参考 [AppImage](../concepts/appimage_cn.md) 。
您可以从 [这里](https://github.com/secretflow/kuscia/tree/main/proto/api/v1alpha1/kusciaapi/appimage.proto) 找到对应的 protobuf 文件。

## 接口总览

| 方法名                                     | 请求类型                    | 响应类型                     | 描述       |
|-----------------------------------------|-------------------------|--------------------------|----------|
| [CreateAppImage](#create-appimage)          | CreateAppImageRequest     | CreateAppImageResponse     | 创建应用镜像模版     |
| [UpdateAppImage](#update-appimage)          | UpdateAppImageRequest     | UpdateAppImageResponse     | 更新应用镜像模版     |
| [DeleteAppImage](#delete-appimage)          | DeleteAppImageRequest     | DeleteAppImageResponse     | 删除应用镜像模版     |
| [QueryAppImage](#query-appimage)            | QueryAppImageRequest      | QueryAppImageResponse      | 查询应用镜像模版     |
| [BatchQueryAppImage](#batch-query-appimage) | BatchQueryAppImageRequest | BatchQueryAppImageResponse | 批量查询应用镜像模版 |

## 接口详情

{#create-appimage}

### 创建应用镜像模版

#### HTTP 路径

/api/v1/appimage/create

#### 请求（CreateAppImageRequest）

| 字段              | 类型                                           | 选填 | 描述                                                                                                                         |
|-----------------|----------------------------------------------|----|----------------------------------------------------------------------------------------------------------------------------|
| header          | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容                                                                                                                    |
| name          | string                                       | 必填 | 应用镜像名称，满足 [RFC 1123 标签名规则要求](https://kubernetes.io/zh-cn/docs/concepts/overview/working-with-objects/names/#dns-label-names) |
| image       | [AppImageInfo](#AppImageInfo)                                       | 必填 | 基础镜像信息 ID                                                                                                                   |
| config_templates | map<string, string>                                        | 可选 | 应用启动依赖的配置模版信息，参考 [AppImage 概念](../concepts/appimage_cn.md)                                                                         |
| deploy_templates           | [DeployTemplate](#DeployTemplate)[]                              | 必填 | 应用部署模版配置信息                                                                                                                       |

#### 响应（CreateAppImageResponse）

| 字段          | 类型                             | 描述    |
|-------------|--------------------------------|-------|
| status      | [Status](summary_cn.md#status) | 状态信息  |

#### 请求示例

发起请求：

```sh
# Execute example in container
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/appimage/create' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "name": "appimage-template",
  "config_templates": {
   "task-config.conf": "{\n  \"task_id\": \"{{.TASK_ID}}\",\n  \"task_input_config\": \"{{.TASK_INPUT_CONFIG}}\",\n  \"task_input_cluster_def\": \"{{.TASK_CLUSTER_DEFINE}}\",\n  \"allocated_ports\": \"{{.ALLOCATED_PORTS}}\"\n}\n"
  },
  "deploy_templates": [
   {
    "name": "app",
    "replicas": 1,
    "role": "server",
    "containers": [
     {
      "args": [
       "-c",
       "./app --role=server --task_config_path=./kuscia/task-config.conf"
      ],
      "command": [
       "sh"
      ],
      "config_volume_mounts": [
       {
        "mount_path": "/work/kuscia/task-config.conf",
        "sub_path": "task-config.conf"
       }
      ],
      "env": [
       {
        "name": "APP_NAME",
        "value": "app"
       }
      ],
      "env_from": [
       {
        "prefix": "xxx",
        "config_map_ref": {
         "name": "config-template"
        },
        "secret_map_ref": {
         "name": "secret-template"
        }
       }
      ],
      "image_pull_policy": "IfNotPresent",
      "liveness_probe": {
       "failure_threshold": 1,
       "http_get": {
        "path": "/healthz",
        "port": "global"
       },
       "period_seconds": 20
      },
      "name": "app",
      "ports": [
       {
        "name": "global",
        "protocol": "HTTP",
        "scope": "Cluster"
       }
      ],
      "readiness_probe": {
       "exec": {
        "command": [
         "cat",
         "/tmp/healthy"
        ]
       },
       "initial_delay_seconds": 5,
       "period_seconds": 5
      },
      "resources": {
       "limits": {
        "cpu": "100m",
        "memory": "100Mi"
       },
       "requests": {
        "cpu": "100m",
        "memory": "100Mi"
       }
      },
      "startup_probe": {
       "failure_threshold": 30,
       "http_get": {
        "path": "/healthz",
        "port": "global"
       },
       "period_seconds": 10
      },
      "working_dir": "/work"
     }
    ],
    "restart_policy": "Never"
   }
  ],
  "image": {
   "id": "adlipoidu8yuahd6",
   "name": "app-image",
   "sign": "nkdy7pad09iuadjd",
   "tag": "v1.0.0"
  }
 }
'
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

{#update-appimage}

### 更新应用镜像模版

#### HTTP 路径

/api/v1/appimage/update

#### 请求（UpdateAppImageRequest）

注：请求使用增量更新策略，即只更新提供的字段，不影响未提供的字段。

| 字段               | 类型                                           | 选填 | 描述                                                                                   |
|------------------|----------------------------------------------|----|--------------------------------------------------------------------------------------|
| header           | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容                                                                              |
| name          | string                                       | 必填 | 应用镜像名称，满足 [RFC 1123 标签名规则要求](https://kubernetes.io/zh-cn/docs/concepts/overview/working-with-objects/names/#dns-label-names) |
| image       | [AppImageInfo](#AppImageInfo)                                       | 必填 | 基础镜像信息 ID                                                                                                                   |
| config_templates | [ConfigTemplate](#ConfigTemplate)                                        | 可选 | 应用启动依赖的配置模版信息，参考 [AppImage 概念](../concepts/appimage_cn.md)                                                                         |
| deploy_templates           | [DeployTemplate](#DeployTemplate)[]                              | 可选 | 应用部署模版配置信息                                                                                                                       |

#### 响应（UpdateAppImageResponse）

| 字段     | 类型                             | 描述   |
|--------|--------------------------------|------|
| status | [Status](summary_cn.md#status) | 状态信息 |

#### 请求示例

发起请求：

```sh
# Execute example in container
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/appimage/update' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "name": "appimage-template",
  "config_templates": {},
  "deploy_templates": [
   {
    "name": "app",
    "replicas": 1,
    "role": "server",
    "containers": [
     {
      "args": [
       "-c",
       "./app --role=server --task_config_path=./kuscia/task-config.conf"
      ],
      "command": [
       "sh"
      ],
      "config_volume_mounts": [
       {
        "mount_path": "/work/kuscia/task-config.conf",
        "sub_path": "task-config.conf"
       }
      ],
      "image_pull_policy": "IfNotPresent",
      "name": "app",
      "ports": [
       {
        "name": "global",
        "protocol": "HTTP",
        "scope": "Cluster"
       }
      ],
      "working_dir": "/work"
     }
    ],
    "restart_policy": "Never"
   }
  ],
  "image": {
   "id": "adlipoidu8yuahd6",
   "name": "app-image",
   "sign": "nkdy7pad09iuadjd",
   "tag": "v1.0.0"
  }
 }
'
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

{#delete-appimage}

### 删除应用镜像模版

#### HTTP 路径

/api/v1/appimage/delete

#### 请求（DeleteAppImageRequest）

| 字段        | 类型                                           | 选填 | 描述      |
|-----------|----------------------------------------------|----|---------|
| header    | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容 |
| name | string                                       | 必填 | 应用镜像名称  |

#### 响应（DeleteAppImageResponse）

| 字段     | 类型                             | 描述   |
|--------|--------------------------------|------|
| status | [Status](summary_cn.md#status) | 状态信息 |

#### 请求示例

发起请求：

```sh
# Execute example in container
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/appimage/delete' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "name": "appimage-template"
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

{#query-appimage}

### 查询应用镜像模版

#### HTTP 路径

/api/v1/appimage/query

#### 请求（QueryAppImageRequest）

| 字段        | 类型                                           | 选填 | 描述      |
|-----------|----------------------------------------------|----|---------|
| header    | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容 |
| name | string                                       | 必填 | 应用镜像模版名称   |

#### 响应（QueryAppImageResponse）

| 字段                         | 类型                                          | 描述                                                           |
|----------------------------|---------------------------------------------|--------------------------------------------------------------|
| status                     | [Status](summary_cn.md#status)              |  状态信息                                                         |
| data                       | QueryAppImageResponseData                   |                                                              |
| data.name                  | string                                      | 应用镜像名称 |
| data.image                 | [AppImageInfo](#AppImageInfo)               | 基础镜像信息                                                       |
| data.config_templates      | [ConfigTemplate](#ConfigTemplate)           | 应用启动依赖的配置模版信息，参考 [AppImage 概念](../concepts/appimage_cn.md)                                                                         |
| data.deploy_templates      | [DeployTemplate](#DeployTemplate)[]       | 应用部署模版配置信息                                                 |

#### 请求示例

```sh
# Execute example in container
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/appimage/query' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "name": "appimage-template"
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
    "name": "appimage-template",
    "image": {
      "name": "xxxxxx",
      "tag": "xxxx",
      "id": "",
      "sign": ""
    },
    "config_templates": {
      "task-config.conf": ""
    },
    "deploy_templates": [],
  }
}
```

{#batch-query-appimage}

### 批量查询节点状态

#### HTTP 路径

/api/v1/appimage/batch_query

#### 请求（BatchQueryAppImageRequest）

| 字段         | 类型                                           | 选填 | 描述           |
|------------|----------------------------------------------|----|--------------|
| header     | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容      |
| names | string[]                                     | 必填 | 待查询的appimage名 |

#### 响应（ BatchQueryAppImageResponse）

| 字段           | 类型                             | 描述   |
|--------------|--------------------------------|------|
| status       | [Status](summary_cn.md#status) | 状态信息 |
| data         | QueryAppImageResponseData[]   |     |
| data[].name                  | string                                      | 应用镜像名称 |
| data[].image                 | [AppImageInfo](#AppImageInfo)               | 基础镜像信息                                                       |
| data[].config_templates      | [ConfigTemplate](#ConfigTemplate)           | 应用启动依赖的配置模版信息，参考 [AppImage 概念](../concepts/appimage_cn.md)                                                                         |
| data[].deploy_templates      | [DeployTemplate](#DeployTemplate)[]       | 应用部署模版配置信息                                                 |

#### 请求示例

发起请求：

```sh
# Execute example in container
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/appimage/batchQuery' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "names": [
    "appimage-template"
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
  "data": [
      {
  "name": "appimage-template",
  "image": {
   "name": "xxxxxx",
   "tag": "xxxx",
   "id": "",
   "sign": ""
  },
  "config_templates": {
   "task-config.conf": ""
  },
  "deploy_templates": [],
      }
    ]
}
```

## 公共

{#AppImageInfo}

### AppImageInfo

| 字段                    | 类型                                          | 描述                                                           |
|-----------------------|---------------------------------------------|--------------------------------------------------------------|
| name             | string                                      |   应用镜像的名称信息                                                      |
| tag                  | string                                      | 应用镜像的 Tag 信息 |
| id                  | string                                      |   应用镜像的 ID 信息    |
| sign         | string                | 应用镜像的签名信息                                                     |

{#DeployTemplate}

### DeployTemplate

| 字段                    | 类型                                          | 描述                                                           |
|-----------------------|---------------------------------------------|--------------------------------------------------------------|
| name             | string                                      |   应用部署模版名称                                                      |
| role                  | string                                      | 应用作为该角色时，使用的部署模版配置 |
| replicas                  | int32                                      |   应用运行的副本数，默认为1    |
| restart_policy         | string                | 应用的重启策略, 支持Always，Never，OnFailure。Always：当容器终止退出后，总是重启容器；OnFailure：当容器终止异常退出（退出码非0）时，才重启容器；Never：当容器终止退出时，从不重启容器。                                                     |
| containers         | [Container](#Container)[]               | 应用容器配置信息                                                     |

### Container

| 字段                    | 类型                                          | 描述                                                           |
|-----------------------|---------------------------------------------|--------------------------------------------------------------|
| name             | string                                      |  应用容器的名称                                |
| command             | string[]                                      |   应用的启动命令                               |
| args             | string[]                                      |    应用的启动参数                              |
| working_dir             | string                                      |   应用容器的工作目录                               |
| config_volume_mounts             | [ConfigVolumeMount](#ConfigVolumeMount)                                      |    应用启动的挂载配置                              |
| ports             | [ContainerPort](#ContainerPort)[]                                      |   应用容器的启动端口色                               |
| env_from             | [EnvFromSource](#EnvFromSource)                                      |  使用envFrom为应用容器设置环境变量                                |
| env             | [EnvVar](#EnvVar)                                      |   使用env为应用容器设置环境变量                               |
| resources             | [ResourceRequirements](#ResourceRequirements)                                      |    应用容器申请的资源配置                              |
| liveness_probe             | [Probe](#Probe)                                      |   应用容器的存活探针配置                               |
| readiness_probe             | [Probe](#Probe)                                      |   应用容器的就绪探针配置                               |
| startup_probe             | [Probe](#Probe)                                      |   应用容器的启动探针配置                               |
| image_pull_policy             | string                                      |   应用容器的镜像拉取策略                               |

{#ConfigVolumeMount}

### ConfigVolumeMount

| 字段                    | 类型                                          | 描述                                                           |
|-----------------------|---------------------------------------------|--------------------------------------------------------------|
| mount_path             | string                                      |  文件挂载到容器中的路径                                |
| sub_path             | string                                      |   文件挂载到容器中的子路径                               |

{#ContainerPort}

### ContainerPort

| 字段                    | 类型                                          | 描述                                                           |
|-----------------------|---------------------------------------------|--------------------------------------------------------------|
| name             | string                                      |  应用容器的端口名称                                |
| protocol             | string                                      |   应用容器的端口使用的协议类型。支持两种类型：HTTP、GRPC                               |
| scope             | string                                      |    应用端口使用范围。支持三种模式：Cluster、Domain、Local                              |

{#EnvFromSource}

### EnvFromSource

| 字段                    | 类型                                          | 描述                                                           |
|-----------------------|---------------------------------------------|--------------------------------------------------------------|
| prefix             | string                                      | 添加在config_map的key上的前缀                                 |
| config_map_ref             |  object                                     |   使用config_map里的变量                               |
| config_map_ref.name             | string                                      |   config_map的名称                               |
| secret_ref             | object                                      |    使用secret里的变量                              |
| secret_ref.name             | string                                      |    secret的名称                              |

{#EnvVar}

### EnvVar

| 字段                    | 类型                                          | 描述                                                           |
|-----------------------|---------------------------------------------|--------------------------------------------------------------|
| name             | string                                      |  变量名                                |
| value             | string                                      |   变量值                               |

{#Probe}

### Probe

| 字段                    | 类型                                          | 描述                                                           |
|-----------------------|---------------------------------------------|--------------------------------------------------------------|
| exec             | [ExecAction](#ExecAction)                                      |  使用命令作为探针handler                                |
| http_get             | [HTTPGetAction](#HTTPGetAction)                                      |   使用HTTP请求作为探针handler                               |
| tcp_socket             | [TCPSocketAction](#TCPSocketAction)                                      |  使用TCP请求作为探针handler                                |
| grpc             | [GRPCAction](#GRPCAction)                                      |   使用GRPC请求作为探针handler                               |
| initial_delay_seconds             | int32                                      |  [探针初始化时间](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes)                                |
| timeout_seconds             | int32                                      |   探针超时时间                               |
| period_seconds             | int32                                      |  探针周期                                |
| success_threshold             | int32                                      |   探针成功阈值                               |
| failure_threshold             | int32                                      |  探针失败阈值                                |
| termination_grace_period_seconds             | int64                                      |   优雅中止时长                               |

{#ExecAction}

### ExecAction

| 字段                    | 类型                                          | 描述                                                           |
|-----------------------|---------------------------------------------|--------------------------------------------------------------|
| command             | string[]                                      |  命令                                |

{#HTTPGetAction}

### HTTPGetAction

| 字段                    | 类型                                          | 描述                                                           |
|-----------------------|---------------------------------------------|--------------------------------------------------------------|
| path             | string                                      |  HTTP路径                                |
| port             | string                                      |   HTTP端口                               |
| host             | string                                      |  HTTP域名                                |
| scheme             | string                                      |   HTTP协议                               |
| http_headers             | object[]                                      |  HTTP头                                |
| http_headers[].name             | string                                      |   HTTP头的键                               |
| http_headers[].value             | string                                      |   HTTP头的值                               |

{#TCPSocketAction}

### TCPSocketAction

| 字段                    | 类型                                          | 描述                                                           |
|-----------------------|---------------------------------------------|--------------------------------------------------------------|
| port             | string                                      |   TCP端口                               |
| host             | string                                      |  TCP域名                                |

{#GRPCAction}

### GRPCAction

| 字段                    | 类型                                          | 描述                                                           |
|-----------------------|---------------------------------------------|--------------------------------------------------------------|
| port             | string                                      |   GRPC端口                               |
| service             | string                                      |  GRPC服务名                               |
