# Log

## 接口详情

### 查询日志

#### 说明

查询某个 Kuscia 任务的运行日志

- 仅支持 Lite 和 Autonomy节点，不支持 Master 节点
- 仅支持查询任务本方节点的日志，即任务如果有 Alice，Bob 多方参与，调用 Alice 节点的接口只会查询 Alice 节点的运行日志
- 调用查询日志接口时，对于重启了多次的 pod 容器，会查询最新一次重启的 pod 容器的日志文件并返回

#### HTTP 路径

/api/v1/log/task/query

#### 请求（QueryLogRequest）

| 字段              | 类型                                           | 选填 | 描述                                                                                                                         |
|-----------------|----------------------------------------------|----|----------------------------------------------------------------------------------------------------------------------------|
| header          | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容                                                                                                                    |
| task_id          | string                                       | 必填 | TaskID |
| replica_idx       | int                                       | 可选 | Task对应的Pod副本索引(从0开始)；默认不填时，单副本时直接展示，多副本时选择第一个副本展示                                                                                                                   |
| container | string                                        | 可选 | 容器名，默认不填时，Task对应的Pod只有一个容器时展示，存在多个容器时报错                                                                         |
| follow           | bool                              | 可选 | 是否跟踪pod日志，默认为false（不跟踪）                                                                                                                       |

#### 响应（QueryLogResponse）

流式返回响应结果，每次返回的结果如下：

| 字段                   | 类型                                    | 描述     |
|----------------------|---------------------------------------|--------|
| status | [Status](summary_cn.md#status) | 状态信息 |
| log               | string        | 正确时返回日志内容（每次返回多行），错误时返回错误信息。  |

#### 请求示例

发起请求

```sh
# 在容器内执行示例
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -v -k -X POST 'https://localhost:8082/api/v1/log/task/query' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "task_id": "secretflow-task-20241101160338-single-psi",
  "replica_idx": 0,
  "container": "secretflow",
  "follow": false
}'
```

请求响应成功结果：

```
{"status":{"message":"success"},"log":"abcd"}
{"status":{"message":"success"},"log":"efgh"}
```
