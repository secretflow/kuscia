# 作业运行失败

## 排查步骤

### 检查机器配置

若机器不满足[官网推荐配置](https://www.secretflow.org.cn/docs/kuscia/latest/zh-Hans/getting_started/quickstart_cn#id2)，可能会造成部分服务无法正常工作，从而导致作业运行失败。

### 检查网络授权
大部分任务失败问题都是网络问题，检查网络授权可以登录到容器（中心化模式在 master 容器、点对点模式在 automony 容器）并执行`kubectl get cdr`命令查看授权信息，READY 为 True 时表明通信正常，为空时表明通信异常，可以先看下 HOST 和端口是否正确或者执行 `kubectl get cdr ${cdr_name} -oyaml` 命令看下详细信息，参数确认无误仍无法正常通信请参考[授权错误排查](./networkauthorizationcheck.md)。

### 查看作业失败原因

登陆查看作业的容器

- 若以中心化组网模式部署，请登陆到 Master 容器中

```shell
docker exec -it ${USER}-kuscia-master bash
```

- 若以点对点组网模式部署，请登陆到下发作业的容器中，下面以 Alice 容器为例

```shell
docker exec -it ${USER}-kuscia-autonomy-alice bash
```

查看作业信息

```shell
kubectl get kj
```

查看作业下任务的详细信息

- 具体失败原因可以查看 status 字段下的相关内容

```shell
# 列出所有任务
kubectl get kt

# 查看任务的详细信息，任务名称来自上述命令
kubectl get kt {任务名称} -o yaml
```

查看任务 Pod 的详细信息

- 具体失败原因可以查看 status 字段下的相关内容

```shell
# 查看 alice 节点下的 pod 列表
kubectl get pod -n alice

# 查看 alice 节点下某个 pod 的详细信息
kubectl get pod xxxx -o yaml -n alice

# 查看 bob 节点下的 pod 列表
kubectl get pod -n bob

# 查看 bob 节点下某个 pod 的详细信息
kubectl get pod xxxx -o yaml -n bob

# 若 pod 状态为 Pending，可以继续查看相应节点详细信息
kubectl get node

# 查看 node 详细信息
kubectl describe node xxxx
```


查看任务 Pod 详细日志

- 若以中心化组网模式部署，登陆到 Alice 和 Bob 容器命令如下

```shell
# 登陆到 alice 容器中
docker exec -it ${USER}-kuscia-lite-alice bash

# 查看 alice 节点上任务 pod 日志
cat /home/kuscia/var/stdout/pods/podName_xxxx/xxxx/x.log

# 登陆到 bob 容器中
docker exec -it ${USER}-kuscia-lite-bob bash

# 查看 bob 节点上任务 pod 日志
cat /home/kuscia/var/stdout/pods/podName_xxxx/xxxx/x.log
```

- 若以点对点组网模式部署，登陆到 Alice 和 Bob 容器命令如下

```shell
# 登陆到 alice 容器中
docker exec -it ${USER}-kuscia-autonomy-alice bash

# 查看 alice 节点上任务 pod 日志
cat /home/kuscia/var/stdout/pods/podName_xxxx/xxxx/x.log

# 登陆到 bob 容器中
docker exec -it ${USER}-kuscia-autonomy-bob bash

# 查看 bob 节点上任务 pod 日志
cat /home/kuscia/var/stdout/pods/podName_xxxx/xxxx/x.log
```
任务运行遇到网络错误时，可以参考[这里](../reference/troubleshoot/networktroubleshoot.md)排查