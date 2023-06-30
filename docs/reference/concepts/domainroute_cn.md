# DomainRoute

DomainRoute 用于在中心化网络中配置 Lite 节点与 Master 之间的路由规则、Lite 节点之间的路由规则，以及点对点（P2P）网络中 Autonomy 节点之间的路由规则。
ClusterDomainRoute 用于在中心化网络中配置 Lite 节点之间的路由规则，DomainRouteController 会根据 ClusterDomainRoute 在各方 Lite 节点的 Namespace 中创建 DomainRoute。具体用例请参考下文。

## 用例

以下是一些 DomainRoute 和 ClusterDomainRoute 的典型用例
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
* `.spec.authenticationType`：表示节点到 Master 的身份认证方式，目前仅支持 MTLS 和 None（表示不校验）。
* `.spec.source`：表示源节点的 Namespace，这里即 Lite 节点的 Namespace。
* `.spec.destination`：表示目标节点的 Namespace，这里即 Master 的命名空间。
* `.spec.requestHeadersToAdd`：表示 Master 侧的 Envoy 在转发源节点的请求时添加的 headers，示例中 key 为 
  Authorization 的 header 是 Master 为 alice 分配访问 k3s 的令牌。

你可以通过 kubectl 命令来创建、修改、查看、删除 DomainRoute。

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
  * `ports`：表示目标节点的端口，以及该端口的协议，支持 HTTP 端口和 GRPC 端口。
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

你还可以通过 kubectl 命令来修改、删除 ClusterDomainRoute。

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
* `.spec.authenticationType`：表示源节点到目标节点的身份认证方式，目前仅支持 MTLS 和 None（表示不校验）。
* `.spec.source`：表示源节点的 Namespace，这里即 alice 的 Namespace。
* `.spec.destination`：表示目标节点的 Namespace，这里即 bob 的 Namespace。
* `.spec.requestHeadersToAdd`：表示 bob 侧的 Envoy 转发源节点请求时添加的 headers，示例中 key 为 Authorization 的 header 是 bob 为 
  alice 分配访问 K3s 的令牌，
这个 header 仅在目标节点为调度方时有必要配置。

第二步，在源节点 alice 的 Namespace 下创建创建 alice 访问 bob 的 DomainRoute：
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
* `.spec.authenticationType`：表示源节点到目标节点的身份认证方式，目前仅支持 MTLS 和 None（表示不校验）。
* `.spec.source`：表示源节点的 Namespace，这里即 alice 的 Namespace。
* `.spec.destination`：表示目标节点的 Namespace，这里即 bob 的 Namespace。
* `.spec.endpoint`：表示目标节点表示目标节点的地址。
* `.spec.mTLSConfig`：表示源节点作为客户端访问目标节点的 MTLS 配置，具体字段如下：
  * `sourceClientCert`：表示源节点的客户端证书，value 值是经 BASE64 编码过的。
  * `sourceClientPrivateKey`：表示源节点的私钥，value 值是经 BASE64 编码过的。
  * `tlsCA`：表示校验服务端（目标节点）证书的 CA，value 值是经过 BASE64 编码过的。不配置则表示不校验服务端证书。

你可以通过 kubectl 命令来创建、修改、查看、删除 DomainRoute。

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
* `endpoint`：表示目标节点的访问地址。
  * `host`：表示目标节点的访问域名或 IP。
  * `ports`：表示目标节点的访问端口。
    * `name`：表示端口名称。
    * `port`：表示端口号。
    * `protocol`：表示端口协议，支持`HTTP`或`GRPC`。
    * `isTLS`：表示是否开启`HTTPS`或`GRPCS`。
* `mTLSConfig`：表示 MTLS 配置，authenticationTyp 为`MTLS`时，源节点需配置 mTLSConfig。该配置项在目标节点不生效。
  * `sourceClientCert`：表示 BASE64 编码格式的源节点的客户端证书。
  * `sourceClientPrivateKey`：表示 BASE64 编码格式的源节点的客户端私钥。
  * `tlsCA`：表示 BASE64 编码格式的目标节点的服务端 CA，为空则表示不校验服务端证书。
* `tokenConfig`：表示 Token 配置，authenticationTyp 为`Token`或 bodyEncryption 非空时，源节点需配置 TokenConfig。该配置项在目标节点不生效。
  * `rollingUpdatePeriod`：表示 Token 轮转周期，默认值为 0。
  * `destinationPublicKey`：表示目标节点的公钥，该字段由 DomainRouteController 根据目标节点的 Cert 设置，无需用户填充。
  * `sourcePublicKey`：表示源节点的公钥，该字段由 DomainRouteController 根据源节点的 Cert 设置，无需用户填充。
  * `tokenGenMethod`：表示 Token 生成算法，支持`RSA-GEN`和`RAND-GEN`。
* `transit`：表示配置中转路由，如 alice-bob 的通信链路是 alice-joke-bob（alice-joke 必须为直连）。若该配置不为空，endpoint 配置项将不生效。
  * `domain`：表示中转节点的信息。
  * `domainID`：表示中转节点的 ID。
* `bodyEncryption`：表示 Body 加密配置项，通常在配置转发路由时开启 bodyEncryption。
  * `algorithm`：表示加密算法，当前仅支持 AES 加密算法。
* `requestHeadersToAdd`：表示 Envoy 在向集群内转发来自源节点的请求时，添加的 headers，该配置仅在目标节点生效。

DomainRoute `status` 的子字段详细介绍如下：

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
* `mTLSConfig`：表示 MTLS 配置，authenticationTyp 为`MTLS`时，源节点需配置 mTLSConfig。该配置项在目标节点不生效。
  * `sourceClientCert`：表示 BASE64 编码格式的源节点的客户端证书。
  * `sourceClientPrivateKey`：表示 BASE64 编码格式的源节点的客户端私钥。
  * `tlsCA`：表示 BASE64 编码格式的目标节点的服务端 CA，为空则表示不校验服务端证书。
* `tokenConfig`：表示 Token 配置，authenticationTyp 为`Token`或 bodyEncryption 非空时，源节点需配置 TokenConfig。该配置项在目标节点不生效。
  * `rollingUpdatePeriod`：表示 Token 轮转周期，默认值为 0。
  * `destinationPublicKey`：表示目标节点的公钥，该字段由 DomainRouteController 根据目标节点的 Cert 设置，无需用户填充。
  * `sourcePublicKey`：表示源节点的公钥，该字段由 DomainRouteController 根据源节点的 Cert 设置，无需用户填充。
  * `tokenGenMethod`：表示 Token 生成算法，支持`RSA-GEN`和`RAND-GEN`。
* `transit`：表示配置中转路由，如 alice-bob 的通信链路是 alice-joke-bob（alice-joke 必须为直连）。若该配置不为空，endpoint 配置项将不生效。
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