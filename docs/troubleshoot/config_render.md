# 应用配置文件渲染

一般应用都是由可执行文件（or代码）和配置文件组成，配置文件可以用来定义应用执行时的参数，Kuscia 在任务/服务的每次运行前都会动态渲染出来真实的配置文件，并且 `挂载` 到应用执行时的文件系统空间。

如果应用需要在 Kuscia 上运行，需要适配 Kuscia 的配置文件渲染模式来解决应用运行时的端口冲突，参与方地址，数据访问地址等问题，详细如下：
- 端口： Kuscia 要求应用能够支持指定监听端口，主要是解决 RunP 场景下，端口冲突的问题
- 任务输入： 用户通过 KusciaAPI/KusciaJob/KusciaTask/KusciaDeployment 指定的任务参数，比如： `taskInputConfig`
- 集群信息： 其他参与方的地址等信息
- 内部环境： 比如 DataMesh 地址 等信息
- 其他动态配置: 应用依赖的某个配置在不同机构，可能对应的值不一样，需要从 Kuscia 配置系统 中提取对应的值

注： Kuscia也继续兼容支持应用从环境变量中读取相关信息，但推荐使用配置文件模式，因为配置文件模式渲染能力更强大，并且可以将应用和Kuscia通过配置文件解耦

## 配置示例
Kuscia 支持自定义隐私计算应用配置文件渲染， 比如一个下面一个简单的例子，我们想要在 Kuscia 中运行一个 Nginx 服务

需要首先定义对应的 AppImage 模版，之后再通过 KusciaDeployment(/KusciaJob) 把 Nginx 运行起来。

因为 Kuscia 需要运行之上的服务支持设置动态端口，所以需要在配置文件中指定 Kuscia 分配的端口，下面是 Nginx 的配置示例：

```yaml
apiVersion: kuscia.secretflow/v1alpha1
kind: AppImage
metadata:
  name: nginx-serving
spec:
  configTemplates:
    default.conf: |
      server {
        listen       {{{.ALLOCATED_PORTS.ports[name=test].port}}};
        server_name  _;
        location / {
          root   /usr/share/nginx/html;
          index  index.html index.htm;
        }
      }
  deployTemplates:
    - name: psi
      replicas: 1
      spec:
        containers:
          - name: test
            configVolumeMounts:
              - mountPath: /etc/nginx/conf.d/default.conf
                subPath: default.conf
            workingDir: /root
            ports:
              - name: test
                protocol: HTTP
                scope: Cluster
        restartPolicy: Never
  image:
    id:
    name: nginx
    tag: latest


```

注：
- `configTemplates`: 应用自己的配置文件模版， Kuscia运行应用前，会根据模版渲染出真实的配置文件； 详细参考： [AppImage](../reference/concepts/appimage_cn.md)
- `deployTemplates[].spec.containers[].configVolumeMounts[]`: 供应用读取的配置文件挂载地址（配置文件内容，对对应到`configTemplates`）；详细参考： [AppImage](../reference/concepts/appimage_cn.md)

## 配置文件渲染

### 变量介绍
配置文件的内容会在应用实际执行前，由 `Agent` 模块完成渲染； `Agent` 支持将以下内容渲染到配置文件模版中
- 内置变量： 由 Kuscia 产生的运行时变量
- 任务变量： 任务输入变量，如：任务配置中的 `taskInputConfig`
- 配置文件（暂不支持）： 来自于 Kuscia [配置文件](../deployment/kuscia_config_cn.md) 的配置参数
- 配置系统（暂不支持）： 来自于 Kuscia 配置系统的配置参数


