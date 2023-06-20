# KusciaTask

在 Kuscia 中，任务是用 KusciaTask 描述的。如果要运行一个任务，那么需要创建一个 KusciaTask。KusciaTask Controller 将会根据 KusciaTask 的描述信息，在参与方节点下创建任务相关的资源。
具体用例请参考下文。

## 用例

以下是一些 KusciaTask 的典型用例：

* 创建 KusciaTask，你将体验如何使用 KusciaTask 创建一个任务。实际场景中，推荐使用 KusciaJob 管理任务流程。
* 查看 KusciaTask，你将熟悉如何查看已创建的 KusciaTask 的运行状态。
* 清理 KusciaTask，你将熟悉如何清理运行结束或运行失败的 KusciaTask。
* 参考 KusciaTask 对象定义，你将获取详细的 KusciaTask 描述信息。

## 创建 KusciaTask

下面以 `secretflow-task-psi.yaml` 的内容为例，介绍创建 KusciaTask。

```yaml
apiVersion: kuscia.secretflow/v1alpha1
kind: KusciaTask
metadata:
  name: secretflow-task-psi
spec:
  initiator: bob
  parties:
  - appImageRef: secretflow-image
    domainID: alice
  - appImageRef: secretflow-image
    domainID: bob
  taskInputConfig: '{"domain":"psi","name":"2pc_balanced","version":"0.0.1","params":[{"prefixes":[],"name":"protocol","atomic":{"s":"ECDH_PSI_2PC"}},{"prefixes":[],"name":"receiver","atomic":{"s":"alice"}},{"prefixes":[],"name":"precheck_input","atomic":{"b":true}},{"prefixes":[],"name":"sort","atomic":{"b":true}},{"prefixes":[],"name":"broadcast_result","atomic":{"b":true}},{"prefixes":[],"name":"bucket_size","atomic":{"i64":1048576}},{"prefixes":[],"name":"curve_type","atomic":{"s":"CURVE_25519"}}],"inputs":[{"name":"receiver_input","type":"TABLE","table_params":{"col_params":[{"name":"key","cols":["id1"]}]},"table_metadata":{"type":"INDIVIDUAL_TABLE","indivial":{"path":"/home/kuscia/var/storage/data/alice.csv","party":"alice"}}},{"name":"sender_input","type":"TABLE","table_params":{"col_params":[{"name":"key","cols":["id2"]}]},"table_metadata":{"type":"INDIVIDUAL_TABLE","indivial":{"path":"/home/kuscia/var/storage/data/bob.csv","party":"bob"}}}],"outputs":[{"name":"receiver_output","type":"TABLE","table_metadata":{"type":"INDIVIDUAL_TABLE","indivial":{"path":"/home/kuscia/var/storage/data/alice_psi_out.csv","party":"alice"}}},{"name":"sender_output","type":"TABLE","table_metadata":{"type":"INDIVIDUAL_TABLE","indivial":{"path":"/home/kuscia/var/storage/data/bob_psi_out.csv","party":"bob"}}}]}'
```

在该示例中:

* `.metadata.name`：表示 KusciaTask 的名称，当前示例为`secretflow-task-psi`。
* `.spec.initiator`：表示任务参与方中负责发起任务的节点标识，当前示例为`bob`。
* `.spec.parties`：表示所有任务参与方的信息。该字段下主要包含以下子字段：
  * `.spec.parties[0].domainID`：表示任务的一个参与方节点标识为`bob`。
  * `.spec.parties[0].appImageRef`：表示节点标识为`bob`的任务参与方所依赖的应用镜像 AppImage 名称为`secretflow-image`。
  * `.spec.parties[1].domainID`：表示任务的另一个参与方节点标识为`alice`。
  * `.spec.parties[1].appImageRef`：表示节点标识为`alice`的任务参与方所依赖的应用镜像 AppImage 名称为`secretflow-image`。
* `.spec.taskInputConfig`：表示任务输入参数配置。

1. 运行以下命令创建 KusciaTask。

