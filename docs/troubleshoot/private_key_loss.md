# Lite 节点遗漏证书之后如何重新部署

## 前言

出于安全性的考虑，如果 master 已经有了节点的证书并且机构侧节点已经部署完成，此时节点证书丢失且想要复用节点，需要执行下文步骤。

## 重新部署节点

### 步骤一
在 master 节点上删除 lite 之前的节点证书，证书字段设置为空。详情参考[这里](../reference/apis/domain_cn.md#update-domain)
示例如下：
```bash
curl --cert /home/kuscia/var/certs/kusciaapi-server.crt \
     --key /home/kuscia/var/certs/kusciaapi-server.key \
     --cacert /home/kuscia/var/certs/ca.crt \
     --header 'Token: {token}' --header 'Content-Type: application/json' \
     'https://{{USER}-kuscia-master}:8082/api/v1/domain/update' \
     -d '{
    "domain_id": "${节点ID}",
    "cert": ""
}'
```

### 步骤二
向 master 重新申请 Token 并部署节点。详情参考[这里](../deployment/Docker_deployment_kuscia/deploy_master_lite_cn.md#lite-alice)