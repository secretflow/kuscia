# 网络要求

## 前言

在部署的过程中，复杂的网络环境导致网络通信出现问题时需要花费更多时间去排查问题。特别是在引入了机构网关的情况，因此为了确保节点间通信正常，我们需要对机构网关提出一些要求。

## 参数要求

如果节点与节点、节点与 master 之间存在网关，网关参数则需要满足如下要求：
- 需要支持 HTTP/1.1 协议
- Keepalive 超时时间大于 20 分钟
- 网关支持发送 Body <= 2MB 的内容
- 不针对 request/response 进行缓冲，以免造成性能低下；如果是 nginx 网关可以参考下文的配置

## nginx 代理参数配置示例

```bash
http {
    # Default is HTTP/1, keepalive is only enabled in HTTP/1.1
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_set_header Host $http_host;

    # To allow special characters in headers
    ignore_invalid_headers off;

    # Maximum number of requests through one keep-alive connection
    keepalive_requests 1000;
    keepalive_timeout 20m;

    client_max_body_size 2m;

    # To disable buffering
    proxy_buffering off;
    proxy_request_buffering off;

    upstream backend {
        server 0.0.0.1;

        keepalive 32;
        keepalive_timeout 600s;
        keepalive_requests 1000;
    }

    server {
        location / {
            proxy_pass http://backend;
        }
    }
}
```