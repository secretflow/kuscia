# AppImage

在 Kuscia 中，你可以使用 AppImage 存储注册应用镜像模版信息。后续在运行任务时，通过在任务中指定 AppImage 名称，从而实现任务应用 Pod 镜像启动模版的绑定。具体用例请参考下文。

## 用例

以下是一些 AppImage 的典型用例：

- 创建 AppImage，你将体验如何使用 AppImage 存储应用镜像的模版信息。
- 更新 AppImage，你将熟悉如何更新现有的 AppImage，从而实现应用镜像模版信息的变更。
- 清理 AppImage，你将熟悉如何清理不需要的 AppImage。
- 参考 AppImage 对象定义，你将获取详细的 AppImage 描述信息。

## 创建 AppImage

下面以 `secretflow-image.yaml` 的内容为例，介绍创建 AppImage。

```yaml
apiVersion: kuscia.secretflow/v1alpha1
kind: AppImage
metadata:
  name: secretflow-image
spec:
  configTemplates:
    task-config.conf: |
      {
        "task_id": "{{.TASK_ID}}",
        "task_input_config": "{{.TASK_INPUT_CONFIG}}",
        "task_cluster_def": "{{.TASK_CLUSTER_DEFINE}}",
        "allocated_ports": "{{.ALLOCATED_PORTS}}"
      }
  deployTemplates:
    - name: secretflow
      replicas: 1
      spec:
        containers:
          - command:
              - sh
              - -c
              - "python -m secretflow.kuscia.entry ./kuscia/task-config.conf"
            configVolumeMounts:
              - mountPath: ./kuscia/task-config.conf
                subPath: task-config.conf
            name: secretflow
            ports:
              - name: spu
                protocol: GRPC
                scope: Cluster
              - name: fed
                protocol: GRPC
                scope: Cluster
              - name: global
                protocol: GRPC
                scope: Domain
            workingDir: /work
        restartPolicy: Never
  image:
    id: d4ffa123aa45mnkj
    name: secretflow/secretflow-lite-anolis8
    sign: abc13mnjh1olkkp1
    tag: "latest"
```

在该示例中:

- `.metadata.name`：表示 AppImage 名称，当前示例为`secretflow-image`。
- `.spec.configTemplates`：表示启动应用依赖的配置模版信息。
- `.spec.deployTemplates`：表示应用部署模版配置信息，在该模版中，可以配置应用的启动命令，监听的端口等信息。该字段下主要包含以下子字段：
  - `.spec.deployTemplates[0].name`：表示应用部署模版名称，当前示例为`secretflow`。
  - `.spec.deployTemplates[0].replicas`：表示应用副本数，当前示例为`1`。相应地，在创建任务应用 Pod 资源时，会将应用 Pod 副本数设置为`1`。
  - `.spec.deployTemplates[0].spec.containers[0]`：表示应用容器信息。该字段主要复用 K8s Pod containers 部分字段以及扩展了一些自定义字段。该字段下主要包含以下子字段：
    - `.spec.deployTemplates[0].spec.containers[0].command`：表示应用容器启动命令。
    - `.spec.deployTemplates[0].spec.containers[0].configVolumeMounts`：表示应用容器启动挂载卷信息。
    - `.spec.deployTemplates[0].spec.containers[0].name`：表示应用容器名称。
    - `.spec.deployTemplates[0].spec.containers[0].ports`：表示应用容器的启动端口。该字段下主要包含以下子字段：
      - `.spec.deployTemplates[0].spec.containers[0].ports[].name`：表示应用容器的端口名称。
      - `.spec.deployTemplates[0].spec.containers[0].ports[].protocol`：表示应用容器的端口使用的协议类型。
      - `.spec.deployTemplates[0].spec.containers[0].ports[].scope`：表示应用端口使用范围。支持三种模式：`Cluster`、`Domain`、`Local`。
    - `.spec.deployTemplates[0].spec.containers[0].workingDir`：表示应用容器的工作目录。
  - `.spec.deployTemplates[0].spec.restartPolicy`：表示应用的重启策略。对应于应用 Pod 的重启策略。
- `.spec.image`：表示应用镜像的信息。
  - `.spec.image.id`：表示应用镜像的 ID 信息。
  - `.spec.image.name`：表示应用镜像的名称信息。
  - `.spec.image.sign`：表示应用镜像的签名信息。Kuscia 会对应用镜像做签名校验，以保证镜像的合法性。
  - `.spec.image.tag`：表示应用镜像的 Tag 信息。

1. 运行以下命令创建 AppImage。

```shell
kubectl apply -f secretflow-image.yaml
```

2. 检查 AppImage 是否创建成功。

```shell
kubectl get appimage secretflow-image
```

3. 查询创建的 AppImage 详细信息。

```shell
kubectl get appimage secretflow-image -o yaml
```

## 更新 AppImage

下面以 `secretflow-image.yaml` 的内容为例，介绍更新 AppImage。