```shell
kubectl apply -f secretflow-task-psi.yaml
```

## 查看 KusciaTask

下面以 KusciaTask `secretflow-task-psi` 为例，介绍如何查看任务运行状态。

1. 运行以下命令查看 KusciaTask。

```shell
kubectl get kt secretflow-task-psi
NAME                  STARTTIME   COMPLETIONTIME   LASTRECONCILETIME   PHASE
secretflow-task-psi   7s          7s               7s                  Succeeded
```

上述命令输出内容，各个列字段的含义如下：
* `NAME`：表示 KusciaTask 的名称，当前示例为`secretflow-task-psi`。
* `STARTTIME`：表示 KusciaTask 从开始执行到现在经历的时间。
* `COMPLETIONTIME`：表示 KusciaTask 从完成执行到现在经历的时间。
* `LASTRECONCILETIME`：表示 KusciaTask 从上次被更新到现在经历的时间。
* `PHASE`：表示 KusciaTask 当前所处的阶段。当前示例阶段为`Succeeded`。

2. 运行以下命令查看 KusciaTask 详细的状态信息。

```shell
kubectl get kt secretflow-task-psi -o jsonpath={.status} | jq
{
  "completionTime": "2023-04-04T01:45:46Z",
  "conditions": [
    {
      "lastTransitionTime": "2023-04-04T01:45:29Z",
      "lastUpdateTime": "2023-04-04T01:45:29Z",
      "status": "True",
      "type": "Initialized"
    },
    {
      "lastTransitionTime": "2023-04-04T01:45:29Z",
      "lastUpdateTime": "2023-04-04T01:45:29Z",
      "status": "True",
      "type": "ResourceCreated"
    },
    {
      "lastTransitionTime": "2023-04-04T01:45:46Z",
      "lastUpdateTime": "2023-04-04T01:45:46Z",
      "message": "Find pods for task",
      "reason": "TaskRunning",
      "status": "True",
      "type": "TaskIsRunning"
    },
    {
      "lastTransitionTime": "2023-04-04T01:45:46Z",
      "lastUpdateTime": "2023-04-04T01:45:46Z",
      "message": "The succeeded party number is greater than minReservedMembers",
      "reason": "TaskSucceeded",
      "status": "True",
      "type": "TaskRunSuccess"
    }
  ],
  "lastReconcileTime": "2023-04-04T01:45:46Z",
  "message": "The succeeded party number is greater than minReservedMembers",
  "phase": "Succeeded",
  "podStatuses": {
    "alice/secretflow-task-psi-0": {
      "namespace": "alice",
      "podName": "secretflow-task-psi-0",
      "podPhase": "Succeeded"
    },
    "bob/secretflow-task-psi-0": {
      "namespace": "bob",
      "nodeName": "dd3bdda2b853",
      "podName": "secretflow-task-psi-0",
      "podPhase": "Succeeded"
    }
  },
  "reason": "TaskSucceeded",
  "startTime": "2023-04-04T01:45:29Z"
}
```

## 清理 KusciaTask

下面以 KusciaTask `secretflow-task-psi` 为例，介绍如何清理 KusciaTask。

1. 运行以下命令清理 KusciaTask。

```shell
kubectl delete kt secretflow-task-psi
```

2. 检查 KusciaTask 是否已被清理。

```shell
kubectl get kt secretflow-task-psi
Error from server (NotFound): kusciatasks.kuscia.secretflow "secretflow-task-psi" not found
```

## 参考

下面以 `task-template` 模版为例，介绍 KusciaTask 所包含的完整字段。

