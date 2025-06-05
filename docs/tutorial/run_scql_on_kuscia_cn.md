# 如何在 Kuscia 上运行 SCQL 联合分析任务

本教程将以 [KusciaAPI](../reference/apis/summary_cn.md) 创建本地数据源作为示例，介绍如何在 Kuscia 上运行 SCQL 联合分析任务。

## 准备节点

- 体验部署请选择[快速入门](../getting_started/quickstart_cn.md)。
- 生产部署请选择[多机部署](../deployment/Docker_deployment_kuscia/index.rst)。

本示例在**点对点组网模式**下完成。在中心化组网模式下，证书的配置会有所不同。

{#cert-and-token}

## 获取 KusciaAPI 证书和 Token

在下面[准备数据](./run_scql_on_kuscia_cn.md#alice-准备测试数据)步骤中需要使用到 KusciaAPI，如果 KusciaAPI 启用了 MTLS 协议，则需要提前准备好 MTLS 证书和 Token。协议参考[这里](../troubleshoot/concept/protocol_describe.md)。

### 点对点组网模式

证书的配置参考[配置授权](../deployment/Docker_deployment_kuscia/deploy_p2p_cn.md#配置授权)

这里以 Alice 节点为例，接口需要的证书文件在 ${USER}-kuscia-autonomy-alice 节点的 `/home/kuscia/var/certs/` 目录下：

| 文件名               | 文件功能                                                |
| -------------------- | ------------------------------------------------------- |
| kusciaapi-server.key | 服务端私钥文件                                          |
| kusciaapi-server.crt | 服务端证书文件                                          |
| ca.crt               | CA 证书文件                                             |
| token                | 认证 Token ，在 headers 中添加 Token: { token 文件内容} |

### 中心化组网模式

证书文件在 ${USER}-kuscia-master 节点的 `/home/kuscia/var/certs/` 目录下：

| 文件名               | 文件功能                                                |
| -------------------- | ------------------------------------------------------- |
| kusciaapi-server.key | 服务端私钥文件                                          |
| kusciaapi-server.crt | 服务端证书文件                                          |
| ca.crt               | CA 证书文件                                             |
| token                | 认证 Token ，在 headers 中添加 Token: { token 文件内容} |

## 准备数据

您可以使用本文示例的测试数据文件，或者使用您自己的数据文件。

在 Kuscia 中，在节点容器的 `/home/kuscia/var/storage` 目录存放内置测试数据文件，下面 Alice 和 Bob 节点分别使用的是 scql-alice.csv 和 scql-bob.csv，您可以在容器中查看这两个数据文件。

### 准备测试数据

#### Alice 准备测试数据

1. 这里以 Docker 部署模式为例，登录到 alice 节点中

    ```bash
    docker exec -it ${USER}-kuscia-autonomy-alice bash
    ```

2. 创建 DomainDataSource

    下面 datasource_id 名称以 scql-demo-local-datasource 为例：

    ```bash
    export CTR_CERTS_ROOT=/home/kuscia/var/certs
    curl -k -X POST 'https://localhost:8082/api/v1/domaindatasource/create' \
     --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
     --header 'Content-Type: application/json' \
     --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
     --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
     --cacert ${CTR_CERTS_ROOT}/ca.crt \
     -d '{
      "domain_id": "alice",
      "datasource_id":"scql-demo-local-datasource",
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

    :::{tip}
    K8S RunK 模式部署 Kuscia 时，此处需要使用 [OSS 数据源](../reference/apis/domaindatasource_cn.md#id5)，并将 /home/kuscia/var/storage/data/scql-alice.csv 示例数据放入 OSS 中。
    :::

3. 创建 DomainData

    下面 domaindata_id 名称以 scql-alice-table 为例：

    ```bash
    export CTR_CERTS_ROOT=/home/kuscia/var/certs
    curl -k -X POST 'https://localhost:8082/api/v1/domaindata/create' \
     --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
     --header 'Content-Type: application/json' \
     --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
     --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
     --cacert ${CTR_CERTS_ROOT}/ca.crt \
     -d '{
      "domain_id": "alice",
      "domaindata_id": "scql-alice-table",
      "datasource_id": "scql-demo-local-datasource",
      "name": "alice001",
      "type": "table",
      "relative_uri": "scql-alice.csv",
      "columns": [
        {
          "name": "ID",
          "type": "str"
        },
        {
          "name": "credit_rank",
          "type": "int"
        },
        {
          "name": "income",
          "type": "int"
        },
        {
          "name": "age",
          "type": "int"
        }
      ]
    }'
    ```

#### Bob 准备测试数据

1. 这里以 Docker 部署模式为例，登录到 Bob 节点中

    ```bash
    docker exec -it ${USER}-kuscia-autonomy-bob bash
    ```

2. 创建 DomainDataSource

    下面 datasource_id 名称以 scql-demo-local-datasource 为例：

    ```bash
    export CTR_CERTS_ROOT=/home/kuscia/var/certs
    curl -k -X POST 'https://localhost:8082/api/v1/domaindatasource/create' \
     --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
     --header 'Content-Type: application/json' \
     --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
     --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
     --cacert ${CTR_CERTS_ROOT}/ca.crt \
     -d '{
      "domain_id": "bob",
      "datasource_id":"scql-demo-local-datasource",
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

    :::{tip}
    K8S RunK 模式部署 Kuscia 时，此处需要使用 [OSS 数据源](../reference/apis/domaindatasource_cn.md#id5)，并将 /home/kuscia/var/storage/data/scql-bob.csv 示例数据放入 OSS 中。
    :::

3. 创建 DomainData

    下面 domaindata_id 名称以 scql-bob-table 为例：

    ```bash
    export CTR_CERTS_ROOT=/home/kuscia/var/certs
    curl -k -X POST 'https://localhost:8082/api/v1/domaindata/create' \
     --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
     --header 'Content-Type: application/json' \
     --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
     --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
     --cacert ${CTR_CERTS_ROOT}/ca.crt \
     -d '{
      "domain_id": "bob",
      "domaindata_id": "scql-bob-table",
      "datasource_id": "scql-demo-local-datasource",
      "name": "bob001",
      "type": "table",
      "relative_uri": "scql-bob.csv",
      "columns": [
        {
          "name": "ID",
          "type": "str"
        },
        {
          "name": "order_amount",
          "type": "int"
        },
        {
          "name": "is_active",
          "type": "int"
        }
      ]
    }'
    ```

## 部署 SCQL

### Alice 部署 SCQL

1. 登陆到 alice 节点容器中

    ```bash
    docker exec -it ${USER}-kuscia-autonomy-alice bash
    ```

    如果是中心化组网模式，则需要登录到 master 节点容器中。

    ```bash
    docker exec -it ${USER}-kuscia-master bash
    ```

2. 获取 SCQL 应用的镜像模版 AppImage

    从 SCQL 官方文档中，获取 AppImage 具体内容，并将其内容保存到 scql-image.yaml 文件中。 具体模版内容，可参考 [SCQL AppImage](https://www.secretflow.org.cn/zh-CN/docs/scql/main/topics/deployment/run-scql-on-kuscia)。

    > 注意：
    >
    > 1. 如果 `secretflow/scql` 仓库访问网速较慢，可以替换为 `secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/scql`。
    > 2. 请删除 `#--datasource_router=kusciadatamesh` 代码行前面的 # 符号，以启用 Datamesh 本地数据源配置。
    > 3. 在 `engineConf` 字段加上 `--enable_restricted_read_path=false` 限制 csv 文件的读取路径。
    > 4. K8S RunK 模式部署 Kuscia 时，需要使用 MySQL 存储 Broker 元数据。修改 `storage` 字段的 `type` 为 MySQL 和 `conn_str` 对应的数据库连接字符串。

3. 创建 SCQL 应用的镜像模版 AppImage

```bash
kubectl apply -f scql-image.yaml
```

4. 部署 Broker

```bash
kubectl apply -f /home/kuscia/scripts/templates/scql/broker_alice.yaml
```

### Bob 部署 SCQL

1. 登陆到 Bob 节点容器中

    ```bash
    docker exec -it ${USER}-kuscia-autonomy-bob bash
    ```

    如果是中心化组网模式，则需要登录到 master 节点容器中。

2. ```bash
    docker exec -it ${USER}-kuscia-master bash
    ```

2. 获取 SCQL 应用的镜像模版 AppImage

    从 SCQL 官方文档中，获取 AppImage 具体内容，并将其内容保存到 scql-image.yaml 文件中。 具体模版内容，可参考 [SCQL AppImage](https://www.secretflow.org.cn/zh-CN/docs/scql/main/topics/deployment/run-scql-on-kuscia)。

    > 注意：
    >
    > 1. 如果 `secretflow/scql` 仓库访问网速较慢，可以替换为 `secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/scql`。
    > 2. 请删除 `#--datasource_router=kusciadatamesh` 代码行前面的 # 符号，以启用 Datamesh 本地数据源配置。
    > 3. 在 `engineConf` 字段加上 `--enable_restricted_read_path=false` 限制 csv 文件的读取路径。
    > 4. K8S RunK 模式部署 Kuscia 时，需要使用 MySQL 存储 Broker 元数据。修改 `storage` 字段的 `type` 为 MySQL 和 `conn_str` 对应的数据库连接字符串。
    > 5. 如果 AppImage 配置有改动可以重启 Kuscia 或重新创建 Broker 使配置生效。示例命令：`kubectl delete KusciaDeployment scql -n cross-domain` `kubectl apply -f broker-deploy.yaml` 。

3. 创建 SCQL 应用的镜像模版 AppImage

    ```bash
    kubectl apply -f appimage.yaml
    ```

4. 部署 Broker

    ```bash
    kubectl apply -f /home/kuscia/scripts/templates/scql/broker_bob.yaml
    ```

### 查看 broker 是否部署成功

下面以 Alice 节点为例，Bob 节点类似

```bash
docker exec -it ${USER}-kuscia-autonomy-alice kubectl get po -A

# When the Pod status is Running, it indicates that the deployment was successful:
NAMESPACE   NAME                           READY   STATUS    RESTARTS   AGE
alice       scql-broker-6f4f85b64f-fsgq8   1/1     Running   0          2m42s
```

## 使用 SCQL 进行联合分析

下面仅以流程步骤作为示例展示，更多接口参数请参考 [SCQL API](https://www.secretflow.org.cn/zh-CN/docs/scql/main/reference/broker-api)。

### 创建项目并邀请参与方加入

#### Alice 创建项目，并邀请 Bob 加入

1. 登录到 Alice 节点容器中

    ```bash
    docker exec -it ${USER}-kuscia-autonomy-alice bash
    ```

2. 创建项目

    下面项目名称以 "demo" 为例：

    ```bash
    curl -X POST http://127.0.0.1:80/intra/project/create \
    --header "host: scql-broker-intra.alice.svc" \
    --header "kuscia-source: alice" \
    -d '{
        "project_id":"demo",
        "name":"demo",
        "conf":{
            "spu_runtime_cfg":{
            "protocol":"SEMI2K",
            "field":"FM64"
            }
        },
       "description":"this is a project"
    }'
    ```

3. 查看项目

    ```bash
    curl -X POST http://127.0.0.1:80/intra/project/list \
    --header "host: scql-broker-intra.alice.svc" \
    --header "kuscia-source: alice"
    ```

4. 邀请 Bob 加入到 "demo" 项目中

    ```bash
    curl -X POST http://127.0.0.1:80/intra/member/invite \
    --header "host: scql-broker-intra.alice.svc" \
    --header "kuscia-source: alice" \
    -d '{
        "invitee": "bob",
        "project_id": "demo"
    }'
    ```

5. 查看邀请状态

    ```bash
    curl -X POST http://127.0.0.1:80/intra/invitation/list \
    --header "host: scql-broker-intra.alice.svc" \
    --header "kuscia-source: alice"
    ```

#### Bob 接受邀请

1. 登录到 Bob 节点容器中

    ```bash
    docker exec -it ${USER}-kuscia-autonomy-bob bash
    ```

2. Bob 接受 Alice 的入项邀请

    ```bash
    curl -X POST http://127.0.0.1:80/intra/invitation/process \
    --header "host: scql-broker-intra.bob.svc" \
    --header "kuscia-source: bob" \
    -d '{
        "invitation_id":1,
        "respond":0
    }'
    ```

### 创建数据表

#### Alice 创建数据表

1. 登录到 Alice 节点容器中

    ```bash
    docker exec -it ${USER}-kuscia-autonomy-alice bash
    ```

2. 创建数据表

> 下面 table_name 以 ta 为例，ref_table 参数的值为[创建 DomainData](./run_scql_on_kuscia_cn.md#alice-准备测试数据)时的 `domaindata_id`

```bash
curl -X POST http://127.0.0.1:80/intra/table/create \
--header "host: scql-broker-intra.alice.svc" \
--header "kuscia-source: alice" \
-H "Content-Type: application/json" \
-d '{
    "project_id": "demo",
    "table_name": "ta",
    "ref_table": "scql-alice-table",
    "db_type": "csvdb",
    "columns": [
        {"name":"ID","dtype":"string"},
        {"name":"credit_rank","dtype":"int"},
        {"name":"income","dtype":"int"},
        {"name":"age","dtype":"int"}
    ]
}'
```

#### Bob 创建数据表

1. 登录到 Bob 节点容器中

    ```bash
        docker exec -it ${USER}-kuscia-autonomy-bob bash
    ```

2. 创建数据表

> 下面 table_name 以 ta 为例，ref_table 参数的值为[创建 DomainData](./run_scql_on_kuscia_cn.md#bob-准备测试数据)时的 `domaindata_id`

```bash
curl -X POST http://127.0.0.1:80/intra/table/create \
--header "host: scql-broker-intra.bob.svc" \
--header "kuscia-source: bob" \
-H "Content-Type: application/json" \
-d '{
    "project_id": "demo",
    "table_name": "tb",
    "ref_table": "scql-bob-table",
    "db_type": "csvdb",
    "columns": [
        {"name":"ID","dtype":"string"},
        {"name":"order_amount","dtype":"double"},
        {"name":"is_active","dtype":"int"}
    ]
}'
```

### 查看数据表

下面以 Alice 为例，Bob 节点类似

```bash
curl -X POST http://127.0.0.1:80/intra/table/list \
--header "host: scql-broker-intra.alice.svc" \
--header "kuscia-source: alice" \
-H "Content-Type: application/json" \
-d '{
    "project_id": "demo"
}'
```

### 删除数据表

若想删除创建的数据表时，可以参考下面命令。以 Alice 节点为例，Bob 节点类似。

```bash
curl -X POST http://127.0.0.1:80/intra/table/drop \
--header "host: scql-broker-intra.alice.svc" \
--header "kuscia-source: alice" \
-H "Content-Type: application/json" \
-d '{
    "project_id": "demo",
    "table_name":"ta"
}'
```

### 数据表授权

#### Alice 的数据表授权

1. 将 ta 数据表授权给 Alice

    ```bash
    curl -X POST http://127.0.0.1:80/intra/ccl/grant \
    --header "host: scql-broker-intra.alice.svc" \
    --header "kuscia-source: alice" \
    -H "Content-Type: application/json" \
    -d '{
        "project_id": "demo",
        "column_control_list":[
        {"col":{"column_name":"ID","table_name":"ta"},"party_code":"alice","constraint":1},
        {"col":{"column_name":"age","table_name":"ta"},"party_code":"alice","constraint":1},
        {"col":{"column_name":"income","table_name":"ta"},"party_code":"alice","constraint":1},
        {"col":{"column_name":"credit_rank","table_name":"ta"},"party_code":"alice","constraint":1}
        ]
    }'
    ```

2. 将 ta 表授权给 Bob 节点

    ```bash
    curl -X POST http://127.0.0.1:80/intra/ccl/grant \
    --header "host: scql-broker-intra.alice.svc" \
    --header "kuscia-source: alice" \
    -H "Content-Type: application/json" \
    -d '{
        "project_id": "demo",
        "column_control_list":[
        {"col":{"column_name":"ID","table_name":"ta"},"party_code":"bob","constraint":1},
        {"col":{"column_name":"age","table_name":"ta"},"party_code":"bob","constraint":1},
        {"col":{"column_name":"income","table_name":"ta"},"party_code":"bob","constraint":1},
        {"col":{"column_name":"credit_rank","table_name":"ta"},"party_code":"bob","constraint":1}
        ]
    }'
    ```

#### Bob 的数据表授权

1. 将 tb 表授权给 Alice 节点

    ```bash
    curl -X POST http://127.0.0.1:80/intra/ccl/grant \
    --header "host: scql-broker-intra.bob.svc" \
    --header "kuscia-source: bob" \
    -H "Content-Type: application/json" \
    -d '{
          "project_id": "demo",
          "column_control_list":[
          {"col":{"column_name":"ID","table_name":"tb"},"party_code":"alice","constraint":1},
          {"col":{"column_name":"is_active","table_name":"tb"},"party_code":"alice","constraint":1},
          {"col":{"column_name":"order_amount","table_name":"tb"},"party_code":"alice","constraint":1}
          ]
    }'
    ```

2. 将 tb 表授权给 Bob 节点

    ```bash
    curl -X POST http://127.0.0.1:80/intra/ccl/grant \
    --header "host: scql-broker-intra.bob.svc" \
    --header "kuscia-source: bob" \
    -H "Content-Type: application/json" \
    -d '{
        "project_id": "demo",
        "column_control_list":[
        {"col":{"column_name":"ID","table_name":"tb"},"party_code":"bob","constraint":1},
        {"col":{"column_name":"is_active","table_name":"tb"},"party_code":"bob","constraint":1},
        {"col":{"column_name":"order_amount","table_name":"tb"},"party_code":"bob","constraint":1}
        ]
    }'
    ```

### 查看数据表授权

下面以 Alice 为例，Bob 节点类似

```bash
curl -X POST http://127.0.0.1:80/intra/ccl/show \
--header "host: scql-broker-intra.alice.svc" \
--header "kuscia-source: alice" \
-H "Content-Type: application/json" \
-d '{
    "project_id": "demo",
    "tables":["ta"],
    "dest_parties":["alice"]
}'
```

### 撤销数据表授权

若想撤销数据表授权，那么可以参考下面命令。以 Alice 节点为例，Bob 节点类似。

```bash
curl -X POST http://127.0.0.1:80/intra/ccl/revoke \
--header "host: scql-broker-intra.alice.svc" \
--header "kuscia-source: alice" \
-H "Content-Type: application/json" \
-d '{
    "project_id": "demo",
    "column_control_list":[
    {"col":{"column_name":"ID","table_name":"ta"},"party_code":"alice","constraint":1},
    {"col":{"column_name":"age","table_name":"ta"},"party_code":"alice","constraint":1},
    {"col":{"column_name":"income","table_name":"ta"},"party_code":"alice","constraint":1},
    {"col":{"column_name":"credit_rank","table_name":"ta"},"party_code":"alice","constraint":1}
    ]
}'
```

### 进行联合分析

#### 同步查询

下面以 Alice 节点查询为例 Bob 节点类似。

```bash
curl -X POST http://127.0.0.1:80/intra/query \
--header "host: scql-broker-intra.alice.svc" \
--header "kuscia-source: alice" \
-H "Content-Type: application/json" \
-d '{
    "project_id": "demo",
    "query":"SELECT ta.credit_rank, COUNT(*) as cnt, AVG(ta.income) as avg_income, AVG(tb.order_amount) as avg_amount FROM ta INNER JOIN tb ON ta.ID = tb.ID WHERE ta.age >= 20 AND ta.age <= 30 AND tb.is_active=1 GROUP BY ta.credit_rank;"
}'
```

返回的成功结果如下:

```bash
{
    "status": {
        "code": 0,
        "message": "",
        "details": []
    },
    "affected_rows": "0",
    "warnings": [],
    "cost_time_s": 7.171298774,
    "out_columns": [{
        "name": "credit_rank",
        "shape": {
            "dim": [{
                "dim_value": "2"
            }, {
                "dim_value": "1"
            }]
        },
        "elem_type": "INT64",
        "option": "VALUE",
        "annotation": {
            "status": "TENSORSTATUS_UNKNOWN"
        },
        "int32_data": [],
        "int64_data": ["6", "5"],
        "float_data": [],
        "double_data": [],
        "bool_data": [],
        "string_data": [],
        "ref_num": 0
    }, {
        "name": "cnt",
        "shape": {
            "dim": [{
                "dim_value": "2"
            }, {
                "dim_value": "1"
            }]
        },
        "elem_type": "INT64",
        "option": "VALUE",
        "annotation": {
            "status": "TENSORSTATUS_UNKNOWN"
        },
        "int32_data": [],
        "int64_data": ["3", "1"],
        "float_data": [],
        "double_data": [],
        "bool_data": [],
        "string_data": [],
        "ref_num": 0
    }, {
        "name": "avg_income",
        "shape": {
            "dim": [{
                "dim_value": "2"
            }, {
                "dim_value": "1"
            }]
        },
        "elem_type": "FLOAT64",
        "option": "VALUE",
        "annotation": {
            "status": "TENSORSTATUS_UNKNOWN"
        },
        "int32_data": [],
        "int64_data": [],
        "float_data": [],
        "double_data": [438000, 30070],
        "bool_data": [],
        "string_data": [],
        "ref_num": 0
    }, {
        "name": "avg_amount",
        "shape": {
            "dim": [{
                "dim_value": "2"
            }, {
                "dim_value": "1"
            }]
        },
        "elem_type": "FLOAT64",
        "option": "VALUE",
        "annotation": {
            "status": "TENSORSTATUS_UNKNOWN"
        },
        "int32_data": [],
        "int64_data": [],
        "float_data": [],
        "double_data": [4060.6666666666665, 3598],
        "bool_data": [],
        "string_data": [],
        "ref_num": 0
    }]
}
```

#### 异步查询

下面以 Alice 节点为例，Bob 节点类似。

1. 提交 query

    ```bash
    curl -X POST http://127.0.0.1:80/intra/query/submit \
    --header "host: scql-broker-intra.alice.svc" \
    --header "kuscia-source: alice" \
    -H "Content-Type: application/json" \
    -d '{
        "project_id": "demo",
        "query":"SELECT ta.credit_rank, COUNT(*) as cnt, AVG(ta.income) as avg_income, AVG(tb.order_amount) as avg_amount FROM ta INNER JOIN tb ON ta.ID = tb.ID WHERE ta.age >= 20 AND ta.age <= 30 AND tb.is_active=1 GROUP BY ta.credit_rank;"
    }'
    ```

2. 获取结果

    ```bash
    curl -X POST http://127.0.0.1:80/intra/query/fetch \
    --header "host: scql-broker-intra.alice.svc" \
    --header "kuscia-source: alice" \
    -H "Content-Type: application/json" \
    -d '{
          "job_id":"3c4723fb-9afa-11ee-8934-0242ac12000"
    }'
    ```

## 参考

### 常用命令

查看 broker kd 状态：

```bash
docker exec -it ${USER}-kuscia-autonomy-alice kubectl get kd -n cross-domain
```

查看 broker deployment 状态

```bash
docker exec -it ${USER}-kuscia-autonomy-alice kubectl get deployment -A
```

查看 broker 应用状态

```bash
docker exec -it ${USER}-kuscia-autonomy-alice kubectl get po -A
```

查看 broker configmap

```bash
docker exec -it ${USER}-kuscia-autonomy-alice kubectl get cm scql-broker-configtemplate -n alice -oyaml
```

查看 appImage

```bash
docker exec -it ${USER}-kuscia-autonomy-alice kubectl get appimage
```

删除 broker

```bash
docker exec -it ${USER}-kuscia-autonomy-alice kubectl delete kd scql -n cross-domain
```

### 如何查看 SCQL 应用容器日志

在 Kuscia 中，可以登陆到节点容器内查看 SCQL 应用容器的日志。具体方法如下。

1. 登陆到节点容器中

    下面以 Alice 节点为例：

    ```bash
    docker exec -it ${USER}-kuscia-autonomy-alice bash
    ```

2. 查看日志

    在目录 `/home/kuscia/var/stdout/pods` 下可以看到对应 SCQL Broker 和 Engine 应用容器的目录。后续进入到相应目录下，即可查看应用的日志。

    ```bash
    # View the current application container's directory
    ls /home/kuscia/var/stdout/pods

    # View the application container's logs, example as follows:
    cat /home/kuscia/var/stdout/pods/alice_xxxx_engine_xxxx/secretflow/0.log
    cat /home/kuscia/var/stdout/pods/alice_xxxx_broker_xxxx/secretflow/0.log
    ```
