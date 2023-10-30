# Health

Health 提供了服务的健康检查，你可以借助这些 API 了解服务的健康状态。
你可以从 [这里](https://github.com/secretflow/kuscia/tree/main/proto/api/v1alpha1/kusciaapi/health.proto) 找到对应的 protobuf 文件。

## 接口总览

| 方法名                 | 请求类型          | 响应类型           | 描述     |
|---------------------|---------------|----------------|--------|
| [healthZ](#healthZ) | HealthRequest | HealthResponse | 服务是否健康 |

## 接口详情

{#healthZ}

### 健康检查

#### HTTP路径
/healthZ

#### 请求（HealthRequest）

| 字段     | 类型                                           | 选填 | 描述      |
|--------|----------------------------------------------|----|---------|
| header | [RequestHeader](summary_cn.md#requestheader) | 可选 | 自定义请求内容 |

#### 响应（HealthResponse）

| 字段         | 类型                             | 选填 | 描述   |
|------------|--------------------------------|----|------|
| status     | [Status](summary_cn.md#status) | 必填 | 状态信息 |
| data       | HealthResponseData             |    |      |
| data.ready | bool                           | 必填 | 是否就绪 |

