# 多机部署点对点集群

## 前言

本教程帮助你在多台机器上使用 [点对点组网模式](../../reference/architecture_cn.md#peer-to-peer) 来部署 Kuscia 集群。

当前 Kuscia 节点之间只支持 MTLS 的身份认证方式，在跨机器部署的场景下流程较为繁琐，后续本教程会持续更新优化。



## 部署流程（基于MTLS认证）

### 部署 alice 节点

准备参数：

```bash
# 部署的节点 ID
export DOMAIN_ID=alice
# 节点容器对外暴露的 IP，通常是主机 IP
export DOMAIN_HOST_IP=xx.xx.xx.xx
# 节点容器映射到主机的端口，保证和主机上现有的端口不冲突即可
export DOMAIN_HOST_PORT=8081
# 使用的 Kuscia 镜像，这里使用 latest 版本镜像
export KUSCIA_IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia
```

获取部署脚本，部署脚本会下载到当前目录：

```
docker run --rm $KUSCIA_IMAGE cat /home/kuscia/scripts/deploy/deploy.sh > deploy.sh && chmod u+x deploy.sh
```

启动节点，默认会在当前目录下创建 kuscia-autonomy-${DOMAIN_ID}-certs 目录用来存放节点的公私钥和证书：

```bash
./deploy.sh autonomy -n ${DOMAIN_ID} -i ${DOMAIN_HOST_IP} -p ${DOMAIN_HOST_PORT}
```



### 部署 bob 节点

你可以选择在另一台机器上部署 bob 节点，详细步骤参考上述 alice 节点部署的流程，唯一不同的是在部署前准备参数时配置 bob 节点相关的参数：

```bash
# 部署的节点 ID
export DOMAIN_ID=bob
# 节点容器对外暴露的 IP，通常是主机 IP
export DOMAIN_HOST_IP=xx.xx.xx.xx
# 节点容器映射到主机的端口，保证和主机上现有的端口不冲突即可
export DOMAIN_HOST_PORT=8082
```



### 配置授权

如果要发起由两个 Autonomy 节点参与的任务，你需要给这两个节点之间建立授权。

#### 创建 alice 到 bob 的授权

准备 alice 的公钥，在 alice 节点的机器上，可以看到包含公钥的 csr 文件：

```bash 
# [alice 机器] domain.csr 位于部署节点时创建的 ${DOMAIN_CERTS_DIR} 目录中，默认为以下路径
export DOMAIN_CERTS_DIR=${PWD}/kuscia-autonomy-${DOMAIN_ID}-certs
ls ${DOMAIN_CERTS_DIR}/domain.csr

# [alice 机器] 或者可以从容器内部拷贝出来
docker cp ${DOMAIN_CTR}:/home/kuscia/etc/certs/domain.csr .
```



将 alice 的公钥拷贝到 bob 的机器上的 ${DOMAIN_CERTS_DIR} 目录中并重命名为 alice.domain.csr：

```bash
# [bob 机器] 确保 alice.domain.csr 位于 bob 的 ${DOMAIN_CERTS_DIR} 目录中
export DOMAIN_CERTS_DIR=${PWD}/kuscia-autonomy-${DOMAIN_ID}-certs
export ALICE_DOMAIN_ID=alice
ls ${DOMAIN_CERTS_DIR}/${ALICE_DOMAIN_ID}.domain.csr
```



bob 给 alice 签发 MTLS 证书：

```bash 
# [bob 机器] 给 alice 签发 MTLS 证书
export DOMAIN_CTR=${USER}-kuscia-autonomy-${DOMAIN_ID}
docker exec -it ${DOMAIN_CTR} scripts/deploy/add_domain.sh ${ALICE_DOMAIN_ID} ${DOMAIN_CTR} p2p

# [bob 机器] 确保证书生成成功
ls ${DOMAIN_CERTS_DIR}/${ALICE_DOMAIN_ID}.domain.crt
ls ${DOMAIN_CERTS_DIR}/ca.crt
```

 

将 alice.domain.crt 和 ca.crt 拷贝至 alice 机器上的 ${DOMAIN_CERTS_DIR} 目录并重命名为 domain-2-bob.crt 和 bob.host.ca.crt：

```bash
# [alice 机器] 确保证书存在且命名正确
export DOMAIN_CERTS_DIR=${PWD}/kuscia-autonomy-${DOMAIN_ID}-certs
export BOB_DOMAIN_ID=bob
ls ${DOMAIN_CERTS_DIR}/domain-2-${BOB_DOMAIN_ID}.crt
ls ${DOMAIN_CERTS_DIR}/${BOB_DOMAIN_ID}.host.ca.crt
```



alice 建立到 bob 的通信：

```bash 
# [alice 机器] 
# bob_host_ip 为部署 bob 节点时的 ${DOMAIN_HOST_IP}， bob_host_port 则是部署 bob 节点时的  ${DOMAIN_HOST_PORT}
export BOB_ENDPOINT=${bob_host_ip}:${bob_host_port}
export DOMAIN_CTR=${USER}-kuscia-autonomy-${DOMAIN_ID}
docker exec -it ${DOMAIN_CTR} scripts/deploy/join_to_host.sh ${DOMAIN_ID} ${BOB_DOMAIN_ID} ${BOB_ENDPOINT}
```



#### 创建 bob 到 alice 的授权

创建 bob 到 alice 的授权可参考上述建立 alice 到 bob 的授权的流程，以下是简要步骤。



将 bob 的公钥拷贝到 alice 的机器上的 ${DOMAIN_CERTS_DIR} 目录中并重命名为 bob.domain.csr：

```bash
# [alice 机器] 确保 bob.domain.csr 位于 alice 的 ${DOMAIN_CERTS_DIR} 目录中
export DOMAIN_CERTS_DIR=${PWD}/kuscia-autonomy-${DOMAIN_ID}-certs
export BOB_DOMAIN_ID=bob
ls ${DOMAIN_CERTS_DIR}/${BOB_DOMAIN_ID}.domain.csr
```



alice 给 bob 签发 MTLS 证书：

```bash
# [alice 机器] 给 bob 签发 MTLS 证书
export DOMAIN_CTR=${USER}-kuscia-autonomy-${DOMAIN_ID}
docker exec -it ${DOMAIN_CTR} scripts/deploy/add_domain.sh ${BOB_DOMAIN_ID} ${DOMAIN_CTR} p2p
```



将 bob.domain.crt 和 ca.crt 拷贝至 bob 机器上的 ${DOMAIN_CERTS_DIR} 目录并重命名为 domain-2-alice.crt 和 alice.host.ca.crt：

```bash
# [bob 机器] 确保证书存在且命名正确
export DOMAIN_CERTS_DIR=${PWD}/kuscia-autonomy-${DOMAIN_ID}-certs
export ALICE_DOMAIN_ID=alice
ls ${DOMAIN_CERTS_DIR}/domain-2-${ALICE_DOMAIN_ID}.crt
ls ${DOMAIN_CERTS_DIR}/${ALICE_DOMAIN_ID}.host.ca.crt
```



bob 建立到 alice 的通信：

```bash 
# [bob 机器] 
# alice_host_ip 为部署 alice 节点时的 ${DOMAIN_HOST_IP}， alice_host_port 则是部署 alice 节点时的  ${DOMAIN_HOST_PORT}
export ALICE_ENDPOINT=${alice_host_ip}:${alice_host_port}
export DOMAIN_CTR=${USER}-kuscia-autonomy-${DOMAIN_ID}
docker exec -it ${DOMAIN_CTR} scripts/deploy/join_to_host.sh ${DOMAIN_ID} ${ALICE_DOMAIN_ID} ${ALICE_ENDPOINT}
```



#### 执行作业

```bash 
# 登入 alice 节点容器（或 bob 节点容器）
docker exec -it ${DOMAIN_CTR} bash

# 创建并启动作业（两方 PSI 任务）
scripts/user/create_example_job.sh

# 查看作业状态。
kubectl get kj
```