```yaml
apiVersion: kuscia.secretflow/v1alpha1
kind: AppImage
metadata:
  name: secretflow-image
spec:
  configTemplates:
    task-config.conf: |
      {
        "task_id": "{{.TASK_ID}}",
        "task_input_config": "{{.TASK_INPUT_CONFIG}}",
        "task_input_cluster_def": "{{.TASK_CLUSTER_DEFINE}}",
        "allocated_ports": "{{.ALLOCATED_PORTS}}"
      }
  deployTemplates:
    - name: secretflow
      replicas: 1
      spec:
        containers:
          - command:
              - sh
              - -c
              - "python -m secretflow.kuscia.entry ./kuscia/task-config.conf"
            configVolumeMounts:
              - mountPath: ./kuscia/task-config.conf
                subPath: task-config.conf
            name: secretflow
            ports:
              - name: spu
                protocol: GRPC
                scope: Cluster
              - name: fed
                protocol: GRPC
                scope: Cluster
              - name: global
                protocol: GRPC
                scope: Domain
            workingDir: /work
        restartPolicy: Never
  image:
    id: d4ffa123aa45mnkj
    name: secretflow/secretflow-lite-anolis8
    sign: abc13mnjh1olkkp1
    tag: "latest"
```

在该示例中，将 ports 下 name 为`global`的 port 端口号改为`8082`。

1. 运行以下命令更新 AppImage。

```shell
kubectl apply -f secretflow-image.yaml
```

2. 检查 AppImage 是否更新成功。

```shell
kubectl get appimage secretflow-image -o  yaml | grep '\- name: global' -A 3
        - name: global
          protocol: GRPC
          scope: Domain
```

## 清理 AppImage

下面以 AppImage `secretflow-image` 为例，介绍清理 AppImage。

1. 运行以下命令清理 AppImage。

```shell
kubectl delete appimage secretflow-image
```

2. 检查 AppImage 是否已被清理成功。

```shell
kubectl get appimage secretflow-image
Error from server (NotFound): appimages.kuscia.secretflow "secretflow-image" not found
```

