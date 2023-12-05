# FATE 作业运行失败

## 排查步骤

### 检查机器配置

若机器不满足[官网推荐配置](https://www.secretflow.org.cn/docs/kuscia/latest/zh-Hans/getting_started/quickstart_cn#id2)，可能会造成部分服务无法正常工作，从而导致作业运行失败。

### 检查 fate-alice 容器是否运行正常

查看 fate-alice 容器日志，检查是否运行正常

```shell
docker logs -f fate-alice

# 正常日志输出示例:
wait to upload data, sleep 10
...
wait to upload data, sleep 10
/data/projects
upload guest data
{
    "data": {
        "board_url": "http://172.17.0.2:8080/index.html#/dashboard?job_id=202307270223007537850&role=local&party_id=0",
        "code": 0,
        "dsl_path": "/data/projects/fate/fateflow/jobs/202307270223007537850/job_dsl.json",
        "job_id": "202307270223007537850",
        "logs_directory": "/data/projects/fate/fateflow/logs/202307270223007537850",
        "message": "success",
        "model_info": {
            "model_id": "local-0#model",
            "model_version": "202307270223007537850"
        },
        "namespace": "experiment",
        "pipeline_dsl_path": "/data/projects/fate/fateflow/jobs/202307270223007537850/pipeline_dsl.json",
        "runtime_conf_on_party_path": "/data/projects/fate/fateflow/jobs/202307270223007537850/local/0/job_runtime_on_party_conf.json",
        "runtime_conf_path": "/data/projects/fate/fateflow/jobs/202307270223007537850/job_runtime_conf.json",
        "table_name": "lr_guest",
        "train_runtime_conf_path": "/data/projects/fate/fateflow/jobs/202307270223007537850/train_runtime_conf.json"
    },
    "jobId": "202307270223007537850",
    "retcode": 0,
    "retmsg": "success"
}

sleep 30
upload host data
{
    "data": {
        "board_url": "http://172.17.0.2:8080/index.html#/dashboard?job_id=202307270223327659150&role=local&party_id=0",
        "code": 0,
        "dsl_path": "/data/projects/fate/fateflow/jobs/202307270223327659150/job_dsl.json",
        "job_id": "202307270223327659150",
        "logs_directory": "/data/projects/fate/fateflow/logs/202307270223327659150",
        "message": "success",
        "model_info": {
            "model_id": "local-0#model",
            "model_version": "202307270223327659150"
        },
        "namespace": "experiment",
        "pipeline_dsl_path": "/data/projects/fate/fateflow/jobs/202307270223327659150/pipeline_dsl.json",
        "runtime_conf_on_party_path": "/data/projects/fate/fateflow/jobs/202307270223327659150/local/0/job_runtime_on_party_conf.json",
        "runtime_conf_path": "/data/projects/fate/fateflow/jobs/202307270223327659150/job_runtime_conf.json",
        "table_name": "lr_host",
        "train_runtime_conf_path": "/data/projects/fate/fateflow/jobs/202307270223327659150/train_runtime_conf.json"
    },
    "jobId": "202307270223327659150",
    "retcode": 0,
    "retmsg": "success"
}
```

### 查看 fate-deploy-bob Pod 是否运行正常

登陆查看作业的容器

- 若以中心化组网模式部署，请登陆到 Master 容器中

```shell
docker exec -it ${USER}-kuscia-master bash
```

- 若以点对点组网模式部署，请登陆到 Bob 容器中

```shell
docker exec -it ${USER}-kuscia-autonomy-bob bash
```

检查 fate-deploy-bob Pod 运行情况

- 确保 fate-deploy-bob 前缀开头的 Pod 状态为 Running

```shell
# 查看 bob 节点下的 pod 列表
kubectl get pod -n bob

## 正常输出示例:
NAME                               READY   STATUS    RESTARTS   AGE
fate-deploy-bob-6798765d84-84rm7   1/1     Running   0          6m34s
...

# 若 fate-deploy-bob Pod 状态非 Running，通过以下命令查看 Pod 详细信息，具体原因可以查看 status 字段下的相关内容
kubectl get pod fate-deploy-bob-6798765d84-84rm7 -o yaml -n bob

# 若 fate-deploy-bob Pod 状态为 Pending 且由于机器内存不足无法完成调度，则可以尝试使用下面命令减小 Pod 的请求内存大小
# 不推荐：调整后，虽然 Pod 可以 Running, 但是也可能会由于机器内存不足而导致任务失败
kubectl patch deploy fate-deploy-bob -n bob --patch '{"spec": {"template": {"spec": {"containers": [{"name": "fate-deploy-bob","resources": {"requests": {"memory": "1G"}}}]}}}}'
```

### 查看作业失败原因

请参考[作业运行失败](./runjobfailed.md)