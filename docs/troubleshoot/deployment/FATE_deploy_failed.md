# FATE 部署失败

## Pod 镜像拉取失败

例如：

```shell
kubectl get pods -A

NAMESPACE   NAME                               READY   STATUS         RESTARTS   AGE
bob         fate-deploy-bob-6b85647f8b-5nvtz   0/1     ErrImagePull   0          43s
```

### 检查容器内的镜像

FATE 部署时会把相关的镜像 import 到节点容器中。

进入节点，使用 `crictl images` 命令查看镜像。中心化组网的节点容器中完备的镜像如下：

```shell
docker exec -it ${USER}-kuscia-lite-bob bash

crictl images | grep fate

docker.io/secretflow/fate-adapter                                                    0.0.1               8b19bcdc69d4c       260MB
secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/fate-adapter              0.0.1               8b19bcdc69d4c       260MB
docker.io/secretflow/fate-deploy-basic                                               0.0.1               38ba174f12520       3.23GB
secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/fate-deploy-basic         0.0.1               38ba174f12520       3.23GB
```

P2P 组网的节点容器中完备的镜像如下：

```shell
docker exec -it ${USER}-kuscia-autonomy-bob bash

crictl images | grep fate

docker.io/secretflow/fate-adapter                                                    0.0.1               8b19bcdc69d4c       260MB
secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/fate-adapter              0.0.1               8b19bcdc69d4c       260MB
docker.io/secretflow/fate-deploy-basic                                               0.0.1               38ba174f12520       3.23GB
secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/fate-deploy-basic         0.0.1               38ba174f12520       3.23GB
```

如果缺少 `secretflow/fate-adapter:0.0.1` 和 `secretflow/fate-deploy-basic:0.0.1` 相关的任意镜像（注意镜像地址的前缀，一共有四个），请使用 `crictl rmi` 命令移除镜像。之后删除已部署好的 Kuscia 集群，按照[部署文档](https://www.secretflow.org.cn/docs/kuscia/latest/zh-Hans/tutorial/run_fate_cn)中的流程重新部署。

```shell
crictl rmi secretflow/fate-adapter:0.0.1
crictl rmi secretflow/fate-deploy-basic:0.0.1
```