{#appimage-ref}

## 参考

下面以 `app-template` 模版为例，介绍 AppImage 所包含的完整字段。

```yaml
apiVersion: kuscia.secretflow/v1alpha1
kind: AppImage
metadata:
  name: app-template
spec:
  configTemplates:
    task-config.conf: |
      {
        "task_id": "{{.TASK_ID}}",
        "task_input_config": "{{.TASK_INPUT_CONFIG}}",
        "task_input_cluster_def": "{{.TASK_CLUSTER_DEFINE}}",
        "allocated_ports": "{{.ALLOCATED_PORTS}}"
      }
  deployTemplates:
    - name: app
      role: server
      replicas: 1
      networkPolicy:
        ingresses:
          - from:
              roles:
                - client
            ports:
              - port: global
      spec:
        containers:
          - command:
              - sh
            args:
              - -c
              - ./app --role=server --task_config_path=./kuscia/task-config.conf
            configVolumeMounts:
              - mountPath: ./kuscia/task-config.conf
                subPath: task-config.conf
            name: app
            ports:
              - name: global
                protocol: TCP
                scope: Cluster
            envFrom:
              - configMapRef:
                  name: config-template
            env:
              - name: APP_NAME
                value: app
            resources:
              limits:
                cpu: 100m
                memory: 100Mi
              requests:
                cpu: 100m
                memory: 100Mi
            readinessProbe:
              exec:
                command:
                  - cat
                  - /tmp/healthy
              initialDelaySeconds: 5
              periodSeconds: 5
            livenessProbe:
              httpGet:
                path: /healthz
                port: global
              failureThreshold: 1
              periodSeconds: 20
            startupProbe:
              httpGet:
                path: /healthz
                port: global
              failureThreshold: 30
              periodSeconds: 10
            imagePullPolicy: IfNotPresent
            workingDir: /work
        restartPolicy: Never
    - name: app
      role: client
      replicas: 1
      spec:
        containers:
          - command:
              - sh
            args:
              - -c
              - ./app --role=client --task_config_path=/etc/kuscia/task-config.conf
            configVolumeMounts:
              - mountPath: /etc/kuscia/task-config.conf
                subPath: task-config.conf
            name: app
            ports:
              - name: global
                protocol: TCP
                scope: Cluster
            envFrom:
              - configMapRef:
                  name: config-template
            env:
              - name: APP_NAME
                value: app
            resources:
              limits:
                cpu: 100m
                memory: 100Mi
              requests:
                cpu: 100m
                memory: 100Mi
            readinessProbe:
              exec:
                command:
                  - cat
                  - /tmp/healthy
              initialDelaySeconds: 5
              periodSeconds: 5
            livenessProbe:
              httpGet:
                path: /healthz
                port: global
              failureThreshold: 1
              periodSeconds: 20
            startupProbe:
              httpGet:
                path: /healthz
                port: global
              failureThreshold: 30
              periodSeconds: 10
            imagePullPolicy: IfNotPresent
            workingDir: /work
        restartPolicy: Never
  image:
    id: adlipoidu8yuahd6
    name: app-image
    sign: nkdy7pad09iuadjd
    tag: v1.0.0
```

AppImage `metadata` 的子字段详细介绍如下：

- `name`：表示 AppImage 的名称，当前示例为`app-template`。

AppImage `spec` 的子字段详细介绍如下：

- `configTemplates`：表示应用启动依赖的配置模版信息。在该字段下，应用可以自定义启动依赖的配置文件，当前示例包含的配置文件为`task-config.conf`。 更多的信息请 [参考这里](../../tutorial/config_render.md)
- `deployTemplates`：表示应用部署模版配置信息。Kuscia 控制器会根据该名称和角色，选择合适的 AppImage。若在 AppImage 没有查询到符合的部署模版，则将使用第一个部署模版。
  - `deployTemplates[].name`：表示应用部署模版名称。
  - `deployTemplates[].role`：表示应用作为该角色时，使用的部署模版配置。这里的角色可以由应用自定义，示例：Host/Guest；如果应用不需要区分角色部署，这里可以填空。
  - `deployTemplates[].replicas`：表示应用运行的副本数，默认为`1`。
  - `deployTemplates[].networkPolicy`：表示应用的网络访问策略。该示例表示：当前角色为`server`的应用允许角色为`client`的应用访问名称为`global`的端口。
    - `deployTemplates[].networkPolicy.ingresses[].from.roles`：表示允许访问当前应用的角色。该示例表示：当前角色为`server`的应用允许角色为`client`的应用访问。
    - `deployTemplates[].networkPolicy.ingresses[].ports[].port`：表示允许访问当前应用的端口。该示例表示：当前角色为`server`的应用允许所允许被访问的端口名称为`global`。
    - `deployTemplates[].spec.containers`：表示应用容器配置信息。该字段复用了 [K8s Pod containers](https://kubernetes.io/zh-cn/docs/reference/kubernetes-api/workload-resources/pod-v1/#Container) 部分字段以及扩展了一些自定义字段。主要包含以下子字段：
      - `deployTemplates[].spec.containers[].command`：表示应用的启动命令。
      - `deployTemplates[].spec.containers[].args`：表示应用的启动参数。
      - `deployTemplates[].spec.containers[].configVolumeMounts`：表示应用启动的挂载配置。当前仅支持挂载一个配置文件。当前示例中，将`.spec.configTemplates`中定义的配置文件`task-config.conf`挂载到容器中的位置为`/etc/kuscia/task-config.conf`。
        - `deployTemplates[].spec.containers[].configVolumeMounts[].mountPath`：表示文件挂载到容器中的路径。
        - `deployTemplates[].spec.containers[].configVolumeMounts[].subPath`：表示文件挂载到容器中的子路径。
      - `deployTemplates[].spec.containers[].name`：表示应用容器的名称。
      - `deployTemplates[].spec.containers[].ports`：表示应用容器的启动端口。
        - `.spec.deployTemplates[].spec.containers[].ports[].name`：表示应用容器的端口名称。
        - `.spec.deployTemplates[].spec.containers[].ports[].protocol`：表示应用容器的端口使用的协议类型。支持两种类型：`HTTP`、`GRPC`。
        - `.spec.deployTemplates[].spec.containers[].ports[].scope`：表示应用端口使用范围。支持三种模式：`Cluster`、`Domain`、`Local`。Kuscia 会根据 scope 取值的不同，限制 port 的网络访问策略。具体含义如下所示：
          - `Cluster`：表示该 port 用于节点外部和节点内部访问，Kuscia 会给该 port 创建相对应的 K3s service 资源。
          - `Domain`：表示该 port 用于节点内部访问，Kuscia 会给该 port 创建相对应的 K3s service 资源。
          - `Local`：表示该 port 用于 Pod 内部容器本地访问，Kuscia 不会给该 port 创建相对应的 K3s service 资源。
      - `deployTemplates[].spec.containers[].envFrom`：表示使用`envFrom`为应用容器设置环境变量。
      - `deployTemplates[].spec.containers[].env`：表示使用`env`为应用容器设置环境变量。
      - `deployTemplates[].spec.containers[].resources`：表示应用容器申请的资源配置。
      - `deployTemplates[].spec.containers[].readinessProbe`：表示应用容器的就绪探针配置。
      - `deployTemplates[].spec.containers[].livenessProbe`：表示应用容器的存活探针配置。
      - `deployTemplates[].spec.containers[].startupProbe`：表示应用容器的启动探针配置。
      - `deployTemplates[].spec.containers[].imagePullPolicy`：表示应用容器的镜像拉取策略。
      - `deployTemplates[].spec.containers[].workingDir`：表示应用容器的工作目录。
      - `deployTemplates[].spec.restartPolicy`：表示应用的重启策略。对应于应用 Pod 的重启策略。
- `image`：表示应用镜像的信息。该字段包含以下子字段。
  - `image.id`：表示应用镜像的 ID 信息。
  - `image.name`：表示应用镜像的名称信息。
  - `image.sign`：表示应用镜像的签名信息。Kuscia 会对应用镜像做签名校验，以保证镜像的合法性。
  - `image.tag`：表示应用镜像的 Tag 信息。
