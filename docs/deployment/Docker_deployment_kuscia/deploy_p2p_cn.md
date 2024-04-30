# 多机部署点对点集群

## 前言

本教程帮助你在多台机器上使用 [点对点组网模式](../reference/architecture_cn.md#点对点组网模式) 来部署 Kuscia 集群。

当前 Kuscia 节点之间只支持 Token 的身份认证方式，在跨机器部署的场景下流程较为繁琐，后续本教程会持续更新优化。

## 前置准备

在部署 Kuscia 之前，请确保环境准备齐全，包括所有必要的软件、资源、操作系统版本和网络环境等满足要求，以确保部署过程顺畅进行，详情参考[部署要求](../deployment/deploy_check.md)

## 部署流程（基于 TOKEN 认证）

### 部署 alice 节点

登录到安装 alice 的机器上，本文为叙述方便，假定节点 ID 为 alice ，对外可访问的 PORT 是 11080 。

指定 Kuscia 使用的镜像版本，这里使用 latest 版本
```bash
export KUSCIA_IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia
```

获取部署脚本，部署脚本会下载到当前目录：

```
docker pull $KUSCIA_IMAGE && docker run --rm $KUSCIA_IMAGE cat /home/kuscia/scripts/deploy/kuscia.sh > kuscia.sh && chmod u+x kuscia.sh
```

生成 alice 节点配置文件：
```bash
# --domain 参数传递的是节点 ID
docker run -it --rm ${KUSCIA_IMAGE} kuscia init --mode autonomy --domain "alice" > autonomy_alice.yaml
```

启动节点，默认会在当前目录下创建 ${USER}-kuscia-autonomy-alice/data 目录用来存放 alice 的数据。部署节点需要使用 `kuscia.sh` 脚本并传入节点配置文件：

```bash
# -p 参数传递的是节点容器映射到主机的 HTTPS 端口，保证和主机上现有的端口不冲突即可
# -k 参数传递的是节点容器 KusciaAPI 映射到主机的 MTLS 端口，保证和主机上现有的端口不冲突即可
./kuscia.sh start -c autonomy_alice.yaml -p 11080 -k 11081
```
> 如果多个 lite 节点部署在同一个物理机上，可以用 -p -k -g -q 参数指定下端口号（例如：./kuscia.sh start -c autonomy_alice.yaml -p 11080 -k 11081 -g 11082 -q 11083），防止出现端口冲突。

<span style="color:red;">注意：<br>
1、如果节点之间的入口网络存在网关时，为了确保节点与 master 之间通信正常，需要网关符合一些要求，详情请参考[这里](./networkrequirements.md) <br>
2、alice、bob 节点默认使用 sqlite 作为存储，如果生产部署，需要配置链接到 mysql 数据库的连接串，具体配置可以参考[这里](./kuscia_config_cn.md#id3)<br>
3、需要对合作方暴露的 Kuscia 端口，可参考 [Kuscia 端口介绍](../kuscia_ports_cn.md) </span>


### 部署 bob 节点

你可以选择在另一台机器上部署 bob 节点，详细步骤参考上述 alice 节点部署的流程，唯一不同的是在部署前准备参数时配置 bob 节点相关的参数。假定节点 ID 为 bob ，对外可访问的 PORT 是 21080 。


### 配置证书
在两个 Autonomy 节点建立通信之前，你需要先给这两个节点互换证书。

#### alice 颁发证书给 bob

准备 alice 的公钥，在 alice 节点的机器上，可以看到包含公钥的 crt 文件：
```bash 
# [alice 机器] 将 domain.crt 从容器内部拷贝出来并重命名为 alice.domain.crt
docker cp ${USER}-kuscia-autonomy-alice:/home/kuscia/var/certs/domain.crt alice.domain.crt
```

将 alice 的公钥 alice.domain.crt 拷贝到 bob 容器的 /home/kuscia/var/certs/ 目录中：

```bash
# [bob 机器] 确保 alice.domain.crt 位于 bob 容器的 /home/kuscia/var/certs/ 目录中
docker cp alice.domain.crt ${USER}-kuscia-autonomy-bob:/home/kuscia/var/certs/
```

在 bob 里添加 alice 的证书等信息：

```bash
# [bob 机器] 添加 alice 的证书等信息
docker exec -it ${USER}-kuscia-autonomy-bob scripts/deploy/add_domain.sh alice p2p
```

#### bob 颁发证书给 alice

准备 bob 的公钥，在 bob 节点的机器上，可以看到包含公钥的 crt 文件：

```bash
# [bob 机器] 将 domain.crt 从容器内部拷贝出来并重命名为 bob.domain.crt
docker cp ${USER}-kuscia-autonomy-bob:/home/kuscia/var/certs/domain.crt bob.domain.crt
```

将 bob 的公钥 bob.domain.crt 拷贝到 alice 容器的 /home/kuscia/var/certs/ 目录中：

```bash
# [alice 机器] 确保 bob.domain.crt 位于 alice 容器的 /home/kuscia/var/certs/ 目录中
docker cp bob.domain.crt ${USER}-kuscia-autonomy-alice:/home/kuscia/var/certs/
```

在 alice 里添加 bob 的证书等信息：

```bash
# [alice 机器] 添加 alice 的证书等信息
docker exec -it ${USER}-kuscia-autonomy-alice scripts/deploy/add_domain.sh bob p2p
```

### 配置授权

如果要发起由两个 Autonomy 节点参与的任务，你需要给这两个节点之间建立授权。

#### 创建 alice 到 bob 的授权

```bash
# [alice 机器]
# 假设 bob 的对外 IP 是 2.2.2.2，21080 是上文中 bob 暴露的访问端口
# 为了减少授权错误的排查成本，建议在 alice 容器内(curl)访问 bob 地址判定是否能联通，之后再授权
# 示例：curl -kvvv https://2.2.2.2:21080 返回正常的 HTTP 错误码是 401
docker exec -it ${USER}-kuscia-autonomy-alice scripts/deploy/join_to_host.sh alice bob https://2.2.2.2:21080
```

#### 创建 bob 到 alice 的授权

```bash
# [bob 机器]
# 假设 alice 的对外 IP 是 1.1.1.1，11080 是上文中 alice 暴露的访问端口
# 为了减少授权错误的排查成本，建议在 bob 容器内(curl)访问 alice 地址判定是否能联通，之后再授权
# 示例：curl -kvvv https://1.1.1.1:11080 返回正常的 HTTP 错误码是 401
docker exec -it ${USER}-kuscia-autonomy-bob scripts/deploy/join_to_host.sh bob alice https://1.1.1.1:11080
```

#### 检查节点之间网络通信状态
- 方法一：

[alice 机器] 执行以下命令：
```bash
docker exec -it ${USER}-kuscia-autonomy-alice kubectl get cdr alice-bob
```
[bob 机器] 执行以下命令：
```bash
docker exec -it ${USER}-kuscia-autonomy-bob kubectl get cdr bob-alice
```
当 "READR" 列为 "True"时，说明 alice 和 bob 之间授权建立成功。

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

授权失败，请参考[授权错误排查](../reference/troubleshoot/networkauthorizationcheck.md)文档

<span style="color:red;">注意：如果节点之间的入口网络存在网关时，为了确保节点与节点之间通信正常，需要网关符合一些要求，详情请参考[这里](./networkrequirements.md)</span>

### 准备测试数据
- alice 节点准备测试数据

登录到安装 alice 的机器上，将默认的测试数据拷贝到之前部署目录的 ${USER}-kuscia-autonomy-alice/data 下

```bash
docker pull $KUSCIA_IMAGE && docker run --rm $KUSCIA_IMAGE cat /home/kuscia/var/storage/data/alice.csv > ${USER}-kuscia-autonomy-alice/data/alice.csv
```
为 alice 的测试数据创建 domaindata
```bash
docker exec -it ${USER}-kuscia-autonomy-alice scripts/deploy/create_domaindata_alice_table.sh alice
```
为 alice 的测试数据创建 domaindatagrant

```bash
docker exec -it ${USER}-kuscia-autonomy-alice curl -X POST 'https://127.0.0.1:8082/api/v1/domaindatagrant/create' --header "Token: $(docker exec -it ${USER}-kuscia-autonomy-alice cat /home/kuscia/var/certs/token)" --header 'Content-Type: application/json' -d '{
 "grant_domain": "bob",
 "description": {"domaindatagrant":"alice-bob"},
 "domain_id": "alice",
 "domaindata_id": "alice-table"
}' --cacert /home/kuscia/var/certs/ca.crt --cert /home/kuscia/var/certs/ca.crt --key /home/kuscia/var/certs/ca.key
```
- bob 节点准备测试数据

登录到安装 bob 的机器上，将默认的测试数据拷贝到之前部署目录的 ${USER}-kuscia-autonomy-alice/data 下

```bash
docker pull $KUSCIA_IMAGE && docker run --rm $KUSCIA_IMAGE cat /home/kuscia/var/storage/data/bob.csv > ${USER}-kuscia-autonomy-bob/data/bob.csv
```
为 bob 的测试数据创建 domaindata
```bash
docker exec -it ${USER}-kuscia-autonomy-bob scripts/deploy/create_domaindata_bob_table.sh bob
```
为 bob 的测试数据创建 domaindatagrant

```bash
docker exec -it ${USER}-kuscia-autonomy-bob curl -X POST 'https://127.0.0.1:8082/api/v1/domaindatagrant/create' --header "Token: $(docker exec -it ${USER}-kuscia-autonomy-bob cat /home/kuscia/var/certs/token)" --header 'Content-Type: application/json' -d '{
 "grant_domain": "alice",
 "description": {"domaindatagrant":"bob-alice"},
 "domain_id": "bob",
 "domaindata_id": "bob-table"
}' --cacert /home/kuscia/var/certs/ca.crt --cert /home/kuscia/var/certs/ca.crt --key /home/kuscia/var/certs/ca.key
```

### 执行作业

创建并启动作业（两方 PSI 任务）, 以 alice 节点机器上执行命令为例
```bash 
docker exec -it ${USER}-kuscia-autonomy-alice scripts/user/create_example_job.sh
```

查看作业状态
```bash
docker exec -it ${USER}-kuscia-autonomy-alice kubectl get kj -n cross-domain
```
任务运行遇到网络错误时，可以参考[这里](../reference/troubleshoot/networktroubleshoot.md)排查
