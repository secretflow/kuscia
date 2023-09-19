# 多机部署中心化集群

## 前言

本教程帮助你在多台机器上使用 [中心化组网模式](../reference/architecture_cn.md#中心化组网模式) 来部署 Kuscia 集群。


## 部署流程（基于TOKEN认证）

### 部署 master 节点
登录到安装 master 的机器上，假设对外ip是1.1.1.1。
指定 kuscia 版本：
```bash
# 使用的 Kuscia 镜像，这里使用 latest 版本镜像
export KUSCIA_IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia
```

获取部署脚本，部署脚本会下载到当前目录：

```bash
docker run --rm $KUSCIA_IMAGE cat /home/kuscia/scripts/deploy/deploy.sh > deploy.sh && chmod u+x deploy.sh
```

启动 master，默认会在当前目录下创建 kuscia-master-kuscia-system-certs 目录用来存放 master 的公私钥和证书：

```bash
# -i 参数传递的是 master 容器对外暴露的 IP，通常是主机 IP。
# -p 参数传递的是 master 容器映射到主机的端口，保证和主机上现有的端口不冲突即可
# -k 参数传递的是 master 容器 KusciaAPI 映射到主机的 HTTP 端口，保证和主机上现有的端口不冲突即可
./deploy.sh master -i 1.1.1.1 -p 18080 -k 18082
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
在部署 alice 节点之前，我们需要在 master 上注册 alice 节点并获取部署时需要用到的 token 。
执行以下命令，完成节点注册并从返回中得到 token （下文将以abcdefg为例）。
```bash
docker exec -it ${USER}-kuscia-master sh scripts/deploy/add_domain_lite.sh alice
abcdefg
```
如果token遗忘了，可以通过该命令重新获取
```bash
docker exec -it ${USER}-kuscia-master kubectl get domain alice -o=jsonpath='{.status.deployTokenStatuses[?(@.state=="unused")].token}' && echo
abcdefg
```

接下来，登录到安装 alice 的机器上，假设对外ip是2.2.2.2。
指定 kuscia 版本：
```bash
# 使用的 Kuscia 镜像，这里使用 latest 版本镜像
export KUSCIA_IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia
```

获取部署脚本，部署脚本会下载到当前目录：

```bash
docker run --rm $KUSCIA_IMAGE cat /home/kuscia/scripts/deploy/deploy.sh > deploy.sh && chmod u+x deploy.sh
```

启动 alice。默认会在当前目录下创建 kuscia-lite-alice-certs 目录用来存放 alice 的公私钥和证书。默认会在当前目录下创建 kuscia-lite-alice-data 目录用来存放 alice 的数据：
```bash
# -n 参数传递的是节点 ID
# -t 参数传递的是节点部署的 token
# -m 参数传递的是 master 容器对外暴露的 https://IP:PORT，如上文中 master 的 ip 是1.1.1.1，端口是18080
# -p 参数传递的是节点容器映射到主机的端口，保证和主机上现有的端口不冲突即可
./deploy.sh lite -n alice -t abcdefg -m https://1.1.1.1:18080 -p 28080
```


#### 部署 lite 节点 bob

在部署 bob 节点之前，我们需要在 master 注册 bob 节点，并获取到部署时需要用到的 token 。
执行以下命令，完成节点注册并从返回中得到 token （下文将以hijklmn为例）。
```bash
docker exec -it ${USER}-kuscia-master sh scripts/deploy/add_domain_lite.sh bob
hijklmn
```
如果token遗忘了，可以通过该命令重新获取
```bash
docker exec -it ${USER}-kuscia-master kubectl get domain bob -o=jsonpath='{.status.deployTokenStatuses[?(@.state=="unused")].token}' && echo
hijklmn
```

接下来，登录到安装 bob 的机器上，假设对外ip是3.3.3.3。
指定 kuscia 版本：
```bash
# 使用的 Kuscia 镜像，这里使用 latest 版本镜像
export KUSCIA_IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia
```

获取部署脚本，部署脚本会下载到当前目录：

```bash
docker run --rm $KUSCIA_IMAGE cat /home/kuscia/scripts/deploy/deploy.sh > deploy.sh && chmod u+x deploy.sh
```

启动 bob。默认会在当前目录下创建 kuscia-lite-bob-certs 目录用来存放 bob 的公私钥和证书。默认会在当前目录下创建 kuscia-lite-bob-data 目录用来存放 bob 的数据：
```bash
# -n 参数传递的是节点 ID
# -t 参数传递的是节点部署的 token
# -m 参数传递的是 master 容器对外暴露的 https://IP:PORT，如上文中 master 的 ip 是1.1.1.1，端口是18080
# -p 参数传递的是节点容器映射到主机的端口，保证和主机上现有的端口不冲突即可
./deploy.sh lite -n bob -t hijklmn -m https://1.1.1.1:18080 -p 38080
```


### 配置授权

如果要发起由两个 lite 节点参与的任务，你需要给这两个节点之间建立授权。

#### 创建 alice 和 bob 之间的授权

在 master 机器上执行创建授权的命令
```bash
# bob 节点的访问地址，一般是 bob 的 http://ip:port，如上文，bob 的 ip 是 3.3.3.3 ，port 如上文为 38080.
docker exec -it ${USER}-kuscia-master sh scripts/deploy/create_cluster_domain_route.sh alice bob http://3.3.3.3:38080
# alice 节点的访问地址，一般是 alice 的 http://ip:port，如上文，alice 的 ip 是 3.3.3.3，port 如上文为 28080.
docker exec -it ${USER}-kuscia-master sh scripts/deploy/create_cluster_domain_route.sh bob alice http://2.2.2.2:28080
```

执行以下命令，查看是否有内容，如果有说明 alice 到 bob 授权建立成功。
```bash
docker exec -it ${USER}-kuscia-master kubectl get cdr alice-bob -o=jsonpath="{.status.tokenStatus.sourceTokens[*]}"
```

执行以下命令，查看是否有内容，如果有说明 bob 到 alice 授权建立成功。
```bash
docker exec -it ${USER}-kuscia-master kubectl get cdr bob-alice -o=jsonpath="{.status.tokenStatus.sourceTokens[*]}"
```

### 运行任务
接下来，我们运行一个测试任务以验证部署是否成功。
#### 准备数据

##### 获取测试数据集
登录到安装 alice 的机器上，将默认的测试数据拷贝到当前目录的kuscia-lite-alice-data下

```bash
docker run --rm $KUSCIA_IMAGE cat /home/kuscia/var/storage/data/alice.csv > kuscia-lite-alice-data/alice.csv
```

登录到安装 bob 的机器上，将默认的测试数据拷贝到当前目录的kuscia-lite-bob-data下

```bash
docker run --rm $KUSCIA_IMAGE cat /home/kuscia/var/storage/data/bob.csv > kuscia-lite-bob-data/bob.csv
```

##### 创建测试数据表

登录到安装 master 的机器上，为alice和bob的测试数据创建domaindata

```
docker exec -it ${USER}-kuscia-master scripts/deploy/create_domaindata_alice_table.sh alice
docker exec -it ${USER}-kuscia-master scripts/deploy/create_domaindata_bob_table.sh bob
```

#### 执行测试作业

登录到安装 master 的机器上

创建并启动作业（两方 PSI 任务）
```bash
docker exec -it ${USER}-kuscia-master scripts/user/create_example_job.sh
```

查看作业状态
```bash
docker exec -it ${USER}-kuscia-master kubectl get kj
```
