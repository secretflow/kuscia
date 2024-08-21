# 非 root 用户部署 Kuscia 节点

## 前言

本教程将指导您在权限受限制的环境中，以非 root 用户来部署 Kuscia 集群。

## 部署

在 K8s 模式中，非 root 用户部署需要对 root 用户部署的 deployment 和 configmap 部署模板进行一些修改。

我们已经为您准备好了修改后的模板文件，您只需参考[k8s 部署文档](./K8s_p2p_cn.md)进行部署即可，本文不做过多赘述。
- autonomy：[deployment.yaml](https://github.com/secretflow/kuscia/blob/main/hack/k8s/autonomy/rootless/deployment.yaml) 和 [configmap.yaml](https://github.com/secretflow/kuscia/blob/main/hack/k8s/autonomy/rootless/configmap.yaml)
- master：[deployment.yaml](https://github.com/secretflow/kuscia/blob/main/hack/k8s/master/rootless/deployment.yaml) 和 [configmap.yaml](https://github.com/secretflow/kuscia/blob/main/hack/k8s/master/rootless/configmap.yaml)
- lite：[deployment.yaml](https://github.com/secretflow/kuscia/blob/main/hack/k8s/lite/rootless/deployment.yaml) 和 [configmap.yaml](https://github.com/secretflow/kuscia/blob/main/hack/k8s/lite/rootless/configmap.yaml)

:::{tip}
1. 目前支持 [RunP](deploy_with_runp_cn.md) 或者 RunK 模式以非 root 用户来部署 Kuscia。
2. 更多部署要求请参考[这里](../deploy_check.md)。
:::

## 变更内容

下面展示非 root 用户部署时，deployment 和 configmap 模版中需要修改的地方。

- deployment 模版

1. 在 spec 字段增加如下配置：

```yaml
    spec:
      dnsPolicy: None # Explicitly set dnsPolicy to None to fully manage DNS settings
      dnsConfig:
        nameservers:
          - 127.0.0.1
```

2. configmap 文件挂载到容器内：

```yaml
          volumeMounts:
            - mountPath: /home/kuscia/var/tmp/resolv.conf
              name: kuscia-config
              subPath: resolv.conf
```

3. 在 containers 字段增加如下配置：
```yaml
          securityContext:
            runAsNonRoot: true
            runAsUser: 1000
            runAsGroup: 1000
            capabilities:
              add:
                - NET_BIND_SERVICE
```

- configmap 模版

在 configmap 模版中，只需要增加 resolv.conf 文件内容即可：

```yaml
data:
  resolv.conf: |-
    # k8s 集群中的 dns 配置
    nameserver 172.21.0.10
    search default.svc.cluster.local svc.cluster.local cluster.local
    options ndots:5
```