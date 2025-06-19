# Domain

在 Kuscia 中将隐私计算的节点称为 Domain，一个 Domain 中可以包含多个 K3s 的工作节点（Node）。Kuscia
复用了 K3s 的 Namespace 机制来管理节点权限，一个 Domain 对应 K3s 中的一个 Namespace。你可以使用 Domain 来管理和维护隐私计算节点。具体用例请参考下文。

## 用例

以下是一些 Domain 的典型用例：

- 创建 Domain，你将体验如何使用 Domain 创建隐私计算节点相关的 Namespace, ResourceQuota 资源。
- 更新 Domain，你将熟悉如何更新现有的 Domain，从而变更隐私计算节点相关的 Namespace, ResourceQuota 资源。
- 清理 Domain，你将熟悉如何清理不需要的 Domain。在 Kuscia 中，清理 Domain 并不会真正的删除 Domain 相关的 Namespace, ResourceQuota 资源，而是会在节点相关的 Namespace 资源上添加标记 Domain 被删除相关标签。
- 参考 Domain 对象定义，你将获取详细的 Domain 描述信息。

## 创建 Domain

下面以 `alice-domain.yaml` 的内容为例，介绍创建 Domain。

```yaml
apiVersion: kuscia.secretflow/v1alpha1
kind: Domain
metadata:
  name: alice
spec:
  role: partner
  cert: base64<certificate>
  resourceQuota:
    podMaxCount: 100
```

在该示例中:

- `.metadata.name`：表示隐私计算节点 Domain 名称，当前示例为 `alice` 。相应地，Kuscia 控制器会创建名称和 Domain 同名的 `alice` Namespace 资源。在 Kuscia 中，通过 Namespace 资源对不同机构用户进行资源隔离。
- `.spec.role`：表示隐私计算节点 Domain 的角色，默认为 `""`。支持两种取值：`partner` 和 `""`。
  - `partner`：表示外部节点，用在点对点组网模式下的协作方节点。点对点组网模式下，需要在任务调度方的集群中创建协作方的 Domain，在创建该 Domain 时，需要将 `role` 的值设置为 `partner` 。
  - `""`：表示内部节点。
