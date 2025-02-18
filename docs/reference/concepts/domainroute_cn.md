# DomainRoute

DomainRoute 用于在中心化网络中配置 Lite 节点与 Master 之间的路由规则、Lite 节点之间的路由规则，以及点对点（P2P）网络中 Autonomy 节点之间的路由规则。
ClusterDomainRoute 用于在中心化网络中配置 Lite 节点之间的路由规则，DomainRouteController 会根据 ClusterDomainRoute 在各方 Lite 节点的 Namespace 中创建 DomainRoute。具体用例请参考下文。

## 用例

以下是一些 DomainRoute 和 ClusterDomainRoute 的典型用例：

* 中心化集群配置 Lite 访问 Master 的授权
* 中心化集群配置 Lite 节点之间的路由规则
* 点对点集群配置 Autonomy 节点之间的路由规则

## 中心化集群配置 Lite 访问 Master 的授权

在中心化集群中配置 Lite 访问 Master 的授权，需要在 Master 的 Namespace 下创建一条 DomainRoute。

在 Kuscia 中不同节点的服务域名是通过 Namespace 来区分的。Kuscia 支持多个中心化网络互联，为了区分不同的中心化网络中 Master 侧的服务（ApiServer 等），
故而给 Master 也分配了 Namespace。

```yaml
apiVersion: kuscia.secretflow/v1alpha1
kind: DomainRoute
metadata:
  name: alice-master
  namespace: master
spec:
  authenticationType: MTLS
  source: alice
  destination: master
  requestHeadersToAdd:
    Authorization: Bearer 781292.db7bc3a58fc5f07e
```

在示例中

* `.metadata.name`：表示路由规则的名称。
* `.metadata.namespace`：表示路由规则所在的命名空间，这里是 Master 的 Namespace。
* `.spec.authenticationType`：表示节点到目标节点的身份认证方式，目前仅支持 Token 、MTLS 和 None（表示不校验）。
* `.spec.source`：表示源节点的 Namespace，这里即 Lite 节点的 Namespace。
* `.spec.destination`：表示目标节点的 Namespace，这里即 Master 的命名空间。
* `.spec.requestHeadersToAdd`：表示目标节点侧的 Envoy 在转发源节点的请求时添加的 headers，示例中 key 为
  Authorization 的 header 是 Master 为 alice 分配访问 k3s 的令牌。

您可以通过 kubectl 命令来创建、修改、查看、删除 DomainRoute。

## 中心化集群配置 Lite 节点之间的路由规则

在中心化集群中，Lite 节点之间进行数据通信，以 alice 访问 bob 为例，需要创建一条 ClusterDomainRoute。
下面是 alice-bob-ClusterDomainRoute.yaml 的配置：

```yaml
apiVersion: kuscia.secretflow/v1alpha1
kind: ClusterDomainRoute
metadata:
  name: alice-bob
spec:
  authenticationType: MTLS
  source: alice
  destination: bob
  endpoint:
    host: 172.2.0.2
    ports:
    - name: http
      port: 1080
      protocol: HTTP
      isTLS: true
  mTLSConfig:
    sourceClientCert: MIICqjCCAZICFEiujM
    tlsCA: MIIEpAIBAAKCAQEAxK
```

在示例中：

* `.metadata.name`：表示路由规则的名称。
* `.spec.authenticationType`：表示节点到 Master 的身份认证方式，支持 Token、MTLS、None（不认证）。
* `.spec.source`：表示源节点的 Namespace。
* `.spec.destination`：表示目标节点的 Namespace。
* `.spec.endpoint`：表示目标节点的地址，具体字段包括：
  * `host`：表示目标节点域名或 IP。
  * `ports`：表示目标节点的端口，以及该端口的协议，协议类型为 HTTP或 GRPC协议，节点网关可在不同端口上分别支持http协议和GRPC协议。当前仅支持一个端口。
    * `name`: 表示端口名称。
    * `port`: 表示目标端口号。
    * `protocol`: 表示端口协议，支持 HTTP 和 GRPC。
    * `isTLS`：表示是否开启 HTTPS 或 GRPCS。
