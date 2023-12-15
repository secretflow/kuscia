# Kuscia 配置文件

Kuscia 配置文件默认位置为 Kuscia 容器的 /home/kuscia/etc/conf/kuscia.yaml, 不同的部署模式所需的配置内容不同。
Kuscia的配置文件由公共配置和每个模式的特殊配置组成， 具体细节可以参考下文。kuscia的自动部署脚本会产生一份默认的配置文件，如果需要调整，可以将调整后的配置文件挂载到容器相应位置。

## Kuscia 配置
### 配置项示例
```yaml
#############################################################################
############                       公共配置                       ############
#############################################################################
# 部署模式
mode: Lite
# 节点ID
domainID: alice
# 节点私钥配置, 用于节点间的通信认证, 节点应用的证书签发
# 注意: 目前节点私钥仅支持 pkcs#1 格式的: "BEGIN RSA PRIVATE KEY/END RSA PRIVATE KEY"
# 执行命令 "docker run -it --rm secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia scripts/deploy/generate_rsa_key.sh" 生成私钥
domainKeyData: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNRDhDQVFBQ0NRREdsY1Y3MTd5V3l3SURBUUFCQWdrQXR5RGVueG0wUGVFQ0JRRHJVTGUvQWdVQTJBcUQ5UUlFCmFuYkxtd0lFZWFaYUxRSUZBSjZ1S2tjPQotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQo
# 日志级别 INFO、DEBUG、WARN
logLevel: INFO

#############################################################################
############                       Lite 配置                      ############
#############################################################################
# 节点连接 master 的部署 token，用于节点向 master 注册证书， 只在节点第一次向 master 注册证书时有效
liteDeployToken: LS0tLS1CRUdJTi
# 节点连接 master 的地址
masterEndpoint: https://172.18.0.2:1080

#############################################################################
############               Lite、Autonomy 配置                    ############
#############################################################################
# runc or runk
runtime: runc
# 当 runtime 为 runk 时配置
runk:
  # 任务调度到指定的机构 k8s namespace上
  namespace: ""
  # 机构 k8s 集群的 pod dns 配置， 用于解析节点的应用域名
  dnsServers:
  # 机构 k8s 集群的 kubeconfig, 不填默认 serviceaccount; 当前请不填，默认使用 serviceaccount
  kubeconfigFile:

# 节点可用于调度应用的容量，runc 不填会自动获取当前容器的系统资源, runk 模式下需要手动配置
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
############                   Master 配置                       ############
#############################################################################
# 数据库连接串，不填默认使用 sqlite
# 示例：mysql://username:password@tcp(hostname:3306)/database-name
datastoreEndpoint: ""
```

### 配置项详解
- `mode`: 当前 Kuscia 节点部署模式 支持 Lite、Master、Autonomy（不区分大小写）, 不同部署模式详情请参考[这里](../reference/architecture_cn)
- `domainID`: 当前 Kuscia 实例的 [节点 ID](../reference/concepts/domain_cn)， 需要符合 DNS 子域名规则要求，详情请参考[这里](https://kubernetes.io/zh-cn/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names)
- `domainKeyData`: 节点私钥配置, 用于节点间的通信认证, 节点应用的证书签发， 经过 base64 编码。 可以通过命令 `docker run -it --rm secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia scripts/deploy/generate_rsa_key.sh` 生成
- `logLevel`: 日志级别 INFO、DEBUG、WARN，默认 INFO
- `liteDeployToken`: 节点连接 master 的部署 token，用于节点向 master 注册证书， 只在节点第一次向 master 注册证书时有效，详情请参考[节点中心化部署](./deploy_master_lite_cn)
- `masterEndpoint`: 节点连接 master 的地址，比如 https://172.18.0.2:1080
- `runtime`: 节点运行时 runc、runk，运行时详解请参考[这里](../reference/architecture_cn.md#agent)
- `runk`: 当 runtime 为 runk 时配置
  - `namespace`: 任务调度到指定的机构 k8s namespace 上
  - `dnsServers`: 机构 k8s 集群的 pod dns 配置， 用于解析节点的应用域名
  - `kubeconfigFile`: 机构 k8s 集群的 kubeconfig，不填默认 serviceaccount；当前请不填，默认使用 serviceaccount
- `capacity`: 节点可用于调度应用的容量，runc 不填会自动获取当前容器的系统资源, runk 模式下需要手动配置
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
- `datastoreEndpoint`: 数据库连接串，不填默认使用 sqlite。示例：mysql://username:password@tcp(hostname:3306)/database-name

### 配置示例
- [Lite 节点配置示例](https://github.com/secretflow/kuscia/tree/main/scripts/templates/kuscia-lite.yaml)
- [Master 节点配置示例](https://github.com/secretflow/kuscia/tree/main/scripts/templates/kuscia-master.yaml)
- [Autonomy 节点配置示例](https://github.com/secretflow/kuscia/tree/main/scripts/templates/kuscia-autonomy.yaml)

## 修改默认配置文件
如果使用 [start_standalone.sh](https://github.com/secretflow/kuscia/blob/main/scripts/deploy/start_standalone.sh) 或者 [deploy.sh](https://github.com/secretflow/kuscia/blob/main/scripts/deploy/deploy.sh) 脚本部署的 kuscia，kuscia.yaml 文件路径默认是在以下位置（其他部署模式可以借鉴）。
- 宿主机路径：
  - master：\$HOME/kuscia/\${USER}-kuscia-master/kuscia.yaml
  - lite：\$HOME/kuscia/\${USER}-kuscia-lite-\${domainID}/kuscia.yaml
  - autonomy：\$HOME/kuscia/\${USER}-kuscia-autonomy-\${domainID}/kuscia.yaml
- 容器内路径：/home/kuscia/etc/conf/kuscia.yaml

宿主机路径下修改 kuscia.yaml 配置后，重启容器 `docker restart ${container_name}` 生效。