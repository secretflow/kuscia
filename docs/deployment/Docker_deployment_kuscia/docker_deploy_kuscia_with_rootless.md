# 非 root 用户部署节点

## 前言

本教程将帮助您在权限受限制的环境中，以非 root 用户来部署 Kuscia 集群。

## 部署

您可以参考[这里](./deploy_p2p_cn.md)了解如何使用 Docker 部署 Kuscia，本文不做过多赘述。

启动 Kuscia 时，非 root 用户只需在启动命令后面加上 `--rootless` 即可。示例如下：

```bash
./kuscia.sh start -c autonomy_alice.yaml -p 11080 -k 11081 --rootless
```

:::{tip}

1. 目前仅支持 RunP 模式以非 root 用户来部署 Kuscia。
2. 如果主机用户是 root，Kuscia 还是以 root 用户启动，`--rootless` 不生效。如果主机用户不是 root，Kuscia 以主机用户启动。
3. 更多部署要求请参考[这里](../deploy_check.md)。
:::

4.
