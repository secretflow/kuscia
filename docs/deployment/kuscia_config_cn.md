# Kuscia 配置文件

Kuscia 配置文件默认位置为 Kuscia 容器的 /home/kuscia/etc/conf/kuscia.yaml, 不同的部署模式所需的配置内容不同。
Kuscia 的配置文件由公共配置和每个模式的特殊配置组成，具体细节可以参考下文。 Kuscia 的自动部署脚本会产生一份默认的配置文件，如果需要调整，可以将调整后的配置文件挂载到容器相应位置。

## Kuscia 配置
### 配置项示例
```yaml
#############################################################################
############                       公共配置                       ############ 
#############################################################################
# 部署模式
mode: Lite
# 节点ID 
# 生产环境使用时建议将domainID设置为全局唯一，建议使用：公司名称-部门名称-节点名称，如：
# domainID: antgroup-secretflow-trainlite
domainID: alice
# 节点私钥配置, 用于节点间的通信认证, 节点应用的证书签发
# 注意: 目前节点私钥仅支持 pkcs#1 格式的: "BEGIN RSA PRIVATE KEY/END RSA PRIVATE KEY"
# 执行命令 "docker run -it --rm secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia scripts/deploy/generate_rsa_key.sh" 生成私钥
domainKeyData: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNRDhDQVFBQ0NRREdsY1Y3MTd5V3l3SURBUUFCQWdrQXR5RGVueG0wUGVFQ0JRRHJVTGUvQWdVQTJBcUQ5UUlFCmFuYkxtd0lFZWFaYUxRSUZBSjZ1S2tjPQotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQo
# KusciaAPI 以及节点对外网关使用的通信协议, NOTLS/TLS/MTLS
protocol: NOTLS
# 日志级别 INFO、DEBUG、WARN
logLevel: INFO
# 指标采集周期，单位: 秒
metricUpdatePeriod: 5
#############################################################################
############                       Lite 配置                      ############ 
#############################################################################
# 当节点首次部署链接 Master 时，Master 通过该 Token 来验证节点的身份（Token 由 Master 颁发)，因为安全原因，该 Token 在节点部署成功后，立即失效
# 多机部署时，请保持该 Token 不变即可
# 如果节点私钥丢失，请在 Master 删除节点公钥，并重新申请 Token 部署
liteDeployToken: LS0tLS1CRUdJTi
# 节点连接 master 的地址
masterEndpoint: https://172.18.0.2:1080

#############################################################################
############               Lite、Autonomy 配置                    ############ 
#############################################################################
# runc or runk or runp
runtime: runc
# 当 runtime 为 runk 时配置
runk:
  # 任务调度到指定的机构 K8s namespace 下
  namespace: ""
  # 机构 K8s 集群的 pod dns 配置，用于解析节点的应用域名，runk 拉起 pod 所使用的 dns 地址，应配置为 kuscia service 的 clusterIP
  dnsServers:
  # 机构 K8s 集群的 kubeconfig, 不填默认 serviceaccount; 当前请不填，默认使用 serviceaccount
  kubeconfigFile:

# 节点可用于调度应用的容量，runc/runp 不填会自动获取当前容器的系统资源, runk 模式下需要手动配置
capacity:
  cpu: #4
  memory: #8Gi
  pods: #500
  storage: #100Gi

# agent 镜像配置
image:
  pullPolicy: #使用镜像仓库|使用本地
  defaultRegistry: ""
  registries:
    - name: ""
      endpoint: ""
      username: ""
      password: ""

#############################################################################
############               Autonomy、Master 配置                  ############ 
#############################################################################
# 数据库连接串，不填默认使用 sqlite
# 示例：mysql://username:password@tcp(hostname:3306)/database-name
datastoreEndpoint: ""
# 工作负载审核配置
# 默认情况下，工作负载审核配置为关闭状态。若开启审核配置，则当本方作为参与方时，所有的 Job 需要调用 KusciaAPI 进行作业审核。生产环境建议开启审核
enableWorkloadApprove: false
```

