# 如何使用 Kuscia API 部署 DataProxy

本教程将以 [Kuscia API Serving](../reference/apis/serving_cn) 作为参考，介绍如何使用 Kuscia API 部署 DataProxy。

## 准备节点

- Docker 部署节点请参考[这里](../deployment/Docker_deployment_kuscia/index.rst)。
- K8s 部署节点请参考[这里](../deployment/K8s_deployment_kuscia/index.rst)。

本示例在**点对点组网模式**下完成。在中心化组网模式下，证书的配置会有所不同。

{#cert-and-token}

## 获取 KusciaAPI 证书和 Token

如果 KusciaAPI 启用了 MTLS 协议，则需要提前准备好 MTLS 证书和 Token。协议参考[这里](../troubleshoot/concept/protocol_describe.md)。

### 点对点组网模式

证书的配置参考[配置授权](../deployment/Docker_deployment_kuscia/deploy_p2p_cn.md#配置授权)

这里以 alice 节点为例，接口需要的证书文件在 ${USER}-kuscia-autonomy-alice 节点的 `/home/kuscia/var/certs/` 目录下：

| 文件名               | 文件功能                                                |
| -------------------- | ------------------------------------------------------- |
| kusciaapi-server.key | 服务端私钥文件                                          |
| kusciaapi-server.crt | 服务端证书文件                                          |
| ca.crt               | CA 证书文件                                             |
| token                | 认证 Token ，在 headers 中添加 Token: { token 文件内容} |

### 中心化组网模式

证书文件在 ${USER}-kuscia-master 节点的 `/home/kuscia/var/certs/` 目录下：

| 文件名               | 文件功能                                                |
| -------------------- | ------------------------------------------------------- |
| kusciaapi-server.key | 服务端私钥文件                                          |
| kusciaapi-server.crt | 服务端证书文件                                          |
| ca.crt               | CA 证书文件                                             |
| token                | 认证 Token ，在 headers 中添加 Token: { token 文件内容} |

## K8s 模式部署 DataProxy

下面以 K8s RunK 模式为例。

1. 修改 cm 配置文件，并重启节点

   ```bash
   # Edit the cm configuration file
   kubectl edit cm kuscia-autonomy-alice-cm -n autonomy-alice

   # Add dataMesh configuration, align the format with the protocol field
   protocol: MTLS
   dataMesh:
     dataProxyList:
       - endpoint: "dataproxy-grpc:8023"
         dataSourceTypes:
           - "odps"

   # Restart the node
   kubectl delete pod kuscia-autonomy-alice-xxxx -n autonomy-alice
   ```

2. 登录节点

   ```bash
   kubectl exec -it kuscia-autonomy-alice-xxxx bash -n autonomy-alice
   ```

3. 注册 AppImage

   ```bash
   scripts/deploy/register_app_image.sh -i "secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/dataproxy:0.1.0b1" -m
   ```

4. 使用 Kuscia API 部署 DataProx

   下面以 MTLS 协议为例，其他协议请参考[这里](../troubleshoot/concept/protocol_describe.md)。

   部署 DataProxy

   ```bash
   export CTR_CERTS_ROOT=/home/kuscia/var/certs
   curl -X POST 'https://localhost:8082/api/v1/serving/create' \
    --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
    --header 'Content-Type: application/json' \
    --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
    --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
    --cacert ${CTR_CERTS_ROOT}/ca.crt \
    -d '{
       "serving_id": "dataproxy-alice",
       "initiator": "alice",
       "parties": [{
           "app_image": "dataproxy-image",
           "domain_id": "alice",
           "service_name_prefix": "dataproxy"
         }
       ]
    }'
   ```

   查询 DataProxy

   ```bash
   export CTR_CERTS_ROOT=/home/kuscia/var/certs
   curl -X POST 'https://localhost:8082/api/v1/serving/query' \
    --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
    --header 'Content-Type: application/json' \
    --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
    --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
    --cacert ${CTR_CERTS_ROOT}/ca.crt \
    -d '{
       "serving_id": "dataproxy-alice",
       "domain_id": "alice"
    }' | jq
   ```

   删除 DataProxy

   ```bash
   export CTR_CERTS_ROOT=/home/kuscia/var/certs
   curl -X POST 'https://localhost:8082/api/v1/serving/delete' \
    --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
    --header 'Content-Type: application/json' \
    --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
    --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
    --cacert ${CTR_CERTS_ROOT}/ca.crt \
    -d '{
       "domain_id": "alice",
       "serving_id": "dataproxy-alice"
    }'
   ```

5. 验证部署完成

   执行如下命令看到 pod 为 running 代表 DataProxy 部署成功。

   ```bash
   # View pod status
   kubectl get po -n alice

   NAME                               READY   STATUS    RESTARTS   AGE
   dataproxy-alice-67cfc45499-xdfxr   1/1     Running   0          97s

   # Service network detection
   ping dataproxy-grpc

   PING dataproxy-grpc (1.1.1.1) 56(84) bytes of data.
   64 bytes from 1.1.1.1 (1.1.1.1): icmp_seq=1 ttl=63 time=0.074 ms
   From kubernetes.default.svc.cluster.local (1.1.1.1) icmp_seq=2 Redirect Host(New nexthop: 1.1.1.1 (1.1.1.1))
   64 bytes from 1.1.1.1 (1.1.1.1): icmp_seq=2 ttl=63 time=0.053 ms
   ```

## Docker 模式部署 DataProxy

下面以 Docker RunC 模式为例。

1. 修改 cm 配置文件，并重启节点

   ```bash
   # Edit the kuscia.yaml configuration file
   vi autonomy_alice.yaml

   # Add dataMesh configuration, align the format with the datastoreEndpoint field
   datastoreEndpoint: ""
   dataMesh:
     dataProxyList:
       - endpoint: "dataproxy-grpc:8023"
         dataSourceTypes:
           - "odps"

   # Restart the node
   docker restart root-kuscia-autonomy-alice
   ```

2. 登录节点

   ```bash
   docker exec -it root-kuscia-autonomy-alice bash
   ```

3. 注册 AppImage

   ```bash
   scripts/deploy/register_app_image.sh -i "secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/dataproxy:0.1.0b1" -m
   ```

4. 使用 Kuscia API 部署 DataProx

   下面以 MTLS 协议为例，其他协议请参考[这里](../troubleshoot/concept/protocol_describe.md)。

   部署 DataProxy

   ```bash
   export CTR_CERTS_ROOT=/home/kuscia/var/certs
   curl -X POST 'https://localhost:8082/api/v1/serving/create' \
    --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
    --header 'Content-Type: application/json' \
    --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
    --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
    --cacert ${CTR_CERTS_ROOT}/ca.crt \
    -d '{
       "serving_id": "dataproxy-alice",
       "initiator": "alice",
       "parties": [{
           "app_image": "dataproxy-image",
           "domain_id": "alice",
           "service_name_prefix": "dataproxy"
         }
       ]
    }'
   ```

   查询 DataProxy

   ```bash
   export CTR_CERTS_ROOT=/home/kuscia/var/certs
   curl -X POST 'https://localhost:8082/api/v1/serving/query' \
    --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
    --header 'Content-Type: application/json' \
    --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
    --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
    --cacert ${CTR_CERTS_ROOT}/ca.crt \
    -d '{
       "serving_id": "dataproxy-alice",
       "domain_id": "alice"
    }' | jq
   ```

   删除 DataProxy

   ```bash
   export CTR_CERTS_ROOT=/home/kuscia/var/certs
   curl -X POST 'https://localhost:8082/api/v1/serving/delete' \
    --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
    --header 'Content-Type: application/json' \
    --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
    --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
    --cacert ${CTR_CERTS_ROOT}/ca.crt \
    -d '{
       "domain_id": "alice",
       "serving_id": "dataproxy-alice"
    }'
   ```

5. 验证部署完成

   执行如下命令看到 pod 为 running 代表 DataProxy 部署成功。

   ```bash
   # View pod status
   kubectl get po -n alice

   NAME                               READY   STATUS    RESTARTS   AGE
   dataproxy-alice-67cfc45499-xdfxr   1/1     Running   0          97s

   # Service network detection
   ping dataproxy-grpc

   PING dataproxy-grpc (1.1.1.1) 56(84) bytes of data.
   64 bytes from 1.1.1.1 (1.1.1.1): icmp_seq=1 ttl=64 time=0.035 ms
   64 bytes from 1.1.1.1 (1.1.1.1): icmp_seq=2 ttl=64 time=0.029 ms
   ```
