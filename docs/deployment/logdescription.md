# 日志说明

## 前言

日志在应用部署、业务运行和故障排除的过程中起到了非常的重要，本文将详细的描述日志对应的路径。

## Kuscia 目录结构
<pre>
/home/kuscia
    ├── bin
    ├── crds
    |   └── v1alpha1
    ├── etc
    |   ├── certs
    |   ├── cni
    |   ├── conf
    |   ├── kubeconfig
    |   ├── kuscia.kubeconfig
    |   └── kuscia.yaml
    ├── pause
    |   └──  pause.tar
    ├── scripts
    |   ├── deploy
    |   ├── templates
    |   ├── test
    |   ├── tools
    |   └── user
    └──  var
        ├── k3s
        ├── logs
        |   ├── envoy
        |   |   ├── envoy.log
        |   |   ├── envoy_admin.log
        |   |   ├── external.log
        |   |   ├── internal.log
        |   |   ├── kubernetes.log
        |   |   ├── prometheus.log
        |   |   └── zipkin.log
        |   ├── k3s.log
        |   ├── kusciaapi.log
        |   ├── containerd.log
        |   └── kuscia.log
        |
        ├── stdout
        └── storage
            └── data
</pre>

## 日志文件说明

| 路径| 内容   |
|:---------|:-------|
| `/home/kuscia/var/logs/kuscia.log` |  记录了 Kuscia 启动状态、节点状态、健康检查、任务调度等相关的日志，当 Kuscia 启动、任务运行失败时，可以通过该日志排查问题  |
| `/home/kuscia/var/logs/k3s.log` |  记录了 k3s 相关的日志，当检测到 k3s 启动失败时，可以通过该日志排查问题  |
| `/home/kuscia/var/logs/containerd.log` | 记录了 containerd 相关的日志，containerd 启动失败、镜像更新存储等可以通过该日志查询 |
| `/home/kuscia/var/logs/kusciaapi.log` | 记录了所有 KusciaAPI 的调用请求与响应日志 |
| `/home/kuscia/var/logs/envoy/internal.log`   |  记录了节点发出的请求日志（即本节点（+内部应用）访问其他节点的网络请求）,日志格式参考下文  |
| `/home/kuscia/var/logs/envoy/external.log`  |  记录了节点收到的请求日志（即其他节点访问本节点的网络请求）,日志格式参考下文 |
| `/home/kuscia/var/logs/envoy/envoy.log`      |  envoy 代理的日志文件,记录了 envoy 网关的运行状态、连接情况、流量信息以及问题排查等相关的内容        |
| `/home/kuscia/var/stdout/pods/alice_xxxx/xxx/*.log` |  任务的标准输出(stdout)的内容  |

:::{tip}
K8s RunK 部署模式需要在 Kuscia Pod 所在的 K8s 集群里执行 `kubectl logs ${engine_pod_name} -n xxx` 查看任务的标准输出日志
:::

### envoy 日志格式

`internal.log` 日志格式如下：
```bash
%DOWNSTREAM_REMOTE_ADDRESS_WITHOUT_PORT% - [%START_TIME(%d/%b/%Y:%H:%M:%S %z)%] %REQ(Kuscia-Source)% %REQ(Kuscia-Host?:authority)% \"%REQ(:METHOD)% %REQ(X-ENVOY-ORIGINAL-PATH?:PATH)% %PROTOCOL%\" %REQ(x-b3-traceid)% %REQ(x-b3-spanid)% %RESPONSE_CODE% %RESPONSE_FLAGS% %REQ(content-length)% %DURATION% %REQUEST_DURATION% %RESPONSE_DURATION% %RESPONSE_TX_DURATION% %DYNAMIC_METADATA(envoy.kuscia:request_body)% %DYNAMIC_METADATA(envoy.kuscia:response_body)%
```
```bash
# 示例如下：
1.2.3.4 - [23/Oct/2023:01:58:02 +0000] alice fgew-cwqearkz-node-4-0-fed.bob.svc "POST /org.interconnection.link.ReceiverService/Push HTTP/1.1" 743d0da7e6814c2e 743d0da7e6814c2e 200 - 1791 0 0 0 0 - -
1.2.3.4 - [23/Oct/2023:01:58:02 +0000] alice fgew-cwqearkz-node-4-0-fed.bob.svc "POST /org.interconnection.link.ReceiverService/Push HTTP/1.1" b2f636af87a047f8 b2f636af87a047f8 200 - 56 0 0 0 0 - -
1.2.3.4 - [23/Oct/2023:01:58:03 +0000] alice fgew-cwqearkz-node-4-0-fed.bob.svc "POST /org.interconnection.link.ReceiverService/Push HTTP/1.1" fdd0c66dfb0fbe45 fdd0c66dfb0fbe45 200 - 56 0 0 0 0 - -
1.2.3.4 - [23/Oct/2023:01:58:03 +0000] alice fgew-cwqearkz-node-4-0-fed.bob.svc "POST /org.interconnection.link.ReceiverService/Push HTTP/1.1" dc52437872f6e051 dc52437872f6e051 200 - 171 0 0 0 0 - -
```
 internal.log 格式说明如下：
