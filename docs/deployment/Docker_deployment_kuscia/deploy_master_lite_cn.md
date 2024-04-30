# 多机部署中心化集群

## 前言

本教程帮助你在多台机器上使用 [中心化组网模式](../reference/architecture_cn.md#中心化组网模式) 来部署 Kuscia 集群。

## 前置准备

在部署 Kuscia 之前，请确保环境准备齐全，包括所有必要的软件、资源、操作系统版本和网络环境等满足要求，以确保部署过程顺畅进行，详情参考[部署要求](../deployment/deploy_check.md)

## 部署流程（基于TOKEN认证）

### 部署 master 节点
登录到安装 master 的机器上，假设对外ip是1.1.1.1。
指定 Kuscia 版本：
```bash
# 使用的 Kuscia 镜像，这里使用 latest 版本镜像
export KUSCIA_IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia
```

获取部署脚本，部署脚本会下载到当前目录：

```bash
docker pull $KUSCIA_IMAGE && docker run --rm $KUSCIA_IMAGE cat /home/kuscia/scripts/deploy/kuscia.sh > kuscia.sh && chmod u+x kuscia.sh
```

生成 master 节点的配置文件：
```bash
# -n 参数传递的是 master 节点 ID，DomainID 需全局唯一，生产环境建议使用公司名称-部门名称-节点名称，如: antgroup-secretflow-master
docker run -it --rm ${KUSCIA_IMAGE} kuscia init --mode master --domain "antgroup-secretflow-master" > kuscia_master.yaml
```

启动 master，默认会在当前目录下创建 ${USER}-kuscia-master/{data、logs} 用来存储 master 的数据、日志：

```bash
# -p 参数传递的是 master 容器映射到主机的端口，保证和主机上现有的端口不冲突即可
# -k 参数传递的是 master 容器 KusciaAPI 映射到主机的 HTTP 端口，保证和主机上现有的端口不冲突即可
./kuscia.sh start -c kuscia_master.yaml -p 18080 -k 18081
```

<span style="color:red;">注意：<br>
1、如果 master 的入口网络存在网关时，为了确保节点与 master 之间通信正常，需要网关符合一些要求，详情请参考[这里](./networkrequirements.md) <br>
2、master 节点默认使用 sqlite 作为存储，如果生产部署，需要配置链接到 mysql 数据库的连接串，具体配置可以参考[这里](./kuscia_config_cn.md#id3)<br>
3、需要对合作方暴露的 Kuscia 端口，可参考 [Kuscia 端口介绍](../kuscia_ports_cn.md) </span>

建议使用 curl -kvvv https://ip:port; 检查一下是否访问能通，正常情况下返回的 HTTP 错误码是 401，内容是：unauthorized。
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
unauthorized
```
#### Tips
本文后续还会经常使用到 docker exec -it ${USER}-kuscia-master xxxxx 类似的命令。建议以如下方式简化输入。
```bash
alias km="docker exec -it ${USER}-kuscia-master"
```
后续相关命令可以简化为km xxxxx。

### 部署 lite 节点

你可以选择在任一台机器上部署 lite 节点 。

#### 部署 lite 节点 alice
在部署 alice 节点之前，我们需要在 master 上注册 alice 节点并获取部署时需要用到的 Token 。
执行以下命令，完成节点注册并从返回中得到 Token （下文将以 abcdefg 为例）。
```bash
docker exec -it ${USER}-kuscia-master sh scripts/deploy/add_domain_lite.sh alice
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

<span style="color:red;">注意：节点 ID 需要符合 DNS 子域名规则要求，详情请参考[这里](https://kubernetes.io/zh-cn/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names)</span>

接下来，登录到安装 alice 的机器上，假设对外暴露的 IP 是 2.2.2.2。
指定 Kuscia 版本：
```bash
# 使用的 Kuscia 镜像，这里使用 latest 版本镜像
export KUSCIA_IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia
```

获取部署脚本，部署脚本会下载到当前目录：

```bash
docker pull $KUSCIA_IMAGE && docker run --rm $KUSCIA_IMAGE cat /home/kuscia/scripts/deploy/kuscia.sh > kuscia.sh && chmod u+x kuscia.sh
```

生成 alice 节点的配置文件：
```bash
# --domain 参数传递的是节点 ID
# --lite-deploy-token 参数传递的是节点部署的 Token
# --master-endpoint 参数传递的是 master 容器对外暴露的 https://IP:PORT，假设 master 对外暴露的 IP 是 1.1.1.1，端口是18080
docker run -it --rm ${KUSCIA_IMAGE} kuscia init --mode lite --domain "alice" --master-endpoint "https://1.1.1.1:18080" --lite-deploy-token "abcdefg" > lite_alice.yaml
```

启动 alice，默认会在当前目录下创建 ${USER}-kuscia-lite-alice/data 目录用来存放 alice 的数据：
```bash
# -p 参数传递的是节点容器映射到主机的端口，保证和主机上现有的端口不冲突即可
# -k 参数传递的是 lite 容器 KusciaAPI 映射到主机的 HTTP 端口，保证和主机上现有的端口不冲突即可
./kuscia.sh start -c lite_alice.yaml -p 28080 -k 28081
```
> 如果 master 与多个 lite 节点部署在同一个物理机上，可以用 -p -k -g -q 参数指定下端口号（例如：./kuscia.sh start -c lite_alice.yaml -p 28080 -k 28081 -g 28082 -q 28083），防止出现端口冲突。

#### 部署 lite 节点 bob

在部署 bob 节点之前，我们需要在 master 注册 bob 节点，并获取到部署时需要用到的 Token 。
执行以下命令，完成节点注册并从返回中得到 Token （下文将以 hijklmn 为例）。
```bash
docker exec -it ${USER}-kuscia-master sh scripts/deploy/add_domain_lite.sh bob
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

接下来，登录到安装 bob 的机器上，假设对暴露的 IP 是 3.3.3.3。
指定 Kuscia 版本：
```bash
# 使用的 Kuscia 镜像，这里使用 latest 版本镜像
export KUSCIA_IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia
```
<span style="color:red;">注意：节点 id 需要符合 DNS 子域名规则要求，详情请参考[这里](https://kubernetes.io/zh-cn/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names)</span>

获取部署脚本，部署脚本会下载到当前目录：

```bash
docker pull $KUSCIA_IMAGE && docker run --rm $KUSCIA_IMAGE cat /home/kuscia/scripts/deploy/kuscia.sh > kuscia.sh && chmod u+x kuscia.sh
```

生成 bob 节点的配置文件：
```bash
# --domain 参数传递的是节点 ID
# --lite-deploy-token 参数传递的是节点部署的 Token
# --master-endpoint 参数传递的是 master 容器对外暴露的 https://IP:PORT，假设 master 对外暴露的 IP 是 1.1.1.1，端口是18080
docker run -it --rm ${KUSCIA_IMAGE} kuscia init --mode lite --domain "bob" --master-endpoint "https://1.1.1.1:18080" --lite-deploy-token "hijklmn" > lite_bob.yaml
```

启动 bob，默认会在当前目录下创建 ${USER}-kuscia-lite-bob/data 目录用来存放 bob 的数据：
```bash
# -p 参数传递的是节点容器映射到主机的端口，保证和主机上现有的端口不冲突即可
# -k 参数传递的是 lite 容器 KusciaAPI 映射到主机的 HTTP 端口，保证和主机上现有的端口不冲突即可
./kuscia.sh start -c lite_bob.yaml -p 38080 -k 38081
```
> 如果 master 与多个 lite 节点部署在同一个物理机上，可以用 -p -k -g -q 参数指定下端口号（例如：./kuscia.sh start -c lite_bob.yaml -p 38080 -k 38081 -g 38082 -q 38083），防止出现端口冲突。

### 配置授权

如果要发起由两个 lite 节点参与的任务，你需要给这两个节点之间建立授权。

#### 创建 alice 和 bob 之间的授权

在 master 机器上执行创建授权的命令
```bash
# 为了减少授权错误的排查成本，建议在alice/bob容器内分别（curl）访问的对方地址判定是否能联通，之后再授权
# 示例：curl -vvv http://ip:port 返回正常的HTTP错误码是401
# bob 节点的访问地址，一般是 bob 的 http://ip:port，如上文，bob 的 ip 是 3.3.3.3 ，port 如上文为 38080.
docker exec -it ${USER}-kuscia-master sh scripts/deploy/create_cluster_domain_route.sh alice bob http://3.3.3.3:38080
# alice 节点的访问地址，一般是 alice 的 http://ip:port，如上文，alice 的 ip 是 3.3.3.3，port 如上文为 28080.
docker exec -it ${USER}-kuscia-master sh scripts/deploy/create_cluster_domain_route.sh bob alice http://2.2.2.2:28080
```

执行以下命令：
```bash
docker exec -it ${USER}-kuscia-master kubectl get cdr
```

当 `type` 为 Ready 的 condition 的 `status` 值为 "True" 则说明 alice 和 bob 之间授权建立成功。

<span style="color:red;">注意：如果节点之间的入口网络存在网关时，为了确保节点与节点之间通信正常，需要网关符合一些要求，详情请参考[这里](./networkrequirements.md)</span>

### 运行任务
接下来，我们运行一个测试任务以验证部署是否成功。
#### 准备数据

##### 获取测试数据集
登录到安装 alice 的机器上，将默认的测试数据拷贝到之前部署目录的 ${USER}-kuscia-lite-alice/data 下

```bash
docker pull $KUSCIA_IMAGE && docker run --rm $KUSCIA_IMAGE cat /home/kuscia/var/storage/data/alice.csv > ${USER}-kuscia-lite-alice/data/alice.csv
```

登录到安装 bob 的机器上，将默认的测试数据拷贝到之前部署目录的 ${USER}-kuscia-lite-bob/data 下

```bash
docker pull $KUSCIA_IMAGE && docker run --rm $KUSCIA_IMAGE cat /home/kuscia/var/storage/data/bob.csv > ${USER}-kuscia-lite-bob/data/bob.csv
```

##### 创建测试数据表

登录到安装 master 的机器上，为 alice 和 bob 的测试数据创建 domaindata

```bash
docker exec -it ${USER}-kuscia-master scripts/deploy/create_domaindata_alice_table.sh alice
docker exec -it ${USER}-kuscia-master scripts/deploy/create_domaindata_bob_table.sh bob
```

##### 创建测试数据表授权

登录到安装 master 的机器上，为 alice 的测试数据创建 domaindatagrant

```bash
docker exec -it ${USER}-kuscia-master curl -X POST 'https://127.0.0.1:8082/api/v1/domaindatagrant/create' --header "Token: $(docker exec -it ${USER}-kuscia-master cat /home/kuscia/var/certs/token)" --header 'Content-Type: application/json' -d '{
 "grant_domain": "bob",
 "description": {"domaindatagrant":"alice-bob"},
 "domain_id": "alice",
 "domaindata_id": "alice-table"
}' --cacert /home/kuscia/var/certs/ca.crt --cert /home/kuscia/var/certs/ca.crt --key /home/kuscia/var/certs/ca.key
```

同理，登录到安装 master 的机器上，为 bob 的测试数据创建 domaindatagrant

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
任务运行遇到网络错误时，可以参考[这里](../reference/troubleshoot/networktroubleshoot.md)排查