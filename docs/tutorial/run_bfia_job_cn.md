# 如何运行一个互联互通银联 BFIA 协议作业

> Tips：由于内部 Kuscia P2P 协议升级，当前版本暂不支持银联 BFIA 协议，适配银联 BFIA 正在进行中。

若你使用第三方算法镜像提交互联互通作业，强烈建议你检查镜像安全性，同时参考[安全加固方案](./security_plan_cn.md)。

本教程以秘密分享-逻辑回归（SS-LR）算子为示例，介绍如何通过互联互通银联 BFIA（Beijing FinTech Industry Alliance 北京金融科技产业联盟）协议运行一个包含两方任务的作业。 

在本教程中，通过两个 Kuscia Autonomy 节点来模拟不同框架底座的节点。在这两个节点之间，通过互联互通银联 BFIA 协议运行一个包含两方任务的作业。

## 准备环境

### 准备运行银联 BFIA 协议的节点

部署运行银联 BFIA 协议的节点，请参考 [快速入门点对点组网模式](../getting_started/quickstart_cn.md/#点对点组网模式)。

在执行启动集群命令时，需要新增一个命令行参数`-p bfia`，详细命令如下：

```shell
# 启动集群，会拉起两个 docker 容器，分别表示 Autonomy 节点 alice 和 bob。
./start_standalone.sh p2p -p bfia
```

### 准备工具脚本

```shell
docker cp ${USER}-kuscia-autonomy-alice:/home/kuscia/scripts/user/bfia/ .
```

### 准备秘密分享-逻辑回归（SS-LR）算子镜像

1. 准备 alice 节点中算子镜像

```shell
./bfia/prepare_ss_lr_image.sh -k ${USER}-kuscia-autonomy-alice
```

2. 准备 bob 节点中算子镜像

```shell
./bfia/prepare_ss_lr_image.sh -k ${USER}-kuscia-autonomy-bob
```

3. 工具帮助信息

```shell
./bfia/prepare_ss_lr_image.sh -h
```

### 部署 TTP Server 服务

因为秘密分享-逻辑回归（SS-LR）算子依赖一个可信第三方 TTP（Trusted Third Patry）Server，所以本教程使用本地 Docker 容器的方式和运行银联 BFIA 协议的节点容器部署在同一套环境中，
从而方便快速搭建运行互联互通银联 BFIA 协议作业的体验环境。

1. 部署 TTP Server 服务 

```shell
./bfia/deploy_ttp_server.sh
```

2. 工具帮助信息

```shell
./bfia/deploy_ttp_server.sh -h
```

## 准备数据

你可以使用 Kuscia 中自带的数据文件，或者使用你自己的数据文件。

在 Kuscia 中，节点数据文件的存放路径为节点容器的`/home/kuscia/var/storage`，你可以在容器中查看这个数据文件。

### 查看 Kuscia 示例数据

#### 查看 alice 节点示例数据

```shell
docker exec -it ${USER}-kuscia-autonomy-alice more /home/kuscia/var/storage/data/perfect_logit_a.csv
```

#### 查看 bob 节点示例数据

```shell
docker exec -it ${USER}-kuscia-autonomy-bob more /home/kuscia/var/storage/data/perfect_logit_b.csv
```

### 准备你自己的数据

你也可以使用你自己的数据文件，首先你要将数据文件复制到节点容器中，以 alice 节点为例：

```shell
docker cp {your_alice_data} ${USER}-kuscia-autonomy-alice:/home/kuscia/var/storage/data/
```

接下来你可以像 [查看 Kuscia 示例数据](#kuscia) 一样查看你的数据文件，这里不再赘述。

## 提交一个银联 BFIA 协议的作业 

目前在 Kuscia 中有两种方式提交银联 BFIA 协议的作业
- 通过配置 KusciaJob 提交作业
- 通过银联 BFIA 协议创建作业 API 接口提交作业


{#configure-bfia-kuscia-job}
### 通过配置 KusciaJob 提交作业

数据准备好之后，我们将 alice 作为任务发起方，进入 alice 节点容器中，然后配置和运行作业。

```shell
docker exec -it ${USER}-kuscia-autonomy-alice bash
```

#### 使用 Kuscia 示例数据配置 KusciaJob

下面的示例展示了一个 KusciaJob， 该作业包含 1 个任务

- 算子通过读取 alice 和 bob 的数据文件，完成秘密分享逻辑回归任务。

- KusciaJob 的名称为 job-ss-lr，在一个 Kuscia 集群中，这个名称必须是唯一的，由`.metadata.name`指定。

在 alice 容器中，创建文件 job-ss-lr.yaml，内容如下：

```yaml
apiVersion: kuscia.secretflow/v1alpha1
kind: KusciaJob
metadata:
  name: job-ss-lr
  namespace: cross-domain
spec:
  initiator: alice
  tasks:
  - alias: ss_lr_1
    appImage: ss-lr
    parties:
    - domainID: alice
      role: host
    - domainID: bob
      role: guest
    taskInputConfig: '{"name":"ss_lr_1","module_name":"ss_lr","output":[{"type":"dataset","key":"result"}],"role":{"host":["alice"],"guest":["bob"]},"initiator":{"role":"host","node_id":"alice"},"task_params":{"host":{"0":{"has_label":true,"name":"perfect_logit_a.csv","namespace":"data"}},"guest":{"0":{"has_label":false,"name":"perfect_logit_b.csv","namespace":"data"}},"common":{"skip_rows":1,"algo":"ss_lr","protocol_families":"ss","batch_size":21,"last_batch_policy":"discard","num_epoch":1,"l0_norm":0,"l1_norm":0,"l2_norm":0.5,"optimizer":"sgd","learning_rate":0.0001,"sigmoid_mode":"minimax_1","protocol":"semi2k","field":64,"fxp_bits":18,"trunc_mode":"probabilistic","shard_serialize_format":"raw","use_ttp":true,"ttp_server_host":"ttp-server:9449","ttp_session_id":"interconnection-root","ttp_adjust_rank":0}}}'
    tolerable: false
```

#### 算子参数描述

KusciaJob 中算子参数由`taskInputConfig`字段定义，对于不同的算子，算子的参数不同
- 秘密分享-逻辑回归（SS-LR）算子相关信息可参考 [SS-LR 参考实现](https://github.com/secretflow/interconnection-impl)
- 本教程秘密分享-逻辑回归（SS-LR）算子对应的 KusciaJob TaskInputConfig 结构可参考 [TaskInputConfig 结构示例](#ss-lr-task-input-config)

#### 提交 KusciaJob

现在我们已经配置好了一个 KusciaJob，接下来，让我们运行以下命令提交这个 KusciaJob。

```shell
kubectl apply -f job-ss-lr.yaml
```

### 通过银联 BFIA 协议 API 接口提交作业

数据准备好之后，我们将 alice 作为任务发起方，进入 alice 节点容器中。

```shell
docker exec -it ${USER}-kuscia-autonomy-alice bash
```

下面使用银联 BFIA 协议创建作业接口提交作业，该作业会提交给 Kuscia 互联互通 InterConn 控制器，该控制器将银联 BFIA 协议规定的创建作业请求参数转化为 Kuscia 中的 KusciaJob 作业定义。
最后，InterConn 控制器在 Kuscia 中创建 KusciaJob 资源。

```shell
curl -v -X POST 'http://127.0.0.1:8084/v1/interconn/schedule/job/create' \
--header 'Content-Type: application/json' \
-d '{"job_id":"job-ss-lr","dag":{"version":"2.0.0","components":[{"code":"ss-lr","name":"ss_lr_1","module_name":"ss_lr","version":"v1.0.0","input":[],"output":[{"type":"dataset","key":"result"}]}]},"config":{"role":{"host":["alice"],"guest":["bob"]},"initiator":{"role":"host","node_id":"alice"},"job_params":{"host":{"0":{}},"guest":{"0":{}}},"task_params":{"host":{"0":{"ss_lr_1":{"name":"perfect_logit_a.csv","namespace":"data","has_label":true}}},"arbiter":{},"guest":{"0":{"ss_lr_1":{"name":"perfect_logit_b.csv","namespace":"data","has_label":false}}},"common":{"ss_lr_1":{"skip_rows":1,"algo":"ss_lr","protocol_families":"ss","batch_size":21,"last_batch_policy":"discard","num_epoch":1,"l0_norm":0,"l1_norm":0,"l2_norm":0.5,"optimizer":"sgd","learning_rate":0.0001,"sigmoid_mode":"minimax_1","protocol":"semi2k","field":64,"fxp_bits":18,"trunc_mode":"probabilistic","shard_serialize_format":"raw","use_ttp":true,"ttp_server_host":"ttp-server:9449","ttp_session_id":"interconnection-root","ttp_adjust_rank":0}}},"version":"2.0.0"}}'

```

提交作业接口请求参数内容结构请参考 [提交 SS-LR 作业接口请求内容示例](#bfia-create-job-req-body)。


{#get-kuscia-job-phase}
## 查看 KusciaJob 运行状态

在提交完 KusciaJob 作业后，我们可以在 alice 容器中通过下面的命令查看 alice 方的 KusciaJob 的运行情况。
同样，也可以登陆到 bob 容器中查看 bob 方的 KusciaJob 的运行情况。下面以 alice 节点容器为例。

### 查看所有的 KusciaJob

```shell
kubectl get kj -n cross-domain
```

你可以看到如下输出：

```shell
NAME            STARTTIME   COMPLETIONTIME   LASTRECONCILETIME   PHASE
job-ss-lr       3s                           3s                  Running
```

* job-ss-lr  就是我们刚刚创建出来的 KusciaJob。

### 查看运行中的 KusciaJob 的详细状态

通过指定`-o yaml`参数，我们能够以 Yaml 的形式看到 KusciaJob 的详细状态。job-ss-lr 是提交的作业名称。

```shell
kubectl get kj job-ss-lr -n cross-domain -o yaml
```

如果任务成功了，你可以看到如下输出：

```yaml
apiVersion: kuscia.secretflow/v1alpha1
kind: KusciaJob
metadata:
  creationTimestamp: "2023-07-01T02:21:04Z"
  generation: 3
  labels:
    kuscia.secretflow/interconn-protocol-type: bfia
    kuscia.secretflow/self-cluster-as-initiator: "true"
  name: job-ss-lr
  namespace: cross-domain
  resourceVersion: "50438"
  uid: 408a03ae-69c2-4fa8-a638-b47b6dbf530f
spec:
  initiator: alice
  maxParallelism: 2
  scheduleMode: Strict
  stage: Start
  tasks:
  - alias: ss_lr_1
    appImage: ss-lr
    parties:
    - domainID: alice
      role: host
    - domainID: bob
      role: guest
    taskID: job-ss-lr-26e3489ac66e
    taskInputConfig: '{"name":"ss_lr_1","module_name":"ss_lr","output":[{"type":"dataset","key":"result"}],"role":{"host":["alice"],"guest":["bob"]},"initiator":{"role":"host","node_id":"alice"},"task_params":{"host":{"0":{"has_label":true,"name":"perfect_logit_a.csv","namespace":"data"}},"guest":{"0":{"has_label":false,"name":"perfect_logit_b.csv","namespace":"data"}},"common":{"algo":"ss_lr","batch_size":21,"field":64,"fxp_bits":18,"l0_norm":0,"l1_norm":0,"l2_norm":0.5,"last_batch_policy":"discard","learning_rate":0.0001,"num_epoch":1,"optimizer":"sgd","protocol":"semi2k","protocol_families":"ss","shard_serialize_format":"raw","sigmoid_mode":"minimax_1","skip_rows":1,"trunc_mode":"probabilistic","ttp_adjust_rank":0,"ttp_server_host":"ttp-server:9449","ttp_session_id":"interconnection-root","use_ttp":true}}}'
    tolerable: false
status:
  completionTime: "2023-07-01T02:21:14Z"
  conditions:
  - lastTransitionTime: "2023-07-01T02:21:04Z"
    status: "True"
    type: JobValidated
  - lastTransitionTime: "2023-07-01T02:21:04Z"
    status: "True"
    type: JobCreateInitialized
  - lastTransitionTime: "2023-07-01T02:21:04Z"
    status: "True"
    type: JobCreateSucceeded
  - lastTransitionTime: "2023-07-01T02:21:04Z"
    status: "True"
    type: JobStartInitialized
  - lastTransitionTime: "2023-07-01T02:21:04Z"
    status: "True"
    type: JobStartSucceeded
  lastReconcileTime: "2023-07-01T02:21:14Z"
  phase: Succeeded
  startTime: "2023-07-01T02:21:04Z"
  taskStatus:
    job-ss-lr-26e3489ac66e: Succeeded
```

- `status`字段记录了 KusciaJob 的运行状态，`.status.phase`字段描述了 KusciaJob 的整体状态，而 `.status.taskStatus`则描述了包含的 KusciaTask 的状态。
详细信息请参考 [KusciaJob](../reference/concepts/kusciajob_cn.md)。

### 查看 KusciaJob 中 KusciaTask 的详细状态。

KusciaJob 中的每一个 KusciaTask 都有一个`taskID`，通过`taskID`我们可以查看 KusciaTask 的详细状态。

```shell
kubectl get kt job-ss-lr-26e3489ac66e -n cross-domain -o yaml
```

KusciaTask 的介绍，请参考 [KusciaTask](../reference/concepts/kusciatask_cn.md)。

## 查看 SS-LR 算子运行结果

可以通过 [查看 KusciaJob 运行状态](#get-kuscia-job-phase) 查询作业的运行状态。 当作业状态 PHASE 变成 `Succeeded` 时，可以查看算子输出结果。

1. 进入节点 alice 或 bob 容器
 
- 若已经在容器中，跳过该步骤

```shell
# 进入 alice 节点容器
docker exec -it ${USER}-kuscia-autonomy-alice bash

# 进入 bob 节点容器
docker exec -it ${USER}-kuscia-autonomy-bob bash
```

2. 查看 KusciaJob 作业状态

```shell
kubectl get kj job-ss-lr -n cross-domain
NAME        STARTTIME   COMPLETIONTIME   LASTRECONCILETIME   PHASE
job-ss-lr   13s         2s               2s                  Succeeded
```

3. 查看 SS-LR 算子输出结果

- 输出内容表示 SS-LR 算子权重向量的密态分片

```shell
# 在 alice 容器中查看输出结果
more /home/kuscia/var/storage/job-ss-lr-host-0/job-ss-lr-{random-id}-result

# 在 bob 容器中查看输出结果
more /home/kuscia/var/storage/job-ss-lr-guest-0/job-ss-lr-{random-id}-result
```

## 删除 KusciaJob

当你想清理这个 KusciaJob 时，你可以通过下面的命令完成：

```shell
kubectl delete kj job-ss-lr -n cross-domain
```

当这个 KusciaJob 被清理时， 这个 KusciaJob 创建的 KusciaTask 也会一起被清理。


## 参考

{#ss-lr-task-input-config}

### SS-LR 算子对应的 TaskInputConfig 结构示例

```json
{
  "name": "ss_lr_1",
  "module_name": "ss_lr",
  "input":[],
  "output": [{
    "type": "dataset",
    "key": "result"
  }],
  "role": {
    "host": ["alice"],
    "guest": ["bob"]
  },
  "initiator": {
    "role": "host",
    "node_id": "alice"
  },
  "task_params": {
    "host": {
      "0": {
        "has_label": true,
        "name": "perfect_logit_a.csv",
        "namespace": "data"
      }
    },
    "guest": {
      "0": {
        "has_label": false,
        "name": "perfect_logit_b.csv",
        "namespace": "data"
      }
    },
    "common": {
      "skip_rows": 1,
      "algo": "ss_lr",
      "protocol_families": "ss",
      "batch_size": 21,
      "last_batch_policy": "discard",
      "num_epoch": 1,
      "l0_norm": 0,
      "l1_norm": 0,
      "l2_norm": 0.5,
      "optimizer": "sgd",
      "learning_rate": 0.0001,
      "sigmoid_mode": "minimax_1",
      "protocol": "semi2k",
      "field": 64,
      "fxp_bits": 18,
      "trunc_mode": "probabilistic",
      "shard_serialize_format": "raw",
      "use_ttp": true,
      "ttp_server_host": "ttp-server:9449",
      "ttp_session_id": "interconnection-root",
      "ttp_adjust_rank": 0
    }
  }
}
```

字段说明

- `name`描述了任务算子的名称。
- `module_name`描述了任务算子所属模块名称。
- `input`描述了任务算子的输入，若任务不依赖其他任务的输出，则可以将该项置为空。
- `output`描述了任务算子的输出。
- `role`描述了任务的角色。
- `initiator`描述了任务发起方的信息。
- `task_params`描述了任务算子依赖的参数。


{#bfia-create-job-req-body}

### 提交 SS-LR 作业接口请求内容示例

```json
{
  "job_id": "job-ss-lr",
  "dag": {
    "version": "2.0.0",
    "components": [{
      "code": "ss-lr",
      "name": "ss_lr_1",
      "module_name": "ss_lr",
      "version": "v1.0.0",
      "input": [],
      "output": [{
        "type": "dataset",
        "key": "result"
      }]
    }]
  },
  "config": {
    "role": {
      "host": ["alice"],
      "guest": ["bob"]
    },
    "initiator": {
      "role": "host",
      "node_id": "alice"
    },
    "job_params": {
      "host": {
        "0": {}
      },
      "guest": {
        "0": {}
      }
    },
    "task_params": {
      "host": {
        "0": {
          "ss_lr_1": {
            "name": "perfect_logit_a.csv",
            "namespace": "data",
            "has_label": true
          }
        }
      },
      "arbiter": {},
      "guest": {
        "0": {
          "ss_lr_1": {
            "name": "perfect_logit_b.csv",
            "namespace": "data",
            "has_label": false
          }
        }
      },
      "common": {
        "ss_lr_1": {
          "skip_rows": 1,
          "algo": "ss_lr",
          "protocol_families": "ss",
          "batch_size": 21,
          "last_batch_policy": "discard",
          "num_epoch": 1,
          "l0_norm": 0,
          "l1_norm": 0,
          "l2_norm": 0.5,
          "optimizer": "sgd",
          "learning_rate": 0.0001,
          "sigmoid_mode": "minimax_1",
          "protocol": "semi2k",
          "field": 64,
          "fxp_bits": 18,
          "trunc_mode": "probabilistic",
          "shard_serialize_format": "raw",
          "use_ttp": true,
          "ttp_server_host": "ttp-server:9449",
          "ttp_session_id": "interconnection-root",
          "ttp_adjust_rank": 0
        }
      }
    },
    "version": "2.0.0"
  }
}
```

##### 字段说明

- `job_id`描述了作业的标识。
- `dag`描述了作业的组件之间组合的配置。
- `config`描述了作业运行时的参数配置。

