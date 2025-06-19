# Kuscia 引擎指标监控

在生产环境中，Kuscia 中运行的引擎（如 SecretFlow-Serving）可能需要统计引擎相关的指标，比如引擎成功调用次数，引擎错误率，引擎运行延时等。

本文描述如何配置 Kuscia monitor 监控引擎层透出的指标，包括引擎配置和集群监控配置两个部分。

## 1 引擎配置

假设你的引擎为一个 Node Exporter 服务，并注册了 /metrics 接口，你希望将该接口用于 Prometheus 监控数据，可以通过以下例子来编写 appimage

~~~
apiVersion: kuscia.secretflow/v1alpha1
kind: AppImage
metadata:
  name: node-exporter
spec:
  configTemplates:
    task-config.conf: |
      {{{.ALLOCATED_PORTS.ports[name=metric].port}}}
  deployTemplates:
  - name: secretflow
    replicas: 1
    spec:
      containers:
      - configVolumeMounts:
        - mountPath: /work/kuscia/task-config.conf
          subPath: task-config.conf
        name: secretflow
        command:
        - sh
        args:
        - -c
        - node_exporter --web.listen-address=:$(cat /work/kuscia/task-config.conf)
        ports:
        - name: metric
          protocol: HTTP
          scope: Domain
        workingDir: /work
        metricProbe:
          path: /metrics
          port: metric
      restartPolicy: Never
  image:
    name: docker.io/prom/node-exporter
    tag: latest
~~~

其中值得注意的是：

~~~
ports:
- name: metric
  protocol: HTTP
  scope: Domain
metricProbe:
  path: /metrics
  port: metric
~~~

ports[0] 为引擎定义了一个名为 metric 的 HTTP 端口，该端口的端口号会在引擎启动时分配，在本示例中会将端口号渲染至 `\{{.ALLOCATED_PORTS.ports[name=metric].port}}` 的变量里，具体渲染规则详见[如何在 Kuscia 中给自定义应用渲染配置文件](../tutorial/config_render.md)

metricProbe 表示该引擎和外部交互的指标统计接口，metricProbe.path 定义了接口路径（此处为 /metrics），metricProbe.port 定义了接口名称（此处为 metric，和 port[0] 的端口名称相互对应）。

## 2 集群配置

### 前置准备

在部署 Kuscia monitor 前，您需要参考 [Docker 多机部署 Kuscia](./Docker_deployment_kuscia/index.rst) 和 [K8s 集群部署 Kuscia](./K8s_deployment_kuscia/index.rst) 文档部署 Kuscia 节点，并确保 Kuscia 节点正常运行。

### 部署

引擎会在 Kuscia 集群内运行，需要在 Kuscia 集群内部署 Kuscia monitor。

假设你的 Kuscia 实例运行在 alice（bob 同理）domain 下, 可以分为 Center 和 P2P 两种情况进行部署：

#### P2P 模式部署

1. alice 节点导入 monitor 镜像

    ~~~bash
    # Docker mode, K8s deployment does not require importing the image
    export KUSCIA_MONITOR_IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia-monitor:latest
    docker cp <alice-container-id>:/home/kuscia/scripts/deploy/register_app_image.sh . && chmod u+x register_app_image.sh
    bash register_app_image.sh -c <alice-container-id> -i "${KUSCIA_MONITOR_IMAGE}" --import
    ~~~

2. 登录到安装 alice 的容器里部署 monitor

    ~~~bash
    # Docker mode
    docker exec -it <alice-container-id> bash -c "scripts/deploy/kuscia.sh monitor"

    # k8s mode
    kubectl exec -it <alice-pod-name> -n <alice-pod-namespace> -- bash -c "scripts/deploy/kuscia.sh monitor"
    ~~~

#### Center 模式部署

1. alice 节点导入 monitor 镜像

    ~~~bash
    # Docker mode, K8s deployment does not require importing the image
    export KUSCIA_MONITOR_IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia-monitor:latest
    docker cp <alice-container-id>:/home/kuscia/scripts/deploy/register_app_image.sh . && chmod u+x register_app_image.sh
    bash register_app_image.sh -c <alice-container-id> -i "${KUSCIA_MONITOR_IMAGE}" --import
    ~~~

2. 登录到安装 master 的容器里部署 monitor

    ~~~bash
    # Unlike P2P mode, Center mode requires specifying the domain of the lite node inside the master container
    # Docker mode
    docker exec -it <master-container-id> bash -c "scripts/deploy/kuscia.sh monitor alice"

    # k8s mode
    # lite node is using runc/runp runtime
    kubectl exec -it <master-pod-name> -n <master-pod-namespace> -- bash -c "scripts/deploy/kuscia.sh monitor alice"

    # When the lite node is using the runk runtime, you need to add --runk and specify the namespace of the lite node pod
    kubectl exec -it <master-pod-name> -n <master-pod-namespace> -- bash -c "export LITE_NAMESPACE=<lite-pod-namespace>;scripts/deploy/kuscia.sh monitor alice --runk"
    ~~~