```yaml
apiVersion: kuscia.secretflow/v1alpha1
kind: KusciaTask
metadata:
  name: task-template
spec:
  initiator: bob
  scheduleConfig:
    minReservedMembers: 2
    resourceReservedSeconds: 30
    lifecycleSeconds: 300
    retryIntervalSeconds: 15
  taskInputConfig: '{"domain":"psi","name":"2pc_balanced","version":"0.0.1","params":[{"prefixes":[],"name":"protocol","atomic":{"s":"ECDH_PSI_2PC"}},{"prefixes":[],"name":"receiver","atomic":{"s":"alice"}},{"prefixes":[],"name":"precheck_input","atomic":{"b":true}},{"prefixes":[],"name":"sort","atomic":{"b":true}},{"prefixes":[],"name":"broadcast_result","atomic":{"b":true}},{"prefixes":[],"name":"bucket_size","atomic":{"i64":1048576}},{"prefixes":[],"name":"curve_type","atomic":{"s":"CURVE_25519"}}],"inputs":[{"name":"receiver_input","type":"TABLE","table_params":{"col_params":[{"name":"key","cols":["id1"]}]},"table_metadata":{"type":"INDIVIDUAL_TABLE","indivial":{"path":"/home/kuscia/var/storage/data/alice.csv","party":"alice"}}},{"name":"sender_input","type":"TABLE","table_params":{"col_params":[{"name":"key","cols":["id2"]}]},"table_metadata":{"type":"INDIVIDUAL_TABLE","indivial":{"path":"/home/kuscia/var/storage/data/bob.csv","party":"bob"}}}],"outputs":[{"name":"receiver_output","type":"TABLE","table_metadata":{"type":"INDIVIDUAL_TABLE","indivial":{"path":"/home/kuscia/var/storage/data/alice_psi_out.csv","party":"alice"}}},{"name":"sender_output","type":"TABLE","table_metadata":{"type":"INDIVIDUAL_TABLE","indivial":{"path":"/home/kuscia/var/storage/data/bob_psi_out.csv","party":"bob"}}}]}'
  parties:
    - domainID: alice
      appImageRef: app-template
      role: client
      minReservedPods: 2
      template:
        replicas: 2
        spec:
          restartPolicy: Never
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
                  port: 8080
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
                  port: 8080
                failureThreshold: 1
                periodSeconds: 20
              startupProbe:
                httpGet:
                  path: /healthz
                  port: 8080
                failureThreshold: 30
                periodSeconds: 10
              imagePullPolicy: IfNotPresent
              workingDir: /work
        restartPolicy: Never          
    - domainID: bob
      appImageRef: app-template
status:
  conditions:
    - lastTransitionTime: "2023-04-04T01:45:29Z"
      lastUpdateTime: "2023-04-04T01:45:29Z"
      status: "True"
      type: Initialized
    - lastTransitionTime: "2023-04-04T01:45:29Z"
      lastUpdateTime: "2023-04-04T01:45:29Z"
      status: "True"
      type: ResourceCreated
    - lastTransitionTime: "2023-04-04T01:45:46Z"
      lastUpdateTime: "2023-04-04T01:45:46Z"
      message: Find pods for task
      reason: TaskRunning
      status: "True"
      type: TaskIsRunning
    - lastTransitionTime: "2023-04-04T01:45:46Z"
      lastUpdateTime: "2023-04-04T01:45:46Z"
      message: The succeeded party number is greater than minReservedMembers
      reason: TaskSucceeded
      status: "True"
      type: TaskRunSuccess
  completionTime: "2023-04-04T01:45:46Z"
  lastReconcileTime: "2023-04-04T01:45:46Z"
  phase: Succeeded
  podStatuses: 
    alice/task-template-psi-client-0: 
      namespace: alice
      podName: task-template-psi-0
      nodeName: dd8ijhy7po09
      podPhase: Succeeded
      message: ""
      terminationLog: ""
      reason: ""
    bob/task-template-psi-0: 
      namespace: bob
      nodeName: dd3bdda2b853
      podName: task-template-psi-0
      podPhase: Succeeded
  startTime: "2023-04-04T01:45:29Z"
```

KusciaTask `metadata` 的子字段详细介绍如下：

* `name`：表示 KusciaTask 的名称。

KusciaTask `spec` 的子字段详细介绍如下：

