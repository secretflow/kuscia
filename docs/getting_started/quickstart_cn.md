# 快速入门

## 说明

本教程帮助你在单台机器上快速部署 Kuscia 集群并体验 SecretFlow。

在部署时有两种组网模式可供选择：

- [中心化组网模式](../reference/architecture_cn.html#id7)：启动一个控制平面（master）容器和两个 Lite 节点（alice 和 bob）容器
- [点对点组网模式](../reference/architecture_cn.html#id8)：启动两个 Autonomy 节点（alice 和 bob）容器

你可以选择其中任意一种或两种模式进行部署体验，在单台机器上可以同时部署两种模式。

## 环境

### 机器

操作系统：macOS, CentOS7, CentOS8

资源：8 core / 16G memory / 200G hard disk

### 关于 macOS

macOS 默认给单个 docker container 分配了 2G 内存，请参考[官方文档](https://docs.docker.com/desktop/settings/mac/)将内存上限提高为 6G（Kuscia 2G + SecretFlow 4G) 。

### 环境准备

Kuscia 的部署需要依赖 docker 环境，docker 的安装请参考[官方文档](https://docs.docker.com/engine/install/)。以下为 CentOS 系统安装 docker 的示例：

```bash
# 安装 docker。
yum install -y yum-utils
yum-config-manager \
	--add-repo \
	https://download.docker.com/linux/centos/docker-ce.repo
yum install -y docker-ce docker-ce-cli containerd.io

# 启动 docker。
systemctl start docker
```

## 部署体验

### 前置操作

配置 Kuscia 镜像，以下示例选择使用 latest 版本镜像：

```bash
export KUSCIA_IMAGE=secretflow/kuscia
```

获取 Kuscia 安装脚本，安装脚本会下载到当前目录：

```
docker run --rm -v $(pwd):/tmp/kuscia $KUSCIA_IMAGE cp -f /home/kuscia/scripts/deploy/start_standalone.sh /tmp/kuscia
```

### 中心化组网模式

```bash
# 启动集群，会拉起 3 个 docker 容器，包括一个控制平面 master 和两个 Lite 节点 alice 和 bob。
./start_standalone.sh center

# 登入 master 容器。
docker exec -it ${USER}-kuscia-master bash

# 创建并启动作业（两方 PSI 任务）。
scripts/user/create_example_job.sh

# 查看作业状态。
kubectl get kj
```

### 点对点组网模式

```bash
# 启动集群，会拉起两个 docker 容器，分别表示 Autonomy 节点 alice 和 bob。
./start_standalone.sh p2p

# 登入 alice 节点容器（或 bob 节点容器）。
docker exec -it ${USER}-kuscia-autonomy-alice bash

# 创建并启动作业（两方 PSI 任务）。
scripts/user/create_example_job.sh

# 查看作业状态。
kubectl get kj
```

## 作业状态

如果作业执行成功，则 `kubectl get kj` 命令会显示类似下方的输出，Succeeded 表示成功状态：

```bash
NAME                             STARTTIME   COMPLETIONTIME   LASTRECONCILETIME   PHASE
secretflow-task-20230406162606   50s         50s              50s                 Succeeded
```

同时，在 alice 和 bob 节点容器中能看到 PSI 结果输出文件，以中心化集群模式下的 alice 节点为例：

```bash
docker exec -it ${USER}-kuscia-lite-alice cat var/storage/data/alice_psi_out.csv
```

结果输出：

```bash
id1,item,feature1
K200,B,BBB
K200,C,CCC
K300,D,DDD
K400,E,EEE
K400,F,FFF
K500,G,GGG
```



## 接下来

如果你希望使用自己的数据来执行作业，请参考[如何运行一个 Secretflow 任务](../tutorial/run_secretflow_cn.html)。

如果你希望体验在安全沙箱中执行作业，请参考[如何启用安全沙箱](../tutorial/enable_security_sandbox_cn.html)。