* `.spec.mTLSConfig`：表示源节点作为客户端访问目标节点的 MTLS 配置，具体字段如下：
  * `sourceClientCert`：表示源节点的客户端证书，value 值是经 BASE64 编码过的。
  * `tlsCA`：表示校验服务端（目标节点）证书的 CA，value 值是经过 BASE64 编码过的。不配置则表示不校验服务端证书。

通过 kubectl 命令来创建 ClusterDomainRoute：

```shell
kubectl apply -f alice-bob-ClusterDomainRoute.yaml
```

您还可以通过 kubectl 命令来修改、删除 ClusterDomainRoute。

## 点对点集群配置 Autonomy 节点之间的路由规则

对于点对点集群的两个 Autonomy 节点，alice 访问 bob，需要分别在 alice 和 bob 的 Namespace 下各创建一条 DomainRoute。

第一步，在目标节点 bob 的 Namespace 下创建 alice 访问 bob 的 DomainRoute：

```yaml
apiVersion: kuscia.secretflow/v1alpha1
kind: DomainRoute
metadata:
  name: alice-bob
  namespace: bob
spec:
  authenticationType: MTLS
  destination: bob
  endpoint:
    host: 172.2.0.2
    ports:
    - name: http
      port: 1080
      protocol: HTTP
      isTLS: true
  mTLSConfig: {}
  requestHeadersToAdd:
    Authorization: Bearer 781293.db7bc3a58fc5f07f
  source: alice
```

在示例中：

* `.metadata.name`：表示路由规则的名称。
* `.metadata.namespace`：表示路由规则所在的命名空间，这里是目标节点的 Namespace。
* `.spec.authenticationType`：表示源节点到目标节点的身份认证方式，目前仅支持 Token、MTLS 和 None（表示不校验）。
* `.spec.source`：表示源节点的 Namespace，这里即 Alice 的 Namespace。
* `.spec.destination`：表示目标节点的 Namespace，这里即 Bob 的 Namespace。
* `.spec.requestHeadersToAdd`：表示 bob 侧的 Envoy 转发源节点请求时添加的 headers，示例中 key 为 Authorization 的 header 是 Bob 为
  Alice 分配访问 K3s 的令牌，

这个 header 仅在目标节点为调度方时有必要配置。

第二步，在源节点 Alice 的 Namespace 下创建创建 Alice 访问 Bob 的 DomainRoute：

```yaml
apiVersion: kuscia.secretflow/v1alpha1
kind: DomainRoute
metadata:
  name: alice-bob
  namespace: alice
spec:
  authenticationType: MTLS
  source: alice
  destination: bob
  endpoint:
    host: 172.2.0.2
    ports:
    - name: http
      port: 1080
      protocol: HTTP
      isTLS: true
  mTLSConfig:
    sourceClientCert: MIICqjCCAZICFEiujM
    sourceClientPrivateKey: MIIEpAIBAAKCAQEAxK
    tlsCA: MIIEpAIBAAKCAQEAxK
```

在示例中：

* `.metadata.name`：表示路由规则的名称。
* `.metadata.namespace`：表示路由规则所在的命名空间，这里是源节点的 Namespace。
* `.spec.authenticationType`：表示源节点到目标节点的身份认证方式，目前仅支持 Token、MTLS 和 None（表示不校验）。
* `.spec.source`：表示源节点的 Namespace，这里即 alice 的 Namespace。
* `.spec.destination`：表示目标节点的 Namespace，这里即 bob 的 Namespace。
* `.spec.endpoint`：表示目标节点表示目标节点的地址。
* `.spec.mTLSConfig`：表示源节点作为客户端访问目标节点的 MTLS 配置，具体字段如下：
  * `sourceClientCert`：表示源节点的客户端证书，value 值是经 BASE64 编码过的。
  * `sourceClientPrivateKey`：表示源节点的私钥，value 值是经 BASE64 编码过的。
  * `tlsCA`：表示校验服务端（目标节点）证书的 CA，value 值是经过 BASE64 编码过的。不配置则表示不校验服务端证书。