* `initiator`：表示任务参与方中负责发起任务的节点标识。
* `scheduleConfig`：表示任务调度的相关配置。默认为空，表示使用默认值。
  * `scheduleConfig.minReservedMembers`：表示任务调度成功时，需要最小的已预留成功的任务参与方个数。默认为空，表示所有任务参与方都需成功预留资源。
  * `scheduleConfig.resourceReservedSeconds`：表示成功预留资源的任务参与方，在等待其他任务参与方成功预留资源期间，占用资源的时长，默认为30s。若占用资源超过该时长，则释放该资源，等待下一轮调度。
  * `scheduleConfig.lifecycleSeconds`：表示任务调度的生命周期，默认为300s。若在规定的时间内，任务没有完成调度，则将任务置为失败。
  * `scheduleConfig.retryIntervalSeconds`：表示任务在一个调度周期失败后，等待下次调度的时间间隔，默认为30s。
* `taskInputConfig`：表示任务输入参数配置。
* `parties`：表示所有任务参与方的信息。
  * `parties[].domainID`：表示任务参与方的节点标识。
  * `parties[].appImageRef`：表示任务参与方所依赖的应用镜像名称。
  * `parties[].role`：表示任务参与方的角色。
  * `parties[].minReservedPods`：表示任务参与方最小已预留资源的 Pod 数量，默认为空，表示任务参与方所有的 Pod 数量。Kuscia 调度器对每个任务参与方使用 Co-Scheduling 调度策略，仅当任务参与方下已预留资源的 Pod 数量大于等于该值时，设置该参与方为已完成预留资源。
  * `parties[].template`：表示任务参与方应用的模版信息。若配置该模版，则使用模版中配置的信息替换从`parties[].appImageRef`获取的模版信息。该字段下所包含的子字段含义，请参考概念 AppImage。

KusciaTask `status` 的子字段详细介绍如下：

* `phase`：表示 KusciaTask 当前所处的阶段。当前包括以下几种 PHASE：
  * `Pending`：表示 KusciaTask 被创建，但是 KusciaTask Controller 还没有开始处理该资源。
  * `Creating`：表示 KusciaTask 被 KusciaTask Controller 处理，KusciaTask Controller 会根据 KusciaTask 的描述信息，创建跟 KusciaTask 相关的任务资源，例如：Configmap、Service、Pod 等。
  * `Running`：表示 KusciaTask 正处于运行状态。
  * `Succeeded`：表示 KusciaTask 运行成功。
  * `Failed`：表示 KusciaTask 运行失败。
* `reason`: 表示为什么 KusciaTask 处于该阶段。
* `message`: 表示 KusciaTask 处于该阶段的详细描述信息，用于对`reason`的补充。
* `conditions`: 表示 KusciaTask 处于该阶段时所包含的一些状况。
  * `conditions[].type`: 表示状况的名称。
  * `conditions[].status`: 表示该状况是否适用，可能的取值有`True`、`False`或`Unknown`。
  * `conditions[].reason`: 表示该状况的原因。
  * `conditions[].message`: 表示该状况的详细信息。
  * `conditions[].lastTransitionTime`: 表示转换为该状态的时间戳。
* `podStatuses`: 表示 KusciaTask 相关的所有参与方的 Pod 状态信息。
  * `podStatuses[].podName`: 表示 Pod 的名称。
  * `podStatuses[].namespace`: 表示 Pod 的所在的 Namespace。
  * `podStatuses[].nodeName`: 表示 Pod 的所在的 Node 名称。
  * `podStatuses[].podPhase`: 表示 Pod 的所处阶段。
  * `podStatuses[].reason`: 表示 Pod 处在该阶段的原因。
  * `podStatuses[].message`: 表示 Pod 处在该阶段的详细描述信息。
  * `podStatuses[].terminationLog`: 表示 Pod 异常终止时的日志信息。
* `startTime`: 表示 KusciaTask 第一次被 Kuscia 控制器处理的时间戳。
* `completionTime`: 表示 KusciaTask 运行完成的时间戳。
* `lastReconcileTime`: 表示 KusciaTask 上次更新的时间戳。
