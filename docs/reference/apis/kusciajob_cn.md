# KusciaJob

在 Kuscia 中，你可以使用 KusciaJob 来表示一个任务流程。请参考 [KusciaJob](../concepts/kusciajob_cn.md) 。
您可以从 [这里](https://github.com/secretflow/kuscia/tree/main/proto/api/v1alpha1/kusciaapi/job.proto) 找到对应的 protobuf 文件。

## 接口总览

| 方法名                                            | 请求类型                       | 响应类型                         | 描述          |
|------------------------------------------------|----------------------------|------------------------------|-------------|
| [CreateJob](#create-job)                       | CreateJobRequest           | CreateJobResponse            | 创建 Job      |
| [QueryJob](#query-job)                         | QueryJobRequest            | QueryJobResponse             | 查询 Job      |
| [BatchQueryJobStatus](#batch-query-job-status) | BatchQueryJobStatusRequest | BatchQueryJobStatusResponse  | 批量查询 Job 状态 |
| [DeleteJob](#delete-job)                       | DeleteJobRequest           | DeleteJobResponse            | 删除 Job      |
| [StopJob](#stop-job)                           | StopJobRequest             | StopJobResponse              | 停止 Job      |
| [WatchJob](#watch-job)                         | WatchJobRequest            | WatchJobEventResponse stream | 监听 Job      |
| [ApproveJob](#approval-job)                    | ApproveJobRequest          | ApproveJobResponse           | 审批 Job      |
| [SuspendJob](#suspend-job)                     | SuspendJobRequest          | SuspendJobResponse           | 暂停 Job      |
| [RestartJob](#restart-job)                     | RestartJobRequest          | RestartJobResponse           | 重跑 Job      |
| [CancelJob](#cancel-job)                       | CancelJobRequest           | CancelJobResponse            | 取消 Job      |

## 接口详情

{#create-job}

### 创建 Job

#### HTTP 路径

/api/v1/job/create

#### 请求（CreateJobRequest）

| 字段              | 类型                                           | 选填 | 描述                                                                                                                         |
|-----------------|----------------------------------------------|----|----------------------------------------------------------------------------------------------------------------------------|
| header          | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容                                                                                                                    |
| job_id          | string                                       | 必填 | JobID，满足 [RFC 1123 标签名规则要求](https://kubernetes.io/zh-cn/docs/concepts/overview/working-with-objects/names/#dns-label-names) |
| initiator       | string                                       | 必填 | 发起方节点 ID                                                                                                                   |
| max_parallelism | int32                                        | 可选 | 并发度，参考 [KusciaJob 概念](../concepts/kusciajob_cn.md)                                                                         |
| tasks           | [Task](#task)[]                              | 必填 | 任务参数                                                                                                                       |
| custom_fields   | map<string, string>                          | 可选 | 自定义参数，会同步给参与方，key不超过38个字符，value不超过63个字符。                                                                                                            |

#### 响应（CreateJobResponse）

| 字段          | 类型                             | 描述    |
|-------------|--------------------------------|-------|
| status      | [Status](summary_cn.md#status) | 状态信息  |
| data        | CreateJobResponseData          |       |
| data.job_id | string                         | JobID |

#### 请求示例

发起请求：

```sh
# Example of execution within a container
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/job/create' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "job_id": "job-alice-bob-001",
  "initiator": "alice",
  "max_parallelism": 2,
  "tasks": [
    {
      "task_id": "job-psi",
      "app_image": "secretflow-image",
      "parties": [
        {
          "domain_id": "alice"
        },
        {
          "domain_id": "bob"
        }
      ],
      "alias": "job-psi",
      "dependencies": [],
      "task_input_config": "{\"sf_datasource_config\":{\"alice\":{\"id\":\"default-data-source\"},\"bob\":{\"id\":\"default-data-source\"}},\"sf_cluster_desc\":{\"parties\":[\"alice\",\"bob\"],\"devices\":[{\"name\":\"spu\",\"type\":\"spu\",\"parties\":[\"alice\",\"bob\"],\"config\":\"{\\\"runtime_config\\\":{\\\"protocol\\\":\\\"REF2K\\\",\\\"field\\\":\\\"FM64\\\"},\\\"link_desc\\\":{\\\"connect_retry_times\\\":60,\\\"connect_retry_interval_ms\\\":1000,\\\"brpc_channel_protocol\\\":\\\"http\\\",\\\"brpc_channel_connection_type\\\":\\\"pooled\\\",\\\"recv_timeout_ms\\\":1200000,\\\"http_timeout_ms\\\":1200000}}\"},{\"name\":\"heu\",\"type\":\"heu\",\"parties\":[\"alice\",\"bob\"],\"config\":\"{\\\"mode\\\": \\\"PHEU\\\", \\\"schema\\\": \\\"paillier\\\", \\\"key_size\\\": 2048}\"}],\"ray_fed_config\":{\"cross_silo_comm_backend\":\"brpc_link\"}},\"sf_node_eval_param\":{\"domain\":\"data_prep\",\"name\":\"psi\",\"version\":\"0.0.5\",\"attr_paths\":[\"protocol\",\"sort_result\",\"allow_duplicate_keys\",\"allow_duplicate_keys/yes/join_type\",\"allow_duplicate_keys/yes/join_type/left_join/left_side\",\"input/receiver_input/key\",\"input/sender_input/key\"],\"attrs\":[{\"s\":\"PROTOCOL_RR22\"},{\"b\":true},{\"s\":\"yes\"},{\"s\":\"left_join\"},{\"ss\":[\"alice\"]},{\"ss\":[\"id1\"]},{\"ss\":[\"id2\"]}]},\"sf_input_ids\":[\"alice-table\",\"bob-table\"],\"sf_output_ids\":[\"psi-output\"],\"sf_output_uris\":[\"psi-output.csv\"]}",
      "priority": 100
    },
    {
      "task_id": "job-split",
      "app_image": "secretflow-image",
      "parties": [
        {
          "domain_id": "alice"
        },
        {
          "domain_id": "bob"
        }
      ],
      "alias": "job-split",
      "dependencies": [
        "job-psi"
      ],
      "task_input_config": "{\"sf_datasource_config\":{\"alice\":{\"id\":\"default-data-source\"},\"bob\":{\"id\":\"default-data-source\"}},\"sf_cluster_desc\":{\"parties\":[\"alice\",\"bob\"],\"devices\":[{\"name\":\"spu\",\"type\":\"spu\",\"parties\":[\"alice\",\"bob\"],\"config\":\"{\\\"runtime_config\\\":{\\\"protocol\\\":\\\"REF2K\\\",\\\"field\\\":\\\"FM64\\\"},\\\"link_desc\\\":{\\\"connect_retry_times\\\":60,\\\"connect_retry_interval_ms\\\":1000,\\\"brpc_channel_protocol\\\":\\\"http\\\",\\\"brpc_channel_connection_type\\\":\\\"pooled\\\",\\\"recv_timeout_ms\\\":1200000,\\\"http_timeout_ms\\\":1200000}}\"},{\"name\":\"heu\",\"type\":\"heu\",\"parties\":[\"alice\",\"bob\"],\"config\":\"{\\\"mode\\\": \\\"PHEU\\\", \\\"schema\\\": \\\"paillier\\\", \\\"key_size\\\": 2048}\"}],\"ray_fed_config\":{\"cross_silo_comm_backend\":\"brpc_link\"}},\"sf_node_eval_param\":{\"domain\":\"data_prep\",\"name\":\"train_test_split\",\"version\":\"0.0.1\",\"attr_paths\":[\"train_size\",\"test_size\",\"random_state\",\"shuffle\"],\"attrs\":[{\"f\":0.75},{\"f\":0.25},{\"i64\":1234},{\"b\":true}]},\"sf_output_uris\":[\"train-dataset.csv\",\"test-dataset.csv\"],\"sf_output_ids\":[\"train-dataset\",\"test-dataset\"],\"sf_input_ids\":[\"psi-output\"]}",
      "priority": 100
    }
  ]
}'
```

请求响应成功结果：

```json
{
  "status": {
    "code": 0,
    "message": "success",
    "details": []
  },
  "data": {
    "job_id": "job-alice-bob-001"
  }
}
```

:::{tip}

上述请求示例中的引擎镜像基于 SecretFlow `1.7.0b0` 版本。算子参数的 `taskInputConfig` 内容可参考[KusciaJob](../concepts/kusciajob_cn.md#创建-kusciajob)

:::

{#query-job}

### 查询 Job

#### HTTP 路径

/api/v1/job/query

#### 请求（QueryJobRequest）

| 字段     | 类型                                           | 选填 | 描述      |
|--------|----------------------------------------------|----|---------|
| header | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容 |
| job_id | string                                       | 必填 | JobID   |

#### 响应（QueryJobResponse）

| 字段                   | 类型                                    | 描述     |
|----------------------|---------------------------------------|--------|
| status               | [Status](summary_cn.md#status)        | 状态信息   |
| data                 | QueryJobResponseData                  |        |
| data.job_id          | string                                | JobID  |
| data.initiator       | string                                | 发起方    |
| data.max_parallelism | int32                                 | 并发度    |
| data.tasks           | [TaskConfig](#task-config)[]          | 任务列表   |
| data.status          | [JobStatusDetail](#job-status-detail) | Job 状态 |
| data.custom_fields | map<string, string> | 可选 | 自定义参数                                                                                                             |

#### 请求示例

发起请求：

```sh
# Example of execution within a container
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/job/query' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "job_id": "job-alice-bob-001"
}'
```

请求响应成功结果：

```json
{
  "status": {
    "code": 0,
    "message": "success",
    "details": []
  },
  "data": {
    "job_id": "job-alice-bob-001",
    "initiator": "alice",
    "max_parallelism": 2,
    "tasks": [
      {
        "app_image": "secretflow-image",
        "parties": [
          {
            "domain_id": "alice"
          },
          {
            "domain_id": "bob"
          }
        ],
        "alias": "job-psi",
        "task_id": "job-psi",
        "dependencies": [
          ""
        ],
        "task_input_config": "{\"sf_datasource_config\":{\"alice\":{\"id\":\"default-data-source\"},\"bob\":{\"id\":\"default-data-source\"}},\"sf_cluster_desc\":{\"parties\":[\"alice\",\"bob\"],\"devices\":[{\"name\":\"spu\",\"type\":\"spu\",\"parties\":[\"alice\",\"bob\"],\"config\":\"{\\\"runtime_config\\\":{\\\"protocol\\\":\\\"REF2K\\\",\\\"field\\\":\\\"FM64\\\"},\\\"link_desc\\\":{\\\"connect_retry_times\\\":60,\\\"connect_retry_interval_ms\\\":1000,\\\"brpc_channel_protocol\\\":\\\"http\\\",\\\"brpc_channel_connection_type\\\":\\\"pooled\\\",\\\"recv_timeout_ms\\\":1200000,\\\"http_timeout_ms\\\":1200000}}\"},{\"name\":\"heu\",\"type\":\"heu\",\"parties\":[\"alice\",\"bob\"],\"config\":\"{\\\"mode\\\": \\\"PHEU\\\", \\\"schema\\\": \\\"paillier\\\", \\\"key_size\\\": 2048}\"}],\"ray_fed_config\":{\"cross_silo_comm_backend\":\"brpc_link\"}},\"sf_node_eval_param\":{\"domain\":\"data_prep\",\"name\":\"psi\",\"version\":\"0.0.5\",\"attr_paths\":[\"protocol\",\"sort_result\",\"allow_duplicate_keys\",\"allow_duplicate_keys/yes/join_type\",\"allow_duplicate_keys/yes/join_type/left_join/left_side\",\"input/receiver_input/key\",\"input/sender_input/key\"],\"attrs\":[{\"s\":\"PROTOCOL_RR22\"},{\"b\":true},{\"s\":\"yes\"},{\"s\":\"left_join\"},{\"ss\":[\"alice\"]},{\"ss\":[\"id1\"]},{\"ss\":[\"id2\"]}]},\"sf_input_ids\":[\"alice-table\",\"bob-table\"],\"sf_output_ids\":[\"psi-output\"],\"sf_output_uris\":[\"psi-output.csv\"]}",
        "priority": 100
      },
      {
        "app_image": "secretflow-image",
        "parties": [
          {
            "domain_id": "alice"
          },
          {
            "domain_id": "bob"
          }
        ],
        "alias": "job-split",
        "task_id": "job-split",
        "dependencies": [
          "job-psi"
        ],
        "task_input_config": "{\"sf_datasource_config\":{\"alice\":{\"id\":\"default-data-source\"},\"bob\":{\"id\":\"default-data-source\"}},\"sf_cluster_desc\":{\"parties\":[\"alice\",\"bob\"],\"devices\":[{\"name\":\"spu\",\"type\":\"spu\",\"parties\":[\"alice\",\"bob\"],\"config\":\"{\\\"runtime_config\\\":{\\\"protocol\\\":\\\"REF2K\\\",\\\"field\\\":\\\"FM64\\\"},\\\"link_desc\\\":{\\\"connect_retry_times\\\":60,\\\"connect_retry_interval_ms\\\":1000,\\\"brpc_channel_protocol\\\":\\\"http\\\",\\\"brpc_channel_connection_type\\\":\\\"pooled\\\",\\\"recv_timeout_ms\\\":1200000,\\\"http_timeout_ms\\\":1200000}}\"},{\"name\":\"heu\",\"type\":\"heu\",\"parties\":[\"alice\",\"bob\"],\"config\":\"{\\\"mode\\\": \\\"PHEU\\\", \\\"schema\\\": \\\"paillier\\\", \\\"key_size\\\": 2048}\"}],\"ray_fed_config\":{\"cross_silo_comm_backend\":\"brpc_link\"}},\"sf_node_eval_param\":{\"domain\":\"data_prep\",\"name\":\"train_test_split\",\"version\":\"0.0.1\",\"attr_paths\":[\"train_size\",\"test_size\",\"random_state\",\"shuffle\"],\"attrs\":[{\"f\":0.75},{\"f\":0.25},{\"i64\":1234},{\"b\":true}]},\"sf_output_uris\":[\"train-dataset.csv\",\"test-dataset.csv\"],\"sf_output_ids\":[\"train-dataset\",\"test-dataset\"],\"sf_input_ids\":[\"psi-output\"]}",
        "priority": 100
      }
    ],
    "status": {
      "state": "Failed",
      "err_msg": "",
      "create_time": "2024-01-17T07:13:39Z",
      "start_time": "2024-01-17T07:13:39Z",
      "end_time": "2024-01-17T07:13:39Z",
      "tasks": [
        {
          "task_id": "job-psi",
          "state": "",
          "err_msg": "",
          "create_time": "",
          "start_time": "",
          "end_time": "",
          "parties": []
        },
        {
          "task_id": "job-split",
          "state": "",
          "err_msg": "",
          "create_time": "",
          "start_time": "",
          "end_time": "",
          "parties": []
        }
      ]
    }
  }
}
```

{#batch-query-job-status}

### 批量查询 Job 状态

#### HTTP 路径

/api/v1/job/status/batchQuery

#### 请求（BatchQueryJobStatusRequest）

| 字段      | 类型                                           | 选填 | 描述       |
|---------|----------------------------------------------|----|----------|
| header  | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容  |
| job_ids | string[]                                     | 必填 | JobID 列表 |

#### 响应（BatchQueryJobStatusResponse）

| 字段        | 类型                              | 描述       |
|-----------|---------------------------------|----------|
| status    | [Status](summary_cn.md#status)  | 状态信息     |
| data      | BatchQueryJobStatusResponseData |         |
| data.jobs | [JobStatus](#job-status)[]      | Job 状态列表 |

#### 请求示例

发起请求：

```sh
# Example of execution within a container
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/job/status/batchQuery' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "job_ids": [
    "job-alice-bob-001"
  ]
}'
```

请求响应成功结果：

```json
{
  "status": {
    "code": 0,
    "message": "success",
    "details": []
  },
  "data": {
    "jobs": [
      {
        "job_id": "job-alice-bob-001",
        "status": {
          "state": "Failed",
          "err_msg": "",
          "create_time": "2024-01-17T07:13:39Z",
          "start_time": "2024-01-17T07:13:39Z",
          "end_time": "2024-01-17T07:13:39Z",
          "tasks": [
            {
              "task_id": "job-psi",
              "state": "",
              "err_msg": "",
              "create_time": "",
              "start_time": "",
              "end_time": "",
              "parties": []
            },
            {
              "task_id": "job-split",
              "state": "",
              "err_msg": "",
              "create_time": "",
              "start_time": "",
              "end_time": "",
              "parties": []
            }
          ]
        }
      }
    ]
  }
}
```

{#delete-job}

### 删除 Job

#### HTTP 路径

/api/v1/job/delete

#### 请求（DeleteJobRequest）

| 字段     | 类型                                           | 选填 | 描述      |
|--------|----------------------------------------------|----|---------|
| header | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容 |
| job_id | string                                       | 必填 | JobID   |

#### 响应（DeleteJobResponse）

| 字段          | 类型                             | 描述    |
|-------------|--------------------------------|-------|
| status      | [Status](summary_cn.md#status) | 状态信息  |
| data        | DeleteJobResponseData          |       |
| data.job_id | string                         | JobID |

#### 请求示例

发起请求：

```sh
# Example of execution within a container
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/job/delete' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "job_id": "job-alice-bob-001"
}'
```

请求响应成功结果：

```json
{
  "status": {
    "code": 0,
    "message": "success",
    "details": []
  },
  "data": {
    "job_id": "job-alice-bob-001"
  }
}
```

{#stop-job}

### 停止 Job

#### HTTP 路径

/api/v1/job/stop

#### 请求（StopJobRequest）

| 字段     | 类型                                           | 选填 | 描述      |
|--------|----------------------------------------------|----|---------|
| header | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容 |
| job_id | string                                       | 必填 | JobID   |
| reason | string                                       | 可选 | 停止Job的原因   |

#### 响应（StopJobResponse）

| 字段          | 类型                             | 描述    |
|-------------|--------------------------------|-------|
| status      | [Status](summary_cn.md#status) | 状态信息  |
| data        | StopJobResponseData            |       |
| data.job_id | string                         | JobID |

#### 请求示例

发起请求：

```sh
# Example of execution within a container
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/job/stop' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "job_id": "job-alice-bob-001"
}'
```

请求响应成功结果：

```json
{
  "status": {
    "code": 0,
    "message": "success",
    "details": []
  },
  "data": {
    "job_id": "job-alice-bob-001"
  }
}
```

{#watch-job}

### 监控 Job

#### HTTP 路径

/api/v1/job/watch

#### 请求（WatchJobRequest）

| 字段              | 类型                                           | 选填 | 描述      |
|-----------------|----------------------------------------------|----|---------|
| header          | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容 |
| timeout_seconds | int64                                        | 可选 | 请求连接的生命周期，服务端将在超时后断开连接，不管是否有活跃事件。超时时间取值范围：[0, 2^31-1]，默认为0，表示不超时。即使在未设置超时时间的情况下, 也会因为网络环境导致断连，客户端根据需求决定是否重新发起请求    |

#### 响应（WatchJobEventResponse）

| 字段     | 类型                       | 描述     |
|--------|--------------------------|--------|
| 类型     | [EventType](#event-type) | 事件类型   |
| object | [JobStatus](#job-status) | Job 状态 |

{#approval-job}

### 审批 Job

只有在 P2P 组网模式下且[开启 Job 审批](../concepts/kusciajob_cn.md#enable-approval)的情况下才需要调用此接口进行 job 审批。

#### HTTP 路径

/api/v1/job/approve

#### 请求（ApprovalJobRequest）

| 字段              | 类型                                           | 选填 | 描述                                                                                                                         |
|-----------------|----------------------------------------------|----|----------------------------------------------------------------------------------------------------------------------------|
| header          | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容                                                                                                                    |
| job_id          | string                                       | 必填 | JobID，满足 [RFC 1123 标签名规则要求](https://kubernetes.io/zh-cn/docs/concepts/overview/working-with-objects/names/#dns-label-names) |
| result          | [ApproveResult](approve-result)              | 必填 | 审批结果，接收或拒绝，接收则作业(job)可以执行，拒绝则作业不可执行                                                                                                                   | |
| reason          | string                                       | 可选 | 接收或拒绝的理由                                                                                                                     |

#### 响应（ApprovalJobResponse）

| 字段          | 类型                             | 描述    |
|-------------|--------------------------------|-------|
| status      | [Status](summary_cn.md#status) | 状态信息  |
| data        | ApprovalJobResponseData        |       |
| data.job_id | string                         | JobID |

{#suspend-job}

### 暂停 Job

#### HTTP 路径

/api/v1/job/suspend

#### 请求（SuspendJobRequest）

| 字段     | 类型                                           | 选填 | 描述      |
|--------|----------------------------------------------|----|---------|
| header | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容 |
| job_id | string                                       | 必填 | JobID   |
| reason | string                                       | 可选 | 暂停Job的原因   |

#### 响应（SuspendJobResponse）

| 字段          | 类型                             | 描述    |
|-------------|--------------------------------|-------|
| status      | [Status](summary_cn.md#status) | 状态信息  |
| data        | SuspendJobResponseData         |       |
| data.job_id | string                         | JobID |

#### 请求示例

发起请求：

```sh
# Example of execution within a container
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/job/suspend' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "job_id": "job-alice-bob-001"
}'
```

请求响应成功结果：

```json
{
  "status": {
    "code": 0,
    "message": "success",
    "details": []
  },
  "data": {
    "job_id": "job-alice-bob-001"
  }
}
```

{#restart-job}

### 重跑 Job

#### HTTP 路径

/api/v1/job/restart

#### 请求（RestartJobRequest）

| 字段     | 类型                                           | 选填 | 描述      |
|--------|----------------------------------------------|----|---------|
| header | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容 |
| job_id | string                                       | 必填 | JobID   |
| reason | string                                       | 可选 | 重跑Job的原因   |

#### 响应（RestartJobResponse）

| 字段          | 类型                             | 描述    |
|-------------|--------------------------------|-------|
| status      | [Status](summary_cn.md#status) | 状态信息  |
| data        | RestartJobResponseData         |       |
| data.job_id | string                         | JobID |

#### 请求示例

发起请求：

```sh
# Example of execution within a container
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/job/restart' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "job_id": "job-alice-bob-001"
}'
```

请求响应成功结果：

```json
{
  "status": {
    "code": 0,
    "message": "success",
    "details": []
  },
  "data": {
    "job_id": "job-alice-bob-001"
  }
}
```

{#cancel-job}

### 取消 Job

#### HTTP 路径

/api/v1/job/cancel

#### 请求（CancelJobRequest）

| 字段     | 类型                                           | 选填 | 描述      |
|--------|----------------------------------------------|----|---------|
| header | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容 |
| job_id | string                                       | 必填 | JobID   |
| reason | string                                       | 可选 | 取消Job的原因   |

#### 响应（CancelJobResponse）

| 字段          | 类型                             | 描述    |
|-------------|--------------------------------|-------|
| status      | [Status](summary_cn.md#status) | 状态信息  |
| data        | CancelJobResponseData          |      |
| data.job_id | string                         | JobID |

#### 请求示例

发起请求：

```sh
# Example of execution within a container
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/job/cancel' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "job_id": "job-alice-bob-001"
}'
```

请求响应成功结果：

```json
{
  "status": {
    "code": 0,
    "message": "success",
    "details": []
  },
  "data": {
    "job_id": "job-alice-bob-001"
  }
}
```

## 公共

{#job-status}

### JobStatus

| 字段     | 类型                                    | 描述       |
|--------|---------------------------------------|----------|
| job_id | string                                | JobID    |
| status | [JobStatusDetail](#job-status-detail) | Job 状态详情 |

{#job-status-detail}

### JobStatusDetail

| 字段          | 类型                           | 描述                       |
|-------------|------------------------------|--------------------------|
| state       | string                       | 作业状态, 参考 [State](#state) |
| err_msg     | string                       | 错误信息                     |
| create_time | string                       | 创建时间                     |
| start_time  | string                       | 启动时间                     |
| end_time    | string                       | 结束时间                     |
| tasks       | [TaskStatus](#task-status)[] | 任务列表                     |

{#party}

### Party

| 字段        | 类型     | 选填 | 描述       |
|-----------|--------|----|----------|
| domain_id | string | 必填 | DomainID |
| role      | string | 可选 | 参与方角色，该字段由引擎自定义，对应到 [appImage](../concepts/appimage_cn.md#appimage-ref) 的部署模版中；更多参考 [KusciaJob](../concepts/kusciajob_cn.md#create-kuscia-job)       |
| resources | JobResource | 可选 | 参与方资源配置 |
| bandwidth_limits | [BandwidthLimit](#bandwidth-limit)[] | 可选 | 节点请求其他节点的带宽限制配置 |

{#JobResource}

### JobResource

| 字段  | 类型   | 选填 | 描述 |
|--------|------------|------|------|
| cpu  | string | 可选 | 参与方可用 CPU 资源上限 |
| memory | string | 可选 | 参与方可用内存资源上限  |

{#party-status}

### PartyStatus

| 字段        | 类型                                        | 描述                          |
|-----------|-------------------------------------------|-----------------------------|
| domain_id | string                                    | 节点 ID                       |
| state     | string                                    | 参与方任务状态, 参考 [State](#state) |
| err_msg   | string                                    | 错误信息                        |
| endpoints | [JobPartyEndpoint](#job-party-endpoint)[] | 应用对外暴露的访问地址信息               |

{#task}

### Task

| 字段                       | 类型                                 | 选填 | 描述                                                                                                                                                        |
|--------------------------|------------------------------------|----|-----------------------------------------------------------------------------------------------------------------------------------------------------------|
| app_image                | string                             | 必填 | 任务镜像                                                                                                                                                      |
| parties                  | [Party](#party)[]                  | 必填 | 参与方节点 ID                                                                                                                                                  |
| alias                    | string                             | 必填 | 任务别名，同一个 Job 中唯一，满足 [RFC 1123 标签名规则要求](https://kubernetes.io/zh-cn/docs/concepts/overview/working-with-objects/names/#dns-label-names)                    |
| task_id                  | string                             | 可选 | 任务 ID，如果不填，Kuscia 将随机生成唯一的 task_id ，满足 [RFC 1123 标签名规则要求](https://kubernetes.io/zh-cn/docs/concepts/overview/working-with-objects/names/#dns-label-names) |
| dependencies             | string[]                           | 必填 | 依赖任务，通过 alias 字段来编排 Job 中 Task 之间的依赖关系                                                                                                                    |
| task_input_config        | string                             | 必填 | 任务配置                                                                                                                                                      |
| priority                 | string                             | 可选 | 优先级，值越大优先级越高                                                                                                                                              |
| schedule_config          | [ScheduleConfig](#schedule-config) | 可选 | 任务调度配置                                                                                                                                                    |
| tolerable                | bool                               | 可选 | 标识 Task 是否可容忍失败（默认为 false），若 tolerable=true 即使此 task 失败也不会导致 Job 失败。若 tolerable=false 则此 task 失败会判定整个 Job 失败。                     |

{#schedule-config}

### ScheduleConfig

| 字段                                      | 类型     | 选填   | 描述                                                                                                 |
|-----------------------------------------|--------|------|----------------------------------------------------------------------------------------------------|
| task_timeout_seconds                    | int32  | 可选   | 任务超时时间，默认值: 300，当任务在 300 秒内没有被调度成功时，会将任务置为失败状态                                                     |
| resource_reserved_seconds               | int32  | 可选   | 任务预留资源时间，默认值: 30，当任务参与方在扣减完资源(cpu/memory)后，会占用 30 秒，如果在 30 秒内，存在部分参与方没有成功扣减资源，那么已扣减资源的参与方将会释放扣减的资源 |
| resource_reallocation_interval_seconds  | int32  | 可选   | 任务重新扣减资源的时间间隔，默认值: 30，当已扣减资源的参与方在释放完扣减的资源后，下次重新扣减资源的时间间隔                                           |

{#task-config}

### TaskConfig

| 字段                | 类型                | 描述           |
|-------------------|-------------------|--------------|
| app_image         | string            | 任务镜像         |
| parties           | [Party](#party)[] | 参与方          |
| alias             | string            | 任务别名         |
| task_id           | string            | 任务 ID        |
| dependencies      | string[]          | 依赖任务         |
| task_input_config | string            | 任务配置         |
| priority          | string            | 优先级，值越大优先级越高 |

{#task-status}

### TaskStatus

| 字段          | 类型                             | 描述                      |
|-------------|--------------------------------|-------------------------|
| task_id     | string                         | 任务 ID                   |
| alias       | string                         | 任务别名                    |
| state       | string                         | 任务状态，参考 [State](#state) |
| err_msg     | string                         | 错误信息                    |
| create_time | string                         | 创建事件                    |
| start_time  | string                         | 开始事件                    |
| end_time    | string                         | 结束事件                    |
| parties     | [PartyStatus](#party-status)[] | 参与方                     |

{#event-type}

### EventType

| Name     | Number | 描述   |
|----------|--------|------|
| ADDED    | 0      | 增加事件 |
| MODIFIED | 1      | 修改事件 |
| DELETED  | 2      | 删除事件 |
| ERROR    | 3      | 错误事件 |
| HEARTBEAT | 4      | 心跳事件 |

{#approve-result}

### ApproveResult

| Name     | Number | 描述   |
|----------|--------|------|
| APPROVE_RESULT_UNKNOWN    | 0  | 未知状态 |
| APPROVE_RESULT_ACCEPT     | 1  | 审批通过 |
| APPROVE_RESULT_REJECT     | 2  | 审批拒绝 |

{#state}

### State

KusciaJob 状态详细介绍见[文档](../concepts/kusciajob_cn.md#kuscia-state)。

| Name               | Number | 描述    |
|-----------         |--------|-------|
| Unknown            | 0      | 未知    |
| Pending            | 1      | 未开始运行 |
| Running            | 2      | 运行中   |
| Succeeded          | 3      | 成功    |
| Failed             | 4      | 失败    |
| AwaitingApproval   | 5      | 等待参与方审批 Job   |
| ApprovalReject     | 6      | Job 被审批为拒绝执行    |
| Cancelled          | 7      | Job 被取消，被取消的 Job 不可被再次执行    |
| Suspended          | 8      | Job 被暂停，可通过 Restart 接口重跑   |
| Initialized        | 9      | Job 初始状态  |

{#job-party-endpoint}

### JobPartyEndpoint

| 字段        | 类型     | 描述                                                                                                  |
|-----------|--------|-----------------------------------------------------------------------------------------------------|
| port_name | string | 应用服务端口名称，详细解释请参考[AppImage](../concepts/appimage_cn.md) `deployTemplates.spec.containers.ports.name` |
| scope     | string | 应用服务使用范围，详细解释请参考[AppImage](../concepts/appimage_cn.md) `deployTemplates.spec.containers.ports.scope` |
| endpoint  | string | 应用服务访问地址                                                                                            |

{#bandwidth-limit}

### BandwidthLimit

| 字段        | 类型                | 选填 | 描述                                                                                                                                                       |
|-----------|-------------------|----|---------------------------------------------------------------------|
| destination_id | string            | 必填 | 目标节点     ID                                                                                                                                                 |
| limit_kbps     | int64            | 必填 | 带宽限制，单位为 KiB/s                                                                                                                                                 |
