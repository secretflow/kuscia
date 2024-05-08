# 概要

隐私计算应用接入 Kuscia，需要遵循一些规范，主要包括配置模板、协议、部署模板、镜像等。



# 镜像

Kuscia 支持拉起 docker 镜像，为了支持 Kuscia 的一些特性，应用在构建镜像时，需要将某些配置以 Labels 的形式打到镜像中。

###  Labels

- kuscia.secretflow.config-templates：配置模板。
- kuscia.secretflow.deploy-templates：部署模板。

###  Dockerfile 示例

```dockerfile
FROM openanolis/anolisos:8.8

LABEL kuscia.secretflow.config-templates="xxxxxx"
LABEL kuscia.secretflow.kuscia.secretflow.deploy-templates="xxxxxx"

COPY secretflow /home/admin/secretflow
```



# 配置模板

应用可以提供一份配置模板给 Kuscia，模板中支持变量并由 Kuscia 动态渲染，渲染出的一份或多份配置文件最终会挂载到应用的容器中，挂载的位置请参考部署模版的介绍。



### 模板格式

配置模板的格式 为 YAML，包含多个 key-value 配置项。其中，每一个配置项的 key 为配置模板的名称，value 为配置模板的内容。示例：

```yaml
task-config.conf: |
{
  task_id: {{.TASK_ID}}
  task_input_config: {{.TASK_INPUT_CONFIG}}
  task_input_cluster_def: {{.TASK_CLUSTER_DEF}}
  allocated_ports: {{.ALLOCATED_PORTS}}
  ...
}
server.conf: |
{
  dbhost={{.DB_HOST}}
  dbport=10000
}
key.conf: abedfalc81230dfaacda
```

Kuscia 不感知 value 具体的格式，它由应用自己制定，最终也由应用自己解析。



### 变量支持