您可以通过 kubectl 命令来创建、修改、查看、删除 DomainRoute。

## 参考

下面以 DomainRoute 和 ClusterDomainRoute 的模版配置为例，详细介绍两个 CRD 的字段。

### DomainRoute-template

* Lite 节点访问 Master、Autonomy 节点之间通信的 DomainRoute 需要用户配置。
* Lite 节点之间通信的 DomainRoute 是由 DomainRouteController 根据 ClusterDomainRoute 在各方 Lite 节点的 Namespace 中创建的。

```yaml
apiVersion: kuscia.secretflow/v1alpha1
kind: DomainRoute
metadata:
  name: alice-bob
  namespace: alice
spec:
  authenticationType: Token
  source: alice
  destination: bob
  endpoint:
    host: 172.2.0.2
    ports:
      - name: http
        port: 1080
        protocol: HTTP
        isTLS: false
  mTLSConfig:
    sourceClientCert: BASE64<Cert>
    sourceClientPrivateKey: BASE64<PriKey>
    tlsCA: BASE64<CA>
  tokenConfig:
    rollingUpdatePeriod: 0
    destinationPublicKey: BASE64<publicKey>
    sourcePublicKey: BASE64<publicKey>
    tokenGenMethod: RSA-GEN
  transit:
    domain:
      domainID: joke
    transitMethod: THIRD-DOMAIN
  bodyEncryption:
    algorithm: AES
  requestHeadersToAdd:
    Authorization: Bearer 781293.db7bc3a58fc5f07f
status:
  tokenStatus:
    revisionInitializer: 4c07d28fe469
    revisionToken:
      revision: 1
      revisionTime: "2023-03-30T12:02:32Z"
      token: BASE64<token>
    tokens:
    - effectiveInstances:
      - 4c07d28fe469
      revision: 1
      revisionTime: "2023-03-30T12:02:32Z"
      token: BASE64<token>
```

DomainRoute `metadata` 的子字段详细介绍如下：

* `name`：表示 DomainRoute 的名称。
* `namespace`: 表示 DomainRoute 所属的命名空间，节点之间的通信需要在双方 Namespace 下分别设置 DomainRoute。

DomainRoute `spec` 的子字段详细介绍如下：

* `authenticationType`：表示鉴权类型，支持`Token`、`MTLS`、`None`，其中 `Token` 类型仅支持在中心化组网模式下使用。
* `source`：表示源节点的 Namespace。
* `destination`：表示目标节点的 Namespace。
* `interConnProtocol`：表示节点之间的互联互通协议，目前支持 `kuscia` 或 `bfia` 协议，默认为 `kuscia` 协议 。
* `endpoint`：表示目标节点的访问地址。
  * `host`：表示目标节点的访问域名或 IP。
  * `ports`：表示目标节点的访问端口。
    * `name`：表示端口名称。
    * `port`：表示端口号。
    * `protocol`：表示端口协议，支持`HTTP`或`GRPC`。
    * `isTLS`：表示是否开启`HTTPS`或`GRPCS`。
* `mTLSConfig`：表示 MTLS 配置，authenticationType 为`MTLS`时，源节点需配置 mTLSConfig。该配置项在目标节点不生效。
  * `sourceClientCert`：表示 BASE64 编码格式的源节点的客户端证书。
  * `sourceClientPrivateKey`：表示 BASE64 编码格式的源节点的客户端私钥。
  * `tlsCA`：表示 BASE64 编码格式的目标节点的服务端 CA，为空则表示不校验服务端证书。
