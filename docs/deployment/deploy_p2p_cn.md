# 多机部署点对点集群

## 前言

本教程帮助你在多台机器上使用 [点对点组网模式](../reference/architecture_cn.md#点对点组网模式) 来部署 Kuscia 集群。

当前 Kuscia 节点之间只支持 MTLS 的身份认证方式，在跨机器部署的场景下流程较为繁琐，后续本教程会持续更新优化。



## 部署流程（基于MTLS认证）

### 部署 alice 节点

登录到安装 alice 的机器上，本文为叙述方便，假定节点ID为 alice ，对外可访问的ip是 1.1.1.1，对外可访问的port是 8081 。
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
# -p 参数传递的是节点容器映射到主机的端口，保证和主机上现有的端口不冲突即可
# -k 参数传递的是节点容器 KusciaAPI 映射到主机的 HTTP 端口，保证和主机上现有的端口不冲突即可
./deploy.sh autonomy -n alice -i 1.1.1.1 -p 8081 -k 8082
```



### 部署 bob 节点

你可以选择在另一台机器上部署 bob 节点，详细步骤参考上述 alice 节点部署的流程，唯一不同的是在部署前准备参数时配置 bob 节点相关的参数。假定节点ID为 bob ，对外可访问的ip是 2.2.2.2，对外可访问的port是 8082 。


{#配置授权}

### 配置授权

如果要发起由两个 Autonomy 节点参与的任务，你需要给这两个节点之间建立授权。

#### 创建 alice 到 bob 的授权

准备 alice 的公钥，在 alice 节点的机器上，可以看到包含公钥的 csr 文件：

```bash 

# [alice 机器] domain.csr 位于部署节点时创建的 ${PWD}/kuscia-autonomy-alice-certs 目录中，默认为以下路径
ls ${PWD}/kuscia-autonomy-alice-certs/domain.csr

# [alice 机器] 或者可以从容器内部拷贝出来
docker cp ${USER}-kuscia-autonomy-alice:/home/kuscia/etc/certs/domain.csr .
```



将 alice 的公钥拷贝到 bob 的机器上的 ${PWD}/kuscia-autonomy-bob-certs 目录中并重命名为 alice.domain.csr：

```bash
# [bob 机器] 确保 alice.domain.csr 位于 bob 的 ${PWD}/kuscia-autonomy-bob-certs 目录中
ls ${PWD}/kuscia-autonomy-bob-certs/alice.domain.csr
```



bob 给 alice 签发 MTLS 证书：

```bash 
# [bob 机器] 给 alice 签发 MTLS 证书
docker exec -it ${USER}-kuscia-autonomy-bob scripts/deploy/add_domain.sh alice ${USER}-kuscia-autonomy-bob p2p

# [bob 机器] 确保证书生成成功
ls ${PWD}/kuscia-autonomy-bob-certs/alice.domain.crt
ls ${PWD}/kuscia-autonomy-bob-certs/ca.crt
```

 

将 alice.domain.crt 和 ca.crt 拷贝至 alice 机器上的 ${PWD}/kuscia-autonomy-alice-certs 目录并重命名为 domain-2-bob.crt 和 bob.host.ca.crt：

```bash
# [alice 机器] 确保证书存在且命名正确
ls ${PWD}/kuscia-autonomy-alice-certs/domain-2-bob.crt
ls ${PWD}/kuscia-autonomy-alice-certs/bob.host.ca.crt
```



alice 建立到 bob 的通信：

```bash 
# [alice 机器] 
# 2.2.2.2是上文中 bob 的访问 ip，8082 是上文中 bob 的访问端口
docker exec -it ${USER}-kuscia-autonomy-alice scripts/deploy/join_to_host.sh alice bob 2.2.2.2:8082
```
`注意：如果节点之间的入口网络存在网关时，为了确保节点与节点之间通信正常，需要网关符合一些要求，详情请参考[这里](https://www.secretflow.org.cn/docs/kuscia/latest/zh-Hans/deployment/networkrequirements)`


#### 创建 bob 到 alice 的授权

创建 bob 到 alice 的授权可参考上述建立 alice 到 bob 的授权的流程，以下是简要步骤。



将 bob 的公钥拷贝到 alice 的机器上的 ${PWD}/kuscia-autonomy-alice-certs 目录中并重命名为 bob.domain.csr：

```bash
# [alice 机器] 确保 bob.domain.csr 位于 alice 的 ${PWD}/kuscia-autonomy-alice-certs 目录中
ls ${PWD}/kuscia-autonomy-alice-certs/bob.domain.csr
```



alice 给 bob 签发 MTLS 证书：

```bash
# [alice 机器] 给 bob 签发 MTLS 证书
docker exec -it ${USER}-kuscia-autonomy-alice scripts/deploy/add_domain.sh bob ${USER}-kuscia-autonomy-alice p2p
```



将 bob.domain.crt 和 ca.crt 拷贝至 bob 机器上的 ${PWD}/kuscia-autonomy-bob-certs 目录并重命名为 domain-2-alice.crt 和 alice.host.ca.crt：

```bash
# [bob 机器] 确保证书存在且命名正确
ls ${PWD}/kuscia-autonomy-bob-certs/domain-2-alice.crt
ls ${PWD}/kuscia-autonomy-bob-certs/alice.host.ca.crt
```



bob 建立到 alice 的通信：

```bash 
# [bob 机器] 
# 1.1.1.1 是上文中 alice 的访问 ip，8081 是上文中 alice 的访问端口
docker exec -it ${USER}-kuscia-autonomy-bob scripts/deploy/join_to_host.sh bob alice 1.1.1.1:8081
```

##### 获取测试数据集
登录到安装 alice 的机器上，将默认的测试数据拷贝到当前目录的kuscia-autonomy-alice-data下

```bash
docker run --rm --pull always $KUSCIA_IMAGE cat /home/kuscia/var/storage/data/alice.csv > kuscia-autonomy-alice-data/alice.csv
```

登录到安装 bob 的机器上，将默认的测试数据拷贝到当前目录的kuscia-autonomy-bob-data下

```bash
docker run --rm --pull always $KUSCIA_IMAGE cat /home/kuscia/var/storage/data/bob.csv > kuscia-autonomy-bob-data/bob.csv
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