{#configuration-detail}

### 配置项详解
- `mode`: 当前 Kuscia 节点部署模式 支持 Lite、Master、Autonomy（不区分大小写）, 不同部署模式详情请参考[这里](../reference/architecture_cn)
- `domainID`: 当前 Kuscia 实例的 [节点 ID](../reference/concepts/domain_cn)， 需要符合 DNS 子域名规则要求，详情请参考[这里](https://kubernetes.io/zh-cn/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names), 生产环境使用时建议将 domainID 设置为全局唯一，建议使用：公司名称-部门名称-节点名称，如: domainID: antgroup-secretflow-trainlite
- `domainKeyData`: 节点私钥配置, 用于节点间的通信认证（通过 2 方的证书来生成通讯的身份令牌），节点应用的证书签发（为了加强通讯安全性，Kuscia 会给每一个任务引擎分配 MTLS 证书，不论引擎访问其他模块（包括外部），还是其他模块访问引擎，都走 MTLS 通讯，以免内部攻破引擎。）。可以通过命令 `docker run -it --rm secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia scripts/deploy/generate_rsa_key.sh` 生成
- `logLevel`: 日志级别 INFO、DEBUG、WARN，默认 INFO
- `liteDeployToken`: 节点首次连接到 Master 时使用的是由 Master 颁发的一次性 Token 进行身份验证[获取Token](../deployment/deploy_master_lite_cn.md#lite-alice)，该 Token 在节点成功部署后立即失效。在多机部署中，请保持该 Token 不变即可；若节点私钥遗失，必须在 Master 上删除相应节点的公钥并重新获取 Token 部署。详情请参考[私钥丢失如何重新部署](./../reference/troubleshoot/private_key_loss.md)
- `masterEndpoint`: 节点连接 Master 的地址，比如 https://172.18.0.2:1080
- `runtime`: 节点运行时 runc、runk、runp，运行时详解请参考[这里](../reference/architecture_cn.md#agent)
- `runk`: 当 runtime 为 runk 时配置
  - `namespace`: 任务调度到指定的机构 K8s Namespace 下
  - `dnsServers`: 机构 K8s 集群的 Pod DNS 配置， 用于解析节点的应用域名
  - `kubeconfigFile`: 机构 K8s 集群的 Kubeconfig，不填默认 serviceaccount；当前请不填，默认使用 serviceaccount
- `capacity`: 节点可用于调度应用的容量，runc/runp 不填会自动获取当前容器的系统资源, runk 模式下需要手动配置
  - `cpu`: cpu 核数， 如 4
  - `memory`: 内存大小，如 8Gi
  - `pods`: pods 数，如 500
  - `storage`: 磁盘容量，如 100Gi
- `image`: 节点镜像配置, 暂未实现
  - `pullPolicy`: 镜像策略，使用本地镜像仓库还是远程镜像仓库
  - `defaultRegistry`: 默认镜像
  - `registries`: 镜像仓库配置，类型数组
    - `name`: 镜像仓库名
    - `endpoint`: 镜像仓库地址
    - `username`: 镜像仓库用户名
    - `password`: 镜像仓库密码
- `datastoreEndpoint`: 数据库连接串，不填默认使用 sqlite。示例：`mysql://username:password@tcp(hostname:3306)/database-name`使用mysql数据库存储需要符合以下规范：
  - database 数据库名称暂不支持 "-"。
  - 创建数据库和表 kine ，建表语句参考[kine](https://github.com/secretflow/kuscia/blob/main/hack/k8s/kine.sql)。
    - 手动建表：如果机构建表是被管控的，或者提供的数据库账号没有建表权限，可以提前手动建立好数据表，kuscia 识别到数据表存在后，会自动跳过建表。
    - 自动建表：如果提供的数据库账号有建表权限（账号具有`DDL+DML`权限），并且数据表不存在，kuscia 会尝试自动建表，如果创建失败 kuscia 会启动失败。
  - 数据库账户对表中字段至少具有 select、insert、update、delete 操作权限。
- `protocol`: KusciaAPI 以及节点对外网关使用的通信协议，有三种通信协议可供选择：NOTLS/TLS/MTLS（不区分大小写）。
  - `NOTLS`: 不使用 TLS 协议，即数据通过未加密的 HTTP 传输，比较安全的内部网络环境或者 Kuscia 已经存在外部网关的情况可以使用该模式。
  - `TLS`: 通过 TLS 协议进行加密，即使用 HTTPS 进行安全传输，不需要手动配置证书。
  - `MTLS`: 使用 HTTPS 进行通信，支持双向 TLS 验证，需要手动交换证书以建立安全连接。
- `enableWorkloadApprove`: 是否开启工作负载审核，默认为 false，即关闭审核。取值范围:[true, false]。

{#configuration-example}
### 配置示例
- [Lite 节点配置示例](https://github.com/secretflow/kuscia/tree/main/scripts/templates/kuscia-lite.yaml)
- [Master 节点配置示例](https://github.com/secretflow/kuscia/tree/main/scripts/templates/kuscia-master.yaml)
- [Autonomy 节点配置示例](https://github.com/secretflow/kuscia/tree/main/scripts/templates/kuscia-autonomy.yaml)

## 修改默认配置文件
如果使用 [start_standalone.sh](https://github.com/secretflow/kuscia/blob/main/scripts/deploy/start_standalone.sh) 或者 [deploy.sh](https://github.com/secretflow/kuscia/blob/main/scripts/deploy/deploy.sh) 脚本部署的 kuscia，kuscia.yaml 文件路径默认是在以下位置（其他部署模式可以借鉴）。
- 宿主机路径：
  - master：\${PWD}/\${USER}-kuscia-master/kuscia.yaml
  - lite：\${PWD}/\${USER}-kuscia-lite-domainID/kuscia.yaml
  - autonomy：\${PWD}/\${USER}-kuscia-autonomy-domainID/kuscia.yaml
- 容器内路径：/home/kuscia/etc/conf/kuscia.yaml

宿主机路径下修改 kuscia.yaml 配置后，重启容器 `docker restart ${container_name}` 生效。
> Tips：如果要修改 Protocol 字段，请确保对该字段有充足的理解，否则会导致 KusciaAPI 调用失败或者和其他节点的通讯异常。详情参考[Protocol 通信协议](../reference/troubleshoot/protocol_describe.md)。

## 指定配置文件
如果使用 [deploy.sh](https://github.com/secretflow/kuscia/blob/main/scripts/deploy/deploy.sh) 脚本部署的 Kuscia，可以指定配置文件，示例：
```bash
# -c 参数传递的是指定的 Kuscia 配置文件路径。
./deploy.sh autonomy -n alice -p 11080 -k 8082 -c kuscia-autonomy.yaml
```
其中，kuscia-autonomy.yaml 可参考 [配置示例](#configuration-example)
