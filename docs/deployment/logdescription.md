# 日志说明

## 前言

日志在应用部署、业务运行和故障排除的过程中起到了非常的重要，本文将详细的描述日志对应的路径。

## kuscia 目录结构
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
        |   └── kuscia.log
        ├── stdout
        └── storage
            └── data
</pre>

## 日志文件说明

| 路径| 内容   |
|:---------|:-------|
| /home/kuscia/var/logs/k3s.log |  记录了 k3s 相关的日志，当检测到 k3s 启动失败时，可以通过该日志排查问题  |
| /home/kuscia/var/logs/envoy/internal.log   |  记录了节点发出的请求日志（即本节点（+内部应用）访问其他节点的网络请求）,日志格式参考下文  |
| /home/kuscia/var/logs/envoy/external.log   |  记录了节点收到的请求日志（即其他节点访问本节点的网络请求）,日志格式参考下文 |
| /home/kuscia/var/logs/envoy/envoy.log      |  envoy 代理的日志文件,记录了 envoy 网关的运行状态、连接情况、流量信息以及问题排查等相关的内容        |
| /home/kuscia/var/stdout/pods/alice_xxxx/xxx/*.log |  任务的标准输出(stdout)的内容  |

### envoy 日志格式

internal.log 日志格式如下：
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



external.log 日志格式如下：
```bash
%DOWNSTREAM_REMOTE_ADDRESS_WITHOUT_PORT% - [%START_TIME(%d/%b/%Y:%H:%M:%S %z)%] %REQ(Kuscia-Source)% %REQ(Kuscia-Host?:authority)% \"%REQ(:METHOD)% %REQ(X-ENVOY-ORIGINAL-PATH?:PATH)% %PROTOCOL%\" %REQ(x-b3-traceid)% %REQ(x-b3-spanid)% %RESPONSE_CODE% %RESPONSE_FLAGS% %REQ(content-length)% %DURATION% %DYNAMIC_METADATA(envoy.kuscia:request_body)% %DYNAMIC_METADATA(envoy.kuscia:response_body)%
```

```bash
1.2.3.4 - [23/Oct/2023:04:36:51 +0000] bob kuscia-handshake.alice.svc "GET /handshake HTTP/1.1" 01e87a178e05f967 01e87a178e05f967 200 - - 0 - -
1.2.3.4 - [23/Oct/2023:04:36:53 +0000] tee kuscia-handshake.alice.svc "GET /handshake HTTP/1.1" 65a07630561d3814 65a07630561d3814 200 - - 0 - -
1.2.3.4 - [23/Oct/2023:04:37:06 +0000] bob kuscia-handshake.alice.svc "GET /handshake HTTP/1.1" 8537c88b929fee67 8537c88b929fee67 200 - - 0 - -
1.2.3.4 - [23/Oct/2023:04:37:08 +0000] tee kuscia-handshake.alice.svc "GET /handshake HTTP/1.1" 875d64696b98c6fa 875d64696b98c6fa 200 - - 0 - -
```
