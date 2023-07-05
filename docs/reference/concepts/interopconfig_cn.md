# InteropConfig

点对点组网模式下，任务参与方通过 InteropController 将任务调度方集群中的任务参与方 Namespace 下任务相关的资源同步到到本集群中，从而协同完成联合计算。
你可以通过 InteropConfig 配置任务调度方和参与方。具体用例请参考下文。

## 用例

以下是一些 InteropConfig 的典型用例：

- 创建 InteropConfig，你将体验如何使用 InteropConfig 从任务调度方同步任务相关资源。
- 更新 InteropConfig，你将熟悉如何更新现有的 InteropConfig，从而变更从任务调度方同步任务相关资源。
- 清理 InteropConfig，你将熟悉如何清理不需要的 InteropConfig。从而停止从任务调度方同步任务相关资源。
- 参考 InteropConfig 对象定义，你将获取详细的 InteropConfig 描述信息。

## 创建 InteropConfig

下面以 `alice-2-bob.yaml` 的内容为例，介绍创建 InteropConfig。

```yaml
apiVersion: kuscia.secretflow/v1alpha1
kind: InteropConfig
metadata:
  name: alice-2-bob
spec:
  host: bob
  members:
  - alice
```

在该示例中，表示将任务调度方`bob`集群中`alice` Namespace 下任务相关的资源同步到任务参与方集群中的`alice` Namespace 下。

- `.metadata.name`：表示 InteropConfig 的名称，当前示例为`alice-2-bob`。
- `.spec.host`：表示任务调度方的节点标识。当前示例为`bob`。
- `.spec.members`：表示任务参与方的节点标识。当前示例仅包含一个参与方，节点标识为`alice`。

1. 在参与方集群中运行以下命令创建 InteropConfig。

```shell
kubectl apply -f alice-2-bob.yaml
```

2. 检查 InteropConfig 是否创建成功。

```shell
kubectl get interop alice-2-bob
```

3. 查看创建的 InteropConfig `alice-2-bob` 详细信息。

```shell
kubectl get interop alice-2-bob -o yaml
apiVersion: kuscia.secretflow/v1alpha1
kind: InteropConfig
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"kuscia.secretflow/v1alpha1","kind":"InteropConfig","metadata":{"annotations":{},"name":"alice-2-bob"},"spec":{"host":"bob","members":["alice"]}}
  creationTimestamp: "2023-04-03T11:51:21Z"
  generation: 1
  name: alice-2-bob
  resourceVersion: "3040657"
  uid: ca92d4a6-cd9b-48f7-9106-68292ff418b5
spec:
  host: bob
  members:
  - alice
```

## 更新 InteropConfig

下面以 `alice-2-bob.yaml` 的内容为例，介绍更新 InteropConfig。

```yaml
apiVersion: kuscia.secretflow/v1alpha1
kind: InteropConfig
metadata:
  name: alice-2-bob
spec:
  host: bob
  members:
  - alice
  - alice-mock
```

在该示例中:

- 在`.spec.members`中新增一个节点标识为`alice-mock`的参与方。相应地，任务参与方通过 InteropController 将任务调度方`bob`集群中`alice-mock` Namespace 下任务相关的资源同步到任务参与方集群中的`alice-mock` Namespace 下。

1. 在参与方集群中运行以下命令更新 InteropConfig。

```shell
kubectl apply -f alice-2-bob.yaml
```

2. 查看更新的 InteropConfig `alice-2-bob` 详细信息。

```shell
kubectl get interop alice-2-bob -o yaml | grep "members:" -A2
  members:
  - alice
  - alice-mock
```

## 清理 InteropConfig

下面以 InteropConfig `alice-2-bob` 为例，介绍清理 InteropConfig。

1. 在参与方集群中运行以下命令清理 InteropConfig。

```shell
kubectl delete interop alice-2-bob
```

2. 检查 InteropConfig 是否已被清理。

```shell
kubectl get interop alice-2-bob
Error from server (NotFound): interopconfigs.kuscia.secretflow "alice-2-bob" not found
```

## 参考

下面以 `interop-template` 模版为例，介绍 InteropConfig 所包含的完整字段。

```yaml
apiVersion: kuscia.secretflow/v1alpha1
kind: InteropConfig
metadata:
  name: interop-template
spec:
  host: bob
  members:
    - alice
```

InteropConfig `metadata` 的子字段详细介绍如下：

- `name`：表示 InteropConfig 的名称。

InteropConfig `spec` 的子字段详细介绍如下：

- `host`：表示任务调度方的节点标识。
- `members[]`：表示任务参与方的节点标识。当前示例仅包含一个参与方，节点标识为`alice`，相应地，任务参与方通过 InteropController 将任务调度方`bob`集群中`alice` Namespace 下任务相关的资源同步到任务参与方集群中的`alice` Namespace 下。
