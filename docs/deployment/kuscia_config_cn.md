# Kuscia 配置文件

Kuscia 配置文件默认位置为 Kuscia 容器的 /home/kuscia/etc/conf/kuscia.yaml, 不同的部署模式所需的配置内容不同。
Kuscia 的配置文件由公共配置和每个模式的特殊配置组成，具体细节可以参考下文。Kuscia 的自动部署脚本会产生一份默认的配置文件，如果需要调整，可以将调整后的配置文件挂载到容器相应位置。

## Kuscia 配置

### 配置项示例

```yaml
######################################################################################
############                     Public Configuration                     ############
######################################################################################
# Deployment mode
mode: lite
# DomianID
# When using in a production environment, it is recommended to set domainID to be globally unique. It is recommended to use: "company name"-"department name"-"node name", such as:
# domainID: mycompany-secretflow-trainlite
domainID: alice
# Node private key configuration, used for communication authentication between nodes and certificate issuance for node applications.
# Run the command "docker run -it --rm secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia scripts/deploy/generate_rsa_key.sh" to generate the private key.
domainKeyData: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNRDhDQVFBQ0NRREdsY1Y3MTd5V3l3SURBUUFCQWdrQXR5RGVueG0wUGVFQ0JRRHJVTGUvQWdVQTJBcUQ5UUlFCmFuYkxtd0lFZWFaYUxRSUZBSjZ1S2tjPQotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQo
# Communication protocol used by KusciaAPI and the node's external gateway, NOTLS/TLS/MTLS
protocol: NOTLS
# Log level INFO, DEBUG, WARN
logLevel: INFO
# Metric collection period, unit: seconds
metricUpdatePeriod: 5
# General log rotation configuration, including kuscia logs, application logs (such as secretflow, dataproxy, etc.)
logrotate:
  # The maximum number of files to keep for logs output by a single module (e.g., kuscia, envoy are different modules), default is 5
  maxFiles: 5
  # Single file rotation threshold, default is 512, unit: MB
  maxFileSizeMB: 512
  # The maximum retention time for each application output log file, default is 30, unit: days
  maxAgeDays: 30
########################################################################################
############                       Lite Configuration                       ############
########################################################################################
# When the node first deploys and connects to the Master, the Master uses this Token to verify the node's identity (Token issued by the Master). For security reasons, this Token is invalidated immediately after the node is successfully deployed.
# When deploying on multiple machines, keep this Token unchanged.
# If the node's private key is lost, delete the node's public key on the Master and reapply for a Token for deployment.
liteDeployToken: LS0tLS1CRUdJTi
# Address of the node connecting to the master
masterEndpoint: https://172.18.0.2:1080

####################################################################################
############                Lite, Autonomy Configuration                ############
####################################################################################
# runc or runk or runp
runtime: runc
# Configuration when runtime is runk
runk:
  # Task scheduling to the specified organization's K8s namespace
  namespace: ""
  # pod dns configuration of the organization's K8s cluster, used to resolve the application domain name of the node. The dns address used by runk to pull up the pod should be configured as the clusterIP of the kuscia service.
  dnsServers:
  # kubeconfig of the organization's K8s cluster, default is serviceaccount if not filled; currently, do not fill, use serviceaccount by default
  kubeconfigFile:
  # Whether to enable kuscia pod log recording, default is false (disabled). When enabled, pods/log permissions need to be enabled in rbac.yaml (example: https://github.com/secretflow/kuscia/blob/main/hack/k8s/autonomy/rbac.yaml)
  enableLogging:

# The capacity that the node can use for scheduling applications. runc/runp automatically obtains the current container's system resources if not filled, manual configuration is required in runk mode
capacity:
  cpu: #4
  memory: #8Gi
  pods: #500
  storage: #100Gi
  ephemeralStorage: #100Gi

# agent image configuration
image:
  pullPolicy: #Whether to allow pulling remote images (remote) | only use locally imported images (local)
  defaultRegistry: ""
  # Proxy address for pulling images, such as: http://127.0.0.1:8080 | do not use proxy if not filled
  httpProxy: ""
  registries:
    - name: ""
      endpoint: ""
      username: ""
      password: ""

##################################################################################
############              Autonomy, Master Configuration              ############
##################################################################################
# Database connection string, default is SQLite if not filled
# Example: mysql://username:password@tcp(hostname:3306)/database-name
datastoreEndpoint: ""
# Workload approval configuration, note: this configuration only takes effect in P2P networking. KusciaJob execution does not require approval in centralized networking.
# By default, workload approval configuration is disabled. If approval configuration is enabled, all Jobs need to call KusciaAPI for job approval when the local party is a participant. Production environment is recommended to enable approval.
enableWorkloadApprove: false
```

