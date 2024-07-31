# 任务运行网络错误排查

> 大多数任务失败是因为在运行任务前没做节点间授权，或者授权有问题，请提前参考[授权错误排查](../troubleshoot/network_authorization_check.md)确保授权没有问题

## 问题排查的准入---找到问题请求的 traceid
envoy 通过 http 请求头中 x-b3-traceid 字段来标识一个请求，根据 traceid 从 envoy 日志中的请求：
- 如果引擎日志有打印 traceid，从 task 日志中获取 traceid 即可。
- 如果引擎日志没有 traceid，则根据请求 taskid、url 或请求时间点，从 envoy 日志找到匹配的请求，从中找出traceid。
如下示例，envoy 的 internal.log 和 external.log 日志中 url 后面的第一个字段即 traceid。

```bash
1.1.1.1 - [06/Dec/2023:16:33:33 +0000] bob hypq-fuexafpf-node-3-0-fed.alice.svc "POST /org.interconnection.link.ReceiverService/Push HTTP/1.1" 0ce06cf5c3249d98 0ce06cf5c3249d98 200 32 0 - -
```

## 认识 gateway 的日志格式

### 日志格式

envoy 的日志参考[envoy日志](../deployment/logdescription.md#envoy)

### 怎么区分是入口流量还是出口流量呢？
通常来说，External.log 是的请求是入口流量，Internal.log里的请求是出口流量。但是存在一种特殊场景，即转发场景，Alice --> Bob --> Carol。在 Bob 上，如果收到一个发送给 Carol 的请求，比如请求的域名是 xxx.carol.svc，Alice 的请求是 ExternalPort 进去，然后转到 InternalPort，最后转发到 Carol 的网关所以 Bob 的 External 和 Intenral.log 里会有对应该请求的 traceid 的两条日志，也可以根据请求的域名判断 ServiceName 中的 NameSpace 是不是自己的节点

示例如下：
```bash
1.1.1.1 - [08/Dec/2023:07:21:10 +00001] alice mavis10-0-psi.carol.svc "POST /org.interconnection.link.ReceiverService/Push HTTP/1.1" 8c57cbc928bb598e 8c57cbc928bb598e 200 - 1398243 10 0 10 0 - -
```

## 正常请求的日志 Demo
查看 traceid=`b257a3410662f1f3`日志
- 发送端的internal.log
```bash
1.1.1.1 - [04/Jan/2024:05:52:40 +0000] bob squu-xaskaali-node-3-0-fed.alice.svc "POST /org.interconnection.link.ReceiverService/Push HTTP/1.1" b257a3410662f1f3 b257a3410662f1f3 200 - 149 0 0 0 0 - -
```
- 接收端的exernal.log
```bash
2.2.2.2 - [04/Jan/2024:05:52:40 +0000] bob squu-xaskaali-node-3-0-fed.alice.svc "POST /org.interconnection.link.ReceiverService/Push HTTP/1.1" b257a3410662f1f3 b257a3410662f1f3 200 - 149 0 - -
```