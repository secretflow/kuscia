# 使用 RunP 模式部署节点

## 前言
本教程帮助你使用进程运行时（RunP）来部署节点（Lite 和 Autonomy），当前本方案仅支持 SecretFlow 引擎，对更多算法引擎的支持敬请期待。

关于 RunP 以及更多运行时的介绍请参考 [容器运行模式](../reference/architecture_cn.md#agent)。

## 部署流程

### 概览
使用 RunP 部署节点，和默认的部署流程主要区别在于：
1. 需要使用 `kuscia-secretflow` 镜像。
2. 需要将 `kuscia.yaml` [配置文件](./kuscia_config_cn.md) 中 `runtime` 字段修改为 `runp`。
    ```yaml
    runtime: runp
    ```
下文将以物理机和 K8s 两种部署环境为例，来介绍基于 RunP 的部署流程。

### 在物理机上部署
完整的详细流程请参考 [多机部署中心化集群](./deploy_master_lite_cn.md) 和 [多机部署点对点集群](./deploy_p2p_cn.md)。

其中，使用 RunP 部署的不同点是：
1. 使用 `kuscia-secretflow` 镜像。
   ```bash
   export KUSCIA_IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia-secretflow
   ```

2. 下载 Kuscia [配置示例](https://github.com/secretflow/kuscia/blob/main/scripts/templates/kuscia-autonomy.yaml)（以 autonomy 为例），
   将 `runtime` 字段修改为 `runp`，以及填充模板变量，例如 `{{.DOMAIN_ID}}`、`{{.DOMAIN_KEY_DATA}}` 等。 并在启动节点时指定配置文件路径。

<span style="color:red;">注意：<br>
1、需要对合作方暴露的 Kuscia 端口，可参考 [Kuscia 端口介绍](../kuscia_ports_cn.md) </span>

   ```bash
   # DOMAIN_KEY_DATA 请用以下命令生成
   docker run -it --rm ${KUSCIA_IMAGE} scripts/deploy/generate_rsa_key.sh

   # -c 参数传递的是指定的 Kuscia 配置文件路径。
   ./deploy.sh autonomy -n alice -i 1.1.1.1 -p 11001 -k 11002 -g 11003 -c kuscia-autonomy.yaml
   ```

### 在 K8s 集群上部署
完整的详细流程请参考 [K8s部署中心化集群](./K8s_deployment_kuscia/K8s_master_lite_cn.md) 和 [K8s部署点对点集群](./K8s_deployment_kuscia/K8s_p2p_cn.md)。

其中，使用 RunP 部署的不同点是：
1. 修改 [configmap.yaml](https://github.com/secretflow/kuscia/blob/main/hack/k8s/autonomy/configmap.yaml) （以 autonomy 为例）文件中 Kusica 的 `runtime` 配置为 `runp`，
   以及填充模板变量，例如 `{{.DOMAIN_ID}}`、`{{.DOMAIN_KEY_DATA}}` 等。 `runk` 相关的配置不需要关注，包括不需要配置 `kubeconfigFile` 以及不需要创建 `rbac.yaml`。
   ```yaml
   runtime: runp

   # runk 配置不需要关注
   runk:
      namespace:
      dnsServers:
      kubeconfigFile:
   ```

2. 创建 [deployment](https://github.com/secretflow/kuscia/blob/main/hack/k8s/autonomy/deployment.yaml) 时修改镜像为 `kuscia-secretflow` 镜像。此外，`automountServiceAccountToken` 字段可以设置为 `false`。
   ```yaml
   spec:
     template:
       spec:
         containers:
           - image: secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia-secretflow:latest
         # 可以设置为 false，降低对 K8s 集群权限的依赖（可选）
         automountServiceAccountToken: false
   ```