# DomainDataSource

Kuscia 通过 DomainDataSource 来管理数据源信息，数据源是指存储数据的地方，其可以是本地文件路径、对象存储服务 OSS（或支持标准 AWS S3 协议的对象存储服务）、MySQL 等。DomainDataSource 对象用于描述数据源的元数据（MetaData），
如一个描述 OSS 数据源的 DomainDataSource 对象会包含：访问 OSS 所需的 Endpoints、Bucket、AK、SK 等信息（敏感信息会加密保存在 DomainDataSource 的 encryptedInfo 字段中）。一个描述本地文件路径数据源的 DomainDataSource 对象会包含：本地文件数据源的绝对路径（如下示例中的 uri 字段）；

以下是一个描述本地文件路径（localfs）数据源的 DomainDataSource CRD 的示例：

```yaml
apiVersion: kuscia.secretflow/v1alpha1
kind: DomainDataSource
metadata:
  labels:
    kuscia.secretflow/domaindatasource-type: localfs
  name: default-data-source
  namespace: alice
spec:
  accessDirectly: true
  data:
    encryptedInfo: joPdxNPMZJk6DiD/M1KkjSSY58BnLxaOM41z3uB5SmkjpDJ14+4qXqdsWRXAqMoPBdxRVDa/GhdUKnr/hL7eBZFMVivFCtCqMTaBJunUYsH6U168wzWaYrERme8BmNChETY3HdIeM4wP7o72+ctDoPDASuAWZNoJ5hxnYfYcpxJ3YoG3STf2DzqYmIZeAWxlneJ32wWFtSBlo1hzIvxuiuwHXZaq7h77a/+H7s1paUpio8wTkqJohID4k37pX3thuit9OUEsqxDxl5SEm+qq9HeVC8XscuKVs3nQw/cmSg4LavtAQWqrk7qjqYWmd370z6cQjuHOCbX1gZ2UbCjpnw==
  name: default-data-source
  type: localfs
  uri: /home/kuscia/var/storage/data

```

DomainDataSource `metadata` 的子字段详细介绍如下：

- `labels`：标签在 K3s 中用于支持高效的查询和监听操作，参考：[标签和选择算符](https://kubernetes.io/zh-cn/docs/concepts/overview/working-with-objects/labels/)。
- `name`：表示数据源的标识，用于唯一标识数据源。
- `namespace`：表示 DomainDataSource 所属的命名空间，即所属的节点，当前示例为`alice`。

DomainDataSource `spec` 的子字段详细介绍如下：

- `accessDirectly`：表示隐私计算应用（如 SecretFlow ）应直连访问数据源还是通过 DataProxy 访问数据源（DataProxy 暂未支持），当前默认直连数据源，不经过 DataProxy。
- `encryptedInfo`：加密存储访问数据源所需的信息，如数据源为 MySQL 时，此字段会加密保存 MySQL 的链接串。
- `name`：数据源的名称，可重复，注意区别于 metadata 中的 name 字段。
- `type`：表示数据源的类型，类型包括：localfs, oss, mysql 。
- `uri`：表示数据源的概括信息，如 localfs 此处为绝对路径（如 /home/kuscia/var/storage/data），如 OSS 此处为 bucket 与 prefix 拼接的路径（alice_bucket/kuscia/data/）。

## 用例

以下是一些 DomainDataSource 的典型用例：

- 创建 DomainDataSource
- 更新 DomainDataSource
- 清理 DomainDataSource

## 创建 DomainDataSource

因 DomainDataSource 中含有 encryptedInfo 字段，其需要使用 Domain 节点的公钥进行加密，推荐使用 Kuscia API 进行创建，[参考 kuscia API 文档](../apis/domaindatasource_cn.md#create-domain-data-source)。

## 更新 DomainDataSource

因 DomainDataSource 中含有 encryptedInfo 字段，其需要使用 Domain 节点的公钥进行加密，推荐使用 Kuscia API 进行更新，[参考 kuscia API 文档](../apis/domaindatasource_cn.md#update-domain-data-source)。

## 清理 DomainDataSource

下面以删除 alice Domain中以 `demo-localfs-datasource` 为数据源标识的 DomainDataSource 为例：
注意：清理 DomainDataSource 并不会清除真实的数据源，只是从 Kuscia 中删除 DomainDataSource CRD 对象。

1. 运行以下命令清理 DomainDataSource。

    ```shell
    kubectl delete domaindatasource demo-localfs-datasource -n alice
    ```

2. 检查 Alice 下的 DomainDataSource 是否已被清理。

    ```shell
    kubectl get domaindatasource demo-localfs-datasource -n alice
    Error from server (NotFound): domaindatasources.kuscia.secretflow "demo-localfs-datasource" not found
    ```
