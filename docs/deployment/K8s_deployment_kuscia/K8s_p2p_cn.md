# 部署点对点集群

## 前言

本教程帮助您在 K8s 集群上使用 [点对点组网模式](../../reference/architecture_cn.md#点对点组网模式) 来部署 Kuscia 集群。

目前 Kuscia 在部署到 K8s 上时，隐私计算任务的运行态支持 RunK 和 RunP 两种模式，RunC 模式目前需要部署 Kuscia 的 Pod 有特权容器，暂时不是特别推荐。详情请参考[容器运行模式](../../reference/architecture_cn.md#agent)

本教程默认以 RunK 模式来进行部署（需要能够有权限在宿主的 K8s 上拉起任务 Pod），RunP 模式的部署请参考 [使用进程运行时部署节点](./deploy_with_runp_cn.md)，非 root 用户部署请参考[这里](./k8s_deploy_kuscia_with_rootless.md)。

![k8s_master_lite_deploy](../../imgs/k8s_deploy_autonomy.png)

## 部署 Autonomy

### 前置准备

部署 Autonomy 需提前准备好 MySQL 数据库表并且符合 [Kuscia 配置](../kuscia_config_cn.md#id3)中的规范，数据库帐号密码等信息配置在步骤三 ConfigMap 中。

### 步骤一：创建 Namespace

> 创建 Namespace 需要先获取 create 权限，避免出现 "namespaces is forbidden" 报错

Namespace 名称可以按照自己的意愿来定，也可以复用已经有的，下文以 autonomy-alice 为例（Namespace 名称需要与 yaml 文件里的 Namespace 字段对应起来）

```bash
kubectl create ns autonomy-alice
```

### 步骤二：创建 Service

获取 [service.yaml](https://github.com/secretflow/kuscia/blob/main/hack/k8s/autonomy/service.yaml) 文件，创建这个 Service

<span style="color:red;">注意：<br>
1、需要对合作方暴露的 Kuscia 端口，可参考 [Kuscia 端口介绍](../kuscia_ports_cn.md) </span>

```bash
kubectl create -f service.yaml
```

### 步骤三：创建 ConfigMap

ConfigMap 是用来配置 Kuscia 的配置文件，详细的配置文件介绍参考 [Kuscia 配置](../kuscia_config_cn.md)

domainID、私钥以及 datastoreEndpoint 字段里的数据库连接串（user、password、host、database）需要替换成真实有效的信息，私钥可以通过命令 `docker run -it --rm secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia scripts/deploy/generate_rsa_key.sh`生成

:::{tip}

- 修改 ConfigMap 配置后，需执行 kubectl delete po {pod-name} -n {namespace} 重新拉起 Pod 生效
- 节点 ID 需要全局唯一并且符合 RFC 1123 标签名规则要求，详情请参考[这里](https://kubernetes.io/zh-cn/docs/concepts/overview/working-with-objects/names/#dns-label-names)。`default`、`kube-system` 、`kube-public` 、`kube-node-lease` 、`master` 以及 `cross-domain` 为 Kuscia 预定义的节点 ID，不能被使用。
:::

特殊说明：为了使 ServiceAccount 具有创建、查看、删除等资源权限，RunK 模式提供两种方式：

- 方式一：在 ConfigMap 的 KubeconfigFile 字段配置具有同等权限的 Kubeconfig
- 方式二：不配置 KubeconfigFile，执行步骤四，创建具有所需权限的 Role 和 RoleBinding

获取 [configmap.yaml](https://github.com/secretflow/kuscia/blob/main/hack/k8s/autonomy/configmap.yaml) 文件，创建这个 ConfigMap；因为这里面涉及很多敏感配置，请在生产时务必重新配置，不使用默认配置。

```bash
kubectl create -f configmap.yaml
```

### 步骤四（可选）：创建 RBAC

获取 [rbac.yaml](https://github.com/secretflow/kuscia/blob/main/hack/k8s/autonomy/rbac.yaml) 文件，创建 Role 和 RoleBinding

```bash
kubectl create -f rbac.yaml
```

### 步骤四：创建 Deployment

获取 [deployment-autonomy.yaml](https://github.com/secretflow/kuscia/blob/main/hack/k8s/autonomy/deployment.yaml) 文件里，创建这个 Deployment

```bash
kubectl create -f deployment.yaml
```

### 创建 autonomy-alice、autonomy-bob 之间的授权

> PS：目前因为安全性和时间因素，节点之间授权还是需要很多手动的操作，未来会优化。

Alice 和 Bob 授权之前可以先检测下相互之间的通信是否正常

建议使用 curl -kvvv http://kuscia-autonomy-bob.autonomy-bob.svc.cluster.local:1080;（此处以 HTTP 为例，HTTPS 可以删除 ConfigMap 里的 Protocol: NOTLS 字段，重启 Pod 生效。[LoadBalancer](https://kubernetes.io/zh-cn/docs/concepts/services-networking/service/#loadbalancer) 或者 [NodePort](https://kubernetes.io/zh-cn/docs/concepts/services-networking/service/#type-nodeport) 方式可以用 curl -kvvv http://ip:port）检查一下是否访问能通，正常情况下返回的 HTTP 错误码是 401，内容是节点ID和版本信息

示例参考[这里](../K8s_deployment_kuscia/K8s_master_lite_cn.md#id6)

<span style="color:red;">注意：如果 Alice/Bob 的入口网络存在网关时，为了确保节点之间通信正常，需要网关符合一些要求，详情请参考[这里](../networkrequirements.md)</span>

建立 Alice 到 Bob 授权

```bash
# 将 Alice 节点的 domain.crt 证书 cp 到 跳板机当前目录并改名 alice.domain.crt
kubectl cp autonomy-alice/kuscia-autonomy-alice-686d6747c-gc2kk:var/certs/domain.crt alice.domain.crt
# 将 alice.domain.crt 证书 cp 到 Bob 节点的里
kubectl cp alice.domain.crt autonomy-bob/kuscia-autonomy-bob-89cf8bc77-cvn9f:var/certs/
# 登录到 Bob 节点
kubectl -n autonomy-bob exec -it kuscia-autonomy-bob-89cf8bc77-cvn9f -- bash 
# [pod 内部] 在 Bob 里添加 Alice 的证书等信息
scripts/deploy/add_domain.sh alice p2p
# 登录到 Alice 节点
kubectl -n autonomy-alice exec -it kuscia-autonomy-alice-686d6747c-gc2kk -- bash 
# [pod 内部] 建立 Alice 到 Bob 的通信
scripts/deploy/join_to_host.sh alice bob http://kuscia-autonomy-bob.autonomy-bob.svc.cluster.local:1080
```

建立 Bob 到 Alice 授权

```bash
# 将 Bob 节点的 domain.crt 证书 cp 到 跳板机当前目录并改 bob.domain.crt
kubectl cp autonomy-bob/kuscia-autonomy-bob-89cf8bc77-cvn9f:var/certs/domain.crt bob.domain.crt
# 将 bob.domain.crt 证书 cp 到 Alice 节点的里
kubectl cp bob.domain.crt autonomy-alice/kuscia-autonomy-alice-686d6747c-gc2kk:var/certs/
# 登录到 Alice 节点
kubectl -n autonomy-alice exec -it kuscia-autonomy-alice-686d6747c-gc2kk -- bash 
# [pod 内部] 在 Alice 里添加 Bob 的证书等信息
scripts/deploy/add_domain.sh bob p2p
# 登录到 Bob 节点
kubectl -n autonomy-bob exec -it kuscia-autonomy-bob-89cf8bc77-cvn9f -- bash 
# [pod 内部] 建立 Bob 到 Alice 的通信
scripts/deploy/join_to_host.sh bob alice http://kuscia-autonomy-alice.autonomy-alice.svc.cluster.local:1080
```

检查双方授权状态

`pod 内部`在 Alice 节点内执行 `kubectl get cdr alice-bob -o=jsonpath="{.status.tokenStatus.sourceTokens[*]}"`，在 Bob 节点内执行 `kubectl get cdr bob-alice -o=jsonpath="{.status.tokenStatus.sourceTokens[*]}"` 得到下面示例返回结果表示授权成功

```bash
{"effectiveInstances":["kuscia-autonomy-alice-686d6747c-h78lr"],"expirationTime":"2123-11-24T02:42:12Z","isReady":true,"revision":1,"revisionTime":"2023-11-24T02:42:12Z","token":"dVYZ4Ld/i7msNwuLoT+F8kFaCXbgXk6FziaU5PMASl8ReFfOVpsUt0qijlQaKTLm+OKzABfMQEI4jGeJ/Qsmhr6XOjc+7rkSCa5bmCxw5YVq+UtIFwNnjyRDaBV6A+ViiEMZwuaLIiFMtsPLki2SXzcA7LiLZY3oZvHfgf0m8LenMfU9tmZEptRoTBeL3kKagMBhxLxXL4rZzmI1bBwi49zxwOmg3c/MbDP8JiI6zIM7/NdIAEJhqsbzC5/Yw1qajr7D+NLXhsdrtTDSHN8gSB8D908FxYvcxeUTHqDQJT1mWcXs2N4r/Z/3OydkwJiQQokpjfZsR0T4xmbVTJd5qw=="}
```

`pod 内部`在 Alice、Bob 节点 pod 内执行 `kubectl get cdr` 返回 Ready 为 True 时，表示授权成功，示例如下：

```bash
NAME        SOURCE   DESTINATION   HOST                                                 AUTHENTICATION   READY
alice-bob   alice    bob           kuscia-autonomy-bob.autonomy-bob.svc.cluster.local   Token            True
bob-alice   bob      alice                                                              Token            True
```

授权失败，请参考[授权错误排查](../../troubleshoot/network/network_authorization_check.md)文档

## 确认部署成功

### 检查 Pod 状态

pod 处于 running 状态表示部署成功

```bash
kubectl get po -n autonomy-alice
```

### 检查数据库连接状态

数据库内生成表格 kine 并且有数据表示数据库连接成功

## 运行任务

> RunK 模式不支持使用本地数据训练，请准备[OSS数据](K8s_p2p_cn.md#准备-oss-测试数据)。使用本地数据请先切换至 RunP 模式，详情请参考 [使用 RunP 运行时部署节点](./deploy_with_runp_cn.md)。

### 准备本地测试数据

#### Alice 节点准备本地测试数据

登录到 Alice 节点的 Pod 中

```bash
kubectl -n autonomy-alice exec -it ${alice_pod_name} -- bash 
```

为 Alice 节点创建本地数据源

创建 DomainData 的时候要指定 datasource_id，所以要先创建数据源，再创建 DomainData，示例如下：

```bash
# 在容器内执行示例
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/domaindatasource/create' \
 --header 'Content-Type: application/json' \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "domain_id": "alice",
  "datasource_id":"default-data-source",
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

为 Alice 的测试数据创建 DomainData

```bash
# 在 alice 容器内执行示例
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -X POST 'http://127.0.0.1:8082/api/v1/domaindata/create' \
--header 'Content-Type: application/json' \
--cacert ${CTR_CERTS_ROOT}/ca.crt \
-d '{
  "domaindata_id": "alice-table",
  "name": "alice.csv",
  "type": "table",
  "relative_uri": "alice.csv",
  "domain_id": "alice",
  "datasource_id": "default-data-source",
  "attributes": {
    "description": "alice demo data"
  },
  "columns": [
    {
      "comment": "",
      "name": "id1",
      "type": "str"
    },
    {
      "comment": "",
      "name": "age",
      "type": "float"
    },
    {
      "comment": "",
      "name": "education",
      "type": "float"
    },
    {
      "comment": "",
      "name": "default",
      "type": "float"
    },
    {
      "comment": "",
      "name": "balance",
      "type": "float"
    },
    {
      "comment": "",
      "name": "housing",
      "type": "float"
    },
    {
      "comment": "",
      "name": "loan",
      "type": "float"
    },
    {
      "comment": "",
      "name": "day",
      "type": "float"
    },
    {
      "comment": "",
      "name": "duration",
      "type": "float"
    },
    {
      "comment": "",
      "name": "campaign",
      "type": "float"
    },
    {
      "comment": "",
      "name": "pdays",
      "type": "float"
    },
    {
      "comment": "",
      "name": "previous",
      "type": "float"
    },
    {
      "comment": "",
      "name": "job_blue-collar",
      "type": "float"
    },
    {
      "comment": "",
      "name": "job_entrepreneur",
      "type": "float"
    },
    {
      "comment": "",
      "name": "job_housemaid",
      "type": "float"
    },
    {
      "comment": "",
      "name": "job_management",
      "type": "float"
    },
    {
      "comment": "",
      "name": "job_retired",
      "type": "float"
    },
    {
      "comment": "",
      "name": "job_self-employed",
      "type": "float"
    },
    {
      "comment": "",
      "name": "job_services",
      "type": "float"
    },
    {
      "comment": "",
      "name": "job_student",
      "type": "float"
    },
    {
      "comment": "",
      "name": "job_technician",
      "type": "float"
    },
    {
      "comment": "",
      "name": "job_unemployed",
      "type": "float"
    },
    {
      "comment": "",
      "name": "marital_divorced",
      "type": "float"
    },
    {
      "comment": "",
      "name": "marital_married",
      "type": "float"
    },
    {
      "comment": "",
      "name": "marital_single",
      "type": "float"
    }
  ],
  "vendor": "manual",
  "author": "alice"
}'
```

将 Alice 的 DomainData 授权给 Bob

```bash
# 在容器内执行示例
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -X POST 'http://127.0.0.1:8082/api/v1/domaindatagrant/create' \
--cacert ${CTR_CERTS_ROOT}/ca.crt \
--header 'Content-Type: application/json' \
-d '{ "grant_domain": "bob",
      "description": {"domaindatagrant":"alice-bob"},
      "domain_id": "alice",
      "domaindata_id": "alice-table"
}'
```

#### Bob 节点准备本地测试数据

登录到 Bob 节点的 Pod 中

```bash
kubectl exec -it ${bob_pod_name} bash -n autonomy-bob
```

为 Bob 节点创建本地数据源

创建 DomainData 的时候要指定 datasource_id，所以要先创建数据源，再创建 DomainData，示例如下：

```bash
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/domaindatasource/create' \
 --header 'Content-Type: application/json' \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "domain_id": "bob",
  "datasource_id":"default-data-source",
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

为 Bob 的测试数据创建 DomainData

```bash
# 在 bob 容器内执行示例
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -X POST 'http://127.0.0.1:8082/api/v1/domaindata/create' \
--header 'Content-Type: application/json' \
--cacert ${CTR_CERTS_ROOT}/ca.crt \
-d '{
  "domaindata_id": "bob-table",
  "name": "bob.csv",
  "type": "table",
  "relative_uri": "bob.csv",
  "domain_id": "bob",
  "datasource_id": "default-data-source",
  "attributes": {
    "description": "bob demo data"
  },
  "columns": [
    {
      "comment": "",
      "name": "id2",
      "type": "str"
    },
    {
      "comment": "",
      "name": "contact_cellular",
      "type": "float"
    },
    {
      "comment": "",
      "name": "contact_telephone",
      "type": "float"
    },
    {
      "comment": "",
      "name": "contact_unknown",
      "type": "float"
    },
    {
      "comment": "",
      "name": "month_apr",
      "type": "float"
    },
    {
      "comment": "",
      "name": "month_aug",
      "type": "float"
    },
    {
      "comment": "",
      "name": "month_dec",
      "type": "float"
    },
    {
      "comment": "",
      "name": "month_feb",
      "type": "float"
    },
    {
      "comment": "",
      "name": "month_jan",
      "type": "float"
    },
    {
      "comment": "",
      "name": "month_jul",
      "type": "float"
    },
    {
      "comment": "",
      "name": "month_jun",
      "type": "float"
    },
    {
      "comment": "",
      "name": "month_mar",
      "type": "float"
    },
    {
      "comment": "",
      "name": "month_may",
      "type": "float"
    },
    {
      "comment": "",
      "name": "month_nov",
      "type": "float"
    },
    {
      "comment": "",
      "name": "month_oct",
      "type": "float"
    },
    {
      "comment": "",
      "name": "month_sep",
      "type": "float"
    },
    {
      "comment": "",
      "name": "poutcome_failure",
      "type": "float"
    },
    {
      "comment": "",
      "name": "poutcome_other",
      "type": "float"
    },
    {
      "comment": "",
      "name": "poutcome_success",
      "type": "float"
    },
    {
      "comment": "",
      "name": "poutcome_unknown",
      "type": "float"
    },
    {
      "comment": "",
      "name": "y",
      "type": "int"
    }
  ],
  "vendor": "manual",
  "author": "bob"
}'
```

将 Bob 的 DomainData 授权给 Alice

```bash
# 在容器内执行示例
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -X POST 'http://127.0.0.1:8082/api/v1/domaindatagrant/create' \
--cacert ${CTR_CERTS_ROOT}/ca.crt \
--header 'Content-Type: application/json' \
-d '{ "grant_domain": "alice",
      "description": {"domaindatagrant":"bob-alice"},
      "domain_id": "bob",
      "domaindata_id": "bob-table"
}'
```

### 准备 OSS 测试数据

#### Alice 节点准备 OSS 数据

请先将 Alice 节点测试数据 [alice.csv](https://github.com/secretflow/kuscia/blob/main/testdata/alice.csv) 上传至 OSS

登录到 Alice 节点的 Pod 中

```bash
kubectl exec -it ${alice_pod_name} bash -n autonomy-alice
```

为 Alice 节点创建 OSS 数据源

创建 DomainData 的时候要指定 datasource_id，所以要先创建数据源，再创建 DomainData，示例如下：

```bash
# 在 alice 容器内执行示例
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'http://localhost:8082/api/v1/domaindatasource/create' \
--header 'Content-Type: application/json' \
--cacert ${CTR_CERTS_ROOT}/ca.crt \
-d '{
   "domain_id": "alice",
   "datasource_id":"default-data-source",
   "type":"oss",
   "name": "DemoDataSource",
   "info": {
      "oss": {
          "endpoint": "https://oss.xxx.cn-xxx.com",
          "bucket": "secretflow",
          "prefix": "kuscia/",
          "access_key_id":"ak-xxxx",
          "access_key_secret" :"sk-xxxx"
#         "virtualhost": true (阿里云 OSS 需要配置此项)
#         "storage_type": "minio" (Minio 需要配置此项)
      }
  },
  "access_directly": true
}'
```

为 Alice 的测试数据创建 DomainData

```bash
# 在 alice 容器内执行示例
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -X POST 'http://127.0.0.1:8082/api/v1/domaindata/create' \
--header 'Content-Type: application/json' \
--cacert ${CTR_CERTS_ROOT}/ca.crt \
-d '{
  "domaindata_id": "alice-table",
  "name": "alice.csv",
  "type": "table",
  "relative_uri": "alice.csv",
  "domain_id": "alice",
  "datasource_id": "default-data-source",
  "attributes": {
    "description": "alice demo data"
  },
  "columns": [
    {
      "comment": "",
      "name": "id1",
      "type": "str"
    },
    {
      "comment": "",
      "name": "age",
      "type": "float"
    },
    {
      "comment": "",
      "name": "education",
      "type": "float"
    },
    {
      "comment": "",
      "name": "default",
      "type": "float"
    },
    {
      "comment": "",
      "name": "balance",
      "type": "float"
    },
    {
      "comment": "",
      "name": "housing",
      "type": "float"
    },
    {
      "comment": "",
      "name": "loan",
      "type": "float"
    },
    {
      "comment": "",
      "name": "day",
      "type": "float"
    },
    {
      "comment": "",
      "name": "duration",
      "type": "float"
    },
    {
      "comment": "",
      "name": "campaign",
      "type": "float"
    },
    {
      "comment": "",
      "name": "pdays",
      "type": "float"
    },
    {
      "comment": "",
      "name": "previous",
      "type": "float"
    },
    {
      "comment": "",
      "name": "job_blue-collar",
      "type": "float"
    },
    {
      "comment": "",
      "name": "job_entrepreneur",
      "type": "float"
    },
    {
      "comment": "",
      "name": "job_housemaid",
      "type": "float"
    },
    {
      "comment": "",
      "name": "job_management",
      "type": "float"
    },
    {
      "comment": "",
      "name": "job_retired",
      "type": "float"
    },
    {
      "comment": "",
      "name": "job_self-employed",
      "type": "float"
    },
    {
      "comment": "",
      "name": "job_services",
      "type": "float"
    },
    {
      "comment": "",
      "name": "job_student",
      "type": "float"
    },
    {
      "comment": "",
      "name": "job_technician",
      "type": "float"
    },
    {
      "comment": "",
      "name": "job_unemployed",
      "type": "float"
    },
    {
      "comment": "",
      "name": "marital_divorced",
      "type": "float"
    },
    {
      "comment": "",
      "name": "marital_married",
      "type": "float"
    },
    {
      "comment": "",
      "name": "marital_single",
      "type": "float"
    }
  ],
  "vendor": "manual",
  "author": "alice"
}'
```

将 Alice 的 DomainData 授权给 Bob

```bash
# 在 alice 容器内执行示例
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -X POST 'http://127.0.0.1:8082/api/v1/domaindatagrant/create' \
--cacert ${CTR_CERTS_ROOT}/ca.crt \
--header 'Content-Type: application/json' \
-d '{ "grant_domain": "bob",
      "description": {"domaindatagrant":"alice-bob"},
      "domain_id": "alice",
      "domaindata_id": "alice-table"
}'
```

#### Bob 节点准备 OSS 测试数据

请先将 Bob 节点测试数据 [bob.csv](https://github.com/secretflow/kuscia/blob/main/testdata/bob.csv) 上传至 OSS

登录到 Bob 节点的 Pod 中

```bash
kubectl exec -it ${bob_pod_name} bash -n autonomy-bob
```

为 Bob 节点创建 OSS 数据源

创建 DomainData 的时候要指定 datasource_id，所以要先创建数据源，再创建 DomainData，示例如下：

```bash
# 在 bob 容器内执行示例
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'http://localhost:8082/api/v1/domaindatasource/create' \
--header 'Content-Type: application/json' \
--cacert ${CTR_CERTS_ROOT}/ca.crt \
-d '{
   "domain_id": "bob",
   "datasource_id":"default-data-source",
   "type":"oss",
   "name": "DemoDataSource",
   "info": {
      "oss": {
          "endpoint": "https://oss.xxx.cn-xxx.com",
          "bucket": "secretflow",
          "prefix": "kuscia/",
          "access_key_id":"ak-xxxx",
          "access_key_secret" :"sk-xxxx"
#         "virtualhost": true (阿里云 OSS 需要配置此项)
#         "storage_type": "minio" (Minio 需要配置此项)
      }
  },
  "access_directly": true
}'
```

为 Bob 的测试数据创建 DomainData

```bash
# 在 bob 容器内执行示例
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -X POST 'http://127.0.0.1:8082/api/v1/domaindata/create' \
--header 'Content-Type: application/json' \
--cacert ${CTR_CERTS_ROOT}/ca.crt \
-d '{
  "domaindata_id": "bob-table",
  "name": "bob.csv",
  "type": "table",
  "relative_uri": "bob.csv",
  "domain_id": "bob",
  "datasource_id": "default-data-source",
  "attributes": {
    "description": "bob demo data"
  },
  "columns": [
    {
      "comment": "",
      "name": "id2",
      "type": "str"
    },
    {
      "comment": "",
      "name": "contact_cellular",
      "type": "float"
    },
    {
      "comment": "",
      "name": "contact_telephone",
      "type": "float"
    },
    {
      "comment": "",
      "name": "contact_unknown",
      "type": "float"
    },
    {
      "comment": "",
      "name": "month_apr",
      "type": "float"
    },
    {
      "comment": "",
      "name": "month_aug",
      "type": "float"
    },
    {
      "comment": "",
      "name": "month_dec",
      "type": "float"
    },
    {
      "comment": "",
      "name": "month_feb",
      "type": "float"
    },
    {
      "comment": "",
      "name": "month_jan",
      "type": "float"
    },
    {
      "comment": "",
      "name": "month_jul",
      "type": "float"
    },
    {
      "comment": "",
      "name": "month_jun",
      "type": "float"
    },
    {
      "comment": "",
      "name": "month_mar",
      "type": "float"
    },
    {
      "comment": "",
      "name": "month_may",
      "type": "float"
    },
    {
      "comment": "",
      "name": "month_nov",
      "type": "float"
    },
    {
      "comment": "",
      "name": "month_oct",
      "type": "float"
    },
    {
      "comment": "",
      "name": "month_sep",
      "type": "float"
    },
    {
      "comment": "",
      "name": "poutcome_failure",
      "type": "float"
    },
    {
      "comment": "",
      "name": "poutcome_other",
      "type": "float"
    },
    {
      "comment": "",
      "name": "poutcome_success",
      "type": "float"
    },
    {
      "comment": "",
      "name": "poutcome_unknown",
      "type": "float"
    },
    {
      "comment": "",
      "name": "y",
      "type": "int"
    }
  ],
  "vendor": "manual",
  "author": "bob"
}'
```

将 Bob 的 DomainData 授权给 Alice

```bash
# 在 bob 容器内执行示例
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -X POST 'http://127.0.0.1:8082/api/v1/domaindatagrant/create' \
--cacert ${CTR_CERTS_ROOT}/ca.crt \
--header 'Content-Type: application/json' \
-d '{ "grant_domain": "alice",
      "description": {"domaindatagrant":"bob-alice"},
      "domain_id": "bob",
      "domaindata_id": "bob-table"
}'
```

### 创建 AppImage

- [Alice 节点]

登录到 Alice pod

  ```bash
  kubectl exec -it ${alice_pod_name} bash -n autonomy-alice
  ```

  `pod 内部`获取 [AppImage.yaml](https://github.com/secretflow/kuscia/blob/main/hack/k8s/AppImage.yaml) 文件并创建 AppImage

  ```bash
  kubectl apply -f AppImage.yaml
  ```

- [Bob 节点]

    登录到 Bob 节点的 Pod 内

    ```bash
    kubectl exec -it ${bob_pod_name} bash -n autonomy-bob
    ```

    `pod 内部`获取 [AppImage.yaml](https://github.com/secretflow/kuscia/blob/main/hack/k8s/AppImage.yaml) 文件并创建 AppImage

    ```bash
    kubectl apply -f AppImage.yaml
    ```

### 执行测试作业

登录到 Alice 节点 的 Pod 内

```bash
kubectl exec -it ${alice_pod_name} bash -n autonomy-alice
```

`pod 内部`创建并启动作业（两方 PSI 任务）

```bash
scripts/user/create_example_job.sh
```

`pod 内部`查看作业状态

```bash
kubectl get kj -n cross-domain
```

`pod 外部` RunK 模式可以在 Kuscia Pod 所在集群中执行如下命令查看引擎日志

```bash
kubectl logs ${engine_pod_name} -n autonomy-alice
```
