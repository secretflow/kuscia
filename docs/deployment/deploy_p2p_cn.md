# 多机部署点对点集群

## 前言

本教程帮助你在多台机器上使用 [点对点组网模式](../reference/architecture_cn.md#点对点组网模式) 来部署 Kuscia 集群。

当前 Kuscia 节点之间只支持 token 的身份认证方式，在跨机器部署的场景下流程较为繁琐，后续本教程会持续更新优化。



## 部署流程（基于 TOKEN 认证）

### 部署 alice 节点

登录到安装 alice 的机器上，本文为叙述方便，假定节点ID为 alice ，对外可访问的ip是 1.1.1.1，对外可访问的port是 11080 。
指定 kuscia 版本：

```bash
# 使用的 Kuscia 镜像，这里使用 latest 版本镜像
export KUSCIA_IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia
```

获取部署脚本，部署脚本会下载到当前目录：

```
docker run --rm --pull always $KUSCIA_IMAGE cat /home/kuscia/scripts/deploy/deploy.sh > deploy.sh && chmod u+x deploy.sh
```

启动节点，默认会在当前目录下创建 kuscia-autonomy-alice-certs 目录用来存放 alice 节点的公私钥和证书。默认会在当前目录下创建 kuscia-autonomy-alice-data 目录用来存放 alice 的数据：：

```bash
# -n 参数传递的是节点 ID。
# -i 参数传递的是节点容器对外暴露的 IP，通常是主机 IP。 如果合作方无法直达主机，请填写网关映射的IP。
# -p 参数传递的是节点容器映射到主机的 HTTPS 端口，保证和主机上现有的端口不冲突即可
# -k 参数传递的是节点容器 KusciaAPI 映射到主机的 MTLS 端口，保证和主机上现有的端口不冲突即可
./deploy.sh autonomy -n alice -i 1.1.1.1 -p 11080 -k 8082
```
<span style="color:red;">注意：节点 id 需要符合 DNS 子域名规则要求，详情请参考[这里](https://kubernetes.io/zh-cn/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names)</span>


### 部署 bob 节点

你可以选择在另一台机器上部署 bob 节点，详细步骤参考上述 alice 节点部署的流程，唯一不同的是在部署前准备参数时配置 bob 节点相关的参数。假定节点ID为 bob ，对外可访问的ip是 2.2.2.2，对外可访问的port是 21080 。


{#配置授权}

### 配置授权

如果要发起由两个 Autonomy 节点参与的任务，你需要给这两个节点之间建立授权。

#### 创建 alice 到 bob 的授权

准备 alice 的公钥，在 alice 节点的机器上，可以看到包含公钥的 crt 文件：

```bash 

# [alice 机器] 将 domain.crt 从容器内部拷贝出来
docker cp ${USER}-kuscia-autonomy-alice:/home/kuscia/var/tmp/domain.crt .
```



将 alice 的公钥拷贝到 bob 的机器上的 ${PWD}/kuscia-autonomy-bob-certs 目录中并重命名为 alice.domain.crt：

```bash
# [bob 机器] 确保 alice.domain.crt 位于 bob 的 ${PWD}/kuscia-autonomy-bob-certs 目录中
ls ${PWD}/kuscia-autonomy-bob-certs/alice.domain.crt
```

在 bob 里添加 alice 的证书等信息：

```bash
# [bob 机器] 添加 alice 的证书等信息
docker exec -it ${USER}-kuscia-autonomy-bob scripts/deploy/add_domain.sh alice p2p
```

alice 建立到 bob 的通信：

```bash
# [alice 机器]
# 为了减少授权错误的排查成本，建议在 alice 容器内(curl)访问 bob 地址判定是否能联通，之后再授权
# 示例：curl -kvvv https://2.2.2.2:21080 返回正常的 HTTP 错误码是401
# 2.2.2.2是上文中 bob 的访问 ip，21080 是上文中 bob 的访问端口
docker exec -it ${USER}-kuscia-autonomy-alice scripts/deploy/join_to_host.sh alice bob https://2.2.2.2:21080
```

执行以下命令，查看是否有内容，如果有说明 alice 到 bob 授权建立成功。
```bash
docker exec -it ${USER}-kuscia-autonomy-alice kubectl get cdr alice-bob -o=jsonpath="{.status.tokenStatus.sourceTokens[*]}"
```

<span style="color:red;">注意：如果节点之间的入口网络存在网关时，为了确保节点与节点之间通信正常，需要网关符合一些要求，详情请参考[这里](./networkrequirements.md)</span>

#### 创建 bob 到 alice 的授权

创建 bob 到 alice 的授权可参考上述建立 alice 到 bob 的授权的流程，以下是简要步骤。


准备 bob 的公钥，在 bob 节点的机器上，可以看到包含公钥的 crt 文件：

```bash

# [bob 机器] 将 domain.crt 从容器内部拷贝出来
docker cp ${USER}-kuscia-autonomy-bob:/home/kuscia/var/tmp/domain.crt .
```

将 bob 的公钥拷贝到 alice 的机器上的 ${PWD}/kuscia-autonomy-alice-certs 目录中并重命名为 bob.domain.crt：

```bash
# [alice 机器] 确保 bob.domain.crt 位于 alice 的 ${PWD}/kuscia-autonomy-alice-certs 目录中
ls ${PWD}/kuscia-autonomy-alice-certs/bob.domain.crt
```


在 alice 里添加 bob 的证书等信息：

```bash
# [alice 机器] 添加 bob 的证书等信息
docker exec -it ${USER}-kuscia-autonomy-alice scripts/deploy/add_domain.sh bob p2p
```


bob 建立到 alice 的通信：

```bash
# [bob 机器]
# 为了减少授权错误的排查成本，建议在 bob 容器内(curl)访问 alice 地址判定是否能联通，之后再授权
# 示例：curl -kvvv https://1.1.1.1:11080 返回正常的 HTTP 错误码是401
# 1.1.1.1 是上文中 alice 的访问 ip，11080 是上文中 alice 的访问端口
docker exec -it ${USER}-kuscia-autonomy-bob scripts/deploy/join_to_host.sh bob alice https://1.1.1.1:11080
```

执行以下命令，查看是否有内容，如果有说明 bob 到 alice 授权建立成功。
```bash
docker exec -it ${USER}-kuscia-autonomy-bob kubectl get cdr bob-alice -o=jsonpath="{.status.tokenStatus.sourceTokens[*]}"
```

<span style="color:red;">注意：如果节点之间的入口网络存在网关时，为了确保节点与节点之间通信正常，需要网关符合一些要求，详情请参考[这里](./networkrequirements.md)</span>

#### 准备测试数据
登录到安装 alice 的机器上，将默认的测试数据拷贝到之前部署目录的 kuscia-autonomy-alice-data 下

```bash
docker run --rm --pull always $KUSCIA_IMAGE cat /home/kuscia/var/storage/data/alice.csv > kuscia-autonomy-alice-data/alice.csv
```
为 alice 的测试数据创建 domaindata
```bash
docker exec -it ${USER}-kuscia-autonomy-alice scripts/deploy/create_domaindata_alice_table.sh alice
```
为 alice 的测试数据创建 domaindatagrant

```bash
docker exec -it ${USER}-kuscia-autonomy-alice curl https://127.0.0.1:8070/api/v1/datamesh/domaindatagrant/create -X POST -H 'content-type: application/json' -d '{"author":"alice","domaindata_id":"alice-table","grant_domain":"bob"}' --cacert var/tmp/ca.crt --cert var/tmp/ca.crt --key var/tmp/ca.key
```

同理，登录到安装 bob 的机器上，将默认的测试数据拷贝到之前部署目录的 kuscia-autonomy-bob-data 下

```bash
docker run --rm --pull always $KUSCIA_IMAGE cat /home/kuscia/var/storage/data/bob.csv > kuscia-autonomy-bob-data/bob.csv
```
为 bob 的测试数据创建 domaindata
```bash
docker exec -it ${USER}-kuscia-autonomy-bob scripts/deploy/create_domaindata_bob_table.sh bob
```
为 bob 的测试数据创建 domaindatagrant

```bash
docker exec -it ${USER}-kuscia-autonomy-bob curl https://127.0.0.1:8070/api/v1/datamesh/domaindatagrant/create -X POST -H 'content-type: application/json' -d '{"author":"bob","domaindata_id":"bob-table","grant_domain":"alice"}' --cacert var/tmp/ca.crt --cert var/tmp/ca.crt --key var/tmp/ca.key
```

#### 执行作业

```bash 
# 登入 alice 节点容器（或 bob 节点容器）, 以 alice 节点为例
docker exec -it ${USER}-kuscia-autonomy-alice bash

# 创建并启动作业（两方 PSI 任务）
scripts/user/create_example_job.sh

# 查看作业状态。
kubectl get kj
```
