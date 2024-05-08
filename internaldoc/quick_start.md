## 环境

### 机器

操作系统：macOS, CentOS7, CentOS8

资源：8 Core / 16G Memory / 200G Hard Disk

### 关于 macOS

MacOS 默认给单个 Docker container 分配了 2G 内存，请参考[官方文档](https://docs.docker.com/desktop/settings/mac/)将内存上限提高为 6G（Kuscia 2G + SecretFlow 4G) 。

### 环境准备

Kuscia 的部署需要依赖 Docker 环境，Docker 的安装请参考[官方文档](https://docs.docker.com/engine/install/)。以下为 CentOS 系统安装 Docker 的示例：

```bash
# 安装 docker
yum install -y yum-utils
yum-config-manager \
	--add-repo \
	https://download.docker.com/linux/centos/docker-ce.repo
yum install -y docker-ce docker-ce-cli containerd.io

# 启动 docker
systemctl start docker
```



## 预备工作

由于内部测试的应用镜像（例如 secretflow 镜像）存储在私有镜像仓库，因此需要提前配置镜像仓库的地址和帐密，此配置内容不对外网暴露。

```bash
mkdir -p ~/kuscia

vim ~/kuscia/env.list
# 写入以下配置项
REGISTRY_ENDPOINT=registry.cn-hangzhou.aliyuncs.com/nueva-stack
REGISTRY_USERNAME={{your_username}}
REGISTRY_PASSWORD={{your_password}}

# 或直接使用命令写入
echo "REGISTRY_ENDPOINT=registry.cn-hangzhou.aliyuncs.com/nueva-stack
REGISTRY_USERNAME={{your_username}}
REGISTRY_PASSWORD={{your_password}}
" > ~/kuscia/env.list
```

说明：如果在部署阶段或任务执行前根据部署提示将 secretflow 镜像准备好，则 REGISTRY_USERNAME 和 REGISTRY_PASSWORD 两个配置项无须配置。



## 部署体验

### 前置操作

配置 Kuscia 镜像，以下示例选择使用 latest 版本镜像：

```bash
# 外网latest镜像
export KUSCIA_IMAGE=secretflow/kuscia
# 内网latest镜像
export KUSCIA_IMAGE=reg.docker.alibaba-inc.com/secretflow/kuscia-anolis
# 内部阿里云latest镜像
export KUSCIA_IMAGE=registry.cn-hangzhou.aliyuncs.com/nueva-stack/kuscia-anolis
```

获取 Kuscia 安装脚本，安装脚本会下载到当前目录：

```
docker run --rm -v $(pwd):/tmp/kuscia $KUSCIA_IMAGE cp -f /home/kuscia/scripts/deploy/start_standalone.sh /tmp/kuscia
```

### 中心化组网模式

```bash
# 启动集群，会拉起 3 个 docker 容器，包括一个控制平面 master 和两个 Lite 节点 alice 和 bob。
./start_standalone.sh center

# 登入 master 容器
docker exec -it ${USER}-kuscia-master bash

# 执行作业 (两方PSI任务)
scripts/user/create_example_job.sh

# 查看作业状态
kubectl get kj -n cross-domain
```

### 点对点组网模式

```bash
# 启动集群，会拉起两个 docker 容器，分别表示 Autonomy 节点 alice 和 bob。
./start_standalone.sh p2p

# 登入 alice 节点容器（或 bob 节点容器）
docker exec -it ${USER}-kuscia-autonomy-alice bash

# 执行作业 (两方PSI任务)
scripts/user/create_example_job.sh

# 查看作业状态
kubectl get kj -n cross-domain
```



## 作业状态

如果作业执行成功，则 `kubectl get kj -n cross-domain` 命令会显示类似下方的输出，Succeeded 表示成功状态：

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
