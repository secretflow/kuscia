# Kuscia 入门教程 —— 了解 KusciaJob

您将会尝试自己配置并且提交一个作业，在这个过程中，您将会认识 Kuscia 中的重要概念 —— **KusciaJob**。

## 准备集群

这里假设您已经在机器上部署并且启动了一个示例集群。如果您还没有这样做，请先按照 [快速开始][quickstart] 的指引启动一个集群。

[quickstart]: ./quickstart_cn.md

## 准备数据

您可以使用 Kuscia 中自带的数据文件，或者使用您自己的数据文件。

在 Kuscia 中，节点数据文件的存放路径为节点容器的 `/home/kuscia/var/storage`，您可以在容器中查看这个数据文件。

### 查看 Kuscia 示例数据

这里以 Alice 节点为例，首先进入节点容器：

```shell
docker exec -it ${USER}-kuscia-lite-alice bash
```

在 Alice 节点容器中查看节点示例数据：

```shell
cat /home/kuscia/var/storage/data/alice.csv
```

Bob 节点同理。

{#prepare-your-own-data}

### 准备您自己的数据

您也可以使用您自己的数据文件，还是以 Alice 节点为例：

1. 将自己的数据文件拷贝到容器中。

```shell
docker cp {your_alice_data} ${USER}-kuscia-lite-alice:/home/kuscia/var/storage/data/
```

2. 使用 [KusciaAPI](../reference/apis/summary_cn) 创建 [DomainData](../reference/apis/domaindata_cn)，得到 `domaindata_id` 。
3. SecretFlow 引擎任务需要获得合作方数据的 schema 信息，使用 [KusciaAPI](../reference/apis/summary_cn) 创建 [DomainDataGrant](../reference/apis/domaindatagrant_cn) 进行数据授权，将 alice 数据 schema 信息授权给 bob 。

Bob 节点准备数据重复上述 1、2、3 操作。

{#configure-kuscia-job}

## 配置 KusciaJob

需要在 kuscia-master 节点容器中配置和运行 Job，首先，让我们先进入 kuscia-master 节点容器：

```shell
docker exec -it ${USER}-kuscia-master bash
```

### 使用 Kuscia 示例数据配置 KusciaJob

此处以 [KusciaJob 示例](../reference/concepts/kusciajob_cn.md#创建-kusciajob)作为展示，该任务流完成 2 个任务：

1. job-psi 读取 alice 和 bob 的数据文件，进行隐私求交，求交的结果分别保存为两个参与方的 `psi-output.csv`。
2. job-split 读取 alice 和 bob 上一步中求交的结果文件，并拆分成训练集和测试集，分别保存为两个参与方的 `train-dataset.csv`、`test-dataset.csv`。

这个 KusciaJob 的名称为 job-best-effort-linear，在一个 Kuscia 集群中，这个名称必须是唯一的，由 `.metadata.name` 指定。

### 使用您自己的数据配置 KusciaJob

如果您要使用您自己的数据，可以将两个算子中的 `taskInputConfig.sf_input_ids` 的数据文件 `id` 修改为您在 [准备您自己的数据](#prepare-your-own-data) 中的 `domaindata_id` 即可。

## 运行 KusciaJob

现在已经配置好了一个 KusciaJob ，接下来，运行此 KusciaJob， 在 kuscia-master 容器中执行 ：

```shell
kubectl apply -f job-best-effort-linear.yaml
```

## 查看 KusciaJob 运行状态

现在提交了这个 KusciaJob ，接下来可以在 kuscia-master 容器中通过下面的命令查看 KusciaJob 的运行情况。

### 查看所有的 KusciaJob

```shell
kubectl get kj -n cross-domain
```

您可以看到如下输出：

```shell
NAME                     STARTTIME   COMPLETIONTIME   LASTRECONCILETIME   PHASE
job-best-effort-linear   3s                           3s                  Running
```

job-best-effort-linear 就是我们刚刚创建出来的 KusciaJob 。

### 查看运行中的某个 KusciaJob 的详细状态

通过指定 `-o yaml` 参数，可以以 Yaml 的形式看到 KusciaJob 的详细状态。job-best-effort-linear 是您在 [配置 Job](#configure-kuscia-job) 中指定的 KusciaJob 的名称。

```shell
kubectl get kj job-best-effort-linear -n cross-domain -o yaml
```

如果任务成功了，您可以看到如下输出：

```shell
apiVersion: kuscia.secretflow/v1alpha1
kind: KusciaJob
metadata:
  creationTimestamp: "2023-03-30T12:11:41Z"
  generation: 1
  name: job-best-effort-linear
  namespace: cross-domain
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
    alias: job-psi
    taskInputConfig: '{"sf_datasource_config":{"alice":{"id":"default-data-source"},"bob":{"id":"default-data-source"}},"sf_cluster_desc":{"parties":["alice","bob"],"devices":[{"name":"spu","type":"spu","parties":["alice","bob"],"config":"{\"runtime_config\":{\"protocol\":\"SEMI2K\",\"field\":\"FM128\"},\"link_desc\":{\"connect_retry_times\":60,\"connect_retry_interval_ms\":1000,\"brpc_channel_protocol\":\"http\",\"brpc_channel_connection_type\":\"pooled\",\"recv_timeout_ms\":1200000,\"http_timeout_ms\":1200000}}"},{"name":"heu","type":"heu","parties":["alice","bob"],"config":"{\"mode\":\"PHEU\", \"schema\": \"paillier\", \"key_size\":2048}"}],"ray_fed_config":{"cross_silo_comm_backend":"brpc_link"}},"sf_node_eval_param":{"domain":"data_prep","name":"psi","version":"1.0.0","attr_paths":["input/input_ds1/keys","input/input_ds2/keys","protocol","sort_result","receiver_parties","allow_empty_result","join_type","input_ds1_keys_duplicated","input_ds2_keys_duplicated"],"attrs":[{"is_na":false,"ss":["id1"]},{"is_na":false,"ss":["id2"]},{"is_na":false,"s":"PROTOCOL_RR22"},{"b":true,"is_na":false},{"is_na":false,"ss":["alice","bob"]},{"is_na":true},{"is_na":false,"s":"inner_join"},{"b":true,"is_na":false},{"b":true,"is_na":false}]},"sf_input_ids":["alice-table","bob-table"],"sf_output_ids":["psi-output-0","psi-output-1"],"sf_output_uris":["psi-output-0","psi-output-1"]}'
    tolerable: false
  - appImage: secretflow-image
    dependencies:
    - job-psi
    parties:
    - domainID: alice
    - domainID: bob
    priority: 100
    taskID: job-split
    alias: job-split
    taskInputConfig: '{"sf_datasource_config":{"alice":{"id":"default-data-source"},"bob":{"id":"default-data-source"}},"sf_cluster_desc":{"parties":["alice","bob"],"devices":[{"name":"spu","type":"spu","parties":["alice","bob"],"config":"{\"runtime_config\":{\"protocol\":\"SEMI2K\",\"field\":\"FM128\"},\"link_desc\":{\"connect_retry_times\":60,\"connect_retry_interval_ms\":1000,\"brpc_channel_protocol\":\"http\",\"brpc_channel_connection_type\":\"pooled\",\"recv_timeout_ms\":1200000,\"http_timeout_ms\":1200000}}"},{"name":"heu","type":"heu","parties":["alice","bob"],"config":"{\"mode\":\"PHEU\", \"schema\": \"paillier\", \"key_size\": 2048}"}],"ray_fed_config":{"cross_silo_comm_backend":"brpc_link"}},"sf_node_eval_param":{"domain":"data_prep","name":"train_test_split","version":"1.0.0"},"sf_output_uris":["train-dataset.csv","test-dataset.csv"],"sf_output_ids":["train-dataset","test-dataset"],"sf_input_ids":["psi-output-0"]}'
    tolerable: false
status:
  completionTime: "2023-03-30T12:12:07Z"
  conditions:
  - lastTransitionTime: "2023-03-30T12:11:41Z"
     status: "True"
    type: JobValidated
  - lastTransitionTime: "2023-03-30T12:11:41Z"
    status: "True"
    type: JobCreateInitialized
  - lastTransitionTime: "2023-03-30T12:11:42Z"
    status: "True"
    type: JobCreateSucceeded
  - lastTransitionTime: "2023-03-30T12:11:44Z"
    status: "True"
    type: JobStartInitialized
  - lastTransitionTime: "2023-03-30T12:11:44Z"
    status: "True"
    type: JobStartSucceeded
  lastReconcileTime: "2023-03-30T12:12:07Z"
  phase: Succeeded
  startTime: "2023-03-30T12:11:41Z"
  taskStatus:
    job-psi: Succeeded
    job-split: Succeeded
```

`status` 字段记录了 KusciaJob 的运行状态，`.status.phase` 字段描述了 KusciaJob 的整体状态，而 `.status.taskStatus` 则描述了每个 KusciaTask 的状态。
详细信息请参考 [KusciaJob](../reference/concepts/kusciajob_cn.md) 。

### 查看 KusciaJob 中的某个 KusciaTask 的详细状态

KusciaJob 中的每一个 KusciaTask 都有一个 `taskID` ，通过 `taskID` 我们可以查看某个 KusciaTask 的详细状态。

```shell
kubectl get kt job-psi -n cross-domain -o yaml
```

KusciaTask 的信息这里不再赘述，请查看 [KusciaTask](../reference/concepts/kusciatask_cn.md) 。

## 删除 KusciaJob

当您想清理这个 KusciaJob 时，您可以通过下面的命令完成：

```shell
kubectl delete kj job-best-effort-linear -n cross-domain
```

当这个 KusciaJob 被清理时， 这个 KusciaJob 创建的 KusciaTask 也会一起被清理。

## 接下来

恭喜！您已经完成了 Kuscia 的入门教程。接下来，您可以：

- 进一步阅读 [Kuscia 架构细节][architecture]，了解 Kuscia 的设计思路和概念。
- 了解 [Kuscia API][kuscia-api]。Kuscia API 是 Kuscia 的一个更上层封装，支持更方便地将 Kuscia 集成到其他系统中。
- 了解 [多机器部署][deploy-p2p] 的更多信息。
- 尝试运行其它算法或是引擎的作业，比如 [FATE 作业][tutorial-fate]。

[architecture]: ../reference/architecture_cn.md
[kuscia-api]: ../reference/apis/summary_cn.md
[deploy-p2p]: ../deployment/Docker_deployment_kuscia/deploy_p2p_cn.md
[tutorial-fate]: ../tutorial/run_fate_cn.md

:::{tip}

如果您想要停止并清理入门教程使用的示例集群，可以查阅 [相关指引][stop-and-uninstall]。

[stop-and-uninstall]: ./quickstart_cn.md#停止体验集群

:::
