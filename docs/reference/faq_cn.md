# 常见问题

## 部署失败

### 排查步骤

#### 检查机器配置

若机器不满足[官网推荐配置](https://www.secretflow.org.cn/docs/kuscia/latest/zh-Hans/getting_started/quickstart_cn#id2)，可能会造成部分服务无法正常工作，从而导致部署失败。

#### 查看服务日志

##### 中心化组网模式

查看 Master 日志

```shell
# 登陆到 master 容器中
docker exec -it ${USER}-kuscia-master bash

# 查看 kuscia 错误日志
cat /home/kuscia/var/logs/kuscia.log | grep -i error

# 查看 k3s 错误日志
cat /home/kuscia/var/logs/k3s.log | grep -i error
```

查看 Alice 节点日志

```shell
# 登陆到 alice 容器中
docker exec -it ${USER}-kuscia-lite-alice bash

# 查看 kuscia 错误日志
cat /home/kuscia/var/logs/kuscia.log | grep -i error
```

查看 Bob 节点日志

```shell
# 登陆到 bob 容器中
docker exec -it ${USER}-kuscia-lite-bob bash

# 查看 kuscia 错误日志
cat /home/kuscia/var/logs/kuscia.log | grep -i error
```

#####  点对点组网模式

查看 Alice 节点日志

```shell
# 登陆到 alice 容器中
docker exec -it ${USER}-kuscia-autonomy-alice bash

# 查看 kuscia 错误日志
cat /home/kuscia/var/logs/kuscia.log | grep -i error

# 查看 k3s 错误日志
cat /home/kuscia/var/logs/k3s.log | grep -i error
```

查看 Bob 节点日志

```shell
# 登陆到 bob 容器中
docker exec -it ${USER}-kuscia-autonomy-bob bash

# 查看 Kuscia 错误日志
cat /home/kuscia/var/logs/kuscia.log | grep -i error

# 查看 K3s 错误日志
cat /home/kuscia/var/logs/k3s.log | grep -i error
```

{#jon-run-failed}

## 作业运行失败

### 排查步骤

#### 检查机器配置

若机器不满足[官网推荐配置](https://www.secretflow.org.cn/docs/kuscia/latest/zh-Hans/getting_started/quickstart_cn#id2)，可能会造成部分服务无法正常工作，从而导致作业运行失败。

#### 查看作业失败原因

登陆查看作业的容器

- 若以中心化组网模式部署，请登陆到 Master 容器中

```shell
docker exec -it ${USER}-kuscia-master bash
```

- 若以点对点组网模式部署，请登陆到下发作业的容器中，下面以 Alice 容器为例

```shell
docker exec -it ${USER}-kuscia-autonomy-alice bash
```

查看作业信息

```shell
kubectl get kj
```

查看作业下任务的详细信息

- 具体失败原因可以查看 status 字段下的相关内容

```shell
# 列出所有任务
kubectl get kt

# 查看任务的详细信息，任务名称来自上述命令
kubectl get kt {任务名称} -o yaml
```

查看任务 Pod 的详细信息

- 具体失败原因可以查看 status 字段下的相关内容

```shell
# 查看 alice 节点下的 pod 列表
kubectl get pod -n alice

# 查看 alice 节点下某个 pod 的详细信息
kubectl get pod xxxx -o yaml -n alice

# 查看 bob 节点下的 pod 列表
kubectl get pod -n bob

# 查看 bob 节点下某个 pod 的详细信息
kubectl get pod xxxx -o yaml -n bob

# 若 pod 状态为 Pending，可以继续查看相应节点详细信息
kubectl get node

# 查看 node 详细信息
kubectl describe node xxxx
```


查看任务 Pod 详细日志

- 若以中心化组网模式部署，登陆到 Alice 和 Bob 容器命令如下

```shell
# 登陆到 alice 容器中
docker exec -it ${USER}-kuscia-lite-alice bash

# 查看 alice 节点上任务 pod 日志
cat /home/kuscia/var/stdout/pods/podName_xxxx/xxxx/x.log

# 登陆到 bob 容器中
docker exec -it ${USER}-kuscia-lite-bob bash

# 查看 bob 节点上任务 pod 日志
cat /home/kuscia/var/stdout/pods/podName_xxxx/xxxx/x.log
```

- 若以点对点组网模式部署，登陆到 Alice 和 Bob 容器命令如下

```shell
# 登陆到 alice 容器中
docker exec -it ${USER}-kuscia-autonomy-alice bash

# 查看 alice 节点上任务 pod 日志
cat /home/kuscia/var/stdout/pods/podName_xxxx/xxxx/x.log

# 登陆到 bob 容器中
docker exec -it ${USER}-kuscia-autonomy-bob bash

# 查看 bob 节点上任务 pod 日志
cat /home/kuscia/var/stdout/pods/podName_xxxx/xxxx/x.log
```


## Fate 作业运行失败

### 排查步骤

#### 检查机器配置

若机器不满足[官网推荐配置](https://www.secretflow.org.cn/docs/kuscia/latest/zh-Hans/getting_started/quickstart_cn#id2)，可能会造成部分服务无法正常工作，从而导致作业运行失败。

#### 检查 fate-alice 容器是否运行正常

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

#### 查看 fate-deploy-bob Pod 是否运行正常

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

#### 查看作业失败原因

请参考[作业运行失败](#jon-run-failed)



