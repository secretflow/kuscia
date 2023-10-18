# Serving

在 Kuscia 中，你可以使用 Serving 接口管理联合预测服务，
从 [这里](https://github.com/secretflow/kuscia/tree/main/proto/api/v1alpha1/kusciaapi/serving.proto) 可以找到对应的 protobuf 文件。

## 接口总览

| 方法名                                                 | 请求类型                            | 响应类型                        | 描述                |
| -------------------------------------------------------- |---------------------------------| --------------------------------- | --------------------- |
| [CreateServing](#create-serving)                       | CreateServingRequest            | CreateServingResponse           | 创建Serving         |
| [UpdateServing](#update-serving)                       | UpdateServingRequest            | UpdateServingResponse           | 更新Serving         |
| [DeleteServing](#delete-serving)                       | DeleteServingRequest            | DeleteServingResponse           | 删除Serving         |
| [QueryServing](#query-serving)                         | QueryServingRequest             | QueryServingResponse            | 查询Serving         |
| [BatchQueryServingStatus](#batch-query-serving-status) | BatchQueryServingStatusRequest  | BatchQueryServingStatusResponse | 批量查询Serving状态 |

## 接口详情

{#create-serving}

### 创建Serving

#### HTTP路径

/api/v1/serving/create

#### 请求（CreateServingRequest）

| 字段                 | 类型                                           | 可选 | 描述                                                                                                                                    |
| ---------------------- |----------------------------------------------| --- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| header               | [RequestHeader](summary_cn.md#requestheader) | 是 | 自定义请求内容                                                                                                                          |
| serving_id           | string                                       | 否 | ServingID，满足[DNS 子域名规则要求](https://kubernetes.io/zh-cn/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names) |
| serving_input_config | string                                       | 否 | 预测配置                                                                                                                                |
| initiator            | string                                       | 否 | 发起方节点ID                                                                                                                            |
| parties              | [ServingParty](#serving-party)[]             | 否 | 参与方信息                                                                                                                              |


#### 响应（CreateServingResponse）

| 字段               | 类型                               | 可选 | 描述         |
|------------------|----------------------------------| ------ |------------|
| status           | [Status](summary_cn.md#status)   | 否   | 状态信息       |


{#update-serving}

### 更新Serving

#### HTTP路径
/api/v1/serving/update

#### 请求（UpdateServingRequest）

| 字段                   | 类型                                           | 可选 | 描述        |
|----------------------|----------------------------------------------|----|-----------|
| header               | [RequestHeader](summary_cn.md#requestheader) | 是  | 自定义请求内容   |
| serving_id           | string                                       | 否  | ServingID |
| serving_input_config | string                                       | 是  | 预测配置      |
| parties              | [ServingParty](#serving-party)[]             | 是  | 参与方信息     |


#### 响应（UpdateServingResponse）

| 字段     | 类型                             | 可选 | 描述   |
|--------|--------------------------------|----|------|
| status | [Status](summary_cn.md#status) | 否  | 状态信息 |


{#delete-serving}

### 删除Serving

#### HTTP路径
/api/v1/serving/delete

#### 请求（DeleteServingRequest）

| 字段           | 类型                                            | 可选 | 描述      |
|--------------|-----------------------------------------------|----|---------|
| header       | [RequestHeader](summary_cn.md#requestheader) | 是  | 自定义请求内容 |
| serving_id   | string                                       | 否  | ServingID  |

#### 响应（DeleteServingResponse）

| 字段     | 类型                             | 可选 | 描述   |
|--------|--------------------------------|----|------|
| status | [Status](summary_cn.md#status) | 否  | 状态信息 |


{#query-serving}

### 查询Serving

#### HTTP路径

/api/v1/serving/query

#### 请求（QueryServingRequest）

| 字段          | 类型                                          | 可选 | 描述           |
|-------------| ----------------------------------------------- | ------ | ---------------- |
| header      | [RequestHeader](summary_cn.md#requestheader) | 是   | 自定义请求内容 |
| serving_id  | string                                       | 否  | ServingID  |

#### 响应（QueryServingResponse）

| 字段                        | 类型                                            | 可选 | 描述        |
|---------------------------|-----------------------------------------------|----|-----------|
| status                    | [Status](summary_cn.md#status)                | 否  | 状态信息      |
| data                      | QueryServingResponseData                      |    |           |
| data.serving_input_config | string                                        | 否  | 预测配置      |
| data.initiator            | string                                        | 否  | 发起方节点ID   |
| data.parties              | [ServingParty](#serving-party)[]              | 否  | 参与方信息     |
| data.status               | [ServingStatusDetail](#serving-status-detail) | 否  | Serving状态 |


{#batch-query-serving-status}

### 批量查询Serving状态

#### HTTP路径

/api/v1/serving/status/batchQuery

#### 请求（BatchQueryServingStatusRequest）

| 字段           | 类型                                           | 可选 | 描述          |
|--------------|----------------------------------------------| ------ |-------------|
| header       | [RequestHeader](summary_cn.md#requestheader) | 是   | 自定义请求内容     |
| serving_ids  | string[]                                     | 否   | ServingID列表 |


#### 响应（BatchQueryServingStatusResponse）

| 字段             | 类型                                     | 可选 | 描述          |
|----------------|----------------------------------------| ------ |-------------|
| status         | [Status](summary_cn.md#status)         | 否   | 状态信息        |
| data           | BatchQueryServingStatusResponseData    |      |             |
| data.servings  | [ServingStatus](#serving-status)[]     | 否   | Serving状态列表 |


## 公共

{#serving-status}

### ServingStatus

| 字段          | 类型                                            | 可选 | 描述          |
|-------------|-----------------------------------------------| ----- |-------------|
| serving_id  | string                                        | 否  | ServingID   |
| status      | [ServingStatusDetail](#serving-status-detail) | 否  | Serving状态详情 |


{#serving-status-detail}

### ServingStatusDetail

| 字段                | 类型                                            | 可选 | 描述                            |
|-------------------|-----------------------------------------------|--|-------------------------------|
| state             | string                                        | 否 | Serving状态                     |
| reason            | string                                        | 是 | Serving处于该状态的原因，一般用于描述失败的状态   |
| message           | string                                        | 是 | Serving处于该状态的详细信息，一般用于描述失败的状态 |
| total_parties     | int32                                         | 否 | 参与方总数                         |
| available_parties | int32                                         | 否 | 可用参与方数量                       |
| create_time       | string                                        | 否 | 创建时间                          |
| party_statuses    | [PartyServingStatus](#party-serving-status)[] | 否 | 参与方状态                         |


{#party-serving-status}

### PartyServingStatus

| 字段                   | 类型                              | 可选 | 描述         |
|----------------------|---------------------------------|----|------------|
| domain_id            | string                          | 否  | 节点ID       |
| role                 | string                          | 是  | 角色         |
| state                | string                          | 否  | 状态         |
| replicas             | int32                           | 否  | 应用副本总数     |
| available_replicas   | int32                           | 否  | 应用可用副本数    |
| unavailable_replicas | int32                           | 否  | 应用不可用副本数   |
| updatedReplicas      | int32                           | 否  | 最新版本的应用副本数 |
| create_time          | string                          | 否  | 创建时间       |
| endpoints            | [Endpoint](#serving-endpoint)[] | 否  | 应用访问地址列表   |


{#serving-endpoint}

### Endpoint

| 字段        | 类型       | 可选  | 描述     |
|-----------|----------|-----|--------|
| endpoint  | string   | 否   | 应用访问地址 |


{#serving-party}

### ServingParty

| 字段            | 类型                               | 可选 | 描述                                                                                                         |
| ----------------- | ------------------------------------ | ------ |------------------------------------------------------------------------------------------------------------|
| domain_id       | string                             | 否   | 节点ID                                                                                                       |
| app_image       | string                             | 否   | 应用镜像，对应[AppImage](../concepts/appimage_cn.md)资源名称。<br/>在调用更新接口时，如果更新该字段，当前仅会使用新的[AppImage](../concepts/appimage_cn.md)资源中的应用镜像Name和Tag信息，更新预测应用。 |
| role            | string                             | 是   | 角色                                                                                                         |
| replicas        | int32                              | 是   | 应用总副本数                                                                                                     |
| update_strategy | [UpdateStrategy](#update-strategy) | 是   | 应用更新策略                                                                                                     |
| resources       | [Resource](#resource)[]            | 是   | 应用运行资源                                                                                                     |


{#update-strategy}

### UpdateStrategy

| 字段            | 类型   | 可选 | 描述                                                                                                                                                             |
| ----------------- | -------- | ------ |----------------------------------------------------------------------------------------------------------------------------------------------------------------|
| type            | string | 否   | 应用更新策略类型：支持"Recreate"和"RollingUpdate"两种类型<br/> "Recreate"：表示重建，在创建新的应用之前，所有现有应用都会被删除<br/> "RollingUpdate"：表示滚动更新，当应用更新时，结合"max_surge"和"max_unavailable"控制滚动更新过程 |
| max_surge       | string | 是   | 用来指定可以创建的超出应用总副本数的应用数量。默认为总副本数的"25%"。max_unavailable为0，则此值不能为0                                                                                                 |
| max_unavailable | string | 是   | 用来指定更新过程中不可用的应用副本个数上限。默认为总副本数的"25%"。max_surge为0，则此值不能为0                                                                                                        |


{#resource}

### Resource

| 字段           | 类型   | 可选 | 描述                                          |
| ---------------- | -------- | ------ |---------------------------------------------|
| container_name | string | 是   | 容器名称。若为空，则"cpu"和"memory"资源适用于所有容器           |
| min_cpu        | string | 是   | 最小cpu数量。例如："0.1"：表示100毫核；"1"：表示1核           |
| max_cpu        | string | 是   | 最大cpu数量。例如："0.1"：表示100毫核；"1"：表示1核           |
| min_memory     | string | 是   | 最小memory数量。单位："Mi"，"Gi"；例如："100Mi"：表示100兆字节 |
| max_memory     | string | 是   | 最大memory数量。单位："Mi"，"Gi"；例如："100Mi"：表示100兆字节 |
