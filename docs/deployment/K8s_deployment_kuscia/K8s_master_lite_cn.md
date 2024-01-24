# 部署中心化集群

## 前言
本教程帮助你在 k8s 集群上使用 [中心化组网模式](../../reference/architecture_cn.md#中心化组网模式) 来部署 Kuscia 集群。

目前 kuscia 在部署到 k8s 上时，隐私计算任务的运行态支持 runk 和 runp 两种模式， runc 模式目前需要部署 kuscia 的 pod 有特权容器，暂时不是特别推荐。详情请参考[容器运行模式](../../reference/architecture_cn.md#agent)

本教程默认以 runk 模式来进行部署（需要能够有权限在宿主的 K8s 上拉起任务 pod）， runp 模式的部署请参考 [使用进程运行时部署节点](../deploy_with_runp_cn.md)。

> Tips：k8s 部署模式暂不支持训练，仅支持预测服务

## 部署 master

### 前置准备

部署 master 需提前准备好 mysql 数据库表并且符合[kuscia配置](../kuscia_config_cn.md#id3)中的规范，数据库帐号密码等信息配置在步骤三 configmap 中。

### 步骤一：创建 Namespace
> 创建 namespace 需要先获取 create 权限，避免出现 "namespaces is forbidden" 报错

namespace 名称可以按照自己的意愿来定，也可以复用已经有的，下文以 kuscia-master 为例（namespace 名称需要与 yaml 文件里的 namespace 字段对应起来）
```bash
kubectl create ns kuscia-master
```

### 步骤二：创建 Service

获取 [service.yaml](https://github.com/secretflow/kuscia/blob/main/hack/k8s/master/service.yaml) 文件，创建 service
```bash
kubectl create -f service.yaml
```

### 步骤三：创建 Configmap
ConfigMap 是用来配置 kuscia 的配置文件，详细的配置文件介绍参考[kuscia配置](../kuscia_config_cn.md)

domainID、私钥以及 datastoreEndpoint 字段里的数据库连接串（user、password、host、database）需要替换成真实有效的信息，私钥可以通过命令 `docker run -it --rm secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia scripts/deploy/generate_rsa_key.sh` 生成
> 注意：<br>
1、database 名称暂不支持 "-" 特殊字符<br>
2、目前节点私钥仅支持 pkcs#1 格式: "BEGIN RSA PRIVATE KEY/END RSA PRIVATE KEY"<br>
3、修改 Configmap 配置后，需执行 kubectl delete po &#36;{pod-name} -n &#36;{namespace} 重新拉起 pod 生效

<span style="color:red;">注意：节点 id 需要符合 DNS 子域名规则要求，详情请参考[这里](https://kubernetes.io/zh-cn/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names)</span>

获取 [configmap.yaml](https://github.com/secretflow/kuscia/blob/main/hack/k8s/master/configmap.yaml) 文件，创建 configmap；因为这里面涉及很多敏感配置，请在生产时务必重新配置，不使用默认配置。

```bash
kubectl create -f configmap.yaml
```

### 步骤四：创建 Deployment

获取 [deployment.yaml](https://github.com/secretflow/kuscia/blob/main/hack/k8s/master/deployment.yaml) 文件，创建 deployment
```bash
kubectl create -f deployment.yaml
```

## 部署 lite

### 步骤一：创建 Namespace
> 创建 namespace 需要先获取 create 权限，避免出现 "namespaces is forbidden" 报错

namespace 名称可以按照自己的意愿来定，也可以复用已经有的，下文以 lite-alice 为例（namespace 名称需要与 yaml 文件里的 namespace 字段对应起来）
```bash
kubectl create ns lite-alice
```

### 步骤二：创建 Service

获取 [service.yaml](https://github.com/secretflow/kuscia/blob/main/hack/k8s/lite/service.yaml) 文件，如果 master 与 lite 不在一个 k8s 集群内，可以将 master service 的端口暴露方式改为 [LoadBalancer](https://kubernetes.io/zh-cn/docs/concepts/services-networking/service/#loadbalancer)（公有云，例如：[阿里云](https://help.aliyun.com/zh/ack/serverless-kubernetes/user-guide/use-annotations-to-configure-load-balancing)） 或者 [NodePort](https://kubernetes.io/zh-cn/docs/concepts/services-networking/service/#type-nodeport)，并在 configmap 的  masterEndpoint 字段改为可正常访问的地址，创建 service
```bash
kubectl create -f service.yaml
```

### 步骤三：创建 Configmap
ConfigMap 是用来配置 kuscia 的配置文件，详细的配置文件介绍参考[kuscia配置](../kuscia_config_cn.md)

部署 configmap 需要提前在 master 节点 pod 内生成 domainID 以及 Token，并填写到 configmap 的 domainID 和 liteDeployToken 字段中，私钥可以通过命令 `docker run -it --rm secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia scripts/deploy/generate_rsa_key.sh` 生成并填写到 domainKeyData 字段中
> 注意：<br>
1、目前节点私钥仅支持 pkcs#1 格式: "BEGIN RSA PRIVATE KEY/END RSA PRIVATE KEY"<br>
2、修改 Configmap 配置后，需执行 kubectl delete po &#36;{pod-name} -n &#36;{namespace} 重新拉起 pod 生效

<span style="color:red;">注意：节点 id 需要符合 DNS 子域名规则要求，详情请参考[这里](https://kubernetes.io/zh-cn/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names)</span>

lite-bob 配置与 lite-alice 一样，下面以 alice 为例：
```bash
kubectl exec -it ${master_pod_name} bash -n kuscia-master
scripts/deploy/add_domain_lite.sh alice
# 示例 Token
BMC4xjNqa7uAmWmyXLuJ4rrZw6brZeax
# 如果 Token 遗忘了，可以通过该命令重新获取
kubectl get domain alice -o=jsonpath='{.status.deployTokenStatuses[?(@.state=="unused")].token}' && echo
```

特殊说明：为了使 ServiceAccount 具有创建、查看、删除等资源权限，runk 模式提供两种方式：
- 方式一：在 configmap 的 kubeconfigFile 字段配置具有同等权限的 kubeconfig
- 方式二：不配置 kubeconfigFile，执行步骤四，创建具有所需权限的 role 和 RoleBinding


获取 [configmap.yaml](https://github.com/secretflow/kuscia/blob/main/hack/k8s/lite/configmap.yaml) 文件，创建 configmap；因为这里面涉及很多敏感配置，请在生产时务必重新配置，不使用默认配置。
```bash
kubectl create -f comfigmap.yaml
```

### 步骤四（可选）：创建 rbac.yaml

获取 [rbac.yaml](https://github.com/secretflow/kuscia/blob/main/hack/k8s/lite/rbac.yaml) 文件，创建 role 和 RoleBinding
```bash
kubectl create -f rbac.yaml
```

### 步骤五：创建 Deployment
拉起 lite pod 前可以先检测下与 master 之前的通信是否正常

建议使用 curl -kvvv http://kuscia-master.kuscia-master.svc.cluster.local:1080;（此处以 http 为例，https 可以删除 master configmap 里的 protocol: NOTLS 字段，重启 pod 生效。LoadBalancer 或者 NodePort 方式可以用 curl -kvvv http://ip:port）检查一下是否访问能通，正常情况下返回的 http 错误码是401，内容是：unauthorized

示例如下：
```bash
* Rebuilt URL to: http://kuscia-master.kuscia-master.svc.cluster.local:1080/
*   Trying 192.168.72.65...
* TCP_NODELAY set
* Connected to kuscia-master.kuscia-master.svc.cluster.local (192.168.72.65) port 1080 (#0)
> GET / HTTP/1.1
> Host: kuscia-master.kuscia-master.svc.cluster.local:1080
> User-Agent: curl/7.61.1
> Accept: */*
>
< HTTP/1.1 401 Unauthorized
< x-accel-buffering: no
< content-length: 13
< content-type: text/plain
< kuscia-error-message: Domain kuscia-system.kuscia-master-7d588b4577-9zxbs<--kuscia-master.kuscia-master.svc.cluster.local return http code 401.
< date: Wed, 29 Nov 2023 07:59:04 GMT
< server: kuscia-gateway
<
* Connection #0 to host kuscia-master.kuscia-master.svc.cluster.local left intact
unauthorized.
```

<span style="color:red;">注意：如果 master 的入口网络存在网关时，为了确保节点与 master 之间通信正常，需要网关符合一些要求，详情请参考[这里](../networkrequirements.md)</span>

获取 [deployment.yaml](https://github.com/secretflow/kuscia/blob/main/hack/k8s/lite/deployment.yaml) 文件，创建 deployment
```bash
kubectl create -f deployement.yaml
```

### 创建 lite-alice、lite-bob 之间的授权
> PS：目前因为安全性和时间因素，节点之间授权还是需要很多手动的操作，未来会优化。

```bash
# 登录 master
kubectl exec -it ${master_pod_name} bash -n kuscia-master
# 创建 alice 到 bob 的授权
scripts/deploy/create_cluster_domain_route.sh alice bob http://kuscia-lite-bob.lite-bob.svc.cluster.local:1080
# 创建 bob 到 alice 的授权
scripts/deploy/create_cluster_domain_route.sh bob alice http://kuscia-lite-alice.lite-alice.svc.cluster.local:1080
# 执行以下命令，查看是否有内容，如果有说明 alice 到 bob 授权建立成功。
kubectl get cdr alice-bob -o=jsonpath="{.status.tokenStatus.sourceTokens[*]}"
# 执行以下命令，查看是否有内容，如果有说明 bob 到 alice 授权建立成功
kubectl get cdr bob-alice -o=jsonpath="{.status.tokenStatus.sourceTokens[*]}"
```
在执行 master pod 内执行 `kubectl get cdr` 返回 Ready 为 True 时，表示授权成功，示例如下：
```bash
NAME                  SOURCE   DESTINATION     HOST                                             AUTHENTICATION   READY
alice-kuscia-system   alice    kuscia-system                                                    Token            True
bob-kuscia-system     bob      kuscia-system                                                    Token            True
alice-bob             alice    bob             kuscia-lite-bob.lite-bob.svc.cluster.local       Token            True
bob-alice             bob      alice           kuscia-lite-alice.lite-alice.svc.cluster.local   Token            True
```
授权失败，请参考[授权错误排查](../../reference/troubleshoot/networkauthorizationcheck.md)文档

## 确认部署成功
### 检查 pod 状态
pod 处于 running 状态表示部署成功
```bash
kuebctl get po -n kuscia-master
kubectl get po -n lite-alice
```
### 检查数据库连接状态
数据库内生成表格 kine 并且有数据表示数据库连接成功