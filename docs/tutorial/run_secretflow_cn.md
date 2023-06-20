# 如何运行一个 Secretflow 任务

## 准备节点

准备节点包含部署节点和创建授权两个步骤，请参考[快速入门](../getting_started/quickstart_cn.md) .

## 准备数据

你可以使用 Kuscia 中自带的数据文件，或者使用你自己的数据文件。

在 Kuscia 中， 节点数据文件的存放路径为节点容器的`/home/kuscia/var/storage`，你可以在容器中查看这个数据文件。

### 查看 Kuscia 示例数据

这里以 alice 节点为例，首先进入节点容器：

```shell
docker exec -it ${USER}-kuscia-lite-alice bash
```

在 alice 节点容器中查看节点示例数据：

```shell
cat /home/kuscia/var/storage/data/alice.csv
```

bob 节点同理。

{#prepare-your-own-data}

### 准备你自己的数据

你也可以使用你自己的数据文件，首先你要你的数据文件复制到节点容器中，还是以 alice 节点为例：

```shell
docker cp {your_alice_data} ${USER}-kuscia-lite-alice:/home/kuscia/var/storage/data/
```

接下来你可以像 [查看 Kuscia 示例数据](#kuscia) 一样查看你的数据文件，这里不再赘述。

{#configure-kuscia-job}

## 配置 KusciaJob

我们需要在 kuscia-master 节点容器中配置和运行 Job，首先，让我们先进入 kuscia-master 节点容器：

```shell
docker exec -it ${USER}-kuscia-master bash
```

### 使用 Kuscia 示例数据配置 KusciaJob

下面的示例展示了一个 KusciaJob， 该任务流完成 2 个任务：

1. job-psi 读取 alice 和 bob 的数据文件，进行隐私求交，求交的结果分别保存为两个参与方的`psi_out.csv`。
2. job-split 读取 alice 和 bob 上一步中求交的结果文件，并拆分成训练集和测试集，分别保存为两个参与方的`train_dataset.csv`、`test_dataset.csv`。

这个 KusciaJob 的名称为 job-best-effort-linear，在一个 Kuscia 集群中，这个名称必须是唯一的，由`.metadata.name`指定。

在 kuscia-master 容器中，在任意路径创建文件 job-best-effort-linear.yaml，内容如下：

```yaml
apiVersion: kuscia.secretflow/v1alpha1
kind: KusciaJob
metadata:
  name: job-best-effort-linear
spec:
  initiator: alice
  scheduleMode: BestEffort
  maxParallelism: 2
  tasks:
    - taskID: job-psi
      priority: 100
      taskInputConfig: '{"sf_storage_config":{"alice":{"type":"local_fs","local_fs":{"wd":"/home/kuscia/var/storage/data"}},"bob":{"type":"local_fs","local_fs":{"wd":"/home/kuscia/var/storage/data"}}},"sf_cluster_desc":{"parties":["alice","bob"],"devices":[{"name":"spu","type":"spu","parties":["alice","bob"],"config":"{\\\"runtime_config\\\":{\\\"protocol\\\":\\\"REF2K\\\",\\\"field\\\":\\\"FM64\\\"},\\\"link_desc\\\":{\\\"connect_retry_times\\\":60,\\\"connect_retry_interval_ms\\\":1000,\\\"brpc_channel_protocol\\\":\\\"http\\\",\\\"brpc_channel_connection_type\\\":\\\"pooled\\\",\\\"recv_timeout_ms\\\":1200000,\\\"http_timeout_ms\\\":1200000}}"}]},"sf_node_eval_param":{"domain":"psi","name":"two_party_balanced_psi","version":"0.0.1","attr_paths":["protocol","receiver","precheck_input","sort","broadcast_result","bucket_size","curve_type","input/receiver_input/key","input/sender_input/key"],"attrs":[{"s":"ECDH_PSI_2PC"},{"s":"alice"},{"b":true},{"b":true},{"b":true},{"i64":1048576},{"s":"CURVE_25519"},{"ss":["id1"]},{"ss":["id2"]}],"inputs":[{"type":"sf.table.individual","data_refs":[{"uri":"alice.csv","party":"alice","format":"csv"}],"meta":{"@type":"type.googleapis.com/secretflow.component.IndividualTable","schema":{"types":["str","str","str"],"features":["id1","item","feature1"]}}},{"type":"sf.table.individual","data_refs":[{"uri":"bob.csv","party":"bob","format":"csv"}],"meta":{"@type":"type.googleapis.com/secretflow.component.IndividualTable","schema":{"types":["str","str"],"features":["id2","feature2"]}}}],"output_uris":["psi_output.csv"]}}'
      appImage: secretflow-image
      parties:
        - domainID: alice
        - domainID: bob
    - taskID: job-split
      priority: 100
      dependencies: ['job-psi']
      taskInputConfig: '{"sf_storage_config":{"alice":{"type":"local_fs","local_fs":{"wd":"/home/kuscia/var/storage/data"}},"bob":{"type":"local_fs","local_fs":{"wd":"/home/kuscia/var/storage/data"}}},"sf_cluster_desc":{"parties":["alice","bob"],"devices":[{"name":"spu","type":"spu","parties":["alice","bob"],"config":"{\\\"runtime_config\\\":{\\\"protocol\\\":\\\"REF2K\\\",\\\"field\\\":\\\"FM64\\\"},\\\"link_desc\\\":{\\\"connect_retry_times\\\":60,\\\"connect_retry_interval_ms\\\":1000,\\\"brpc_channel_protocol\\\":\\\"http\\\",\\\"brpc_channel_connection_type\\\":\\\"pooled\\\",\\\"recv_timeout_ms\\\":1200000,\\\"http_timeout_ms\\\":1200000}}"},{"name":"heu","type":"heu","parties":["alice","bob"],"config":"{\\\"mode\\\": \\\"PHEU\\\", \\\"schema\\\": \\\"paillier\\\", \\\"key_size\\\": 2048}"}]},"sf_node_eval_param":{"domain":"preprocessing","name":"train_test_split","version":"0.0.1","attr_paths":["train_size","test_size","random_state","shuffle"],"attrs":[{"f":0.75},{"f":0.25},{"i64":1234},{"b":true}],"inputs":[{"type":"sf.table.vertical_table","meta":{"@type":"type.googleapis.com/secretflow.component.VerticalTable","schemas":[{"ids":["id1"],"features":["item","feature1"],"types":["f32","f32"],"labels":["y"]},{"ids":["id2"],"features":["feature2"],"types":["f32"]}]},"data_refs":[{"uri":"psi_out.csv","party":"alice","format":"csv"},{"uri":"psi_out.csv","party":"bob","format":"csv"}]}],"output_uris":["train_dataset.csv","test_dataset.csv"]}}'
      appImage: secretflow-image
      parties:
        - domainID: alice
        - domainID: bob
```

### 使用你自己的数据配置 KusciaJob

如果你要使用你自己的数据，可以将两个算子中的`taskInputConfig.sf_node_eval_param`中的`inputs`和`output_uris`中的数据文件路径修改为你在 [准备你自己的数据](#prepare-your-own-data) 中的数据文件目标路径即可。

### 更多相关

更多有关 KusciaJob 配置的信息，请查看 [KusciaJob](../reference/concepts/kusciajob_cn.md) 和 [算子参数描述](#input-config) 。
前者描述了 KusciaJob 的定义和相关说明，后者描述了支持的算子和参数。

## 运行 KusciaJob

现在我们已经配置好了一个 KusciaJob ，接下来，让我们运行这个 KusciaJob， 在 kuscia-master 容器中执行 ：

```shell
kubectl apply -f job-best-effort-linear.yaml
```

## 查看 KusciaJob 运行状态

现在我们提交了这个 KusciaJob ，接下来我们可以在 kuscia-master 容器中通过下面的命令查看 KusciaJob 的运行情况。

### 查看所有的 KusciaJob

```shell
kubectl get kj
```

你可以看到如下输出：

```shell
NAME                     STARTTIME   COMPLETIONTIME   LASTRECONCILETIME   PHASE
job-best-effort-linear   3s                           3s                  Running
```

job-best-effort-linear 就是我们刚刚创建出来的 KusciaJob 。

### 查看运行中的某个 KusciaJob 的详细状态

通过指定`-o yaml`参数，我们可以以 Yaml 的形式看到 KusciaJob 的详细状态。job-best-effort-linear 是你在 [配置 Job](#configure-kuscia-job) 中指定的 KusciaJob 的名称。

```shell
kubectl get kj job-best-effort-linear -o yaml
```

如果任务成功了，你可以看到如下输出：

```shell
apiVersion: kuscia.secretflow/v1alpha1
kind: KusciaJob
metadata:
  creationTimestamp: "2023-03-30T12:11:41Z"
  generation: 1
  name: job-best-effort-linear
  resourceVersion: "19002"
  uid: 085e10e6-5d3e-43cf-adaa-715d76a6af9b
spec:
  initiator: alice
  maxParallelism: 2
  scheduleMode: BestEffort
  tasks:
  - appImage: secretflow-image
    parties:
    - domainID: alice
    - domainID: bob
    priority: 100
    taskID: job-psi
    taskInputConfig: '{"sf_storage_config":{"alice":{"type":"local_fs","local_fs":{"wd":"/home/kuscia/var/storage/data"}},"bob":{"type":"local_fs","local_fs":{"wd":"/home/kuscia/var/storage/data"}}},"sf_cluster_desc":{"parties":["alice","bob"],"devices":[{"name":"spu","type":"spu","parties":["alice","bob"],"config":"{\\\"runtime_config\\\":{\\\"protocol\\\":\\\"REF2K\\\",\\\"field\\\":\\\"FM64\\\"},\\\"link_desc\\\":{\\\"connect_retry_times\\\":60,\\\"connect_retry_interval_ms\\\":1000,\\\"brpc_channel_protocol\\\":\\\"http\\\",\\\"brpc_channel_connection_type\\\":\\\"pooled\\\",\\\"recv_timeout_ms\\\":1200000,\\\"http_timeout_ms\\\":1200000}}"}]},"sf_node_eval_param":{"domain":"psi","name":"two_party_balanced_psi","version":"0.0.1","attr_paths":["protocol","receiver","precheck_input","sort","broadcast_result","bucket_size","curve_type","input/receiver_input/key","input/sender_input/key"],"attrs":[{"s":"ECDH_PSI_2PC"},{"s":"alice"},{"b":true},{"b":true},{"b":true},{"i64":1048576},{"s":"CURVE_25519"},{"ss":["id1"]},{"ss":["id2"]}],"inputs":[{"type":"sf.table.individual","data_refs":[{"uri":"alice.csv","party":"alice","format":"csv"}],"meta":{"@type":"type.googleapis.com/secretflow.component.IndividualTable","schema":{"types":["str","str","str"],"features":["id1","item","feature1"]}}},{"type":"sf.table.individual","data_refs":[{"uri":"bob.csv","party":"bob","format":"csv"}],"meta":{"@type":"type.googleapis.com/secretflow.component.IndividualTable","schema":{"types":["str","str"],"features":["id2","feature2"]}}}],"output_uris":["psi_output.csv"]}}'
    tolerable: false
  - appImage: secretflow-image
    dependencies:
    - job-psi
    parties:
    - domainID: alice
    - domainID: bob
    priority: 100
    taskID: job-split
    taskInputConfig: '{"sf_storage_config":{"alice":{"type":"local_fs","local_fs":{"wd":"/home/kuscia/var/storage/data"}},"bob":{"type":"local_fs","local_fs":{"wd":"/home/kuscia/var/storage/data"}}},"sf_cluster_desc":{"parties":["alice","bob"],"devices":[{"name":"spu","type":"spu","parties":["alice","bob"],"config":"{\\\"runtime_config\\\":{\\\"protocol\\\":\\\"REF2K\\\",\\\"field\\\":\\\"FM64\\\"},\\\"link_desc\\\":{\\\"connect_retry_times\\\":60,\\\"connect_retry_interval_ms\\\":1000,\\\"brpc_channel_protocol\\\":\\\"http\\\",\\\"brpc_channel_connection_type\\\":\\\"pooled\\\",\\\"recv_timeout_ms\\\":1200000,\\\"http_timeout_ms\\\":1200000}}"},{"name":"heu","type":"heu","parties":["alice","bob"],"config":"{\\\"mode\\\": \\\"PHEU\\\", \\\"schema\\\": \\\"paillier\\\", \\\"key_size\\\": 2048}"}]},"sf_node_eval_param":{"domain":"preprocessing","name":"train_test_split","version":"0.0.1","attr_paths":["train_size","test_size","random_state","shuffle"],"attrs":[{"f":0.75},{"f":0.25},{"i64":1234},{"b":true}],"inputs":[{"type":"sf.table.vertical_table","meta":{"@type":"type.googleapis.com/secretflow.component.VerticalTable","schemas":[{"ids":["id1"],"features":["item","feature1"],"types":["f32","f32"],"labels":["y"]},{"ids":["id2"],"features":["feature2"],"types":["f32"]}]},"data_refs":[{"uri":"psi_out.csv","party":"alice","format":"csv"},{"uri":"psi_out.csv","party":"bob","format":"csv"}]}],"output_uris":["train_dataset.csv","test_dataset.csv"]}}'
    tolerable: false
status:
  completionTime: "2023-03-30T12:12:07Z"
  lastReconcileTime: "2023-03-30T12:12:07Z"
  phase: Succeeded
  startTime: "2023-03-30T12:11:41Z"
  taskStatus:
    job-psi: Succeeded
    job-split: Succeeded
```

`status`字段记录了 KusciaJob 的运行状态，`.status.phase`字段描述了 KusciaJob 的整体状态，而 `.status.taskStatus`则描述了每个 KusciaTask 的状态。
详细信息请参考 [KusciaJob](../reference/concepts/kusciajob_cn.md) 。

### 查看 KusciaJob 中的某个 KusciaTask 的详细状态。

KusciaJob 中的每一个 KusciaTask 都有一个`taskID`，通过`taskID`我们可以查看某个 KusciaTask 的详细状态。

```shell
kubectl get kt job-psi -o yaml
```

KusciaTask 的信息这里不再赘述，请查看 [KusciaTask](../reference/concepts/kusciatask_cn.md) 。

## 删除 KusciaJob

当你想取消或者清理这个 KusciaJob 时，你可以通过下面的命令完成：

```shell
kubectl delete kj job-best-effort-linear
```

当这个 KusciaJob 被清理时， 这个 KusciaJob 创建的 KusciaTask 也会一起被清理。

{#input-config}

## 算子参数描述

KusciaJob 的算子参数由`taskInputConfig`字段定义，对于不同的算子，算子的参数不同。

下面是一个隐私求交的算子示例。

### TaskInputConfig 结构示例和定义

#### 示例

```json
{
  "sf_storage_config": {
    "alice": {
      "type": "local_fs",
      "local_fs": {
        "wd": "/home/kuscia/var/storage/data"
      }
    },
    "bob": {
      "type": "local_fs",
      "local_fs": {
        "wd": "/home/kuscia/var/storage/data"
      }
    }
  },
  "sf_cluster_desc": {
    "parties": ["alice", "bob"],
    "devices": [
      {
        "name": "spu",
        "type": "spu",
        "parties": ["alice", "bob"],
        "config": "{\"runtime_config\":{\"protocol\":\"REF2K\",\"field\":\"FM64\"},\"link_desc\":{\"connect_retry_times\":60,\"connect_retry_interval_ms\":1000,\"brpc_channel_protocol\":\"http\",\"brpc_channel_connection_type\":\"pooled\",\"recv_timeout_ms\":1200000,\"http_timeout_ms\":1200000}}"
      }
    ]
  },
  "sf_node_eval_param": {
    "domain": "psi",
    "name": "two_party_balanced_psi",
    "version": "0.0.1",
    "attr_paths": [
      "protocol",
      "receiver",
      "precheck_input",
      "sort",
      "broadcast_result",
      "bucket_size",
      "curve_type",
      "input/receiver_input/key",
      "input/sender_input/key"
    ],
    "attrs": [
      {
        "s": "ECDH_PSI_2PC"
      },
      {
        "s": "alice"
      },
      {
        "b": true
      },
      {
        "b": true
      },
      {
        "b": true
      },
      {
        "i64": 1048576
      },
      {
        "s": "CURVE_25519"
      },
      {
        "ss": ["id1"]
      },
      {
        "ss": ["id2"]
      }
    ],
    "inputs": [
      {
        "type": "sf.table.individual",
        "data_refs": [
          {
            "uri": "alice.csv",
            "party": "alice",
            "format": "csv"
          }
        ],
        "meta": {
          "@type": "type.googleapis.com/secretflow.component.IndividualTable",
          "schema": {
            "types": ["str", "str", "str"],
            "features": ["id1", "item", "feature1"]
          }
        }
      },
      {
        "type": "sf.table.individual",
        "data_refs": [
          {
            "uri": "bob.csv",
            "party": "bob",
            "format": "csv"
          }
        ],
        "meta": {
          "@type": "type.googleapis.com/secretflow.component.IndividualTable",
          "schema": {
            "types": ["str", "str"],
            "features": ["id2", "feature2"]
          }
        }
      }
    ],
    "output_uris": ["psi_output.csv"]
  }
}
```

#### TaskInputConfig 字段说明

- `sf_storage_config`描述了两方数据存放的位置。
- `sf_cluster_desc`描述了 secretflow 参与方和集群信息。
- `sf_node_eval_param`描述了一个 Task 中的任务参数，其中`domain`、`name`、`version`确定一个算子及其版本。

##### sf_storage_config 字段

sf_storage_config 字段是一个 map 结构，其中 key 标识 party，value 的字段描述如下：

| 参数名称    | 类型    | 描述                                |
| ----------- | ------- | ----------------------------------- |
| type        | string  | 数据配置类型，目前仅支持： local_fs |
| local_fs    | object  | local_fs 类型的配置信息             |
| local_fs.wd | integer | 工作目录                            |

##### sf_cluster_desc 字段

sf_storage_config 字段是一个 map 结构，其中 key 标识 party，value 的字段描述如下：

| 参数名称 | 类型  | 描述                    |
| -------- | ----- | ----------------------- |
| parties  | array | 参与方列表              |
| devices  | array | secretflow 集群配置信息 |

##### sf_node_eval_param 字段

| 参数名称                      | 类型    | 描述                                                          |
| ----------------------------- | ------- | ------------------------------------------------------------- |
| domain                        | string  | 算子的 Namespace                                              |
| name                          | string  | 算子名称，和 domain 一起唯一确定一个算子                      |
| version                       | integer | 算子的版本                                                    |
| attr_paths                    | array   | 参数列表，和 attr 按数组位置对应                              |
| attr                          | array   | 参数值，和 attr_paths 按数组位置对应                          |
| inputs                        | array   | 任务输入                                                      |
| inputs[].type                 | string  | 任务输入的类型                                                |
| inputs[].data_refs            | object  | 任务输入数据引用                                              |
| inputs[].data_refs.party      | string  | 任务输入参与方，会使用这个值指向 sf_storage_config 的数据配置 |
| inputs[].data_refs.uri        | string  | 任务输入数据相对 sf_storage_config 的位置                     |
| inputs[].data_refs.format     | string  | 任务输入数据格式                                              |
| inputs[].meta                 | object  | 任务输入数据元信息                                            |
| inputs[].meta.@type           | string  | 任务输入数据类型                                              |
| inputs[].meta.schema          | object  | 任务输入数据模式信息                                          |
| inputs[].meta.schema.types    | array   | 任务输入数据列类型                                            |
| inputs[].meta.schema.features | array   | 任务输入数据列名                                              |
| inputs[].meta.schema.ids      | array   | 任务输入数据 id 列，在多方表中作为关联列                      |
| inputs[].meta.schema.labels   | array   | 任务输入数据 label 列，在模型训练和预测中用作标签             |
| output_uris                   | array   | 任务输出列表，输出会保存在 sf_storage_config 的数据配置中     |

##### attr 结构

Atomic 类型用于泛化的参数值，对于不同的数据类型，请使用不同的字段，其中`s`、`i64`、`f`、`b`分别表示 string 、 integer 、 float 和
bool 类型。
而`ss`、`i64s`、`fs`、`bs`则表示上述类型的列表。

```json
{
  "s": "abc",
  "i64": 123,
  "f": 12.3,
  "b": true,
  "ss": ["a"],
  "i64s": [1],
  "fs": [1.0],
  "bs": [true]
}
```

### 支持的算子、示例和参数

#### 隐私求交

隐私求交将会求出两方数据的交集。

```json
{
  "sf_storage_config": {
    "alice": {
      "type": "local_fs",
      "local_fs": {
        "wd": "/home/kuscia/var/storage/data"
      }
    },
    "bob": {
      "type": "local_fs",
      "local_fs": {
        "wd": "/home/kuscia/var/storage/data"
      }
    }
  },
  "sf_cluster_desc": {
    "parties": ["alice", "bob"],
    "devices": [
      {
        "name": "spu",
        "type": "spu",
        "parties": ["alice", "bob"],
        "config": "{\"runtime_config\":{\"protocol\":\"REF2K\",\"field\":\"FM64\"},\"link_desc\":{\"connect_retry_times\":60,\"connect_retry_interval_ms\":1000,\"brpc_channel_protocol\":\"http\",\"brpc_channel_connection_type\":\"pooled\",\"recv_timeout_ms\":1200000,\"http_timeout_ms\":1200000}}"
      }
    ]
  },
  "sf_node_eval_param": {
    "domain": "psi",
    "name": "two_party_balanced_psi",
    "version": "0.0.1",
    "attr_paths": [
      "protocol",
      "receiver",
      "precheck_input",
      "sort",
      "broadcast_result",
      "bucket_size",
      "curve_type",
      "input/receiver_input/key",
      "input/sender_input/key"
    ],
    "attrs": [
      {
        "s": "ECDH_PSI_2PC"
      },
      {
        "s": "alice"
      },
      {
        "b": true
      },
      {
        "b": true
      },
      {
        "b": true
      },
      {
        "i64": 1048576
      },
      {
        "s": "CURVE_25519"
      },
      {
        "ss": ["id1"]
      },
      {
        "ss": ["id2"]
      }
    ],
    "inputs": [
      {
        "type": "sf.table.individual",
        "data_refs": [
          {
            "uri": "alice.csv",
            "party": "alice",
            "format": "csv"
          }
        ],
        "meta": {
          "@type": "type.googleapis.com/secretflow.component.IndividualTable",
          "schema": {
            "types": ["str", "str", "str"],
            "features": ["id1", "item", "feature1"]
          }
        }
      },
      {
        "type": "sf.table.individual",
        "data_refs": [
          {
            "uri": "bob.csv",
            "party": "bob",
            "format": "csv"
          }
        ],
        "meta": {
          "@type": "type.googleapis.com/secretflow.component.IndividualTable",
          "schema": {
            "types": ["str", "str"],
            "features": ["id2", "feature2"]
          }
        }
      }
    ],
    "output_uris": ["psi_output.csv"]
  }
}
```

参数如下：

| 参数名称         | 类型    | 描述                        |
| ---------------- | ------- | --------------------------- |
| protocol         | string  | PSI 协议                    |
| receiver         | string  | 哪方获得求交数据            |
| precheck_input   | bool    | 求交前是否检查数据          |
| sort             | bool    | 求交后是否排序              |
| broadcast_result | bool    | 是否将求交结果广播给各方    |
| bucket_size      | integer | 指定在 PSI 中的 hash 桶大小 |
| curve_type       | string  | ECDH PSI 的曲线类型         |

#### 随机分割

随机分割将会将两方数据切分成训练集和测试集。

```json
{
  "sf_storage_config": {
    "alice": {
      "type": "local_fs",
      "local_fs": {
        "wd": "/home/kuscia/var/storage/data"
      }
    },
    "bob": {
      "type": "local_fs",
      "local_fs": {
        "wd": "/home/kuscia/var/storage/data"
      }
    }
  },
  "sf_cluster_desc": {
    "parties": ["alice", "bob"],
    "devices": [
      {
        "name": "spu",
        "type": "spu",
        "parties": ["alice", "bob"],
        "config": "{\"runtime_config\":{\"protocol\":\"REF2K\",\"field\":\"FM64\"},\"link_desc\":{\"connect_retry_times\":60,\"connect_retry_interval_ms\":1000,\"brpc_channel_protocol\":\"http\",\"brpc_channel_connection_type\":\"pooled\",\"recv_timeout_ms\":1200000,\"http_timeout_ms\":1200000}}"
      },
      {
        "name": "heu",
        "type": "heu",
        "parties": ["alice", "bob"],
        "config": "{\"mode\": \"PHEU\", \"schema\": \"paillier\", \"key_size\": 2048}"
      }
    ]
  },
  "sf_node_eval_param": {
    "domain": "preprocessing",
    "name": "train_test_split",
    "version": "0.0.1",
    "attr_paths": ["train_size", "test_size", "random_state", "shuffle"],
    "attrs": [
      {
        "f": 0.75
      },
      {
        "f": 0.25
      },
      {
        "i64": 1234
      },
      {
        "b": true
      }
    ],
    "inputs": [
      {
        "type": "sf.table.vertical_table",
        "meta": {
          "@type": "type.googleapis.com/secretflow.component.VerticalTable",
          "schemas": [
            {
              "ids": ["id1"],
              "features": ["item", "feature1"],
              "types": ["f32", "f32"],
              "labels": ["y"]
            },
            {
              "ids": ["id2"],
              "features": ["feature2"],
              "types": ["f32"]
            }
          ]
        },
        "data_refs": [
          {
            "uri": "psi_out.csv",
            "party": "alice",
            "format": "csv"
          },
          {
            "uri": "psi_out.csv",
            "party": "bob",
            "format": "csv"
          }
        ]
      }
    ],
    "output_uris": ["train_dataset.csv", "test_dataset.csv"]
  }
}
```

参数如下：

| 参数名称     | 类型    | 描述       |
| ------------ | ------- | ---------- |
| train_size   | float   | 训练集占比 |
| test_size    | float   | 测试集占比 |
| random_state | integer | 随机种子   |
| shuffle      | bool    | 是否清洗   |
