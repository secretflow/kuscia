# DomainData

在 Kuscia 中，DomainData 表示被 Kuscia 所管理的数据对象，包括 表、模型、规则和报告等。DomainData 属于节点内资源，每一个 DomainData 都有自己所属的 Domain，且仅能被自己所属的 Domain 访问。

一个常见的用例是：隐私求交参与方分别在自己的 Domain 下创建出自己要参与计算的表，即`type`为`table`的 DomainData，然后创建一个 [KusciaJob](kusciajob_cn.md) 进行隐私求交任务，
该任务结束后，会在参与方生成新的隐私求交结果表，即生成`type`为`table`的 DomainData，参与方可以通过这个 DomainData 获取到文件地址或者将这个 DomainData 用于下次计算任务。

值得注意的是：无论是创建、更新、清理 DomainData，都不会对真实的数据内容产生影响，Kuscia 仅仅记录数据的 meta 信息，用于在计算任务中协助应用算法组件读取数据。
当前版本 Kuscia 暂未支持检查真实的数据内容是否满足 DomainData 中提交的 meta 信息定义，后续 Kuscia 会增加对提交的 meta 信息的验证。

如上所述，你可以通过创建一个 DomainData 将你自己的数据加入 Kuscia 的管理，也可以通过任务生成一个新的 DomainData 。


## 用例

以下是一些 DomainData 的典型用例：

- 创建 DomainData，你将体验如何使用通过创建一个 DomainData 将你自己的数据加入 Kuscia 的管理。
- 更新 DomainData，你将熟悉如何更新现有的 DomainData，从而变更 DomainData 的信息。
- 清理 DomainData，你将熟悉如何清理不需要的 DomainData。删除 DomainData 并不会删除真实的数据，只是 Kuscia 不再管理这些数据。
- 在 Domain 侧管理 DomainData，你将熟悉如何通过 Data Mesh API 来在 Domain 侧管理 DomainData。
- 参考 DomainData 对象定义，你将获取详细的 DomainData 描述信息。

## 创建 DomainData

下面以 `alice-table.yaml` 的内容为例，介绍创建 DomainData。

```yaml
apiVersion: kuscia.secretflow/v1alpha1
kind: DomainData
metadata:
  labels:
    kuscia.secretflow/domaindata-type: table
    kuscia.secretflow/domaindata-vendor: manual
  name: alice-table
  namespace: alice
spec:
  attributes:
    description: alice demo data
  columns:
    - comment: ""
      name: id1
      type: str
    - comment: ""
      name: age
      type: float
    - comment: ""
      name: education
      type: float
  dataSource: default-data-source
  name: alice.csv
  relativeURI: alice.csv
  type: table
  vendor: manual
  author: alice
```

在该示例中:

- `.metadata.labels`：标签在 K3s 中用于支持高效的查询和监听操作，参考：[标签和选择算符](https://kubernetes.io/zh-cn/docs/concepts/overview/working-with-objects/labels/)。
- `.metadata.name`：表示隐私计算节点 DomainData 的名称，当前示例为`alice-table`。
- `.metadata.namespace`: 表示 DomainData 所属的命名空间，即所属的节点，当前示例为`alice`。
- `.spec.attributes`：表示 DomainData 的自定义属性，以键值对形式表示，用作用户或应用算法为数据对象添加扩展信息，详细请查看 [参考](#refer)。
- `.spec.columns`：表示对于表类型的 DomainData 的列信息。仅当`type`为`table`时，该字段才存在，详细请查看 [参考](#refer)。
- `.spec.dataSource`：表示 DomainData 所属的数据源。数据源是数据所存放的位置，存在多种类型的数据源，详细请查看 [参考](#refer)。
- `.spec.name`：表示一个人类可读的名称，仅用作展示，可重复。
- `.spec.relativeURI`：表示相对于数据源根路径的位置，当前示例的绝对路径为`/home/kuscia/var/storage/data/alice.csv`，详细请查看 [参考](#refer)。
- `.spec.type`：表示 DomainData 的类型，如 `table`、`model`、`rule`、`report`，分别表示数据表，模型，规则，报告。
- `.spec.vendor`：表示 DomainData 的来源，仅用作标识，详细请查看 [参考](#refer)。
- `.spec.author`：表示 DomainData 的所属者的节点 ID ，用来标识这个 DomainData 是由哪个节点创建的。


1. 准备你的 CSV 数据文件，将你的数据文件重命名为 `alice.csv`，并拷贝到alice节点容器即`${USER}-kuscia-lite-alice`容器的`/home/kuscia/var/storage/data`目录下，运行以下命令可完成：
```shell
mv {YOUR_CSV_DATA_FILE} alice.csv
docker cp alice.csv ${USER}-kuscia-lite-alice:/home/kuscia/var/storage/data/
```

2. 进入 master 容器即 `${USER}-kuscia-master` 容器，创建示例中的`alice-table.yaml`并根据你的 CSV文件 的列字段信息，调整上述示例中的`.spec.columns`字段。

3. 在 master 容器即 `${USER}-kuscia-master` 容器中，运行以下命令创建 DomainData。
```shell
kubectl apply -f alice-table.yaml
```

4. 在 master 容器即 `${USER}-kuscia-master` 容器中，检查 DomainData 是否创建成功。
```shell
kubectl get domaindata alice-table -n alice
```

## 更新 DomainData

下面以 `alice-table.yaml` 的内容为例，介绍更新 DomainData。

```yaml
apiVersion: kuscia.secretflow/v1alpha1
kind: DomainData
metadata:
  labels:
    kuscia.secretflow/domaindata-type: table
    kuscia.secretflow/domaindata-vendor: manual
  name: alice-table
  namespace: alice
spec:
  attributes:
    description: alice demo data
  columns:
    - comment: ""
      name: id1
      type: str
    - comment: ""
      name: age
      type: float
    - comment: ""
      name: education
      type: float
  dataSource: default-data-source
  name: alice-test.csv
  relativeURI: alice.csv
  type: table
  vendor: manual
  author: alice
```

在该示例中，将`.spec.name`的值调整为`alice-test.csv`。


1. 运行以下命令更新 DomainData。

```shell
kubectl apply -f alice-table.yaml
```

2. 检查 DomainData 是否更新成功。

```shell
kubectl get domaindata alice-table -n alice
```

## 清理 DomainData

下面以 DomainData `alice-table.yaml` 为例，介绍清理 DomainData。
注意：清理 DomainData 并不会清除真实的数据内容，只是从 Kuscia 中删除 DomainData 中所记录的 meta 信息。

1. 运行以下命令清理 DomainData。
```shell
kubectl delete domaindata alice-table -n alice
```

2. 检查 DomainData 是否已被清理。

```shell
kubectl get domaindata alice-table -n alice
Error from server (NotFound): domaindatas.kuscia.secretflow "alice-table" not found
```

{#data-mesh}
## 在 Domain 侧管理 DomainData

如 上文所述，DomainData 属于节点内资源，每一个 DomainData 都有自己所属的 Domain，且仅能被自己所属的 Domain 访问。
你可以在 Domain 侧管理属于该 Domain 的 DomainData。Kuscia 在 Domain 侧提供了的 Kuscia API 来管理 DomainData。

Kuscia API 提供 HTTP 和 GRPC 两种访问方法，端口分别为 8082 和 8083 。
端口，详情请参考 [Kuscia API](../apis/domaindata_cn.md)。

1. 进入 alice 容器 `${USER}-kuscia-lite-alice` 容器中，查询 DomainData。
```shell
docker exec -it root-kuscia-lite-alice curl -X POST 'https://127.0.0.1:8082/api/v1/domaindata/query' --header "Token: $(cat /home/kuscia/var/certs/token)" --header 'Content-Type: application/json' -d '{
 "data": {
  "domain_id": "alice",
  "domaindata_id": "alice-table"
  }
}' --cacert /home/kuscia/var/certs/ca.crt --cert /home/kuscia/var/certs/ca.crt --key /home/kuscia/var/certs/ca.key
```


{#refer}

## 参考

下面以 `alice-table` 模版为例，介绍 DomainData 所包含的完整字段。

```yaml
apiVersion: kuscia.secretflow/v1alpha1
kind: DomainData
metadata:
  labels:
    kuscia.secretflow/domaindata-type: table
    kuscia.secretflow/domaindata-vendor: manual
  name: alice-table
  namespace: alice
spec:
  attributes:
    description: alice demo data
  columns:
    - comment: ""
      name: id1
      type: str
    - comment: ""
      name: age
      type: float
    - comment: ""
      name: education
      type: float
  dataSource: default-data-source
  name: alice.csv
  relativeURI: alice.csv
  type: table
  vendor: manual
  author: alice
```

DomainData `metadata` 的子字段详细介绍如下：

- `labels`：标签在 K3s 中用于支持高效的查询和监听操作，参考：[标签和选择算符](https://kubernetes.io/zh-cn/docs/concepts/overview/working-with-objects/labels/)。
- `name`：表示隐私计算节点 DomainData 的名称，当前示例为`alice-table`。
- `namespace`：表示 DomainData 所属的命名空间，即所属的节点，当前示例为`alice`。

DomainData `spec` 的子字段详细介绍如下：

- `attributes`：表示 DomainData 的自定义属性，以键值对形式表示，用作用户或应用算法为数据对象添加扩展信息，Kuscia不感知，仅存储。你可以使用这个字段存储一些你自身业务属性的信息。
  比如你可以存储数据文件的md5值，或者存储行数，应用算法也可用于存储模型的类型等。
- `columns`：表示对于表类型的 DomainData 的列信息。仅当`type`为`table`时，该字段才存在。
    - `name`：列字段名称。
    - `type`：列字段的数据类型，默认支持的数据类型参考[data.proto](https://github.com/secretflow/spec/blob/main/secretflow/spec/v1/data.proto#L112)。该字段当前版本并非可枚举的，列字段的类型由应用算法组件进行定义和消费，Kuscia 仅作存储。
    - `comment`：列字段的注释，仅用作展示。
- `dataSource`：表示 DomainData 所属的数据源。数据源是数据所存放的位置，存在多种类型的数据源，比如`localfs`类型数据源表示节点内的一个文件目录，`mysql`类型数据源表示mysql实例中的一个数据库。
  目前 Kuscia 仅支持`localfs`数据源，并且内置一个数据源实例`default-data-source`，即当前示例中所使用的数据源。该`default-data-source`数据源的根目录为节点内的`/home/kuscia/var/storage/data`目录。
- `name`：表示一个人类可读的名称，仅用作展示，可重复。
- `relativeURI`：表示相对于数据源根路径的位置。对于`localfs`类型数据源来说，是相对于`localfs`类型数据源所表示的文件目录的相对位置，对于`mysql`类型数据源来说，是该mysql实例中的数据库的某个数据表。
  在该示例中是对于`default-data-source`数据源根目录的相对位置，即`/home/kuscia/var/storage/data/alice.csv`。
- `type`：表示 DomainData 的类型，目前支持 `table`、`model`、`rule`、`report`、`unknown`五种类型，分别表示数据表，模型，规则，报告和未知类型。
- `vendor`：表示 DomainData 的来源，仅用作标识，对于你手动创建的 DomainData，可以将其设置为`manual`，对于应用算法组件生成的表，由算法组件本身填充，secretflow算法组件会填充`secretflow`。
- `.spec.author`：表示 DomainData 的所属者的节点 ID ，用来标识这个 DomainData 是由哪个节点创建的。