* `tokenConfig`：表示 Token 配置，authenticationType 为`Token`或 bodyEncryption 非空时，源节点需配置 TokenConfig。该配置项在目标节点不生效。
  * `rollingUpdatePeriod`：表示 Token 轮转周期，默认值为 0。
  * `destinationPublicKey`：表示目标节点的公钥，该字段由 DomainRouteController 根据目标节点的 Cert 设置，无需用户填充。
  * `sourcePublicKey`：表示源节点的公钥，该字段由 DomainRouteController 根据源节点的 Cert 设置，无需用户填充。
  * `tokenGenMethod`：表示 Token 生成算法，使用`RSA-GEN`，表示双方各生成一半，拼成一个32长度的通信 Token，并且用对方的公钥加密，双方都会用自己的私钥验证 Token 有效性。
* `transit`：表示配置中转路由，如 alice-bob 的通信链路是 alice-joke-bob（alice-joke 必须为直连）。若该配置不为空，endpoint 配置项将不生效。
  * `domain`：表示中转节点的信息。
  * `domainID`：表示中转节点的 ID。
* `bodyEncryption`：表示 Body 加密配置项，通常在配置转发路由时开启 bodyEncryption。
  * `algorithm`：表示加密算法，当前仅支持 AES 加密算法。
* `requestHeadersToAdd`：表示 Envoy 在向集群内转发来自源节点的请求时，添加的 headers，该配置仅在目标节点生效。

DomainRoute `status` 的子字段详细介绍如下：

* `isDestinationUnreachable`：表示 到目标节点是否是不可达的。
* `isDestinationAuthorized`：表示 和目标节点是否已经握手成功。
* `tokenStatus`：表示 Token 认证方式下，源节点和目标节点协商的 Token 的信息。
  * `revisionInitializer`：表示源节点中发起 Token 协商的实例。
  * `revisionToken`：表示最新版本的 Token。
    * `revision`：表示 Token 的版本。
    * `revisionTime`：表示 Token 时间戳。
    * `token`：表示 BASE64 编码格式的经过节点私钥加密的 Token。
  * `tokens`：表示 Token 数组，若 TokenConfig.rollingUpdatePeriod 不为 0，则数组中最多包含最新两个版本的 Token。
    * `tokens[].effectiveInstances`：表示 `metadata.namespace` 标识的节点下，路由规则生效的实例。
    * `tokens[].revision`：表示 Token 的版本。
    * `tokens[].revisionTime`：表示 Token 时间戳。
    * `tokens[].token`：表示 BASE64 编码格式的经过节点公钥加密的 Token。
    * `tokens[].isReady`：表示 Token 是否生效。
    * `tokens[].expirationTime`：表示 Token 何时过期。

### ClusterDomainRoute-template

* Lite 节点之间通信需要用户配置 ClusterDomainRoute。

```yaml
apiVersion: kuscia.secretflow/v1alpha1
kind: ClusterDomainRoute
metadata:
  name: alice-bob
spec:
  authenticationType: Token
  source: alice
  destination: bob
  endpoint:
    host: 172.2.0.2
    ports:
      - name: http
        port: 1080
        protocol: HTTP
        isTLS: false
  mTLSConfig:
    sourceClientCert:  BASE64<Cert>
    sourceClientPrivateKey: BASE64<PriKey>
    tlsCA:  BASE64<CA>
  tokenConfig:
    rollingUpdatePeriod: 0
    destinationPublicKey: BASE64<publicKey>
    sourcePublicKey: BASE64<publicKey>
    tokenGenMethod: RSA-GEN
  transit:
    domain:
      domainID: joke
  bodyEncryption:
    algorithm: AES
  requestHeadersToAdd:
    Authorization: Bearer 781293.db7bc3a58fc5f07f
status:
  conditions:
    - lastTransitionTime: "2023-03-30T12:02:33Z"
      lastUpdateTime: "2023-03-30T12:02:33Z"
      message: clusterdomainroute finish rolling revision 1
      reason: PostRollingUpdate
      status: "False"
      type: Running
    - lastTransitionTime: "2023-03-30T12:02:33Z"
      lastUpdateTime: "2023-03-30T12:02:33Z"
      message: clusterdomainroute finish rolling revision 1
      reason: PostRollingUpdate
      status: "True"
      type: Ready
    - lastTransitionTime: "2023-03-30T12:02:33Z"
      lastUpdateTime: "2023-03-30T12:02:33Z"
      message: clusterdomainroute finish rolling revision 1
      reason: PostRollingUpdate
      status: "True"
      type: Pending
  tokenStatus:
    revision: 1
    revisionTime: "2023-03-30T12:02:32Z"
    sourceTokens:
    - revision: 1
      revisionTime: "2023-03-30T12:02:32Z"
      token: BASE64<token>
    destinationTokens:
    - revision: 1
      revisionTime: "2023-03-30T12:02:32Z"
      token: BASE64<token>
```

