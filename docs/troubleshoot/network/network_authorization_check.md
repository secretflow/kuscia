# 授权错误排查

## 排查步骤

### 检查授权状态以及参数是否符合预期
登录到容器（中心化模式在 master 容器、点对点模式在 automony 容器）并执行`kubectl get cdr`命令查看授权信息，READY 为 True 时表明通信正常，为空时表明通信异常，可以先看下 HOST 和端口是否正确或者执行 `kubectl get cdr ${cdr_name} -oyaml` 命令看下详细信息，参数确认无误仍无法正常通信请参考下文。
- 授权正常示例如下：
```bash
[root@root-kuscia-master kuscia]# kubectl get cdr
NAME                     SOURCE     DESTINATION     HOST
bob-alice                bob        alice           1.1.1.1             Token            True
alice-bob                alice      bob             2.2.2.2             Token            True
```

### 查看 curl 返回结果
大部分的网络问题都是网络协议（http/https/mtls）、IP 、端口（包括端口映射）不正确导致的， 最简单的方法就是通过在`源节点`手动访问（curl）`目标节点的地址`来判定。比如：curl -kvvv https://1.1.1.1:18080（此处为示例 ip 与端口），通过 curl 命令能够快速分析网络授权异常原因，curl 返回的结果一般分为以下几种情况：
- 返回正常结果为 401，示例如下：
```bash
*   Trying 1.1.1.1:18080...
* Connected to 1.1.1.1 (1.1.1.1) port 18080 (#0)
* ALPN, offering h2
* ALPN, offering http/1.1
* TLSv1.3 (OUT), TLS handshake, Client hello (1):
* TLSv1.3 (IN), TLS handshake, Server hello (2):
* TLSv1.3 (IN), TLS handshake, Encrypted Extensions (8):
* TLSv1.3 (IN), TLS handshake, Certificate (11):
* TLSv1.3 (IN), TLS handshake, CERT verify (15):
* TLSv1.3 (IN), TLS handshake, Finished (20):
* TLSv1.3 (OUT), TLS change cipher, Change cipher spec (1):
* TLSv1.3 (OUT), TLS handshake, Finished (20):
* SSL connection using TLSv1.3 / TLS_AES_256_GCM_SHA384
* ALPN, server did not agree to a protocol
* Server certificate:
*  subject: CN=kuscia-system_ENVOY_EXTERNAL
*  start date: Sep 14 07:42:47 2023 GMT
*  expire date: Jan 30 07:42:47 2051 GMT
*  issuer: CN=Kuscia
*  SSL certificate verify result: unable to get local issuer certificate (20), continuing anyway.
> GET / HTTP/1.1
> Host: 1.1.1.1:18080
> User-Agent: curl/7.82.0
> Accept: */*
>
* TLSv1.3 (IN), TLS handshake, Newsession Ticket (4):
* TLSv1.3 (IN), TLS handshake, Newsession Ticket (4):
* old SSL session ID is stale, removing
* Mark bundle as not supporting multiuse
< HTTP/1.1 401 Unauthorized
< x-accel-buffering: no
< content-length: 13
< content-type: text/plain
< kuscia-error-message: Domain kuscia-system.root-kuscia-master<--1.1.1.1 return http code 401.
< date: Fri, 15 Sep 2023 02:50:39 GMT
< server: kuscia-gateway
<
* Connection #0 to host 1.1.1.1 left intact
{"domain":"alice","instance":"xyz","kuscia":"v0.1","reason":"unauthorized."}
```

- 一直处于请求连接状态，示例如下：
```bash
* About to connect() to 1.1.1.1 port 18080 (#0)
*   Trying 1.1.1.1...
```
这种情况通常是因为对端机构侧服务器没有给本端服务器出口 ip 加白，可以先 curl cip.cc 查询本端服务器出口 ip 给到对方加白。

-  请求返回报错 Connection refused，示例如下：
```bash
* Closing connection 0
curl: (7) Failed to connect to 1.1.1.1 port 18080 after 248 ms: Connection refused
```
这种情况通常表示对端机构侧服务器已经给本端服务器加白了，但是对端服务器服务没有拉起或者端口没有打开，可以在对端服务器上 `docker ps` `netstat -anutp | grep "${port}"` 看下机器映射的端口是否正常打开。

- 其他各种非正常 401 返回的情形需要配合对方机构侧检查网络链路，例如：防火墙阻拦、网关代理配置有问题等。如果中间有机构网关，可以参考[网络要求](../../deployment/networkrequirements.md)的文档来确保网关符合要求。

### 查看网关日志
通过日志能够很好的分析 Kuscia 运行状态、连接情况、流量信息等，详细内容请参考[日志说明](../../deployment/logdescription.md/#envoy)

### 分析网络拓扑、使用抓包工具
在复杂的网络环境中，可以先整理两方机构之间的网络拓扑，以便于更加清晰、快速的定位，再配合 Tcpdump、Wireshark 等抓包工具进行排查。一个机构的网络拓扑可以参考[网络要求](../../deployment/networkrequirements.md)