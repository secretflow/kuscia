# KusciaJob

在 Kuscia 中，你可以使用 KusciaJob 来表示一个任务流程。请参考 [KusciaJob](../../concepts/kusciajob_cn.md) 。
你可以从 [这里](https://github.com/secretflow/kuscia/tree/main/proto/api/v1alpha1/kusciaapi/job.proto) 找到对应的
protobuf 文件。

## 接口总览

| 方法名                                            | 请求类型                       | 响应类型                         | 描述        |
|------------------------------------------------|----------------------------|------------------------------|-----------|
| [CreateJob](#create-job)                       | CreateJobRequest           | CreateJobResponse            | 创建Job     |
| [QueryJob](#query-job)                         | QueryJobRequest            | QueryJobResponse             | 查询Job     |
| [BatchQueryJobStatus](#batch-query-job-status) | BatchQueryJobStatusRequest | BatchQueryJobStatusResponse  | 批量查询Job状态 |
| [DeleteJob](#delete-job)                       | DeleteJobRequest           | DeleteJobResponse            | 删除Job     |
| [StopJob](#stop-job)                           | StopJobRequest             | StopJobResponse              | 停止Job     |
| [WatchJob](#watch-job)                         | WatchJobRequest            | WatchJobEventResponse stream | 监控Job     |

## 接口详情

{#create-job}

### 创建Job

#### HTTP路径
/api/v1/job/create

#### 请求（CreateJobRequest）

| 字段              | 类型                                            | 可选 | 描述                                                                                                                         |
|-----------------|-----------------------------------------------|----|----------------------------------------------------------------------------------------------------------------------------|
| header          | [RequestHeader](summary_cn.md#request-header) | 是  | 自定义请求内容                                                                                                                    |
| job_id          | string                                        | 否  | JobID，满足 [DNS 子域名规则要求](https://kubernetes.io/zh-cn/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names) |
| initiator       | string                                        | 否  | 发起方节点ID                                                                                                                    |
| max_parallelism | int32                                         | 是  | 并发度，参考 [KusciaJob概念](../../concepts/kusciajob_cn.md)                                                                       |
| tasks           | [Task](#task)[]                               | 否  | 任务参数                                                                                                                       |

#### 响应（CreateJobResponse）

| 字段          | 类型                             | 可选 | 描述    |
|-------------|--------------------------------|----|-------|
| status      | [Status](summary_cn.md#status) | 否  | 状态信息  |
| data        | CreateJobResponseData          |    |       |
| data.job_id | string                         | 否  | JobID |

{#query-job}

### 查询Job

#### HTTP路径
/api/v1/job/query

#### 请求（QueryJobRequest）


| 字段     | 类型                                            | 可选 | 描述      |
|--------|-----------------------------------------------|----|---------|
| header | [RequestHeader](summary_cn.md#request-header) | 是  | 自定义请求内容 |
| job_id | string                                        | 否  | JobID   |

#### 响应（QueryJobResponse）

| 字段                   | 类型                                    | 可选 | 描述    |
|----------------------|---------------------------------------|----|-------|
| status               | [Status](summary_cn.md#status)        | 否  | 状态信息  |
| data                 | QueryJobResponseData                  |    |       |
| data.job_id          | string                                | 否  | JobID |
| data.initiator       | string                                | 否  | 发起方   |
| data.max_parallelism | int32                                 | 否  | 并发度   |
| data.tasks           | [TaskConfig](#task-config)[]          | 否  | 任务列表  |
| data.status          | [JobStatusDetail](#job-status-detail) | 否  | Job状态 |


{#batch-query-job-status}

### 批量查询Job状态

#### HTTP路径
/api/v1/job/status/batchQuery

#### 请求（BatchQueryJobStatusRequest）

| 字段      | 类型                                            | 可选 | 描述      |
|---------|-----------------------------------------------|----|---------|
| header  | [RequestHeader](summary_cn.md#request-header) | 是  | 自定义请求内容 |
| job_ids | string[]                                      | 否  | JobID列表 |

#### 响应（BatchQueryJobStatusResponse）

| 字段        | 类型                              | 可选 | 描述      |
|-----------|---------------------------------|----|---------|
| status    | [Status](summary_cn.md#status)  | 否  | 状态信息    |
| data      | BatchQueryJobStatusResponseData |    |         |
| data.jobs | [JobStatus](#job-status)[]      | 否  | Job状态列表 |


{#delete-job}

### 删除Job

#### HTTP路径
/api/v1/job/delete

#### 请求（DeleteJobRequest）

| 字段     | 类型                                            | 可选 | 描述      |
|--------|-----------------------------------------------|----|---------|
| header | [RequestHeader](summary_cn.md#request-header) | 是  | 自定义请求内容 |
| job_id | string                                        | 否  | JobID   |

#### 响应（DeleteJobResponse）

| 字段          | 类型                             | 可选 | 描述    |
|-------------|--------------------------------|----|-------|
| status      | [Status](summary_cn.md#status) | 否  | 状态信息  |
| data        | DeleteJobResponseData          |    |       |
| data.job_id | string                         | 否  | JobID |


{#stop-job}

### 停止Job

#### HTTP路径
/api/v1/job/stop

#### 请求（StopJobRequest）

| 字段     | 类型                                            | 可选 | 描述      |
|--------|-----------------------------------------------|----|---------|
| header | [RequestHeader](summary_cn.md#request-header) | 是  | 自定义请求内容 |
| job_id | string                                        | 否  | JobID   |

#### 响应（StopJobResponse）

| 字段          | 类型                             | 可选 | 描述    |
|-------------|--------------------------------|----|-------|
| status      | [Status](summary_cn.md#status) | 否  | 状态信息  |
| data        | StopJobResponseData            |    |       |
| data.job_id | string                         | 否  | JobID |


{#watch-job}

### 监控Job

#### HTTP路径
暂不支持 HTTP 接口。

#### 请求（WatchJobRequest）

| 字段              | 类型                                            | 可选 | 描述      |
|-----------------|-----------------------------------------------|----|---------|
| header          | [RequestHeader](summary_cn.md#request-header) | 是  | 自定义请求内容 |
| timeout_seconds | int64                                         | 是  | 超时时间    |

#### 响应（WatchJobEventResponse）

| 字段     | 类型                       | 可选 | 描述    |
|--------|--------------------------|----|-------|
| 类型     | [EventType](#event-type) | 否  | 事件类型  |
| object | [JobStatus](#job-status) | 否  | Job状态 |

## 公共

{#job-status}

### JobStatus

| 字段     | 类型                                    | 可选 | 描述      |
|--------|---------------------------------------|----|---------|
| job_id | string                                | 否  | JobID   |
| status | [JobStatusDetail](#job-status-detail) | 否  | Job状态详情 |

{#job-status-detail}

### JobStatusDetail

| 字段          | 类型                           | 可选 | 描述   |
|-------------|------------------------------|----|------|
| state       | string                       | 否  | 总体状态 |
| err_msg     | string                       | 是  | 错误信息 |
| create_time | string                       | 否  | 创建时间 |
| start_time  | string                       | 否  | 启动时间 |
| end_time    | string                       | 是  | 结束时间 |
| tasks       | [TaskStatus](#task-status)[] | 否  | 任务列表 |

{#party}

### Party

| 字段        | 类型     | 可选 | 描述       |
|-----------|--------|----|----------|
| domain_id | string | 否  | DomainID |
| role      | string | 是  | 角色       |

{#party-status}

### PartyStatus

| 字段        | 类型                       | 可选 | 描述   |
|-----------|--------------------------|----|------|
| domain_id | string                   | 否  | 节点ID |
| state     | [TaskState](#task-state) | 否  | 总体状态 |
| err_msg   | string                   | 是  | 错误信息 |

{#task}

### Task

| 字段                | 类型                | 可选 | 描述                                                                                                                        |
|-------------------|-------------------|----|---------------------------------------------------------------------------------------------------------------------------|
| app_image         | string            | 否  | 任务镜像                                                                                                                      |
| parties           | [Party](#party)[] | 否  | 参与方节点ID                                                                                                                   |
| alias             | string            | 否  | 任务别名                                                                                                                      |
| task_id           | string            | 否  | 任务ID，满足 [DNS 子域名规则要求](https://kubernetes.io/zh-cn/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names) |
| dependencies      | string[]          | 否  | 依赖任务                                                                                                                      |
| task_input_config | string            | 否  | 任务配置                                                                                                                      |
| priority          | string            | 是  | 优先级，值越大优先级越高                                                                                                              |


{#task-config}

### TaskConfig

| 字段                | 类型                | 可选 | 描述           |
|-------------------|-------------------|----|--------------|
| app_image         | string            | 否  | 任务镜像         |
| parties           | [Party](#party)[] | 否  | 参与方          |
| alias             | string            | 否  | 任务别名         |
| task_id           | string            | 否  | 任务ID         |
| dependencies      | string[]          | 否  | 依赖任务         |
| task_input_config | string            | 否  | 任务配置         |
| priority          | string            | 是  | 优先级，值越大优先级越高 |

{#task-status}

### TaskStatus

| 字段          | 类型                             | 可选 | 描述                               |
|-------------|--------------------------------|----|----------------------------------|
| task_id     | string                         | 是  | 任务ID                             |
| state       | string                         | 否  | 任务状态，参考 [TaskState](#task-state) |
| err_msg     | string                         | 是  | 错误信息                             |
| create_time | string                         | 否  | 创建事件                             |
| start_time  | string                         | 否  | 开始事件                             |
| end_time    | string                         | 是  | 结束事件                             |
| parties     | [PartyStatus](#party-status)[] | 否  | 参与方                              |

{#event-type}

### EventType

| Name     | Number | 描述   |
|----------|--------|------|
| ADDED    | 0      | 增加事件 |
| MODIFIED | 1      | 修改事件 |
| DELETED  | 2      | 删除事件 |
| ERROR    | 3      | 错误事件 |

{#task-state}

### TaskState

| Name      | Number | 描述    |
|-----------|--------|-------|
| Pending   | 0      | 未开始运行 |
| Running   | 1      | 运行中   |
| Succeeded | 2      | 成功    |
| Failed    | 3      | 失败    |
