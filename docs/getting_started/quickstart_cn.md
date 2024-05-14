# Kuscia 入门教程 —— 快速开始

你将会在单台机器上准备 Kuscia 需要的环境、快速部署一个示例 Kuscia 集群，然后尝试运行一个 [SecretFlow] 作业。

[SecretFlow]: https://www.secretflow.org.cn/docs/secretflow

## 部署模式：说明

在部署时有两种组网模式可供选择：

- [中心化组网模式](../reference/architecture_cn.md#中心化组网模式)：启动一个控制平面（master）容器和两个 Lite 节点（alice 和 bob）容器
- [点对点组网模式](../reference/architecture_cn.md#点对点组网模式)：启动两个 Autonomy 节点（alice 和 bob）容器

你可以选择其中任意一种或两种模式进行部署体验，在单台机器上可以同时部署两种模式。

## 环境

### 机器

操作系统：macOS, CentOS7, CentOS8, Ubuntu 16.04 及以上版本, Windows(Ubuntu on WSL2)

资源：8 core / 16G memory / 200G hard disk

CPU 架构：x86

> 单机体验版需要部署多个节点和平台，且要预留资源运行各类隐私计算任务，所以这里的资源需求要比节点最低资源大一些

### 环境准备

在部署 Kuscia 之前，请确保环境准备齐全，包括所有必要的软件、资源、操作系统版本和网络环境等满足要求，以确保部署过程顺畅进行，详情参考[部署要求](../deployment/deploy_check.md)

Kuscia 的部署需要依赖 Docker 环境，Docker 的安装请参考[官方文档](https://docs.docker.com/engine/install/)。以下为 CentOS 系统安装 Docker 的示例：

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

### 关于 macOS

macOS 默认给单个 docker container 分配了 2G 内存，请参考[官方文档](https://docs.docker.com/desktop/settings/mac/)将内存上限提高为 6G（Kuscia 2G + SecretFlow 4G) 。

## 部署体验
> 本文旨在帮助您快速体验 Kuscia，不涉及任何宿主机端口暴露配置。如需暴露端口，请前往[多机部署](../deployment/Docker_deployment_kuscia/deploy_p2p_cn.md)

### 前置操作

配置 Kuscia 镜像，以下示例选择使用 latest 版本镜像（更多镜像版本请参考 [Kuscia tags](https://hub.docker.com/r/secretflow/kuscia/tags)）：

```bash
# Docker Hub 镜像
export KUSCIA_IMAGE=secretflow/kuscia:0.8.0b0

# 阿里云镜像（推荐国内用户使用）
export KUSCIA_IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia:0.8.0b0
```

获取 Kuscia 安装脚本，安装脚本会下载到当前目录：

```
docker pull $KUSCIA_IMAGE && docker run --rm $KUSCIA_IMAGE cat /home/kuscia/scripts/deploy/kuscia.sh > kuscia.sh && chmod u+x kuscia.sh
```

### 中心化组网模式

```bash
# 启动集群，会拉起 3 个 docker 容器，包括一个控制平面 master 和两个 Lite 节点 alice 和 bob。
./kuscia.sh center

# 创建并启动作业（两方 PSI 任务）。
docker exec -it ${USER}-kuscia-master scripts/user/create_example_job.sh

# 查看作业状态。
docker exec -it ${USER}-kuscia-master kubectl get kj -n cross-domain
```

{#p2p-network-mode}

### 点对点组网模式

```bash
# 启动集群，会拉起两个 docker 容器，分别表示 Autonomy 节点 alice 和 bob。
./kuscia.sh p2p

# 登入 alice 节点容器（或 bob 节点容器）创建并启动作业（两方 PSI 任务）。
docker exec -it ${USER}-kuscia-autonomy-alice scripts/user/create_example_job.sh

# 查看作业状态。
docker exec -it ${USER}-kuscia-autonomy-alice kubectl get kj -n cross-domain
```

### 中心化 x 中心化组网模式

```bash
# 启动集群，会拉起 4 个 docker 容器，包括两个控制平面 master-alice、master-bob 和两个 Lite 节点 alice、bob。
./kuscia.sh cxc

# 登入 master-alice 容器创建并启动作业（两方 PSI 任务）。
docker exec -it ${USER}-kuscia-master-alice scripts/user/create_example_job.sh

# 查看作业状态。
docker exec -it ${USER}-kuscia-master-alice kubectl get kj -n cross-domain
```

### 中心化 x 点对点组网模式

```bash
# 启动集群，会拉起 3 个 docker 容器，包括一个控制平面 master-alice 和一个 Lite 节点 alice、一个 Autonomy 节点 bob。
./kuscia.sh cxp

# 登入 master-alice 容器创建并启动作业（两方 PSI 任务）。
docker exec -it ${USER}-kuscia-master-alice scripts/user/create_example_job.sh

# 查看作业状态。
docker exec -it ${USER}-kuscia-master-alice kubectl get kj -n cross-domain
```

## 作业状态

如果作业执行成功，则 `kubectl get kj` 命令会显示类似下方的输出，Succeeded 表示成功状态：

```bash
NAME                             STARTTIME   COMPLETIONTIME   LASTRECONCILETIME   PHASE
secretflow-task-20230406162606   50s         50s              50s                 Succeeded
```

同时，在 alice 和 bob 节点容器中能看到 PSI 结果输出文件：

```bash
# 以中心化集群模式下的 alice 节点为例：
docker exec -it ${USER}-kuscia-lite-alice cat var/storage/data/psi-output.csv

# 以点对点集群模式下的 alice 节点为例：
docker exec -it ${USER}-kuscia-autonomy-alice cat var/storage/data/psi-output.csv
```

结果输出（仅前 4 行）：

```bash
id1,age,education,default,balance,housing,loan,day,duration,campaign,pdays,previous,job_blue-collar,job_entrepreneur,job_housemaid,job_management,job_retired,job_self-employed,job_services,job_student,job_technician,job_unemployed,marital_divorced,marital_married,marital_single
0,1.5306293518221676,-0.3053083468611561,-0.117184991347747,0.2145303545250443,1.0358211226635177,-0.3925867711542392,-1.2618906002715358,1.9048694929309795,-0.5762472500554522,-0.4852053503766987,-0.3619838367558999,-0.4639325546169564,-0.1731690076375218,-0.1580237499348341,1.3543943126559297,-0.2734046609851663,-0.1960131708137989,-0.3006459829345367,-0.1700475343179241,-0.4466166954522901,-0.1840186845246444,-0.3589389310523966,0.8806950470683438,-0.6902303314457872
1,1.2763683978477116,-0.3053083468611561,-0.117184991347747,-0.4786597597189064,-0.9654176557324816,-0.3925867711542392,-1.2618906002715358,3.1181827517827982,-0.5762472500554522,-0.4852053503766987,-0.3619838367558999,-0.4639325546169564,-0.1731690076375218,-0.1580237499348341,1.3543943126559297,-0.2734046609851663,-0.1960131708137989,-0.3006459829345367,-0.1700475343179241,-0.4466166954522901,-0.1840186845246444,-0.3589389310523966,0.8806950470683438,-0.6902303314457872
10,-0.2491973259990245,-0.3053083468611561,-0.117184991347747,-0.4620690476721626,1.0358211226635177,-0.3925867711542392,-1.0230801579932494,1.1740266828931782,-0.5762472500554522,-0.4852053503766987,-0.3619838367558999,-0.4639325546169564,-0.1731690076375218,-0.1580237499348341,1.3543943126559297,-0.2734046609851663,-0.1960131708137989,-0.3006459829345367,-0.1700475343179241,-0.4466166954522901,-0.1840186845246444,-0.3589389310523966,-1.1354668149080638,1.4487917358041271
```

## 停止/卸载体验集群

{#stop}

### 停止体验集群

如果你需要停止并卸载体验集群，可以直接运行[卸载脚本](#uninstall)。

获取 Kuscia 停止脚本，脚本会下载到当前目录：

```bash
docker pull $KUSCIA_IMAGE && docker run --rm $KUSCIA_IMAGE cat /home/kuscia/scripts/deploy/stop.sh > stop.sh && chmod u+x stop.sh
```

使用方法：

```bash
./stop.sh [center/p2p/all]

# 停止点对点组网模式集群
./stop.sh p2p

# 停止中心化组网模式集群
./stop.sh center

# 停止所有组网模式集群（可省略参数）
./stop.sh all
```

{#uninstall}

### 卸载体验集群

获取 Kuscia 卸载脚本，脚本会下载到当前目录：

```bash
docker pull $KUSCIA_IMAGE && docker run --rm $KUSCIA_IMAGE cat /home/kuscia/scripts/deploy/uninstall.sh > uninstall.sh && chmod u+x uninstall.sh
```

与[停止脚本](#stop)使用方法相同，运行卸载脚本将卸载相应组网模式的集群，包括删除 Kuscia 容器、volume 和 network（若无其他容器使用）等。例如：

```bash
# 卸载所有组网模式集群
./uninstall.sh
```

## 接下来

请继续阅读 [了解 KusciaJob][part-2] 章节，来了解示例作业背后的细节。

[part-2]: ./run_secretflow_cn.md
