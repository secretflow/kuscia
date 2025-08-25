# 使用 RunP 模式部署节点

## 前言

本教程帮助你使用进程运行时（RunP）来部署节点（Lite 和 Autonomy），当前本方案仅支持 SecretFlow 引擎，对更多算法引擎的支持敬请期待。

关于 RunP 以及更多运行时的介绍请参考 [容器运行模式](../reference/architecture_cn.md#agent)。

**⚠️ 资源限制说明**：Kuscia RunP 模式暂不支持 job 资源限制。

## 部署流程

物理机和 K8s 两种部署环境的配置不同，接下来我们将分别介绍在这两种环境中 RunP 模式的部署流程。

### 在物理机上部署

完整的详细流程请参考 [多机部署中心化集群](./Docker_deployment_kuscia/deploy_master_lite_cn.md) 和 [多机部署点对点集群](./Docker_deployment_kuscia/deploy_p2p_cn.md)。

其中，使用 RunP 部署的不同点是：

需要在生成配置文件时，使用 --runtime 参数指定运行时为 `runp`。

```bash
# Generate the configuration file and specify the runtime as runp
docker run -it --rm ${KUSCIA_IMAGE} kuscia init --mode autonomy --domain "alice" --runtime "runp" > autonomy_alice.yaml 2>&1 || cat autonomy_alice.yaml
```

### 在 K8s 集群上部署

完整的详细流程请参考 [K8s 部署中心化集群](./K8s_deployment_kuscia/K8s_master_lite_cn.md) 和 [K8s 部署点对点集群](./K8s_deployment_kuscia/K8s_p2p_cn.md)。

其中，使用 RunP 部署的不同点是：

1. 修改 [configmap.yaml](https://github.com/secretflow/kuscia/blob/main/hack/k8s/autonomy/configmap.yaml) （以 autonomy 为例）文件中 Kuscia 的 `runtime` 配置为 `runp`，
   以及填充模板变量，例如 `{{.DOMAIN_ID}}`、`{{.DOMAIN_KEY_DATA}}` 等。 `runk` 相关的配置不需要关注，包括不需要配置 `kubeconfigFile` 以及不需要创建 `rbac.yaml`。

   ```yaml
   runtime: runp

   # runk configuration does not need to be concerned
   runk:
      namespace:
      dnsServers:
      kubeconfigFile:
   ```

2. 创建 [deployment](https://github.com/secretflow/kuscia/blob/main/hack/k8s/autonomy/deployment.yaml) 时修改镜像为 `kuscia-secretflow` 镜像，下面是一个示例镜像地址，您可以根据实际需求参考[这里](../development/build_kuscia_cn.md#kuscia-secretflow-image)自行编译。此外，`automountServiceAccountToken` 字段可以设置为 `false`。

   ```yaml
      spec:
        template:
          spec:
            containers:
              - image: secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia-secretflow:latest
            # Can be set to false to reduce dependency on K8s cluster permissions (optional)
            automountServiceAccountToken: false
   ```
