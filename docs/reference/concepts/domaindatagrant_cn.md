# DomainDataGrant

在 Kuscia 中，DomainDataGrant 表示被 Kuscia 所管理的数据对象授权，包括 授权方节点 ID、被授权节点 ID、授权的 DomainData 、授权使用限制和授权信息签名等。DomainDataGrant 属于节点内资源，每一个 DomainDataGrant 都有自己所属的 Domain，且仅能被自己所属的 Domain 访问。

DomainData Controller 会在 DomainDataGrant 被创建之后，把 DomainDataGrant 从授权方节点下复制到被授权节点下，同时也会把授权的 DomainData 也复制过去。同理，如果授权方删除了本条授权，Controller 也会从被授权方下删除本条授权的拷贝。

被授权方可以通过授权方的证书来校验 DomainDataGrant 里的签名来验证授权信息是否合法。由于将 DomainDataGrant 的信息进行签名的操作比较繁琐，建议调用 datamesh 的相关接口进行操作。签名字段可用为空。

## 用例

以下是一些 DomainDataGrant 的典型用例：

- 创建 DomainDataGrant，你将体验如何使用通过创建一个 DomainDataGrant 将你自己的数据加入 Kuscia 的管理。
- 更新 DomainDataGrant，你将熟悉如何更新现有的 DomainDataGrant，从而变更 DomainDataGrant 的信息。
- 清理 DomainDataGrant，你将熟悉如何清理不需要的 DomainDataGrant。删除 DomainDataGrant 并不会删除真实的数据，只是 Kuscia 不再管理这些授权信息。
- 在 Domain 侧管理 DomainDataGrant，你将熟悉如何通过 Data Mesh API 来在 Domain 侧管理 DomainDataGrant。
- 参考 DomainDataGrant 对象定义，你将获取详细的 DomainDataGrant 描述信息。

## 创建 DomainData

下面以 `alice-bob.yaml` 的内容为例，介绍创建 DomainDataGrant。

```yaml
apiVersion: kuscia.secretflow/v1alpha1
kind: DomainDataGrant
metadata:
  labels:
    kuscia.secretflow/domaindataid: alice-table
  name: alice-bob
  namespace: alice
spec:
  author: alice
  domainDataID: alice-table
  grantDomain: bob
  description:
    name: test
```

在该示例中:

- `.metadata.labels`：标签在 K3s 中用于支持高效的查询和监听操作，参考：[标签和选择算符](https://kubernetes.io/zh-cn/docs/concepts/overview/working-with-objects/labels/)。
- `.metadata.name`：表示隐私计算节点 DomainDataGrant 的名称，当前示例为`alice-bob`。
- `.metadata.namespace`: 表示 DomainDataGrant 所属的命名空间，即所属的节点，当前示例为`alice`。
- `.spec.author`：表示授权方节点 ID 。当前示例授权方节点为`alice`。
- `.spec.domainDataID`：表示 DomainDataGrant 所属的 DomainData。
- `.spec.grantDomain`：表示被授权的节点 ID 。当前示例授权方节点为`bob`。




1. 参照 [DomainData](./domaindata_cn.md) 中的方法，先创建 `alice-table` 这个 DomainData 资源。

2. 在 master 容器即 `${USER}-kuscia-master` 容器中，运行以下命令创建 DomainDataGrant。
```shell
kubectl apply -f alice-bob.yaml
```

1. 在 master 容器即 `${USER}-kuscia-master` 容器中，检查 DomainDataGrant 是否创建成功。
```shell
kubectl get domaindatagrant alice-bob -n alice
```

2. 在 master 容器即 `${USER}-kuscia-master` 容器中，检查 bob 是否被成功授权。
```shell
kubectl get domaindatagrant alice-bob -n bob
kubectl get domaindata alice-table -n bob
```

## 更新 DomainDataGrant

下面以 `alice-bob.yaml` 的内容为例，介绍更新 DomainDataGrant。

```yaml
apiVersion: kuscia.secretflow/v1alpha1
kind: DomainDataGrant
metadata:
  labels:
    kuscia.secretflow/domaindataid: alice-table
  name: alice-bob
  namespace: alice
spec:
  author: alice
  domainDataID: alice-table
  grantDomain: bob
  description:
    name: test2
```

在该示例中，将`.spec.description.name`的值调整为`test2`。


1. 运行以下命令更新 DomainDataGrant。

```shell
kubectl apply -f alice-bob.yaml
```

2. 检查 DomainDataGrant 是否更新成功。

```shell
kubectl get domaindatagrant alice-bob -n alice
```

3. 检查 bob 下的 DomainDataGrant 是否更新成功。
```shell
kubectl get domaindatagrant alice-bob -n bob
```


## 清理 DomainData

下面以 DomainDataGrant `alice-bob.yaml` 为例，介绍清理 DomainDataGrant。
注意：清理 DomainDataGrant 并不会清除真实的数据内容，只是从 Kuscia 中删除 DomainDataGrant 的相关资源。

1. 运行以下命令清理 DomainDataGrant。
```shell
kubectl delete domaindatagrant alice-bob -n alice
```

1. 检查 alice 下的 DomainDataGrant 是否已被清理。

```shell
kubectl get domaindatagrant alice-bob -n alice
Error from server (NotFound): domaindatagrants.kuscia.secretflow "alice-bob" not found
```

2. 检查 alice 下的 DomainData 是否还存在。

```shell
kubectl get domaindata alice-table -n alice
```

3. 检查 bob 下的 DomainDataGrant 是否已被清理。

```shell
kubectl get domaindatagrant alice-bob -n bob
Error from server (NotFound): domaindatagrants.kuscia.secretflow "alice-bob" not found
```

4. 检查 bob 下的 DomainDataG 是否已被清理。

```shell
kubectl get domaindata alice-table -n bob
Error from server (NotFound): domaindatas.kuscia.secretflow "alice-table" not found
```

{#data-mesh}
## 在 Domain 侧管理 DomainDataGrant

如 上文所述，DomainDataGrant 属于节点内资源，每一个 DomainDataGrant 都有自己所属的 Domain，且仅能被自己所属的 Domain 访问。
你可以在 Domain 侧管理属于该 Domain 的 DomainDataGrant。Kuscia 在 Domain 侧提供了的 DataMesh API 来管理 DomainDataGrant。

Data Mesh API 提供 HTTP 和 GRPC 两种访问方法，分别位于 8070 和 8071
端口，详情请参考 [Data Mesh API](../apis/datamesh/summary_cn.md#data-mesh-api-约定)。

1. 进入 alice 容器 `${USER}-kuscia-lite-alice` 容器中，查询 DomainDataGrant。
```shell
curl -X POST 'http://{{USER-kuscia-lite-alice}:8070/api/v1/datamesh/domaindatagrant/query' --header 'Content-Type: application/json' -d '{
  "domaindatagrant_id": "alice"
}'
```


{#refer}

## 参考

下面以 `alice-bob` 模版为例，介绍 DomainData 所包含的完整字段。

```yaml
apiVersion: kuscia.secretflow/v1alpha1
kind: DomainDataGrant
metadata:
  labels:
    kuscia.secretflow/domaindataid: alice-table
  name: alice-bob
  namespace: alice
spec:
  author: alice
  domainDataID: alice-table
  grantDomain: bob
  signature: xxxxxxx
  description:
    name: test2
```

DomainDataGrant `metadata` 的子字段详细介绍如下：

- `labels`：标签在 K3s 中用于支持高效的查询和监听操作，参考：[标签和选择算符](https://kubernetes.io/zh-cn/docs/concepts/overview/working-with-objects/labels/)。
- `name`：表示隐私计算节点 DomainDataGrant 的名称，当前示例为`alice-bob`。
- `namespace`：表示 DomainDataGrant 所属的命名空间，即所属的节点，当前示例为`alice`。

DomainDataGrant `spec` 的子字段详细介绍如下：

- `author`：表示授权方节点 ID，用来标识这条授权是由哪个节点发起的。
- `domainDataID`：表示这条授权是基于哪个 DomainData 生成的，DomainData Controller 不仅会将 DomainDataGrannt 拷贝给被授权节点，还会将 DomainData 也拷贝过去，因此需要保证 DomainData 是已经存在的。
- `grantDomain`：表示被授权方节点 ID，请保证该 Domain 已经存在。
- `signature`：表示授权信息的签名，是用 author 的节点私钥进行签名的。grantDomain 可以用 author 的公钥进行验证授权信息的真假。
- `description`：表示用户自定义的描述信息。