ClusterDomainRoute `metadata` 的子字段详细介绍如下：

* `name`：表示 ClusterDomainRoute 的名称。

ClusterDomainRoute `spec` 的子字段详细介绍如下：

* `authenticationType`：表示鉴权类型，支持`Token`、`MTLS`、`None`，其中 `Token` 类型仅支持在中心化组网模式下使用。
* `source`：表示源节点的 Namespace。
* `destination`：表示目标节点的 Namespace。
* `endpoint`：表示目标节点的访问地址。
  * `host`：表示目标节点的访问域名或 IP。
  * `ports`：表示目标节点的访问端口。
    * `name`：表示端口名称。
    * `port`：表示端口号。
    * `protocol`：表示端口协议，支持`HTTP`或`GRPC`。
    * `isTLS`：表示是否开启`HTTPS`或`GRPCS`。
    * `pathPrefix`: 配置非空时，kuscia 会重写请求的 path。例如，pathPrefix 为 /foo，请求 path 为 /bar，发送给对端的请求 path 会被改写为 /foo/bar，对端入口网关需要配置 pathPrefix 卸载规则。配置示例请参考[这里](../../tutorial/kuscia_gateway_with_path.md)。
* `mTLSConfig`：表示 MTLS 配置，authenticationType 为`MTLS`时，源节点需配置 mTLSConfig。该配置项在目标节点不生效。
  * `sourceClientCert`：表示 BASE64 编码格式的源节点的客户端证书。
  * `sourceClientPrivateKey`：表示 BASE64 编码格式的源节点的客户端私钥。
  * `tlsCA`：表示 BASE64 编码格式的目标节点的服务端 CA，为空则表示不校验服务端证书。
* `tokenConfig`：表示 Token 配置，authenticationType 为`Token`或 bodyEncryption 非空时，源节点需配置 TokenConfig。该配置项在目标节点不生效。
  * `rollingUpdatePeriod`：表示 Token 轮转周期，默认值为 0。
  * `destinationPublicKey`：表示目标节点的公钥，该字段由 DomainRouteController 根据目标节点的 Cert 设置，无需用户填充。
  * `sourcePublicKey`：表示源节点的公钥，该字段由 DomainRouteController 根据源节点的 Cert 设置，无需用户填充。
  * `tokenGenMethod`：表示 Token 生成算法，使用`RSA-GEN`，表示双方各生成一半，拼成一个32长度的通信 Token，并且用对方的公钥加密，双方都会用自己的私钥验证 Token 有效性。
* `transit`：表示配置路由转发，如 alice-bob 的通信链路是 alice-joke-bob（alice-joke 必须为直连）。若该配置不为空，endpoint 配置项将不生效。具体参考`DomainRoute 进阶`。
  * `transitMethod`: 表示中转方式，目前支持`THIRD-DOMAIN`和`REVERSE-TUNNEL`两种方式，前者表示经第三方节点转发，后者表示反向隧道。
  * `domain`：表示中转节点的信息。
  * `domainID`：表示中转节点的 ID。
