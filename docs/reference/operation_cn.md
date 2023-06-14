# 运维

## 宿主机

### 查看 Kuscia 相关容器

```shell
docker ps -a
```

如下，是 Kuscia 创建的容器列表：

- alice 节点容器
- bob 节点容器
- master 容器

[//]: # (开源容器镜像地址替换)


在中心化组网模式下会有单独的 master 容器：

```shell
CONTAINER ID   IMAGE                             COMMAND                  CREATED      STATUS      PORTS     NAMES
9c3ee316d37e   secretflow/kuscia-anolis:latest   "tini -- test/script…"   4 days ago   Up 4 days             root-kuscia-lite-alice
3f6ce0aefaa3   secretflow/kuscia-anolis:latest   "tini -- test/script…"   4 days ago   Up 4 days             root-kuscia-lite-bob
be9d3c84ac3f   secretflow/kuscia-anolis:latest   "tini -- test/script…"   4 days ago   Up 4 days             root-kuscia-master
```

而在分布式模式下，master 在节点容器中：

```shell
CONTAINER ID   IMAGE                             COMMAND                 CREATED      STATUS       PORTS     NAMES
ce8d3fb6c429   secretflow/kuscia-anolis:latest   "tini -- test/script…"  4 days ago   Up 4 days              root-kuscia-autonomy-bob
fa0d01f36bc5   secretflow/kuscia-anolis:latest   "tini -- test/script…"  4 days ago   Up 4 days              root-kuscia-autonomy-alice
```

### 进入容器内部

```shell
docker exec -it {container_name} bash
```

- {container_name} 是你要进入的容器的 NAME。

**示例**

```shell
docker exec -it ${USER}-kuscia-lite-alice bash
```

### 复制文件到容器中

```shell
docker cp {src_file} {container_name}:{dst_path_or_file_on_container}
```

- {src_file} 是源文件的路径。
- {container_name} 是目标容器的 NAME。
- {dst_path_or_file_on_container} 是目标容器的路径或目标文件名。

**示例**

```shell
docker cp a.txt ${USER}-kuscia-lite-alice:/home/kuscia/
```

## master 容器

### 前置操作

#### 进入 master 容器

```shell
docker exec -it ${USER}-kuscia-master bash
```

#### 查看 Kuscia 安装的 CRD

Kuscia 基于通过 [Kubernetes](https://kubernetes.io/zh-cn/) 提供的 CRD 拓展能力来实现隐私计算的调度能力。如果你对 Kubernetes 比较熟悉，你可以通过查看 CRD 来了解
Kuscia
提供的能力。

##### 查看 Kuscia 安装的 CRD 列表

通过下面的命令，你可以查看 Kuscia 安装的 CRD：

```shell
kubectl api-resources | grep kuscia.secretflow
```

或者

```shell
kubectl get crd | grep kuscia.secretflow
```

##### 查看 Kuscia 安装的某个 CRD 的详细信息

如果你想要了解某个 CRD 的详细信息，则可以通过下面的命令查看更详细的说明。
我们以 KusciaJob 为例：

```shell
kubectl explain kj 
```

或者

```shell
kubectl get crd kusciajobs.kuscia.secretflow -o yaml
```

##### 更多 Kubectl 使用

如果你想了解 kubectl 命令的使用，你可以查阅 [Kubernetes官方文档](https://kubernetes.io/zh-cn/docs/reference/kubectl/) 。

### Domain

Domain 在 Kuscia 中表示隐私计算节点。

#### 查看所有的 Domain

```shell
kubectl get domain
```

#### 查看某个 Domain 的详细信息

```shell
kubectl get domain {domian_name} -o yaml
```

- {domain_name} 为要查看 Domain 的名称。

**示例**

```shell
kubectl get domain alice -o yaml
```

你会看到类似信息：

```yaml
apiVersion: kuscia.secretflow/v1alpha1
kind: Domain
metadata:
  name: alice
spec:
  cert: { cert_data }
status:
  nodeStatuses:
    - lastHeartbeatTime: "2023-04-04T08:47:56Z"
      lastTransitionTime: "2023-03-30T09:44:45Z"
      name: 9c3ee316d37e
      status: Ready
      version: a50f56e-a50f56e
```

其中，我们需要关注`.status.nodeStatuses[].status`，这个字段记录了 Domain 下的节点是否正常。 有关 Domain
的更多信息，请查看 [Domain](./concepts/domain_cn.html) 。

### DomainRoute

DomainRoute 在 Kuscia 中表示一对隐私计算节点的单向授权，只有授权后才能进行联合隐私计算任务。

#### 查看 Domain 相关的 DomainRoute

```shell
kubectl get dr -n {domain_name}
```

- {domain_name} 为要查看的 Domain 的名称。

**示例**

```shell
kubectl get dr -n alice
```

你会看到类似如下信息：

```shell
NAME         SOURCE   DESTINATION   HOST         AUTHENTICATION
bob-alice    bob      alice         172.17.0.3   TOKEN
alice-bob    alice    bob           172.17.0.4   TOKEN
```

其中，alice-bob 表示 bob 授权给 alice 访问权限，alice 可以通过 172.17.0.4 访问 bob 相关的服务。
而 bob-alice， 则表示 alice 授权给 bob 访问权限，bob 可以通过 172.17.0.3 访问 alice 相关的服务。

#### 查看 DomainRoute 的详细信息

```shell
kubectl get dr {domain_router_name} -o yaml -n {domain_name}
```

- {domain_name} 为要查看的 DomainRoute 所在的 Domain 的名称。
- {domain_router_name} 为要查看的 DomainRoute 的名称。

**示例**

```shell
kubectl get dr alice-bob -o yaml -n alice
```

有关 DomainRoute 的更多信息，请查看 [DomainRoute](./concepts/domainroute_clusterdomainroute_cn.html) 。

### AppImage

AppImage 在 Kuscia 中用于注册应用镜像模版信息。在执行 KusciaJob 时，通过在 KusciaTask 中指定 AppImage 名称从而实现任务应用 Pod 镜像启动模版的绑定。

#### 查看所有的 AppImage

```shell
kubectl get aimg
```

#### 查看 AppImage 的详细信息

```shell
kubectl get aimg {aimg-name} -o yaml
```

- {aimg-name}为要查看的 AppImage 的名称。

**示例**

```shell
kubectl get aimg secretflow-image -o yaml
```

有关 AppImage 的更多信息，请查看 [AppImage](./concepts/appimage_cn.html) 。

### KusciaJob & KusciaTask

关于如何配置、运行和查看 KusciaJob 和 KusciaTask，请参考 [如何运行一个 secretflow 任务](../tutorial/run_secretflow_cn.html)
和 [KusciaJob](./concepts/kusciajob_cn.html) 。

## 节点容器

### 进入 Lite 节点容器

```shell
docker exec -it ${USER}-kuscia-lite-alice bash
```

### 隐私计算任务

隐私计算任务将会以容器的形式在 节点容器 内部运行。以 alice 节点容器为例，每当一个新的隐私计算任务分派到 alice 节点时，alice 节点容器都会启动一个容器来处理任务。
这是一个容器嵌套容器的形式：宿主机上通过docker启动了 alice 节点容器，而在 alice 节点容器中，通过 crictl 启动了任务容器，理解这一点非常重要。

#### 查看当前 Lite 节点上正在进行的任务

在 alice 节点容器内，通过

```shell
crictl ps -a
```

你可以查看到启动的任务容器和它的运行状态。

#### 进入 Lite 节点上的任务容器

如果你需要调试任务，那么你可能需要在某些场景下进入任务容器内部，这时候你可以执行下面的命令：

```shell
crictl exec -it {container_id} bash
```

- {container_id-name}为要进入任务容器的 ID。

### 隐私计算任务使用你自己的数据文件

我们可以通过`docker cp`将数据文件复制到节点容器，但是任务执行在由节点容器创建的任务容器内，这时候任务容器并不和节点容器共用一套文件系统，怎么在隐私计算任务中使用你自己的文件呢？
实际上，节点容器的`/home/kuscia/var/storage`会被挂载到任务容器的同名目录下，所以，你只需要将你自己的数据文件放到节点容器的这个目录下，就能在任务中使用了。

### 如何调试一个任务

通过将 AppImage 的 command 调整为`sleep`命令可以帮助你调试一个任务：

1. 修改 AppImage 或新建一个同样 Spec 的 AppImage， 修改 command 为`sh -c sleep 360000`， 保存原来的 command。
2. 使用这个 AppImage，创建 KusciaJob。
3. 进入任务容器，执行原来的 command 进行调试操作。

## 安全建议
### Envoy
Envoy 的 Admin 端口仅用于本地监听，请勿将 Admin 端口暴露在非安全的网络中，防止信息泄露。