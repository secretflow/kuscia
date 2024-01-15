# KusciaJob

在 Kuscia 中，你可以使用 KusciaJob 来表示一个任务流程。请参考 [KusciaJob](../concepts/kusciajob_cn.md) 。
你可以从 [这里](https://github.com/secretflow/kuscia/tree/main/proto/api/v1alpha1/kusciaapi/job.proto) 找到对应的
protobuf 文件。

## 接口总览

| 方法名                                            | 请求类型                       | 响应类型                         | 描述          |
|------------------------------------------------|----------------------------|------------------------------|-------------|
| [CreateJob](#create-job)                       | CreateJobRequest           | CreateJobResponse            | 创建 Job      |
| [QueryJob](#query-job)                         | QueryJobRequest            | QueryJobResponse             | 查询 Job      |
| [BatchQueryJobStatus](#batch-query-job-status) | BatchQueryJobStatusRequest | BatchQueryJobStatusResponse  | 批量查询 Job 状态 |
| [DeleteJob](#delete-job)                       | DeleteJobRequest           | DeleteJobResponse            | 删除 Job      |
| [StopJob](#stop-job)                           | StopJobRequest             | StopJobResponse              | 停止 Job      |
| [WatchJob](#watch-job)                         | WatchJobRequest            | WatchJobEventResponse stream | 监控 Job      |

## 接口详情

{#create-job}

### 创建 Job

#### HTTP 路径

/api/v1/job/create

#### 请求（CreateJobRequest）

| 字段              | 类型                                           | 选填 | 描述                                                                                                                         |
|-----------------|----------------------------------------------|----|----------------------------------------------------------------------------------------------------------------------------|
| header          | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容                                                                                                                    |
| job_id          | string                                       | 必填 | JobID，满足 [DNS 子域名规则要求](https://kubernetes.io/zh-cn/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names) |
| initiator       | string                                       | 必填 | 发起方节点 ID                                                                                                                   |
| max_parallelism | int32                                        | 可选 | 并发度，参考 [KusciaJob 概念](../concepts/kusciajob_cn.md)                                                                         |
| tasks           | [Task](#task)[]                              | 必填 | 任务参数                                                                                                                       |

#### 响应（CreateJobResponse）

| 字段          | 类型                             | 选填 | 描述    |
|-------------|--------------------------------|----|-------|
| status      | [Status](summary_cn.md#status) | 必填 | 状态信息  |
| data        | CreateJobResponseData          |    |       |
| data.job_id | string                         | 必填 | JobID |

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

| 字段                   | 类型                                    | 选填 | 描述     |
|----------------------|---------------------------------------|----|--------|
| status               | [Status](summary_cn.md#status)        | 必填 | 状态信息   |
| data                 | QueryJobResponseData                  |    |        |
| data.job_id          | string                                | 必填 | JobID  |
| data.initiator       | string                                | 必填 | 发起方    |
| data.max_parallelism | int32                                 | 必填 | 并发度    |
| data.tasks           | [TaskConfig](#task-config)[]          | 必填 | 任务列表   |
| data.status          | [JobStatusDetail](#job-status-detail) | 必填 | Job 状态 |

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

| 字段        | 类型                              | 选填 | 描述       |
|-----------|---------------------------------|----|----------|
| status    | [Status](summary_cn.md#status)  | 必填 | 状态信息     |
| data      | BatchQueryJobStatusResponseData |    |          |
| data.jobs | [JobStatus](#job-status)[]      | 必填 | Job 状态列表 |

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

| 字段          | 类型                             | 选填 | 描述    |
|-------------|--------------------------------|----|-------|
| status      | [Status](summary_cn.md#status) | 必填 | 状态信息  |
| data        | DeleteJobResponseData          |    |       |
| data.job_id | string                         | 必填 | JobID |

{#stop-job}

### 停止 Job

#### HTTP 路径

/api/v1/job/stop

#### 请求（StopJobRequest）

| 字段     | 类型                                           | 选填 | 描述      |
|--------|----------------------------------------------|----|---------|
| header | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容 |
| job_id | string                                       | 必填 | JobID   |

#### 响应（StopJobResponse）

| 字段          | 类型                             | 选填 | 描述    |
|-------------|--------------------------------|----|-------|
| status      | [Status](summary_cn.md#status) | 必填 | 状态信息  |
| data        | StopJobResponseData            |    |       |
| data.job_id | string                         | 必填 | JobID |

{#watch-job}

### 监控 Job

#### HTTP 路径

暂不支持 HTTP 接口。

#### 请求（WatchJobRequest）

| 字段              | 类型                                           | 选填 | 描述      |
|-----------------|----------------------------------------------|----|---------|
| header          | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容 |
| timeout_seconds | int64                                        | 可选 | 超时时间    |

#### 响应（WatchJobEventResponse）

| 字段     | 类型                       | 选填 | 描述     |
|--------|--------------------------|----|--------|
| 类型     | [EventType](#event-type) | 必填 | 事件类型   |
| object | [JobStatus](#job-status) | 必填 | Job 状态 |

## 公共

{#job-status}

### JobStatus

| 字段     | 类型                                    | 选填 | 描述       |
|--------|---------------------------------------|----|----------|
| job_id | string                                | 必填 | JobID    |
| status | [JobStatusDetail](#job-status-detail) | 必填 | Job 状态详情 |

{#job-status-detail}

### JobStatusDetail

| 字段          | 类型                           | 选填 | 描述                       |
|-------------|------------------------------|----|--------------------------|
| state       | string                       | 必填 | 作业状态, 参考 [State](#state) |
| err_msg     | string                       | 可选 | 错误信息                     |
| create_time | string                       | 必填 | 创建时间                     |
| start_time  | string                       | 必填 | 启动时间                     |
| end_time    | string                       | 可选 | 结束时间                     |
| tasks       | [TaskStatus](#task-status)[] | 必填 | 任务列表                     |

{#party}

### Party

| 字段        | 类型     | 选填 | 描述       |
|-----------|--------|----|----------|
| domain_id | string | 必填 | DomainID |
| role      | string | 可选 | 参与方角色，该字段由引擎自定义，对应到 [appImage](../concepts/appimage_cn.md#appimage-ref) 的部署模版中；更多参考 [KusciaJob](../concepts/kusciajob_cn.md#create-kuscia-job)       |

{#party-status}

### PartyStatus

| 字段        | 类型                                        | 选填 | 描述            |
|-----------|-------------------------------------------|----|---------------|
| domain_id | string                                    | 必填 | 节点 ID         |
| state     | string                                    | 必填 | 参与方任务状态, 参考 [State](#state) |
| err_msg   | string                                    | 可选 | 错误信息          |
| endpoints | [JobPartyEndpoint](#job-party-endpoint)[] | 必填 | 应用对外暴露的访问地址信息 |

{#task}

### Task

| 字段                | 类型                | 选填 | 描述                                                                                                                         |
|-------------------|-------------------|----|----------------------------------------------------------------------------------------------------------------------------|
| app_image         | string            | 必填 | 任务镜像                                                                                                                       |
| parties           | [Party](#party)[] | 必填 | 参与方节点 ID                                                                                                                   |
| alias             | string            | 必填 | 任务别名                                                                                                                       |
| task_id           | string            | 必填 | 任务 ID，满足 [DNS 子域名规则要求](https://kubernetes.io/zh-cn/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names) |
| dependencies      | string[]          | 必填 | 依赖任务                                                                                                                       |
| task_input_config | string            | 必填 | 任务配置                                                                                                                       |
| priority          | string            | 可选 | 优先级，值越大优先级越高                                                                                                               |

{#task-config}

### TaskConfig

| 字段                | 类型                | 选填 | 描述           |
|-------------------|-------------------|----|--------------|
| app_image         | string            | 必填 | 任务镜像         |
| parties           | [Party](#party)[] | 必填 | 参与方          |
| alias             | string            | 必填 | 任务别名         |
| task_id           | string            | 必填 | 任务 ID        |
| dependencies      | string[]          | 必填 | 依赖任务         |
| task_input_config | string            | 必填 | 任务配置         |
| priority          | string            | 可选 | 优先级，值越大优先级越高 |

{#task-status}

### TaskStatus

| 字段          | 类型                             | 选填 | 描述                        |
|-------------|--------------------------------|----|---------------------------|
| task_id     | string                         | 可选 | 任务 ID                     |
| state       | string                         | 必填 | 任务状态，参考 [State](#state)   |
| err_msg     | string                         | 可选 | 错误信息                      |
| create_time | string                         | 必填 | 创建事件                      |
| start_time  | string                         | 必填 | 开始事件                      |
| end_time    | string                         | 可选 | 结束事件                      |
| parties     | [PartyStatus](#party-status)[] | 必填 | 参与方                       |

{#event-type}

### EventType

| Name     | Number | 描述   |
|----------|--------|------|
| ADDED    | 0      | 增加事件 |
| MODIFIED | 1      | 修改事件 |
| DELETED  | 2      | 删除事件 |
| ERROR    | 3      | 错误事件 |

{#state}

### State

| Name      | Number | 描述    |
|-----------|--------|-------|
| Unknown   | 0      | 未知    |
| Pending   | 1      | 未开始运行 |
| Running   | 2      | 运行中   |
| Succeeded | 3      | 成功    |
| Failed    | 4      | 失败    |

{#job-party-endpoint}

### JobPartyEndpoint

| 字段        | 类型     | 选填 | 描述                                                                                                  |
|-----------|--------|---|-----------------------------------------------------------------------------------------------------|
| port_name | string | 必填 | 应用服务端口名称，详细解释请参考[AppImage](../concepts/appimage_cn.md) `deployTemplates.spec.containers.ports.name` |
| scope     | string | 必填 | 应用服务使用范围，详细解释请参考[AppImage](../concepts/appimage_cn.md) `deployTemplates.spec.containers.ports.scope` |
| endpoint  | string | 必填 | 应用服务访问地址                                                                                            |