* `bodyEncryption`：表示 Body 加密配置项，通常在配置转发路由时开启 bodyEncryption。
  * `algorithm`：表示加密算法，当前仅支持 AES 加密算法。
* `requestHeadersToAdd`：表示 Envoy 在向集群内转发来自源节点的请求时，添加的 headers，该配置仅在目标节点生效。

ClusterDomainRoute `status` 的子字段详细介绍如下：

* `conditions`：表示 ClusterDomainRoute 处于该阶段时所包含的一些状况。
  * `conditions[].type`: 表示状况的名称。
  * `conditions[].status`: 表示该状况是否适用，可能的取值有`True`、`False`或`Unknown`。
  * `conditions[].reason`: 表示该状况的原因。
  * `conditions[].message`: 表示该状况的详细信息。
  * `conditions[].lastUpdateTime`: 表示状况更新的时间。
  * `conditions[].lastTransitionTime`: 表示转换为该状态的时间戳。
* `tokenStatus`：表示 Token 认证方式下，源节点和目标节点协商的 Token 的信息。
  * `revision`：表示 Token 的最新版本。
  * `revisionTime`：表示 Token 时间戳。
  * `sourceTokens`：表示源节点的 Token 数组，若 TokenConfig.rollingUpdatePeriod 不为 0，则数组中最多包含最新两个版本的 Token。
    * `sourceTokens[].revision`：表示 Token 的版本。
    * `sourceTokens[].revisionTime`：表示 Token 时间戳。
    * `sourceTokens[].token`：表示 BASE64 编码格式的经过节点私钥加密的 Token。
  * `destinationTokens`：表示目标节点的 Token 数组，若 TokenConfig.rollingUpdatePeriod 不为 0，则数组中最多包含最新两个版本的 Token。
    * `destinationTokens[].revision`：表示 Token 的版本。
    * `destinationTokens[].revisionTime`：表示 Token 时间戳。
    * `destinationTokens[].token`：表示 BASE64 编码格式的经过节点公钥加密的 Token。

