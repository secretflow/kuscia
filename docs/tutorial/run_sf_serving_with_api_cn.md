# 如何使用 Kuscia API 运行一个 SecretFlow Serving

本教程将以 Secretflow Serving 内置测试模型为例，介绍如何基于 Kuscia API 运行一个多方的联合预测。

## 准备节点

准备节点请参考[快速入门](../getting_started/quickstart_cn.md)。

本示例在**中心化组网模式**下完成。在点对点组网模式下，证书的配置会有所不同。

{#cert-and-token}

## 确认证书和 Token

Kuscia API 使用双向 HTTPS，所以需要配置你的客户端库的双向 HTTPS 配置。

### 中心化组网模式

证书文件在 ${USER}-kuscia-master 节点的`/home/kuscia/var/certs/`目录下：

| 文件名               | 文件功能                                                |
| -------------------- | ------------------------------------------------------- |
| kusciaapi-server.key | 服务端私钥文件                                          |
| kusciaapi-server.crt | 服务端证书文件                                          |
| ca.crt               | CA 证书文件                                             |
| token                | 认证 Token ，在 headers 中添加 Token: { token 文件内容} |

### 点对点组网模式

这里以 alice 节点为例，接口需要的证书文件在 ${USER}-kuscia-autonomy-alice 节点的`/home/kuscia/var/certs/`目录下：

| 文件名               | 文件功能                                                |
| -------------------- | ------------------------------------------------------- |
| kusciaapi-server.key | 服务端私钥文件                                          |
| kusciaapi-server.crt | 服务端证书文件                                          |
| ca.crt               | CA 证书文件                                             |
| token                | 认证 Token ，在 headers 中添加 Token: { token 文件内容} |

## 准备 SecretFlow Serving 应用镜像模版

1. 登陆到 kuscia-master 节点容器中

```shell
docker exec -it ${USER}-kuscia-master bash
```

如果是点对点组网模式，则需要在 alice 和 bob 节点容器中分别创建上述应用的镜像模版 AppImage。

```shell
# 登陆到 alice 节点容器中
docker exec -it ${USER}-kuscia-autonomy-alice bash

# 登陆到 bob 节点容器中
docker exec -it ${USER}-kuscia-autonomy-bob bash
```

2. 获取 SecretFlow Serving 应用的镜像模版 AppImage

从 SecretFlow Serving 官方文档中，获取 AppImage 具体内容，并将其内容保存到 `secretflow-serving-image.yaml` 文件中。
具体模版内容，可参考 [Serving AppImage](https://www.secretflow.org.cn/zh-CN/docs/serving/0.2.0b0/topics/deployment/serving_on_kuscia)。

3. 创建 SecretFlow Serving 应用的镜像模版 AppImage

```shell
kubectl apply -f secretflow-serving-image.yaml
```


## 提交 SecretFlow Serving

下面以 alice 和 bob 两方为例，提交一个两方的联合预测。

1. 登陆到 kuscia-master 节点容器中

```shell
docker exec -it ${USER}-kuscia-master bash
```

如果是点对点组网模式，则需要进入任务发起方节点容器，以 alice 节点为例：

```shell
docker exec -it ${USER}-kuscia-autonomy-alice bash
```

2. 创建 SecretFlow Serving

我们请求[创建 Serving](../reference/apis/serving_cn.md#请求createservingrequest) 接口提交一个两方的联合预测。

在 kuscia-master 容器终端中，执行以下命令：

```shell
curl -X POST 'https://localhost:8082/api/v1/serving/create' \
--header "Token: $(cat /home/kuscia/var/certs/token)" \
--header 'Content-Type: application/json' \
--cert '/home/kuscia/var/certs/kusciaapi-server.crt' \
--key '/home/kuscia/var/certs/kusciaapi-server.key' \
--cacert '/home/kuscia/var/certs/ca.crt' \
-d '{
    "serving_id": "serving-glm-test-1",
    "initiator": "alice",
    "serving_input_config": "{\"partyConfigs\":{\"alice\":{\"serverConfig\":{\"featureMapping\":{\"v24\":\"x24\",\"v22\":\"x22\",\"v21\":\"x21\",\"v25\":\"x25\",\"v23\":\"x23\"}},\"modelConfig\":{\"modelId\":\"glm-test-1\",\"basePath\":\"/tmp/alice\",\"sourcePath\":\"/root/sf_serving/examples/alice/glm-test.tar.gz\",\"sourceType\":\"ST_FILE\"},\"featureSourceConfig\":{\"mockOpts\":{}},\"channel_desc\":{\"protocol\":\"http\"}},\"bob\":{\"serverConfig\":{\"featureMapping\":{\"v6\":\"x6\",\"v7\":\"x7\",\"v8\":\"x8\",\"v9\":\"x9\",\"v10\":\"x10\"}},\"modelConfig\":{\"modelId\":\"glm-test-1\",\"basePath\":\"/tmp/bob\",\"sourcePath\":\"/root/sf_serving/examples/bob/glm-test.tar.gz\",\"sourceType\":\"ST_FILE\"},\"featureSourceConfig\":{\"mockOpts\":{}},\"channel_desc\":{\"protocol\":\"http\"}}}}",
    "parties": [{
        "app_image": "secretflow-serving-image",
        "domain_id": "alice"
      },
      {
        "app_image": "secretflow-serving-image",
        "domain_id": "bob"
      }
    ]
}'
```

上述命令中 `serving_input_config` 字段定义了联合预测的相关配置。详细介绍可参考 [SecretFlow Serving 官方文档](https://www.secretflow.org.cn/zh-CN/docs/serving/0.2.0b0/topics/deployment/serving_on_kuscia)。

如果提交成功了，你将得到如下返回：

```json
{"status":{"code":0, "message":"success", "details":[]}}
```

恭喜，这说明 alice 和 bob 两方的联合预测已经成功创建。

如果遇到 HTTP 错误（即 HTTP Code 不为 200），请参考 [HTTP Error Code 处理](#http-error-code)。

此外，在 Kuscia 中，使用 KusciaDeployment 资源对 Serving 类型的常驻服务进行管理。详细介绍可参考 [KusciaDeployment](../reference/concepts/kusciadeployment_cn.md)。


{#query-sf-serving-status}

## 查询 SecretFlow Serving 状态

1. 登陆到 kuscia-master 节点容器中

```shell
docker exec -it ${USER}-kuscia-master bash
```

如果是点对点组网模式，需要进入节点容器中，以 alice 为例：

```shell
docker exec -it ${USER}-kuscia-autonomy-alice bash
```

2. 查询状态

我们可以通过请求[批量查询 Serving 状态](../reference/apis/serving_cn.md#批量查询-serving-状态) 接口来查询 Serving 的状态。

请求参数中 `serving_ids` 的值，需要填写前面创建过程中使用的 ID。

```shell
curl -k -X POST 'https://localhost:8082/api/v1/serving/status/batchQuery' \
--header "Token: $(cat /home/kuscia/var/certs/token)" \
--header 'Content-Type: application/json' \
--cert '/home/kuscia/var/certs/kusciaapi-server.crt' \
--key '/home/kuscia/var/certs/kusciaapi-server.key' \
--cacert '/home/kuscia/var/certs/ca.crt' \
-d '{
    "serving_ids": ["serving-glm-test-1"]
}' | jq
```

如果查询成功了，你将得到如下返回：

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
        "serving_id": "serving-glm-test-1",
        "status": {
          "state": "Available",
          "reason": "",
          "message": "",
          "total_parties": 2,
          "available_parties": 2,
          "create_time": "2024-03-08T08:36:42Z",
          "party_statuses": [
            {
              "domain_id": "alice",
              "role": "",
              "state": "Available",
              "replicas": 1,
              "available_replicas": 1,
              "unavailable_replicas": 0,
              "updatedReplicas": 1,
              "create_time": "2024-03-08T08:36:42Z",
              "endpoints": [
                {
                  "port_name": "communication",
                  "scope": "Cluster",
                  "endpoint": "serving-glm-test-1-communication.alice.svc"
                },
                {
                  "port_name": "brpc-builtin",
                  "scope": "Domain",
                  "endpoint": "serving-glm-test-1-brpc-builtin.alice.svc:53511"
                },
                {
                  "port_name": "internal",
                  "scope": "Domain",
                  "endpoint": "serving-glm-test-1-internal.alice.svc:53510"
                },
                {
                  "port_name": "service",
                  "scope": "Domain",
                  "endpoint": "serving-glm-test-1-service.alice.svc:53508"
                }
              ]
            },
            {
              "domain_id": "bob",
              "role": "",
              "state": "Available",
              "replicas": 1,
              "available_replicas": 1,
              "unavailable_replicas": 0,
              "updatedReplicas": 1,
              "create_time": "2024-03-08T08:36:42Z",
              "endpoints": [
                {
                  "port_name": "internal",
                  "scope": "Domain",
                  "endpoint": "serving-glm-test-1-internal.bob.svc:53510"
                },
                {
                  "port_name": "service",
                  "scope": "Domain",
                  "endpoint": "serving-glm-test-1-service.bob.svc:53508"
                },
                {
                  "port_name": "communication",
                  "scope": "Cluster",
                  "endpoint": "serving-glm-test-1-communication.bob.svc"
                },
                {
                  "port_name": "brpc-builtin",
                  "scope": "Domain",
                  "endpoint": "serving-glm-test-1-brpc-builtin.bob.svc:53511"
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

其中部分字段含义如下：

* `data.servings[0].status.state`：表示 Serving 的全局状态，当前状态为 Available。
* `data.servings[0].status.party_statuses[0].state`： 表示 alice 方 Serving 的状态，当前状态为 Available。
* `data.servings[0].status.party_statuses[1].state`： 表示 bob 方 Serving 的状态，当前状态为 Available。
* `data.servings[0].status.party_statuses[0].endpoints`：表示 alice 方应用对外提供的访问地址信息。
* `data.servings[0].status.party_statuses[1].endpoints`：表示 bob 方应用对外提供的访问地址信息。

上述字段详细介绍，请参考[批量查询 Serving 状态](../reference/apis/serving_cn.md#批量查询-serving-状态)。


## 使用 SecretFlow Serving 进行预测

下面以 alice 为例，使用内置模型进行预测。在发起预测请求之前，请确保 Serving 的全局状态为 Available。

1. 获取 alice 方 Serving 应用访问地址

根据前面[查询 SecretFlow Serving 状态](#query-sf-serving-status)，获取 alice 方 Serving 应用对外提供的访问地址，这里需要选择 `port_name` 为 `service` 的 endpoint，
当前示例为 `serving-glm-test-1-service.alice.svc:53508`。

2. 登陆到 alice 节点容器中

```shell
docker exec -it ${USER}-kuscia-lite-alice bash
```

如果是点对点组网模式，命令如下：

```shell
docker exec -it ${USER}-kuscia-autonomy-alice bash
```

3. 发起预测请求

发起预测请求时，需要配置以下三个 Header：

* `Host: {服务地址}`
* `Kuscia-Source: {alice 节点的 Domain ID}`
* `Content-Type: application/json`

```shell
curl --location 'http://127.0.0.1/PredictionService/Predict' \
--header 'Host: serving-glm-test-1-service.alice.svc:53508' \
--header 'Kuscia-Source: alice' \
--header 'Content-Type: application/json' \
--data '{
    "service_spec": {
        "id": "serving-glm-test-1"
    },
    "fs_params": {
        "alice": {
            "query_datas": [
                "a",
                "b",
                "c"
            ],
            "query_context": "test"
        },
        "bob": {
            "query_datas": [
                "a",
                "b",
                "c"
            ],
            "query_context": "test"
        }
    }
}'
```

上述命令中请求内容的详细介绍可参考 [SecretFlow Serving 官方文档](https://www.secretflow.org.cn/zh-CN/docs/serving/0.2.0b0/topics/deployment/serving_on_kuscia)。

如果预测成功了，你将得到如下返回：

```json
{"header":{"data":{}},"status":{"code":1,"msg":""},"service_spec":{"id":"serving-glm-test-1"},"results":[{"scores":[{"name":"score","value":0.9553803827339434}]},{"scores":[{"name":"score","value":0.9553803827339434}]},{"scores":[{"name":"score","value":0.9553803827339434}]}]}
```

## 更新 SecretFlow Serving

1. 登陆到 kuscia-master 节点容器中

```shell
docker exec -it ${USER}-kuscia-master bash
```

如果是点对点组网模式，需要进入任务发起方节点容器中，以 alice 为例：

```shell
docker exec -it ${USER}-kuscia-autonomy-alice bash
```

2. 更新 Serving

当想要更新 SecretFlow Serving 的镜像或输入参数时，我们可以通过请求[更新 Serving](../reference/apis/serving_cn.md#更新-serving) 接口来更新指定的 Serving。

请求参数中 `serving_id` 的值，需要填写前面创建过程中使用的 ID。

```shell
curl -k -X POST 'https://localhost:8082/api/v1/serving/update' \
--header "Token: $(cat /home/kuscia/var/certs/token)" \
--header 'Content-Type: application/json' \
--cert '/home/kuscia/var/certs/kusciaapi-server.crt' \
--key '/home/kuscia/var/certs/kusciaapi-server.key' \
--cacert '/home/kuscia/var/certs/ca.crt' \
-d '{
    "serving_id": "serving-glm-test-1",
    "serving_input_config": "{serving 的输入配置}",
    "parties": [{
        "app_image": "{新的 AppImage 名称}",
        "domain_id": "alice"
      },
      {
        "app_image": "{新的 AppImage 名称}",
        "domain_id": "bob"
      }
    ]
}'
```


## 删除 SecretFlow Serving

1. 登陆到 kuscia-master 节点容器中

```shell
docker exec -it ${USER}-kuscia-master bash
```

如果是点对点组网模式，需要进入任务发起方节点容器中，以 alice 为例：

```shell
docker exec -it ${USER}-kuscia-autonomy-alice bash
```

2. 删除 Serving

我们可以通过请求[删除 Serving](../reference/apis/serving_cn.md#删除-serving) 接口来删除指定的 Serving。

请求参数中 `serving_id` 的值，需要填写前面创建过程中使用的 ID。

```shell
curl -k -X POST 'https://localhost:8082/api/v1/serving/delete' \
--header "Token: $(cat /home/kuscia/var/certs/token)" \
--header 'Content-Type: application/json' \
--cert '/home/kuscia/var/certs/kusciaapi-server.crt' \
--key '/home/kuscia/var/certs/kusciaapi-server.key' \
--cacert '/home/kuscia/var/certs/ca.crt' \
-d '{
    "serving_id": "serving-glm-test-1"
}'
```

如果删除成功了，你将得到如下返回：

```json
{"status":{"code":0, "message":"success", "details":[]}}
```

## 参考

### 如何查看 Serving 应用容器日志

在 Kuscia 中，可以登陆到节点容器内查看 Serving 应用容器的日志。具体方法如下。

1. 登陆到节点容器中，以 alice 为例：

```shell
docker exec -it ${USER}-kuscia-lite-alice bash
```

如果是点对点组网模式，需要登陆到对应 autonomy 容器中。

```shell
docker exec -it ${USER}-kuscia-autonomy-alice bash
```

2. 查看日志

在目录 `/home/kuscia/var/stdout/pods` 下可以看到对应 Serving 应用容器的目录。后续进入到相应目录下，即可查看应用的日志。

```yaml
# 查看当前应用容器的目录
ls /home/kuscia/var/stdout/pods

# 查看应用容器的日志，示例如下:
cat /home/kuscia/var/stdout/pods/alice_serving-glm-test-1-75d449f848-4gth9_2bd2f45f-51d8-4a18-afa2-ed7105bd47b5/secretflow/0.log
```


{#http-client-error}

### HTTP 客户端错误处理

#### curl: (56)

curl: (56) OpenSSL SSL_read: error:14094412:SSL routines:ssl3_read_bytes:sslv3 alert bad certificate, errno 0

未配置 SSL 证书和私钥。请[确认证书和 Token](#cert-and-token).

#### curl: (58)

curl: (58) unable to set XXX file

SSL 私钥、 SSL 证书或 CA 证书文件路径错误。请[确认证书和 Token](#cert-and-token).

{#http-error-code}

### HTTP Error Code 处理

#### 401 Unauthorized

身份认证失败。请检查是否在 Headers 中配置了正确的 Token 。 Token 内容详见[确认证书和 Token](#cert-and-token).

#### 404 Page Not Found

接口 path 错误。请检查请求的 path 是否和文档中的一致。必要时可以提 issue 询问。