| 属性               | 值                                                 |
| ------------------ | -------------------------------------------------- |
| `对端节点的 IP`      | 1.2.3.4                                          |
| `收到请求时间`       | 23/Oct/2023:01:58:02 +0000                        |
| `发送节点`           | alice                                              |
| `请求的域名`         | fgew-cwqearkz-node-4-0-fed.bob.svc             |
| `URL`                | /org.interconnection.link.ReceiverService/Push   |
| `HTTP 方法/版本`          | HTTP/1.1                                             |
| `TRANCEID`            | 743d0da7e6814c2e                                 |
| `SPANID`             | 743d0da7e6814c2e                                 |
| `HTTP 返回码`        | 200                                              |
| `RESPONSE_FLAGS`     | -，表示有关响应或连接的其他详细信息，详情可以参考[envoy官方文档](https://www.envoyproxy.io/docs/envoy/v1.25.0/configuration/observability/access_log/usage#command-operators)                     |
| `CONTENT-LENGTH`     | 1791，表示 body 的长度                              |
| `DURATION`           | 0，表示请求总耗时                                |
| `REQ_META`          |  0，表示请求body的meta信息                      |
| `RES_META`           | 0，表示请求body的meta信息                     |
| `REQUEST_DURATION`   | 0，接收下游请求报文的时间                           |
| `RESPONSE_DURATION`  | -，从请求开始到响应开始的时间                        |
| `RESPONSE_TX_DURATION` |-，发送上游回包的时间                               |



`external.log` 日志格式如下：
```bash
%DOWNSTREAM_REMOTE_ADDRESS_WITHOUT_PORT% - [%START_TIME(%d/%b/%Y:%H:%M:%S %z)%] %REQ(Kuscia-Source)% %REQ(Kuscia-Host?:authority)% \"%REQ(:METHOD)% %REQ(X-ENVOY-ORIGINAL-PATH?:PATH)% %PROTOCOL%\" %REQ(x-b3-traceid)% %REQ(x-b3-spanid)% %RESPONSE_CODE% %RESPONSE_FLAGS% %REQ(content-length)% %DURATION% %DYNAMIC_METADATA(envoy.kuscia:request_body)% %DYNAMIC_METADATA(envoy.kuscia:response_body)%
```

```bash
1.2.3.4 - [23/Oct/2023:04:36:51 +0000] bob kuscia-handshake.alice.svc "GET /handshake HTTP/1.1" 01e87a178e05f967 01e87a178e05f967 200 - - 0 - -
1.2.3.4 - [23/Oct/2023:04:36:53 +0000] tee kuscia-handshake.alice.svc "GET /handshake HTTP/1.1" 65a07630561d3814 65a07630561d3814 200 - - 0 - -
1.2.3.4 - [23/Oct/2023:04:37:06 +0000] bob kuscia-handshake.alice.svc "GET /handshake HTTP/1.1" 8537c88b929fee67 8537c88b929fee67 200 - - 0 - -
1.2.3.4 - [23/Oct/2023:04:37:08 +0000] tee kuscia-handshake.alice.svc "GET /handshake HTTP/1.1" 875d64696b98c6fa 875d64696b98c6fa 200 - - 0 - -
```

 external.log 格式说明如下：
| 属性               | 值                                                 |
| ------------------ | -------------------------------------------------- |
| `对端节点的 IP`      | 1.2.3.4                                          |
| `收到请求时间`       | 23/Oct/2023:01:58:02 +0000                        |
| `发送节点`           | alice                                              |
| `请求的域名`         | fgew-cwqearkz-node-4-0-fed.bob.svc             |
| `URL`                | /org.interconnection.link.ReceiverService/Push   |
| `HTTP 方法/版本`          | HTTP/1.1                                             |
| `TRANCEID`            | 743d0da7e6814c2e                                 |
| `SPANID`             | 743d0da7e6814c2e                                 |
| `HTTP 返回码`        | 200                                              |
| `RESPONSE_FLAGS`     | -，表示有关响应或连接的其他详细信息，详情可以参考[envoy官方文档](https://www.envoyproxy.io/docs/envoy/v1.25.0/configuration/observability/access_log/usage#command-operators)                     |
| `CONTENT-LENGTH`     | 1791，表示 body 的长度                              |
| `DURATION`           | 0，表示请求总耗时                                |
| `REQ_META`          |  0，表示请求body的meta信息                      |
| `RES_META`           | 0，表示请求body的meta信息                     |