配置模板中的变量格式需要符合 [Golang template](https://pkg.go.dev/text/template) 的规则，因此理论上，模板内容语法的支持不仅限于变量，还支持条件判断、循环控制等 Golang template 提供的高级语法。

Kuscia 默认支持以下变量的渲染：

| 变量名                 | 描述      |
|---------------------|---------|
| TASK_ID             | 任务ID    |
| TASK_INPUT_CONFIG   | 任务参数配置  |
| TASK_CLUSTER_DEFINE | 任务参与方信息 |
| ALLOCATED_PORTS     | 给应用的端口  |

若要支持自定义的变量，只需要在节点本地接入的配置系统配置即可。



# 协议

Kuscia 在拉起应用时，会将一些必要的参数以文件或环境变量的方式传递给应用。



### TASK_CLUSTER_DEFINE

TASK_CLUSTER_DEFINE 定义了执行任务的各参与方的信息，包括参与方ID、暴露的端口和对应的 service 等，Value 的格式为 JSON。



#### 结构定义

proto 链接：https://code.alipay.com/secretflow/kuscia/blob/master/proto/api/v1alpha1/appconfig/app_config.proto

```protobuf
// Service represents the service address corresponding to the port.
message Service {
    // Name of port.
    string port_name = 1;
    // Endpoint list corresponding to the port.
    repeated string endpoints = 2;
}

// Party represents the basic information of the party.
message Party {
    // Name of party.
    string name = 1;
    // role carried by party. Examples: client, server...
    string role = 2;
    // List of services exposed by pod.
    repeated Service services = 3;
}

// ClusterDefine represents the information of all parties.
message ClusterDefine {
    // Basic information of all parties.
    repeated Party parties = 1;
    // index of self party.
    int32 self_party_idx = 2;
    // index of self endpoint.
    int32 self_endpoint_idx = 3;
}
```



#### 示例

```json
{
    "parties": [
        {
            "name": "alice-test-ab96242c2bc1a091",
            "role": "client",
            "services": [
                {
                    "port_name": "global",
                    "endpoints": [
                        "fascia-cbd70io7vda5d471l111-client0-global.alice-test-ab96242c2bc1a091.svc"
                    ]
                },
                {
                    "port_name": "worker",
                    "endpoints": [
                        "fascia-cbd70io7vda5d471l111-client0-worker-xxx.alice-test-ab96242c2bc1a091.svc"
                    ]
                }
            ]
        },
        {
            "name": "bob-test-bf44540af100cd29",
				"role": "client",
            "services": [
                {
                    "port_name": "global",
                    "endpoints": [
                        "fascia-cbd70io7vda5d471l222-client0-global.bob-test-bf44540af100cd29.svc"
                    ]
                },
                {
                    "port_name": "worker1",
                    "endpoints": [
                        "fascia-cbd70io7vda5d471l222-client0-worker1.bob-test-bf44540af100cd29.svc"
                    ]
                }
            ]
        },
        {
            "name": "fl-one-afa7729dea3ca42c",
				"role": "server",
            "services": [
                {
                    "port_name": "object-manager",
                    "endpoints": [
                        "fascia-cbd70io7vda5d471lpp2-server0-object-manager.fl-one-afa7729dea3ca42c.svc"
                    ]
                },
                {
                    "port_name": "global",
                    "endpoints": [
                        "fascia-cbd70io7vda5d471lpp2-server0-global.fl-one-afa7729dea3ca42c.svc"
                    ]
                },
                {
                    "port_name": "worker1",
                    "endpoints": [
                        "fascia-cbd70io7vda5d471lpp2-server0-worker1.fl-one-afa7729dea3ca42c.svc"
                    ]
                }
            ]
        }
    ],
    "self_party_idx": "0",
    "self_endpoint_idx": "0"
}
```



### ALLOCATED_PORTS

ALLOCATED_PORTS 定义了 Kuscia 给应用分配的端口号，Value 的格式为 JSON。

#### 结构定义

proto 链接：https://code.alipay.com/secretflow/kuscia/blob/master/proto/api/v1alpha1/appconfig/app_config.proto

```protobuf
// Port represents an allocated port for pod.
message Port {
    // Each named port in a pod must have a unique name.
    string name = 1;
    // Number of port allocated for pod.
    int32 port = 2;
    // Scope of port. Must be Cluster,Domain,Local.
    // Defaults to "Local".
    // +optional
    string scope = 3;
    // Protocol for port. Must be HTTP,GRPC.
    // Defaults to "HTTP".
    // +optional
    string protocol = 4;
}

// AllocatedPorts represents allocated ports for pod.
message AllocatedPorts {
    // Allocated ports.
    repeated Port ports = 1;
}
```



#### 示例

```json
{
    "ports": [
        {
            "name": "worker1",
            "port": 16619,
            "scope": "Cluster",
            "protocol": "GRPC"
        },
        {
            "name": "global",
            "port": 16609,
            "scope": "Cluster",
            "protocol": "GRPC"
        }
    ]
}
```



### TASK_INPUT_CONFIG

TASK_INPUT_CONFIG 定义了任务基础的参数配置，比如输入文件、输出文件、算子类型等，由应用侧定义具体的内容，Kuscia 目前不感知，只做透传。未来 Kuscia 和应用需要共同定义一套结构标准。



# 部署模板

每个应用镜像需要配置一个应用部署模板，它是一个类 K8s Deployment 的 YAML（待定） 格式的描述。



### 模板格式

参照：https://code.alipay.com/secretflow/kuscia/blob/master/pkg/crd/apis/kuscia/v1alpha1/appimage_types.go



### 示例

```yaml
- name: secretflow
  # R1
  # role 不局限于某类特定的值，可自定义
  # 取值范围示例: 'client'/'server'/'client,server'
  # job 使用的模版匹配规则流程如下:
  # 1. 若nuevajob.role [in] role。匹配到，则返回
  # 2. 若role 为空，模版通用。
  # 3. 若nuevajob role 为空，选择第一个模版。
  role: 'client'
  replicas: 1
  networkPolicy:
    ingresses:
    - from:
        # 只允许角色为 server 的参与方访问本方的 global 端口。
        roles:
        - server
      ports:
      - port: global
  spec:
    containers:
    - name: secretflow
      command:
      - /home/admin/bin/secretflow
      - --client-config=/home/admin/conf/client-config.conf
      - --task-config=/home/admin/conf/task-config.conf
      # 引用配置模板
      configVolumeMounts:
      - # 挂载到容器内的路径
        mountPath: /home/admin/conf/task-config.conf
        # 对应配置模板中的 key
        subPath: task-config.conf
      # 需要 listen 的端口列表
      ports:
      - name: global
        port: 10010
        # http/https
        # 创建service时，在annotation中加入nueva.secretflow/protocol: 'HTTP'
        protocol: HTTP
        # Cluster: 创建service，外部或节点内访问
        # Domain: 创建service，节点内部访问
        # Local: 不创建service，仅运用于pod内部localhost访问
        scope: Cluster
      - name: worker
        port: 10011
        protocol: HTTP
        scope: Domain
      resources:
        limits:
          cpu: "2"
          memory: 2Gi
        requests:
          cpu: "2"
          memory: 2Gi
```