{#configuration-detail}

### 配置项详解

- `mode`: 当前 Kuscia 节点部署模式 支持 Lite、Master、Autonomy（不区分大小写）, 不同部署模式详情请参考[这里](../reference/architecture_cn)
- `domainID`: 当前 Kuscia 实例的 [节点 ID](../reference/concepts/domain_cn)， 需要符合 RFC 1123 标签名规则要求，详情请参考[这里](https://kubernetes.io/zh-cn/docs/concepts/overview/working-with-objects/names/#dns-label-names)。 `default`、`kube-system` 、`kube-public` 、`kube-node-lease` 、`master` 以及 `cross-domain` 为 Kuscia 预定义的节点 ID，不能被使用。生产环境使用时建议将 domainID 设置为全局唯一，建议使用：公司名称-部门名称-节点名称，如: domainID: mycompany-secretflow-trainlite
- `domainKeyData`: 节点私钥配置, 用于节点间的通信认证（通过 2 方的证书来生成通讯的身份令牌），节点应用的证书签发（为了加强通讯安全性，Kuscia 会给每一个任务引擎分配 MTLS 证书，不论引擎访问其他模块（包括外部），还是其他模块访问引擎，都走 MTLS 通讯，以免内部攻破引擎。）。可以通过命令 `docker run -it --rm secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia scripts/deploy/generate_rsa_key.sh` 生成
- `logLevel`: 日志级别 INFO、DEBUG、WARN，默认 INFO
- `liteDeployToken`: 节点首次连接到 Master 时使用的是由 Master 颁发的一次性 Token 进行身份验证[获取 Token](./Docker_deployment_kuscia/deploy_master_lite_cn.md#lite-alice)，该 Token 在节点成功部署后立即失效。在多机部署中，请保持该 Token 不变即可；若节点私钥遗失，必须在 Master 上删除相应节点的公钥并重新获取 Token 部署。详情请参考[私钥丢失如何重新部署](../troubleshoot/deployment/private_key_loss.md)
- `masterEndpoint`: 节点连接 Master 的地址，比如 <https://172.18.0.2:1080>
- `runtime`: 节点运行时 runc、runk、runp，运行时详解请参考[这里](../reference/architecture_cn.md#agent)
- `runk`: 当 runtime 为 runk 时配置
  - `namespace`: 任务调度到指定的机构 K8s Namespace 下
  - `dnsServers`: 机构 K8s 集群的 Pod DNS 配置， 用于解析节点的应用域名
  - `kubeconfigFile`: 机构 K8s 集群的 Kubeconfig，不填默认 serviceaccount；当前请不填，默认使用 serviceaccount
- `capacity`: 节点可用于调度应用的容量，runc/runp 不填会自动获取当前容器的系统资源, runk 模式下需要手动配置
  - `cpu`: cpu 核数， 如 4
  - `memory`: 内存大小，如 8Gi
  - `pods`: pods 数，如 500
  - `storage`: 磁盘持久化存储容量，即使 Pod 被删除，数据依然保存。如 100Gi
  - `ephemeralStorage`: 磁盘临时存储，非持久化的存储资源。与 Pod 生命周期绑定的存储，当 Pod 被删除时，这部分存储上的数据也会被清除。如 100Gi
- `image`: 节点镜像配置, 目前仅支持配置 1 个镜像仓库（更多请参考：[自定义镜像仓库](../tutorial/custom_registry.md)）
  - `pullPolicy`: [暂不支持] 镜像策略，使用本地镜像仓库还是远程镜像仓库；可选值有 remote/local，不区分大小写，默认为 local；当为 remote 时，如果发现本地镜像不存在，会根据 registry 账密自动拉取远程的镜像；如果为 local 时，镜像需要手动导入 kuscia 内，如果镜像没有导入 kuscia，任务会启动失败。local 模式因为不拉取远程镜像，安全性会更高，但会有易用性的损失，用户可结合业务场景自行选择。
  - `defaultRegistry`: 默认镜像仓库(对应 registries 中其中一个 registry 的 name 字段)
  - `httpProxy`: 拉取镜像的代理地址，示例：http://127.0.0.1:8080。不填则不使用代理
  - `registries`: 镜像仓库配置。
    - `name`: 镜像仓库名
    - `endpoint`: 镜像仓库地址
    - `username`: 镜像仓库用户名（公开仓库可不填）
    - `password`: 镜像仓库密码（公开仓库可不填）
- `datastoreEndpoint`: 数据库连接串，不填默认使用 SQLite。示例：`mysql://username:password@tcp(hostname:3306)/database-name` 使用 MySQL 数据库存储需要符合以下规范：
  - 提前创建好 Database。
  - 创建 kine 表，建表语句参考 [kine](https://github.com/secretflow/kuscia/blob/main/hack/k8s/kine.sql)。
    - 手动建表：如果机构建表是被管控的，或者提供的数据库账号没有建表权限，可以提前手动建立好数据表，kuscia 识别到数据表存在后，会自动跳过建表。
    - 自动建表：如果提供的数据库账号有建表权限（账号具有 `DDL+DML` 权限），并且数据表不存在，kuscia 会尝试自动建表，如果创建失败 kuscia 会启动失败。
  - 数据库账户对表中字段至少具有 select、insert、update、delete 操作权限。
- `protocol`: KusciaAPI 以及节点对外网关使用的通信协议，有三种通信协议可供选择：NOTLS/TLS/MTLS（不区分大小写）。
  - `NOTLS`: 此模式下，通信并未采用 TLS 协议进行加密，即数据通过未加密的 HTTP 传输。在高度信任且严格管控的内部网络环境，或是已具备外部安全网关防护措施的情况下，可以使用该模式，但在一般情况下，由于存在安全隐患，不推荐使用。
  - `TLS`: 通过 TLS 协议进行加密，即使用 HTTPS 进行安全传输，不需要手动配置证书。
  - `MTLS`: 使用 HTTPS 进行通信，支持双向 TLS 验证，需要手动交换证书以建立安全连接。
- `enableWorkloadApprove`: 是否开启工作负载审批，默认为 false，即关闭审批。取值范围:[true, false]。注：仅 P2P 组网时此配置才生效，中心化组网时执行 KusciaJob 无需审批。
- `logrotate`: 日志轮转设置。为了避免 kuscia、应用等运行产生的日志占用过多的磁盘，而引入了日志轮转功能。您可以根据自己的需要，调整默认配置。在日志轮转时将会根据本地时间进行重命名，超过 2 个文件之后，会进行日志文件压缩。该配置项不是必需项，在没有配置的情况下，仍然以同样的默认值进行轮转工作。注意，应用日志（如 secretflow）和非应用日志（如 kuscia）轮转逻辑略有区别。
  - `maxFiles`: 对于一种日志文件，最多保留的文件数量。该值建议大于 1。对非应用日志，该值为 0 时，视为无数量限制。对应用日志，该值小于等于 1 时，仍会以默认值 5 进行工作。
  - `maxFileSizeMB`: 单个日志文件的轮转阈值，当一次轮转检查发生时，如果文件大小大于该值，将会进行轮转。该值应大于 0。
  - `maxAgeDays`: 日志文件的最大保留天数。对非应用日志，直接删除超保留期限的日志文件。对应用日志，如果日志文件均超过该天数，且对应 Pod 处于结束状态。该 Pod 对应日志文件及其目录将会被删除。该值应大于 0。

{#configuration-example}

### 配置示例

- [Lite 节点配置示例](https://github.com/secretflow/kuscia/tree/main/scripts/templates/kuscia-lite.yaml)
- [Master 节点配置示例](https://github.com/secretflow/kuscia/tree/main/scripts/templates/kuscia-master.yaml)
- [Autonomy 节点配置示例](https://github.com/secretflow/kuscia/tree/main/scripts/templates/kuscia-autonomy.yaml)

### 快速生成配置文件

Kuscia 为您提供了快速生成 kuscia.yaml 文件的小工具，参数及示例如下：

- `-e, --datastore-endpoint <string>`
  - 描述：指定用于连接数据存储的数据库数据源名称（DSN）连接字符串。
  - 使用示例：`--datastore-endpoint "mysql://username:password@tcp(hostname:3306)/database-name"`

- `-d, --domain <string>`
  - 描述：设定必须遵守 DNS 子域命名规则的 Domain ID，详情请参考[这里](https://kubernetes.io/zh-cn/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names)。
  - 使用示例：`--domain "alice"`

- `-f, --domain-key-file <string>`
  - 描述：指定域的 RSA 私钥文件的路径。如果未提供，将生成新的域 RSA 密钥。
  - 使用示例：`--domain-key-file "/path/to/domain/key.pem"`

- `--enable-workload-approve`
  - 描述：设置后可自动批准工作负载的配置，只在 Master 以及 Autonomy 模式下才会生成该配置属性，默认为 false，使用该参数即表示该值为 true。
  - 使用示例：`--enable-workload-approve`

- `-t, --lite-deploy-token <string>`
  - 描述：用于验证连接到主服务器时由 Lite 客户端使用的部署令牌。
  - 使用示例：`--lite-deploy-token "abcdefg"`

- `-l, --log-level <string>`
  - 描述：设置日志记录级别。可接受的值有 INFO、DEBUG 和 WARN，默认为 INFO。
  - 使用示例：`--log-level "DEBUG"`

- `-m, --master-endpoint <string>`
  - 描述：指定 Lite 客户端应连接的主服务器端点。
  - 使用示例：`--master-endpoint "https://1.1.1.1:18080"`

- `--mode <string>`
  - 描述：设置域的部署模式。有效选项为 Master、Lite 和 Autonomy（不区分大小写）。
  - 使用示例：`--mode "Lite"`

- `-p, --protocol <string>`
  - 描述：指定用于 KusciaAPI 和网关的协议。选项包括 NOTLS、TLS 和 MTLS，不指定时默认为 MTLS。
  - 使用示例：`--protocol "TLS"`

- `-r, --runtime <string>`
  - 描述：定义要使用的域运行时。有效选项为 runc、runk 和 runp，默认为 runc。
  - 使用示例：`--runtime "runc"`

Kuscia init 使用示例如下：

```bash
# Specifies the Kuscia image version to use, here using the latest version
export KUSCIA_IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia

# It is recommended to check the generated file in advance after the command is executed to avoid deployment startup problems caused by configuration file errors
docker run -it --rm ${KUSCIA_IMAGE} kuscia init --mode lite --domain "alice" --master-endpoint "https://1.1.1.1:18080" --lite-deploy-token "abcdefg" > lite_alice.yaml 2>&1 || cat lite_alice.yaml
```

## 修改默认配置文件

如果使用 [kuscia.sh](https://github.com/secretflow/kuscia/blob/main/scripts/deploy/kuscia.sh) 脚本部署的 Kuscia，kuscia.yaml 文件路径默认是在以下位置（其他部署模式可以借鉴）。

- 宿主机路径：
  - master：{PWD}/{USER}-kuscia-master/kuscia.yaml
  - lite：{PWD}/{USER}-kuscia-lite-domainID/kuscia.yaml
  - autonomy：{PWD}/{USER}-kuscia-autonomy-domainID/kuscia.yaml
- 容器内路径：/home/kuscia/etc/conf/kuscia.yaml

宿主机路径下修改 kuscia.yaml 配置后，重启容器 `docker restart ${container_name}` 生效。
> Tips：如果要修改 Protocol 字段，请确保对该字段有充足的理解，否则会导致 KusciaAPI 调用失败或者和其他节点的通讯异常。详情参考 [Protocol 通信协议](../troubleshoot/concept/protocol_describe.md)。

## 指定配置文件

如果使用 [kuscia.sh](https://github.com/secretflow/kuscia/blob/main/scripts/deploy/kuscia.sh) 脚本部署的 Kuscia，可以指定配置文件，示例：

```bash
# -c The parameter passes the path of the specified Kuscia configuration file.
./kuscia.sh start -c autonomy_alice.yaml -p 11080 -k 11081
```

其中，kuscia-autonomy.yaml 可参考 [配置示例](#configuration-example)
