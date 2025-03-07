# DataProxy 部署

## 前言

本教程帮助您在 Kuscia 节点上部署 DataProxy。

## 步骤

您可以参考[这里](./deploy_p2p_cn.md)了解如何使用 Docker 部署 Kuscia，本文不做过多赘述。

部署 Kuscia 时，在启动命令后面加上 `--data-proxy` 即可。示例如下：

- 点对点模式

使用 `--data-proxy` 参数在 autonomy 节点上导入 DataProxy 的镜像和注册 DataProxy 的 AppImage

```bash
./kuscia.sh start -c autonomy_alice.yaml -p 11080 -k 11081 --data-proxy
```

- 中心化模式

使用 `--data-proxy` 参数在 master 节点上注册 DataProxy 的 AppImage

```bash
./kuscia.sh start -c kuscia_master.yaml -p 18080 -k 18081 --data-proxy
```

使用 `--data-proxy` 参数在 lite 节点上导入 DataProxy 的镜像

```bash
./kuscia.sh start -c lite_alice.yaml -p 28080 -k 28081 --data-proxy
```

## 验证

- 点对点模式

在成功启动 Kuscia 后，执行如下命令看到 pod 为 running 代表 DataProxy 部署成功。

```bash
docker exec -it ${USER}-kuscia-autonomy-alice kubectl get po -A

NAMESPACE   NAME                              READY   STATUS    RESTARTS   AGE
alice       dataproxy-alice-699dc7455-sxvpj   1/1     Running   0          26s
```

- 中心化模式

各节点成功启动 Kuscia ，其中 master 节点上成功注册 DataProxy 的 AppImage ，并在 lite 节点上成功导入 DataProxy 的镜像后，在 master 节点执行如下命令看到 pod 为 Running 代表 DataProxy 部署成功。

```bash
docker exec -it ${USER}-kuscia-master kubectl get po -A

NAMESPACE   NAME                              READY   STATUS    RESTARTS   AGE
alice       dataproxy-alice-699dc7455-sxvpj   1/1     Running   0          26s
```