{#domain-route-advance}

## DomainRoute 进阶

进阶部分介绍 DomainRoute 的转发能力。在 Kuscia 架构中，路由转发能力是通过配置 ClusterDomainRoute（CDR）中的 `Transit` 字段来实现的。
这种配置方式为 Kuscia 提供了灵活组网的核心基础，允许用户根据不同需求定制网络通信路径。Kuscia 支持了两种不同的路由转发能力，
分别是「`节点转发`」和「`反向隧道`」，两者的配置有所不同，用于解决不同场景下组网面临的问题。

### 节点转发

节点转发，对应的转发类型`transitMethod`为——`THIRD-DOMAIN`，代表请求流量会经第三方节点转发。

#### 什么时候需要节点转发

节点转发用于解决网络节点无法直连的问题。以生产中常见的中心化部署为例，我们注意到，出于对安全性的高度重视，机构往往选择将 Master 节点和 Lite 节点部署于不同的网络环境。此外，这些 Lite 节点常常仅具备有限的网络访问权限。这一做法虽然强化了安全防护，却同时给不同机构之间的无缝联接和数据交互带来了挑战。
![image.png](../../imgs/transit-cxc-cxp.png)
如上图「中心化-中心化」网络中，Alice 与 Bob 无法直接互联，「中心化—P2P」网络中，Alice 与 Bob 无法直接互联。Kuscia 引入了转发能力，通过复用 Lite 与 Master 之间的通道，使得 Lite 节点可以访问对端机构，实现组网。
> 转发能力是 Kuscia 的通用能力，并不局限于上述情形。

#### 怎样配置节点转发

这里为了简单起见，使用路由配置和 CDR 来代指 ClusterDomainRoute，使用$\longrightarrow$代表路由配置指向：
$Alice\longrightarrow Bob$代表「Alice 到 Bob 的 路由配置」，
$Alice\stackrel{Mater-Alice}\longrightarrow Bob$代表「Alice 到 Bob 经Master-Alice 转发的 路由配置」。

#### 一跳转发

在配置好 $Alice\longrightarrow Bob$ 和 $Bob\longrightarrow Carol$ 路由规则的基础上，如果希望 Alice 能够访问 Carol，需要建立经过 Bob 的中转路由规则 $Alice\stackrel{Bob}\longrightarrow Carol$。**需要注意，如果一条路由转发规则跨不同 Kuscia 集群，一个控制面（Master 或 Autonomy）为一个 Kuscia 集群，不同控制面为不同 Kuscia 集群，需要在每个集群中都应用该规则。** 比如 Alice 与 Carol 分属不同的集群，那么需要在 Alice 所属的集群和 Carol 所属的集群中分别创建 CDR( `kubectl 创建` 或 [调用KusciaAPI创建](../apis/domainroute_cn.md))。
![image.png](../../imgs/transit-3rd-single.png)

##### $Alice\stackrel{Bob}\longrightarrow Carol$配置

```bash
apiVersion: kuscia.secretflow/v1alpha1
kind: ClusterDomainRoute
metadata:
  name: alice-carol
spec:
  authenticationType: Token
  source: alice
  destination: carol
  endpoint:
    host: 172.2.0.2
    ports:
      - name: http
        port: 1080
        protocol: HTTP
        isTLS: false
  transit:
    transitMethod: THIRD-DOMAIN
    domain:
      domainID: bob
  tokenConfig:
    tokenGenMethod: RSA-GEN
    rollingUpdatePeriod: 86400
```

#### 多跳转发

Kuscia 通过多条连续的「一跳转发」路由规则，实现「多跳转发」能力。
在配置好 $Alice\longrightarrow Bob$ 、 $Bob\longrightarrow Carol$ 和 $Carol\longrightarrow Joke$路由规则的基础上，希望实现 Alice 访问 Joke，需要建立两条中转路由规则，分别是 $Bob\stackrel{Carol}\longrightarrow Joke$和 $Alice\stackrel{Bob}\longrightarrow Joke$。
![image.png](../../imgs/transit-3rd-multi.png)

##### $Bob\stackrel{Carol}\longrightarrow Joke$配置

```bash
apiVersion: kuscia.secretflow/v1alpha1
kind: ClusterDomainRoute
metadata:
  name: bob-joke
spec:
  authenticationType: Token
  source: bob
  destination: joke
  endpoint:
    host: 172.2.0.4
    ports:
      - name: http
        port: 1080
        protocol: HTTP
        isTLS: false
  transit:
    transitMethod: THIRD-DOMAIN
    domain:
      domainID: carol
  tokenConfig:
    tokenGenMethod: RSA-GEN
    rollingUpdatePeriod: 86400
```

##### $Alice\stackrel{Bob}\longrightarrow Joke$ 配置

```bash
apiVersion: kuscia.secretflow/v1alpha1
kind: ClusterDomainRoute
metadata:
  name: alice-joke
spec:
  authenticationType: Token
  source: alice
  destination: joke
  endpoint:
    host: 172.2.0.4
    ports:
      - name: http
        port: 1080
        protocol: HTTP
        isTLS: false
  transit:
    transitMethod: THIRD-DOMAIN
    domain:
      domainID: bob
  tokenConfig:
    tokenGenMethod: RSA-GEN
    rollingUpdatePeriod: 86400
```

#### 转发安全

请求在传输过程中将经由第三方节点，这引发了中间人攻击的潜在风险。若您对于这些中间节点持有疑虑，您可以考虑启用安全加强措施。在这种模式下，通信双方通过 Kuscia 网关实现数据的加密与解密，使用的是基于AES GCM算法的加密机制。**请注意，这种安全增强可能会对系统性能产生一定影响。**
> authenticationType 必须配置为 Token，才可以启用 AES 加密。转发的端对端加密密钥基于 Token 生成，配置`rollingUpdatePeriod` 后，Token 会进行滚动更新，转发的密钥随之更新版本。

```bash
apiVersion: kuscia.secretflow/v1alpha1
kind: ClusterDomainRoute
metadata:
  name: alice-carol
spec:
  authenticationType: Token
  bodyEncryption:
    algorithm: AES
  source: alice
  destination: carol
  endpoint:
    host: 172.2.0.2
    ports:
      - name: http
        port: 1080
        protocol: HTTP
        isTLS: false
  transit:
    transitMethod: THIRD-DOMAIN
    domain:
      domainID: bob
  tokenConfig:
    tokenGenMethod: RSA-GEN
    rollingUpdatePeriod: 86400
```

### 反向隧道

反向隧道，对应的转发类型为——`REVERSE-TUNNEL`，代表请求流量会经反向隧道转发。

#### 什么时候需要反向隧道

反向隧道用于解决一类特殊的场景。出于安全性考虑，参与隐私计算的一方机构，能够对外访问公网发起请求，但是不愿意对外直接暴露和监听端口。
如下图，机构 B 侧的算法容器 Bob 可以对机构 A 侧的算法容器 Alice 发起连接，反过来则不行。
![image.png](../../imgs/transit-reverse-block.png)

#### 反向隧道是怎么实现的

为了让这种场景下，隐私计算任务可以正常开展，Kuscia 拓展了网关的能力，实现了一种反向隧道，来承接机构 A 对机构 B 发起的请求。
![image.png](../../imgs/transit-reverse-tunnel.png)
Alice 向 Bob 发起请求的环节如下：

1. 隧道建立，对应上图中的 ⓪——用户向 Kuscia 注册一条类型为 `REVERSE-TUNNEL`、起点为 Alice、终点为 Bob 的路由配置。Bob 侧 Kuscia Gateway 监听到这条配置后，会通过网关向 Alice 发起一次特殊的 HTTP 请求，这条请求到达 Alice 网关后，网关会记录这条请求的信息，回复`200状态码`和`Transfer-Encoding: Chunked`头，维持这条连接。这条连接之后会充当`反向隧道`。
2. 请求发送，对应上图中的 ①——Alice 向 Bob 发起的「request」到达 Alice 侧网关后，网关会将「request」包装成特殊格式的 「chunked response」，通过`反向隧道`发送给 Bob 侧网关，同时记录请求信息，对应 Ⓐ。请求到达 Bob 侧网关后，网关会将「chunked response」重新包装成「request」，发送给 Bob，对应 Ⓑ。
3. 响应返回，对应上图中的 ②——Bob 完成请求处理后，返回「response」，到达 Bob 侧网关后，网关会将「response」包装成「request」，发送给 Alice，对应 Ⓒ。请求到达 Alice 侧网关后，网关会从「request」取出数据，包装成「response」，找到记录的请求信息，返回给 Alice，对应 Ⓓ。

#### 怎样配置反向隧道

要配置一条如上图所示的反向隧道路由规则，可以如下所示，设置`Transit`的`transitMethod`字段。**请注意，反向隧道可能会对系统性能产生一定影响。**

```bash
apiVersion: kuscia.secretflow/v1alpha1
kind: ClusterDomainRoute
metadata:
  name: alice-bob
spec:
  authenticationType: Token
  source: alice
  destination: bob
  endpoint:
    host: 172.2.0.2
    ports:
      - name: http
        port: 1080
        protocol: HTTP
        isTLS: false
  transit:
    transitMethod: REVERSE-TUNNEL
  tokenConfig:
    tokenGenMethod: RSA-GEN
    rollingUpdatePeriod: 86400
```
