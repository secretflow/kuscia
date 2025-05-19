# 多机部署中心化集群

## 前言

本教程帮助您在多台机器上使用 [中心化组网模式](../../reference/architecture_cn.md#中心化组网模式) 来部署 Kuscia 集群。

## 前置准备

在部署 Kuscia 之前，请确保环境准备齐全，包括所有必要的软件、资源、操作系统版本和网络环境等满足要求，以确保部署过程顺畅进行，详情参考[部署要求](../deploy_check.md)。

## 部署流程（基于TOKEN认证）

### 部署 master 节点

登录到安装 master 的机器上，假设对外 IP 是 1.1.1.1。

指定 Kuscia 版本：

```bash
# The Kuscia image used, here using version 0.14.0b0 image
export KUSCIA_IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia:0.14.0b0
```

指定 SecretFlow 版本：

```bash
# The Secretflow image used, here using version 1.11.0b1 image
export SECRETFLOW_IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/secretflow-lite-anolis8:1.11.0b1
```

获取部署脚本，部署脚本会下载到当前目录：

```bash
docker pull $KUSCIA_IMAGE && docker run --rm $KUSCIA_IMAGE cat /home/kuscia/scripts/deploy/kuscia.sh > kuscia.sh && chmod u+x kuscia.sh
```

生成 master 节点的配置文件，kuscia init 参数请参考 [Kuscia 配置文件](../kuscia_config_cn.md#id3)：

```bash
# The --domain parameter passes the master node ID, DomainID must be globally unique and conform to the RFC 1123 label name specification. For details, refer to: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-label-names. In a production environment, it is recommended to use the format: company-name-department-name-node-name, e.g., mycompany-secretflow-master
docker run -it --rm ${KUSCIA_IMAGE} kuscia init --mode master --domain "mycompany-secretflow-master" > kuscia_master.yaml 2>&1 || cat kuscia_master.yaml
```

建议检查生成的文件，避免配置文件错误导致的部署启动问题。

启动 master，默认会在当前目录下创建 ${USER}-kuscia-master/{data、logs} 用来存储 master 的数据、日志：

```bash
# The -p parameter passes the port that the master container maps to the host, ensuring it does not conflict with existing ports on the host.
# The -k parameter passes the HTTP port that the master container's KusciaAPI maps to the host, ensuring it does not conflict with existing ports on the host.
# The -a parameter specifies the engine image to be automatically imported. -a none: do not automatically import the engine image, -a secretflow (default): automatically import the secretflow engine image.
# The -m or --memory-limit parameter sets an appropriate memory limit for the node container. For example, '-m 4GiB or --memory-limit=4GiB' indicates a maximum memory limit of 4GiB, '-m -1 or --memory-limit=-1' indicates no limit. If not set, the default is 2GiB for the master node, 4GiB for the lite node, and 6GiB for the autonomy node.
./kuscia.sh start -c kuscia_master.yaml -p 18080 -k 18081
```

:::{tip}

- 节点 ID 需要全局唯一并且符合 RFC 1123 标签名规则要求，详情请参考[这里](https://kubernetes.io/zh-cn/docs/concepts/overview/working-with-objects/names/#dns-label-names)。`default`、`kube-system` 、`kube-public` 、`kube-node-lease` 、`master` 以及 `cross-domain` 为 Kuscia 预定义的节点 ID，不能被使用。
- 目前 kuscia.sh 脚本仅支持导入 Secretflow 镜像，scql、serving 以及其他自定义镜像请移步至[注册自定义算法镜像](../../development/register_custom_image.md)
- 如果 master 的入口网络存在网关时，为了确保节点与 master 之间通信正常，需要网关符合一些要求，详情请参考[这里](../networkrequirements.md)。
- master 节点默认使用 SQLite 作为存储，如果生产部署，需要配置链接到 MySQL 数据库的连接串，具体配置可以参考[这里](../kuscia_config_cn.md#id3)
- 需要对合作方暴露的 Kuscia 端口，可参考 [Kuscia 端口介绍](../kuscia_ports_cn.md)
- 非 root 用户部署请参考[这里](./docker_deploy_kuscia_with_rootless.md)
- 升级引擎镜像请参考[指南](../../tutorial/upgrade_engine.md)
:::

建议使用 `curl -kvvv https://ip:port` 检查一下是否访问能通，正常情况下返回的 HTTP 错误码是 401，内容是：unauthorized。
示例如下：

```bash
*   Trying 127.0.0.1:18080...
* Connected to 127.0.0.1 (127.0.0.1) port 18080 (#0)
* ALPN, offering h2
* ALPN, offering http/1.1
* TLSv1.3 (OUT), TLS handshake, Client hello (1):
* TLSv1.3 (IN), TLS handshake, Server hello (2):
* TLSv1.3 (IN), TLS handshake, Encrypted Extensions (8):
* TLSv1.3 (IN), TLS handshake, Certificate (11):
* TLSv1.3 (IN), TLS handshake, CERT verify (15):
* TLSv1.3 (IN), TLS handshake, Finished (20):
* TLSv1.3 (OUT), TLS change cipher, Change cipher spec (1):
* TLSv1.3 (OUT), TLS handshake, Finished (20):
* SSL connection using TLSv1.3 / TLS_AES_256_GCM_SHA384
* ALPN, server did not agree to a protocol
* Server certificate:
*  subject: CN=kuscia-system_ENVOY_EXTERNAL
*  start date: Sep 14 07:42:47 2023 GMT
*  expire date: Jan 30 07:42:47 2051 GMT
*  issuer: CN=Kuscia
*  SSL certificate verify result: unable to get local issuer certificate (20), continuing anyway.
> GET / HTTP/1.1
> Host: 127.0.0.1:18080
> User-Agent: curl/7.82.0
> Accept: */*
>
* TLSv1.3 (IN), TLS handshake, Newsession Ticket (4):
* TLSv1.3 (IN), TLS handshake, Newsession Ticket (4):
* old SSL session ID is stale, removing
* Mark bundle as not supporting multiuse
< HTTP/1.1 401 Unauthorized
< x-accel-buffering: no
< content-length: 13
< content-type: text/plain
< kuscia-error-message: Domain kuscia-system.root-kuscia-master<--127.0.0.1 return http code 401.
< date: Fri, 15 Sep 2023 02:50:39 GMT
< server: kuscia-gateway
<
* Connection #0 to host 127.0.0.1 left intact
{"domain":"alice","instance":"xyz","kuscia":"v0.1","reason":"unauthorized."}
```

#### Tips

本文后续还会经常使用到 `docker exec -it ${USER}-kuscia-master xxxxx` 类似的命令。建议以如下方式简化输入。

```bash
alias km="docker exec -it ${USER}-kuscia-master"
```

后续相关命令可以简化为 `km xxxxx`。

### 部署 lite 节点

您可以选择在任一台机器上部署 lite 节点（下文以 alice、bob 为例）。

#### 部署 lite 节点 Alice

在部署 Alice 节点之前，我们需要在 master 上注册 Alice 节点并获取部署时需要用到的 Token 。

执行以下命令，完成节点注册并从返回中得到 Token （下文将以 abcdefg 为例）。

```bash
docker exec -i ${USER}-kuscia-master sh scripts/deploy/add_domain_lite.sh alice
```

输出示例：

```bash
abcdefg
```

如果 Token 遗忘了，可以通过该命令重新获取

```bash
docker exec -it ${USER}-kuscia-master kubectl get domain alice -o=jsonpath='{.status.deployTokenStatuses[?(@.state=="unused")].token}' && echo
```

输出示例：

```bash
abcdefg
```

接下来，登录到安装 Alice 的机器上，假设对外暴露的 IP 是 2.2.2.2。

指定 Kuscia 版本：

```bash
# The Kuscia image used, here using version 0.14.0b0 image
export KUSCIA_IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia:0.14.0b0
```

获取部署脚本，部署脚本会下载到当前目录：

```bash
docker pull $KUSCIA_IMAGE && docker run --rm $KUSCIA_IMAGE cat /home/kuscia/scripts/deploy/kuscia.sh > kuscia.sh && chmod u+x kuscia.sh
```

生成 Alice 节点的配置文件：

```bash
# The --domain parameter passes the node ID.
# The --lite-deploy-token parameter passes the Token for node deployment.
# The --master-endpoint parameter passes the master container's exposed https://IP:PORT. For example, if the master is exposed at IP 1.1.1.1 and port 18080, it would be https://1.1.1.1:18080.
docker run -it --rm ${KUSCIA_IMAGE} kuscia init --mode lite --domain "alice" --master-endpoint "https://1.1.1.1:18080" --lite-deploy-token "abcdefg" > lite_alice.yaml 2>&1 || cat lite_alice.yaml
```

启动 Alice，默认会在当前目录下创建 ${USER}-kuscia-lite-alice/data 目录用来存放 alice 的数据：

```bash
# The -p parameter passes the port that the node container maps to the host, ensuring it does not conflict with existing ports on the host.
# The -k parameter passes the HTTP port that the lite container's KusciaAPI maps to the host, ensuring it does not conflict with existing ports on the host.
./kuscia.sh start -c lite_alice.yaml -p 28080 -k 28081
```

> 如果 master 与多个 lite 节点部署在同一个物理机上，可以用 -p -k -g -q -x 参数指定下端口号（例如：./kuscia.sh start -c lite_alice.yaml -p 28080 -k 28081 -g 28082 -q 28083 -x 28084），防止出现端口冲突。

#### 部署 lite 节点 bob

在部署 Bob 节点之前，我们需要在 master 注册 bob 节点，并获取到部署时需要用到的 Token 。

执行以下命令，完成节点注册并从返回中得到 Token （下文将以 hijklmn 为例）。

```bash
docker exec -i ${USER}-kuscia-master sh scripts/deploy/add_domain_lite.sh bob
```

输出示例：

```bash
hijklmn
```

如果 Token 遗忘了，可以通过该命令重新获取

```bash
docker exec -it ${USER}-kuscia-master kubectl get domain bob -o=jsonpath='{.status.deployTokenStatuses[?(@.state=="unused")].token}' && echo
```

输出示例：

```bash
hijklmn
```

接下来，登录到安装 Bob 的机器上，假设对暴露的 IP 是 3.3.3.3。

指定 Kuscia 版本：

```bash
# The Kuscia image used, here using version 0.14.0b0 image
export KUSCIA_IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia:0.14.0b0
```

获取部署脚本，部署脚本会下载到当前目录：

```bash
docker pull $KUSCIA_IMAGE && docker run --rm $KUSCIA_IMAGE cat /home/kuscia/scripts/deploy/kuscia.sh > kuscia.sh && chmod u+x kuscia.sh
```

生成 Bob 节点的配置文件：

```bash
# The --domain parameter passes the node ID.
# The --lite-deploy-token parameter passes the Token for node deployment.
# The --master-endpoint parameter passes the master container's exposed https://IP:PORT. For example, if the master is exposed at IP 1.1.1.1 and port 18080, it would be https://1.1.1.1:18080.
docker run -it --rm ${KUSCIA_IMAGE} kuscia init --mode lite --domain "bob" --master-endpoint "https://1.1.1.1:18080" --lite-deploy-token "hijklmn" > lite_bob.yaml 2>&1 || cat lite_bob.yaml
```

启动 Bob，默认会在当前目录下创建 ${USER}-kuscia-lite-bob/data 目录用来存放 bob 的数据：

```bash
# The -p parameter passes the port that the node container maps to the host, ensuring it does not conflict with existing ports on the host.
# The -k parameter passes the HTTP port that the lite container's KusciaAPI maps to the host, ensuring it does not conflict with existing ports on the host.
./kuscia.sh start -c lite_bob.yaml -p 38080 -k 38081
```

> 如果 master 与多个 lite 节点部署在同一个物理机上，可以用 -p -k -g -q -x 参数指定下端口号（例如：./kuscia.sh start -c lite_bob.yaml -p 38080 -k 38081 -g 38082 -q 38083 -x 38084），防止出现端口冲突。

### 配置授权

如果要发起由两个 lite 节点参与的任务，您需要给这两个节点之间建立授权。

#### 创建 alice 和 bob 之间的授权

在 master 机器上执行创建授权的命令

```bash
# To reduce the cost of troubleshooting authorization errors, it is recommended to separately (using curl) access the other party's address from within the alice/bob containers to determine if connectivity is possible, before proceeding with authorization.
# Example: curl -vvv http://ip:port returns a normal HTTP error code of 401.
# The access address for the bob node is generally http://ip:port for bob. As mentioned above, bob's IP is 3.3.3.3, and the port is 38080 as stated earlier.
docker exec -it ${USER}-kuscia-master sh scripts/deploy/create_cluster_domain_route.sh alice bob http://3.3.3.3:38080
# The access address for the alice node is generally http://ip:port for alice. As mentioned above, alice's IP is 2.2.2.2, and the port is 28080 as stated earlier.
docker exec -it ${USER}-kuscia-master sh scripts/deploy/create_cluster_domain_route.sh bob alice http://2.2.2.2:28080
```

执行以下命令：

```bash
docker exec -it ${USER}-kuscia-master kubectl get cdr
```

当 `type` 为 Ready 的 condition 的 `status` 值为 "True" 则说明 Alice 和 Bob 之间授权建立成功。

:::{tip}

- 如果节点之间的入口网络存在网关时，为了确保节点与节点之间通信正常，需要网关符合一些要求，详情请参考[这里](../networkrequirements.md)
- 授权失败，请参考[授权错误排查](../../troubleshoot/network/network_authorization_check.md)
:::

### 运行任务

接下来，运行一个测试任务以验证部署是否成功。

#### 准备数据

##### 获取测试数据集

登录到安装 Alice 的机器上，将默认的测试数据拷贝到之前部署目录的 ${USER}-kuscia-lite-alice/data 下

```bash
docker pull $KUSCIA_IMAGE && docker run --rm $KUSCIA_IMAGE cat /home/kuscia/var/storage/data/alice.csv > /tmp/alice.csv
docker cp /tmp/alice.csv ${USER}-kuscia-lite-alice:/home/kuscia/var/storage/data/
rm -rf /tmp/alice.csv
```

登录到安装 bob 的机器上，将默认的测试数据拷贝到之前部署目录的 ${USER}-kuscia-lite-bob/data 下

```bash
docker pull $KUSCIA_IMAGE && docker run --rm $KUSCIA_IMAGE cat /home/kuscia/var/storage/data/bob.csv > /tmp/bob.csv
docker cp /tmp/bob.csv ${USER}-kuscia-lite-bob:/home/kuscia/var/storage/data/
rm -rf /tmp/bob.csv
```

##### 创建测试数据表

登录到安装 master 的机器上，为 Alice 和 Bob 的测试数据创建 domaindata

```bash
docker exec -it ${USER}-kuscia-master scripts/deploy/create_domaindata_alice_table.sh alice
docker exec -it ${USER}-kuscia-master scripts/deploy/create_domaindata_bob_table.sh bob
```

##### 创建测试数据表授权

登录到安装 master 的机器上，为 Alice 的测试数据创建 domaindatagrant

```bash
docker exec -it ${USER}-kuscia-master curl -X POST 'https://127.0.0.1:8082/api/v1/domaindatagrant/create' --header "Token: $(docker exec -it ${USER}-kuscia-master cat /home/kuscia/var/certs/token)" --header 'Content-Type: application/json' -d '{
 "grant_domain": "bob",
 "description": {"domaindatagrant":"alice-bob"},
 "domain_id": "alice",
 "domaindata_id": "alice-table"
}' --cacert /home/kuscia/var/certs/ca.crt --cert /home/kuscia/var/certs/ca.crt --key /home/kuscia/var/certs/ca.key
```

同理，登录到安装 master 的机器上，为 Bob 的测试数据创建 domaindatagrant

```bash
docker exec -it ${USER}-kuscia-master curl -X POST 'https://127.0.0.1:8082/api/v1/domaindatagrant/create' --header "Token: $(docker exec -it ${USER}-kuscia-master cat /home/kuscia/var/certs/token)" --header 'Content-Type: application/json' -d '{
 "grant_domain": "alice",
 "description": {"domaindatagrant":"bob-alice"},
 "domain_id": "bob",
 "domaindata_id": "bob-table"
}' --cacert /home/kuscia/var/certs/ca.crt --cert /home/kuscia/var/certs/ca.crt --key /home/kuscia/var/certs/ca.key
```

#### 执行测试作业

登录到安装 master 的机器上

创建并启动作业（两方 PSI 任务）

```bash
docker exec -it ${USER}-kuscia-master scripts/user/create_example_job.sh
```

查看作业状态

```bash
docker exec -it ${USER}-kuscia-master kubectl get kj -n cross-domain
```

任务运行遇到网络错误时，可以参考[这里](../../troubleshoot/network/network_troubleshoot.md)排查
