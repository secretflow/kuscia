# 如何使用 Kuscia API 运行一个 SecretFlow 作业

## 准备节点

准备节点请参考[快速入门](../getting_started/quickstart_cn.md)。

本示例在**中心化组网模式**下完成。在点对点组网模式下，证书的配置会有所不同。

{#cert-and-token}

## 确认证书和 Token

Kuscia API 使用双向 HTTPS，所以需要配置您的客户端库的双向 HTTPS 配置。

### 中心化组网模式

证书文件在 ${USER}-kuscia-master 节点的 `/home/kuscia/var/certs/` 目录下：

| 文件名               | 文件功能                                                |
| -------------------- | ------------------------------------------------------- |
| kusciaapi-server.key | 服务端私钥文件                                          |
| kusciaapi-server.crt | 服务端证书文件                                          |
| ca.crt               | CA 证书文件                                             |
| token                | 认证 Token ，在 headers 中添加 Token: { token 文件内容} |

### 点对点组网模式

证书的配置参考[配置授权](../deployment/Docker_deployment_kuscia/deploy_p2p_cn.md#配置授权)

这里以 Alice 节点为例，接口需要的证书文件在 ${USER}-kuscia-autonomy-alice 节点的 `/home/kuscia/var/certs/` 目录下：

| 文件名               | 文件功能                                                |
| -------------------- | ------------------------------------------------------- |
| kusciaapi-server.key | 服务端私钥文件                                          |
| kusciaapi-server.crt | 服务端证书文件                                          |
| ca.crt               | CA 证书文件                                             |
| token                | 认证 Token ，在 headers 中添加 Token: { token 文件内容} |

同时，还要保证节点间的授权证书配置正确，Alice 节点和 Bob 节点要完成授权的建立，否则双方无法共同参与计算任务。

## 准备数据

您可以使用 Kuscia 中自带的数据文件，或者使用您自己的数据文件。

在 Kuscia 中，节点数据文件的存放路径为节点容器的 `/home/kuscia/var/storage`，您可以在容器中查看这个数据文件。

{#kuscia}

### 查看 Kuscia 示例数据

这里以 Alice 节点为例，首先进入节点容器：

```shell
docker exec -it ${USER}-kuscia-lite-alice bash
```

在 Alice 节点容器中查看节点示例数据：

```shell
cat /home/kuscia/var/storage/data/alice.csv
```

Bob 节点同理。

{#prepare-your-own-data}

### 准备您自己的数据

您也可以使用您自己的数据文件，首先您要将您的数据文件复制到节点容器中，还是以 Alice 节点为例：

```shell
docker cp {your_alice_data} ${USER}-kuscia-lite-alice:/home/kuscia/var/storage/data/
```

然后，您还需要参考[Kuscia API](../reference/apis/domaindata_cn.md#create-domain-data)给新的数据文件创建 domaindata。

接下来您可以像[查看 Kuscia 示例数据](#kuscia)一样查看您的数据文件，这里不再赘述。

{#configure-kuscia-job}

## 配置 KusciaJob

我们需要在 kuscia-master 节点容器中配置和运行 Job，首先，让我们先进入 kuscia-master 节点容器：

```shell
docker exec -it ${USER}-kuscia-master bash
```

如果是点对点组网模式，则需要进入任务发起方节点容器，以 Alice 节点为例：

```shell
docker exec -it ${USER}-kuscia-autonomy-alice
```

注意，您只能向已和 Alice 节点建立了授权的节点发布计算任务。

### 使用 Kuscia 示例数据配置 KusciaJob

此处以[KusciaJob 示例](../reference/apis/kusciajob_cn.md#请求示例)作为任务示例展示，该任务流完成 2 个任务：

1. job-psi 读取 Alice 和 Bob 的数据文件，进行隐私求交，求交的结果分别保存为两个参与方的 `psi-output.csv`。
2. job-split 读取 Alice 和 Bob 上一步中求交的结果文件，并拆分成训练集和测试集，分别保存为两个参与方的 `train-dataset.csv`、`test-dataset.csv`。

这个 KusciaJob 的名称为 job-best-effort-linear，在一个 Kuscia 集群中，这个名称必须是唯一的，由 `job_id` 指定。

我们请求[创建 Job](../reference/apis/kusciajob_cn.md#请求createjobrequest) 接口来创建并运行这个 KusciaJob。

具体字段数据格式和含义请参考[创建 Job](../reference/apis/kusciajob_cn.md#请求createjobrequest) ，本文不再赘述。

如果您成功了，您将得到如下返回：

```json
{ "status": { "code": 0, "message": "success", "details": [] }, "data": { "job_id": "job-best-effort-linear" } }
```

恭喜，这说明 KusciaJob 已经成功创建并运行。

如果遇到 HTTP 错误（即 HTTP Code 不为 200），请参考 [HTTP Error Code 处理](#http-error-code)。

### 使用您自己的数据配置 KusciaJob

如果您要使用您自己的数据，可以将两个算子中的 `taskInputConfig.sf_input_ids` 的数据文件 `id` 修改为您在 [准备您自己的数据](#prepare-your-own-data) 中的 `domaindata_id` 即可。

### 更多相关

更多有关 KusciaJob 配置的信息，请查看 [KusciaJob](../reference/concepts/kusciajob_cn.md) 和[算子参数描述](#input-config) 。
前者描述了 KusciaJob 的定义和相关说明，后者描述了支持的算子和参数。

## 查看 KusciaJob 运行状态

{#job-query}

### 查看运行中的 KusciaJob 的详细状态

job-best-effort-linear 是您在[配置 Job](#configure-kuscia-job) 中指定的 KusciaJob 的名称。

我们请求[批量查询 Job 状态](../reference/apis/kusciajob_cn.md#批量查询-job-状态)接口来批量查询 KusciaJob
的状态。

请求参数 `job_ids` 是一个 Array[String] ，需要列出所有待查询的 KusciaJob 名称。

```shell
curl -k -X POST 'https://localhost:8082/api/v1/job/status/batchQuery' \
--header "Token: $(cat /home/kuscia/var/certs/token)" \
--header 'Content-Type: application/json' \
--cert '/home/kuscia/var/certs/kusciaapi-server.crt' \
--key '/home/kuscia/var/certs/kusciaapi-server.key' \
--cacert '/home/kuscia/var/certs/ca.crt' \
-d '{
    "job_ids": ["job-best-effort-linear"]
}'
```

如果任务成功了，您可以得到如下返回：

```json
{
  "status": {
    "code": 0,
    "message": "success",
    "details": []
  },
  "data": {
    "jobs": [
      {
        "job_id": "job-best-effort-linear",
        "status": {
          "state": "Succeeded",
          "err_msg": "",
          "create_time": "2023-07-27T01:55:46Z",
          "start_time": "2023-07-27T01:55:46Z",
          "end_time": "2023-07-27T01:56:19Z",
          "tasks": [
            {
              "task_id": "job-psi",
              "state": "Succeeded",
              "err_msg": "",
              "create_time": "2023-07-27T01:55:46Z",
              "start_time": "2023-07-27T01:55:46Z",
              "end_time": "2023-07-27T01:56:05Z",
              "parties": [
                {
                  "domain_id": "alice",
                  "state": "Succeeded",
                  "err_msg": "",
                  "endpoints": [
                    {
                      "port_name": "spu",
                      "scope": "Cluster",
                      "endpoint": "job-psi-0-spu.alice.svc"
                    },
                    {
                      "port_name": "fed",
                      "scope": "Cluster",
                      "endpoint": "job-psi-0-fed.alice.svc"
                    },
                    {
                      "port_name": "global",
                      "scope": "Domain",
                      "endpoint": "job-psi-0-global.alice.svc:8081"
                    }
                  ]
                },
                {
                  "domain_id": "bob",
                  "state": "Succeeded",
                  "err_msg": "",
                  "endpoints": [
                    {
                      "port_name": "fed",
                      "scope": "Cluster",
                      "endpoint": "job-psi-0-fed.bob.svc"
                    },
                    {
                      "port_name": "global",
                      "scope": "Domain",
                      "endpoint": "job-psi-0-global.bob.svc:8081"
                    },
                    {
                      "port_name": "spu",
                      "scope": "Cluster",
                      "endpoint": "job-psi-0-spu.bob.svc"
                    }
                  ]
                }
              ]
            },
            {
              "task_id": "job-split",
              "state": "Succeeded",
              "err_msg": "",
              "create_time": "2023-07-27T01:56:05Z",
              "start_time": "2023-07-27T01:56:05Z",
              "end_time": "2023-07-27T01:56:19Z",
              "parties": [
                {
                  "domain_id": "alice",
                  "state": "Succeeded",
                  "err_msg": "",
                  "endpoints": [
                    {
                      "port_name": "spu",
                      "scope": "Cluster",
                      "endpoint": "job-split-0-spu.alice.svc"
                    },
                    {
                      "port_name": "fed",
                      "scope": "Cluster",
                      "endpoint": "job-split-0-fed.alice.svc"
                    },
                    {
                      "port_name": "global",
                      "scope": "Domain",
                      "endpoint": "job-split-0-global.alice.svc:8081"
                    }
                  ]
                },
                {
                  "domain_id": "bob",
                  "state": "Succeeded",
                  "err_msg": "",
                  "endpoints": [
                    {
                      "port_name": "fed",
                      "scope": "Cluster",
                      "endpoint": "job-psi-0-fed.bob.svc"
                    },
                    {
                      "port_name": "global",
                      "scope": "Domain",
                      "endpoint": "job-psi-0-global.bob.svc:8081"
                    },
                    {
                      "port_name": "spu",
                      "scope": "Cluster",
                      "endpoint": "job-psi-0-spu.bob.svc"
                    }
                  ]
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

`data.jobs.status.state` 字段记录了 KusciaJob 的运行状态，`data.jobs.status.tasks.state` 则记录了每个 KusciaTask 的运行状态。

详细信息请参考 [KusciaJob](../reference/concepts/kusciajob_cn.md)
和[批量查询 Job 状态](../reference/apis/kusciajob_cn.md#批量查询-job-状态)

## 删除 KusciaJob

当您想清理这个 KusciaJob 时，我们请求[删除 Job](../reference/apis/kusciajob_cn.md#删除-job) 接口来删除这个
KusciaJob.

```shell
curl -k -X POST 'https://localhost:8082/api/v1/job/delete' \
--header "Token: $(cat /home/kuscia/var/certs/token)" \
--header 'Content-Type: application/json' \
--cert '/home/kuscia/var/certs/kusciaapi-server.crt' \
--key '/home/kuscia/var/certs/kusciaapi-server.key' \
--cacert '/home/kuscia/var/certs/ca.crt' \
-d '{
    "job_id": "job-best-effort-linear"
}'
```

如果任务成功了，您可以得到如下返回：

```json
{ "status": { "code": 0, "message": "success", "details": [] }, "data": { "job_id": "job-best-effort-linear" } }
```

当这个 KusciaJob 被清理时， 这个 KusciaJob 创建的 KusciaTask 也会一起被清理。

{#input-config}

## 算子参数描述

KusciaJob 的算子参数由 `taskInputConfig` 字段定义，对于不同的算子，算子的参数不同。

对于 secretflow ，请参考：[Secretflow 官网](https://www.secretflow.org.cn/)。

{#http-client-error}

## HTTP 客户端错误处理

### curl: (56)

curl: (56) OpenSSL SSL_read: error:14094412:SSL routines:ssl3_read_bytes:sslv3 alert bad certificate, errno 0

未配置 SSL 证书和私钥。请[确认证书和 Token](#cert-and-token).

### curl: (58)

curl: (58) unable to set XXX file

SSL 私钥、 SSL 证书或 CA 证书文件路径错误。请[确认证书和 Token](#cert-and-token).

{#http-error-code}

## HTTP Error Code 处理

### 401 Unauthorized

身份认证失败。请检查是否在 Headers 中配置了正确的 Token 。 Token 内容详见[确认证书和 Token](#cert-and-token).

### 404 Page Not Found

接口 path 错误。请检查请求的 path 是否和文档中的一致。必要时可以提 issue 询问。
