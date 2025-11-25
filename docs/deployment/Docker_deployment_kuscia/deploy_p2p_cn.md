# 多机部署点对点集群

## 前言

本教程帮助您在多台机器上使用 [点对点组网模式](../../reference/architecture_cn.md#点对点组网模式) 来部署 Kuscia 集群。

当前 Kuscia 节点之间只支持 Token 的身份认证方式，在跨机器部署的场景下流程较为繁琐，后续本教程会持续更新优化。

## 前置准备

在部署 Kuscia 之前，请确保环境准备齐全，包括所有必要的软件、资源、操作系统版本和网络环境等满足要求，以确保部署过程顺畅进行，详情参考[部署要求](../deploy_check.md)。

## 部署流程（基于 TOKEN 认证）

### 部署 alice 节点

登录到安装 alice 的机器上，本文为叙述方便，假定节点 ID 为 alice ，对外可访问的 PORT 是 11080 。

指定 Kuscia 使用的镜像版本，这里使用 1.1.0b0 版本

```bash
export KUSCIA_IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia:1.1.0b0
```

指定 Secretflow 版本：

```bash
# Using Secretflow image, version 1.11.0b1 is used here
export SECRETFLOW_IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/secretflow-lite-anolis8:1.11.0b1
```

获取部署脚本，部署脚本会下载到当前目录：

```
docker pull ${KUSCIA_IMAGE} && docker run --rm ${KUSCIA_IMAGE} cat /home/kuscia/scripts/deploy/kuscia.sh > kuscia.sh && chmod u+x kuscia.sh
```

生成 alice 节点配置文件，kuscia init 参数请参考 [Kuscia 配置文件](../kuscia_config_cn.md#id3)：

```bash
# The --domain parameter specifies the node ID
docker run -it --rm ${KUSCIA_IMAGE} kuscia init --mode autonomy --domain "alice" > autonomy_alice.yaml 2>&1 || cat autonomy_alice.yaml
```

建议检查生成的文件，避免配置文件错误导致的部署启动问题。

启动节点，默认会在当前目录下创建 ${USER}-kuscia-autonomy-alice/data 目录用来存放 alice 的数据。部署节点需要使用 `kuscia.sh` 脚本并传入节点配置文件：

```bash
# -p: Specifies the mapping of the HTTPS port from the node container to the host. Ensure this port does not conflict with existing ports on the host.
# -k: Specifies the mapping of the MTLS port for the Kuscia API from the node container to the host. Ensure this port does not conflict with existing ports on the host. 
# -a: Specifies auto-import of engine images. Use -a none to disable auto-import. Use -a secretflow (default) to auto-import the SecretFlow engine image.
# -m or --memory-limit: Sets appropriate memory limits for node containers. For example, '-m 4GiB or --memory-limit=4GiB' means limiting max memory to 4GiB, '-m -1 or --memory-limit=-1' means no limit. If not set, defaults are: master 2GiB, lite node 4GiB, autonomy node 6GiB.
./kuscia.sh start -c autonomy_alice.yaml -p 11080 -k 11081
```

:::{tip}

- 节点 ID 需要全局唯一并且符合 RFC 1123 标签名规则要求，详情请参考[这里](https://kubernetes.io/zh-cn/docs/concepts/overview/working-with-objects/names/#dns-label-names)。`default`、`kube-system` 、`kube-public` 、`kube-node-lease` 、`master` 以及 `cross-domain` 为 Kuscia 预定义的节点 ID，不能被使用。
- 目前 kuscia.sh 脚本仅支持导入 SecretFlow 镜像，scql、serving 以及其他自定义镜像请移步至[注册自定义算法镜像](../../development/register_custom_image.md)
- 如果节点之间的入口网络存在网关时，为了确保节点与节点之间通信正常，需要网关符合一些要求，详情请参考[这里](../networkrequirements.md)
- alice、bob 节点默认使用 SQLite 作为存储，如果生产部署，需要配置链接到 MySQL 数据库的连接串，具体配置可以参考[这里](../kuscia_config_cn.md#id3)
- 需要对合作方暴露的 Kuscia 端口，可参考 [Kuscia 端口介绍](../kuscia_ports_cn.md)。如果多个 Autonomy 节点部署在同一个物理机上，可以用 -p -k -g -q -x 参数指定下端口号（例如：./kuscia.sh start -c autonomy_alice.yaml -p 11080 -k 11081 -g 11082 -q 11083 -x 11084），防止出现端口冲突。
- 非 root 用户部署请参考[这里](./docker_deploy_kuscia_with_rootless.md)
- 升级引擎镜像请参考[指南](../../tutorial/upgrade_engine.md)
:::

### 部署 Bob 节点

您可以选择在另一台机器上部署 bob 节点，详细步骤参考上述 alice 节点部署的流程，唯一不同的是在部署前准备参数时配置 bob 节点相关的参数。假定节点 ID 为 bob ，对外可访问的 PORT 是 21080 。

### 配置证书

在两个 Autonomy 节点建立通信之前，您需要先给这两个节点互换证书。

#### Alice 颁发证书给 Bob

准备 Alice 的公钥，在 Alice 节点的机器上，可以看到包含公钥的 crt 文件：

```bash
# [alice machine] Copy domain.crt from inside the container and rename it to alice.domain.crt
docker cp ${USER}-kuscia-autonomy-alice:/home/kuscia/var/certs/domain.crt alice.domain.crt
```

将 alice 的公钥 alice.domain.crt 拷贝到 bob 容器的 /home/kuscia/var/certs/ 目录中：

```bash
# [bob machine] Make sure alice.domain.crt is in the /home/kuscia/var/certs/ directory of bob container
docker cp alice.domain.crt ${USER}-kuscia-autonomy-bob:/home/kuscia/var/certs/
```

在 Bob 里添加 Alice 的证书等信息：

```bash
# [bob machine] Add alice's certificate and other information
docker exec -it ${USER}-kuscia-autonomy-bob scripts/deploy/add_domain.sh alice p2p
```

#### Bob 颁发证书给 Alice

准备 Bob 的公钥，在 Bob 节点的机器上，可以看到包含公钥的 crt 文件：

```bash
# [bob machine] Copy domain.crt from inside the container and rename it to bob.domain.crt
docker cp ${USER}-kuscia-autonomy-bob:/home/kuscia/var/certs/domain.crt bob.domain.crt
```

将 Bob 的公钥 bob.domain.crt 拷贝到 alice 容器的 /home/kuscia/var/certs/ 目录中：

```bash
# [alice machine] Make sure bob.domain.crt is in the /home/kuscia/var/certs/ directory of alice container
docker cp bob.domain.crt ${USER}-kuscia-autonomy-alice:/home/kuscia/var/certs/
```

在 Alice 里添加 Bob 的证书等信息：

```bash
# [alice machine] Add bob's certificate and other information
docker exec -it ${USER}-kuscia-autonomy-alice scripts/deploy/add_domain.sh bob p2p
```

### 配置授权

如果要发起由两个 Autonomy 节点参与的任务，您需要给这两个节点之间建立授权。

#### 创建 Alice 到 Bob 的授权

```bash
# [alice machine]
# Assuming bob's external IP is 2.2.2.2, 21080 is bob's exposed access port mentioned above
# To reduce troubleshooting costs for authorization errors, it's recommended to test connectivity to bob's address from within the alice container (using curl) before authorizing
# Example: curl -kvvv https://2.2.2.2:21080 should return HTTP error code 401 normally
docker exec -it ${USER}-kuscia-autonomy-alice scripts/deploy/join_to_host.sh alice bob https://2.2.2.2:21080
```

#### 创建 Bob 到 Alice 的授权

```bash
# [bob machine]
# Assuming alice's external IP is 1.1.1.1, 11080 is alice's exposed access port mentioned above
# To reduce troubleshooting costs for authorization errors, it's recommended to test connectivity to alice's address from within the bob container (using curl) before authorizing
# Example: curl -kvvv https://1.1.1.1:11080 should return HTTP error code 401 normally
docker exec -it ${USER}-kuscia-autonomy-bob scripts/deploy/join_to_host.sh bob alice https://1.1.1.1:11080
```

#### 检查节点之间网络通信状态

- 方法一：

    [Alice 机器] 执行以下命令：

    ```bash
    docker exec -it ${USER}-kuscia-autonomy-alice kubectl get cdr alice-bob
    ```

    [Bob 机器] 执行以下命令：

    ```bash
    docker exec -it ${USER}-kuscia-autonomy-bob kubectl get cdr bob-alice
    ```

当 "READR" 列为 "True" 时，说明 Alice 和 Bob 之间授权建立成功。

- 方法二：

    [alice 机器] 执行以下命令：

    ```bash
    docker exec -it ${USER}-kuscia-autonomy-alice kubectl get cdr alice-bob -o=jsonpath="{.status.tokenStatus.sourceTokens[*]}"
    ```

    [bob 机器] 执行以下命令：

    ```bash
    docker exec -it ${USER}-kuscia-autonomy-bob kubectl get cdr bob-alice -o=jsonpath="{.status.tokenStatus.sourceTokens[*]}"
    ```

当命令执行成功得到返回结果时表示授权成功

:::{tip}

- 如果节点之间的入口网络存在网关时，为了确保节点与节点之间通信正常，需要网关符合一些要求，详情请参考[这里](../networkrequirements.md)
- 授权失败，请参考[授权错误排查](../../troubleshoot/network/network_authorization_check.md)文档
:::

### 准备测试数据

- Alice 节点准备测试数据

    登录到安装 Alice 的机器上，将默认的测试数据拷贝到之前部署目录的 ${USER}-kuscia-autonomy-alice/data 下

    ```bash
    docker pull ${KUSCIA_IMAGE} && docker run --rm ${KUSCIA_IMAGE} cat /home/kuscia/var/storage/data/alice.csv > /tmp/alice.csv
    docker cp /tmp/alice.csv ${USER}-kuscia-autonomy-alice:/home/kuscia/var/storage/data/
    rm -rf /tmp/alice.csv
    ```

    为 Alice 的测试数据创建 domaindata

    ```bash
    docker exec -it ${USER}-kuscia-autonomy-alice scripts/deploy/create_domaindata_alice_table.sh alice
    ```

    为 Alice 的测试数据创建 domaindatagrant

    ```bash
    docker exec -it ${USER}-kuscia-autonomy-alice curl -X POST 'https://127.0.0.1:8082/api/v1/domaindatagrant/create' --header "Token: $(docker exec -it ${USER}-kuscia-autonomy-alice cat /home/kuscia/var/certs/token)" --header 'Content-Type: application/json' -d '{
     "grant_domain": "bob",
     "description": {"domaindatagrant":"alice-bob"},
     "domain_id": "alice",
     "domaindata_id": "alice-table"
    }' --cacert /home/kuscia/var/certs/ca.crt --cert /home/kuscia/var/certs/ca.crt --key /home/kuscia/var/certs/ca.key
    ```

- Bob 节点准备测试数据

    登录到安装 Bob 的机器上，将默认的测试数据拷贝到之前部署目录的 ${USER}-kuscia-autonomy-alice/data 下

    ```bash
    docker pull ${KUSCIA_IMAGE} && docker run --rm ${KUSCIA_IMAGE} cat /home/kuscia/var/storage/data/bob.csv > /tmp/bob.csv
    docker cp /tmp/bob.csv ${USER}-kuscia-autonomy-bob:/home/kuscia/var/storage/data/
    rm -rf /tmp/bob.csv
    ```

    为 Bob 的测试数据创建 domaindata

    ```bash
    docker exec -it ${USER}-kuscia-autonomy-bob scripts/deploy/create_domaindata_bob_table.sh bob
    ```

    为 Bob 的测试数据创建 domaindatagrant

    ```bash
    docker exec -it ${USER}-kuscia-autonomy-bob curl -X POST 'https://127.0.0.1:8082/api/v1/domaindatagrant/create' --header "Token: $(docker exec -it ${USER}-kuscia-autonomy-bob cat /home/kuscia/var/certs/token)" --header 'Content-Type: application/json' -d '{
     "grant_domain": "alice",
     "description": {"domaindatagrant":"bob-alice"},
     "domain_id": "bob",
     "domaindata_id": "bob-table"
    }' --cacert /home/kuscia/var/certs/ca.crt --cert /home/kuscia/var/certs/ca.crt --key /home/kuscia/var/certs/ca.key
    ```

### 执行作业

创建并启动作业（两方 PSI 任务）, 以 Alice 节点机器上执行命令为例

```bash
docker exec -it ${USER}-kuscia-autonomy-alice scripts/user/create_example_job.sh
```

查看作业状态

```bash
docker exec -it ${USER}-kuscia-autonomy-alice kubectl get kj -n cross-domain
```

任务运行遇到网络错误时，可以参考[这里](../../troubleshoot/network/network_troubleshoot.md)排查