完整的变量表格
| 类型 | 变量 | 使用示例 | 介绍 | 内容示例 |
| - | - | - | - | - |
| 内置 | TASK_ID | `{{.TASK_ID}}` | 表示任务的 ID，当应用启动为 KusciaJob 时有效 | secretflow-task-20230406162606 |
| 内置 | SERVING_ID | `{{.SERVING_ID}}` | 表示服务的 ID，当应用启动为 KusciaDeployment 时有效 | serving-20230406162606 |
| 内置 | ALLOCATED_PORTS | `{{.ALLOCATED_PORTS}}` | Kuscia为应用动态分配的端口，应用需要根据分配的端口来启动应用，内容结构请[参考这里](https://github.com/secretflow/kuscia/blob/main/proto/api/v1alpha1/appconfig/app_config.proto#L67) | `{"ports":[{"name":"test","port":26409,"scope":"Cluster","protocol":"HTTP"}]}` |
| 内置 | TASK_CLUSTER_DEFINE | `{{.TASK_CLUSTER_DEFINE}}` |  Kuscia 分配的任务集群信息，应用可以根据集群信息访问其他参与方， 内容结构请[参考这里](https://github.com/secretflow/kuscia/blob/main/proto/api/v1alpha1/appconfig/app_config.proto#L41) | `{"parties":[{"name":"alice","role":"","services":[{"portName":"test","endpoints":["alice-test-0-test.alice.svc"]}]}],"selfPartyIdx":0,"selfEndpointIdx":0}` |
| 内置 | KUSCIA_DOMAIN_ID | `{{.KUSCIA_DOMAIN_ID}}` | 节点ID | 对应配置文件中的 `domainId` | alice |
| 任务 | TASK_INPUT_CONFIG | `{{.TASK_INPUT_CONFIG}}` | 任务的运行参数，对应到 KusciaAPI/KusciaJob/KusciaTask 中的 taskInputConfig 参数； Kuscia不会关注实际内容是什么，只当做字符串透传 | |
| 任务 | INPUT_CONFIG | `{{.INPUT_CONFIG}}` | 服务的运行参数，对应到 KusciaAPI/KusciaDeployment 中的 inputConfig 参数；Kuscia不会关注实际内容是什么，只当做字符串透传 |  |
| 任务 | CLUSTER_DEFINE | `{{.CLUSTER_DEFINE}}` | Kuscia 分配的任务集群信息，应用可以根据集群信息访问其他参与方(当为 KusciaDeployment 时生效) |  |


### 渲染规则
Kuscia 基于 Golang 的 `text/template` 库来实现模版的渲染，所以默认该库支持的配置渲染方法， Kuscia 都支持； 最常见的方式为：`{{.VariableName}}`, 更多请参考[text/template](https://pkg.go.dev/text/template)

Kuscia 在 `text/template` 语法之外，为了方便配置文件更加简单，支持了嵌套结构的访问模式，对应的语法是：

| 语法示例 | 介绍 | 示例 |
| - | - | - |
| `{{{ .VariableName.Field1 }}}` | 解决嵌套结构的访问，嵌套层次暂无限制（比如：{{{ .VariableName.Field1.SubField2 }}} ） | 参考上文中的 `TASK_CLUSTER_DEFINE`， 可以使用 `{{{.TASK_CLUSTER_DEFINE.selfPartyIdx}}}` |
| `{{{.VariableName.Field1[<idx>].SubField2}}}` | 选择指定数组索引的元素 | 参考上文中的 `TASK_CLUSTER_DEFINE`， 可以使用 `{{{.TASK_CLUSTER_DEFINE.parties[0].name}}}` |
| `{{{.VariableName.Field1[<key>=<value>].SubField2}}}` | 筛选符合指定条件的数组元素（如果有多个满足条件，只选择第一个） | 参考上文中的 `TASK_CLUSTER_DEFINE`， 可以使用 `{{{.TASK_CLUSTER_DEFINE.parties[name=alice].role}}}` 来查找 `TASK_CLUSTER_DEFINE.parties`所有元素中，满足 `name` 值为 `alice`的元素 |

注：
- Kusia自定义的语法和原生语法区别是： 原生语法使用 `{{<pattern>}}`, Kuscia语法使用 `{{{<pattern>}}}`
- Kuscia语法功能比较限定，请严格按照上述示例来填写，其他行为未知（也不保证非定义行为的兼容性）
- 在过滤筛选语法中 `<key>`, `<value>`请确保没有 `="[]` 等字符串，否则行为会未知
- 如果输出的类型非原子类型（比如：结构体/Map/Array等），默认会使用Json来进行序列化，所以请确保输出内容符合目标配置文件格式