- `.spec.cert`：表示 BASE64 编码格式的隐私计算节点证书，该证书是配置文件中的 `domainKeyData` 私钥产生的，可以通过[这里](https://github.com/secretflow/kuscia/blob/main/scripts/deploy/generate_cert.sh)的命令生成。在中心化模式场景不需要填充该字段。
- `.spec.resourceQuota.podMaxCount`：表示 Domain 所管理的隐私计算节点命名空间（Namespace）下所允许创建的最大 Pod 数量，当前示例为 `100`。

1. 运行以下命令创建 Domain。

    ```shell
    kubectl apply -f alice-domain.yaml
    ```

2. 检查 Domain 是否创建成功。

    ```shell
    kubectl get domain alice
    NAME    AGE
    alice   3s
    ```

3. 检查 Domain 相关的 Namespace, ResourceQuota 资源是否创建成功。

    ```shell
    kubectl get namespace alice
    NAME     STATUS   AGE
    alice    Active   18s

    kubectl get resourcequota -n alice
    NAME                  AGE   REQUEST      LIMIT
    resource-limitation   18s   pods: 0/100
    ```

## 更新 Domain

下面以 `alice-domain.yaml` 的内容为例，介绍更新 Domain。

```yaml
apiVersion: kuscia.secretflow/v1alpha1
kind: Domain
metadata:
  name: alice
spec:
  role: partner
  cert: base64<certificate>
  resourceQuota:
    podMaxCount: 200
```

在该示例中:

- 将 `.spec.resourceQuota.podMaxCount` 的值调整为 `200`。

1. 运行以下命令更新 Domain。

    ```shell
    kubectl apply -f alice-domain.yaml
    ```

2. 检查 Domain 相关的 ResourceQuota 资源是否更新成功。

    ```shell
    kubectl get resourcequota -n alice
    NAME                  AGE   REQUEST      LIMIT
    resource-limitation   4m    pods: 0/200
    ```

## 清理 Domain

下面以 Domain `alice` 为例，介绍清理 Domain。

1. 运行以下命令清理 Domain。

    ```shell
    kubectl delete domain alice
    ```

2. 检查 Domain 是否已被清理。

    ```shell
    kubectl get domain alice
    Error from server (NotFound): domains.kuscia.secretflow "alice" not found
    ```

3. 检查 Domain 相关的 Namespace, ResourceQuota 资源是否还存在。

    ```shell
    kubectl get namespace alice
    NAME    STATUS   AGE
    alice   Active   20m

    kubectl get resourcequota -n alice
    NAME                  AGE    REQUEST      LIMIT
    resource-limitation   20m    pods: 0/200
    ```

4. 检查 Domain 相关的 Namespace 是否已添加标记被清理的标签。

    ```shell
    kubectl get namespace alice -L kuscia.secretflow/deleted
    NAME    STATUS   AGE   DELETED
    alice   Active   20m   true
    ```

## 参考

下面以 `domain-template` 模版为例，介绍 Domain 所包含的完整字段。

```yaml
apiVersion: kuscia.secretflow/v1alpha1
kind: Domain
metadata:
  name: domain-template
spec:
  role: partner
  cert: base64<certificate>
  interConnProtocols:
  - kuscia
  resourceQuota:
    podMaxCount: 100
status:
  nodeStatuses:
    - lastHeartbeatTime: "2023-04-06T08:49:14Z"
      lastTransitionTime: "2023-04-04T12:20:40Z"
      name: 809406b513d3
      status: Ready
      version: e9a8013
```

Domain `metadata` 的子字段详细介绍如下：

- `name`：表示隐私计算节点 Domain 的名称，当前示例为 `domain-template` 。相应地，Kuscia 控制器会创建名称和 Domain 同名的 `domain-template` Namespace 资源。在 Kuscia 中，通过 Namespace 资源对不同机构用户进行资源隔离。

Domain `spec` 的子字段详细介绍如下：

- `role`：表示隐私计算节点 Domain 的角色，默认为`""`。支持两种取值：`partner`和 `""` 。
  - `partner`：表示外部节点，用在点对点组网模式下的协作方节点。 点对点组网模式下，需要在任务调度方的集群中创建协作方的 Domain，在创建该 Domain 时，需要将 `role` 的值设置为 `partner` 。
  - `""`：表示内部节点。
- `cert`：表示 BASE64 编码格式的隐私计算节点证书，该证书是配置文件中的 `domainKeyData` 私钥产生的，可以通过[这里](https://github.com/secretflow/kuscia/blob/main/scripts/deploy/generate_cert.sh)的命令生成。在中心化模式场景不需要填充该字段。
- `interConnProtocols`：表示外部隐私计算节点支持的互联互通作业协议类型，默认为 `""`。支持两种取值：`kuscia` 和 `bfia` 。当前该字段只支持配置一种协议，若配置多个协议，则会选择第一个协议作为互联互通作业的协议类型。未来会支持多种协议。
  - `kuscia`：表示该外部节点参与隐私计算任务时，会使用互联互通蚂蚁 `kuscia` 协议运行隐私计算任务。
  - `bfia`：表示该外部节点参与隐私计算任务时，会使用互联互通银联 `bfia` 协议运行隐私计算任务。
- `resourceQuota.podMaxCount`：表示 Domain 所管理的隐私计算节点 Namespace 下所允许创建的最大 Pod 数量，当前示例为 `100`。相应地，Kuscia 控制器会在 `domain-template` Namespace 下创建名称为 `resource-limitation` 的 ResourceQuota 资源。

Domain `status` 的子字段详细介绍如下：

- `nodeStatuses`：表示隐私计算节点 Domain 下所有 Kuscia Agent 的状态信息。
  - `nodeStatuses[].lastHeartbeatTime`：表示 Kuscia Agent 最近一次上报心跳的时间。
  - `nodeStatuses[].lastTransitionTime`：表示 Kuscia Agent 最近一次发生更新的时间。
  - `nodeStatuses[].name`：表示 Kuscia Agent 的名称。
  - `nodeStatuses[].status`：表示 Kuscia Agent 的状态。支持两种取值 `Ready` 、`NotReady` 。
    - `Ready`：表示 Kuscia Agent 状态正常。
    - `NotReady`：表示 Kuscia Agent 状态异常。
  - `nodeStatuses[].version`：表示 Kuscia Agent 的版本。
