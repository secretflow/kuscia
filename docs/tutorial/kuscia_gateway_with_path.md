# 如何配置 Kuscia 对请求进行 Path Rewrite

## 背景

隐私计算合作机构之间的网络较为复杂，经常存在多层次的网关，网关根据 Path 将请求路由到真正的业务节点。为了给这种组网提供支持，Kuscia 能够对业务请求进行 Path Rewrite，将对应的路由前缀添加到请求 Path。
![image.png](../imgs/gateway_path.png)

:::{tip}
网关要求：机构网关处需要进行 Path 前缀卸载。
:::

## 多机部署配置 Path Rewrite
- Kuscia 中心化部署参考[这里](../deployment/Docker_deployment_kuscia/deploy_master_lite_cn.md)
- Kuscia 点对点部署参考[这里](../deployment/Docker_deployment_kuscia/deploy_p2p_cn.md)

节点之间通信配置示例：
```bash
# lite 访问 master 地址，此处 1.1.1.1 为网关地址
http://1.1.1.1/master
```

## 使用 KusciaAPI 配置 Path Rewrite
使用 KusciaAPI 要配置一条 Path Rewrite 路由规则，需要设置`endpoint`的`prefix`字段。

下面以 alice 访问 bob 的场景为例，当 bob 机构网关地址带 Path 时如何调用 KusciaAPI 设置`endpoint`的`prefix`字段。
```bash
# 在容器内执行示例
# --cert 是请求服务端进行双向认证使用的证书
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/route/create'
--header "Token: $(cat ${CTR_CERTS_ROOT}/token)"
--header 'Content-Type: application/json'
--cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt
--key ${CTR_CERTS_ROOT}/kusciaapi-server.key
--cacert ${CTR_CERTS_ROOT}/ca.crt  -d '{
  "authentication_type": "Token",
  "destination": "bob",
  "endpoint": {
    "host": "1.1.1.1",
    "ports": [
      {
        "port": 80,
        "protocol": "HTTP",
        "isTLS": true, # 如果网关为域名并且支持 https, 可以设置 true, 否则为 false
        "path_prefix": "/bob"
      }
    ]
  },
  "source": "alice",
  "token_config": {
    "token_gen_method": "RSA-GEN"
  }
}'
```