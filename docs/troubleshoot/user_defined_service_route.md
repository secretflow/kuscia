# 用户自定义 Service 路由

## 说明
通过复用 Kuscia 提供的网络打平功能，可以在 alice 和 bob 中自定义 service，进行请求转发。

## 示例
下面是Alice和Bob的管理平台之间需要进行通信示例：

- 在Alice节点容器内，手动创建一个 ExternalName 类型的 Service， 其中 ExternalName 设置为 Alice 平台的地址，例如:
```bash
apiVersion: v1
kind: Service
metadata:
  name: alice-pad
  namespace: alice
spec:
  # 10.88.0.2为alice平台服务的地址
  externalName: 10.88.0.2
  ports:
  - name: cluster
  # 10010为 alice 平台服务的端口
    port: 10010
  type: ExternalName
status:
  loadBalancer: {}
```
内容 copy 到 alice-pad.yaml，执行 `kubectl create -f alice-pad.yaml` 创建

- 在 Bob 节点容器内，手动创建一个 ExternalName 类型的 Service, 其中 ExternalName 设置为 Bob 平台的地址，例如:
```bash
apiVersion: v1
kind: Service
metadata:
  name: bob-pad
  namespace: bob
spec:
  # 10.88.0.3为bob平台服务的地址
  externalName: 10.88.0.3
  ports:
  - name: cluster
  # 10010为 bob 平台服务的端口
    port: 10010
  type: ExternalName
status:
  loadBalancer: {}
```
内容 copy 到 bob-pad.yaml，执行 `kubectl create -f bob-pad.yaml` 创建

## 访问方法
下面是Alice访问Bob侧平台的方法，反之类似：

- 若在 Alice Docker 容器内，直接访问 Bob 平台的方式：`curl -v http://bob-pad.bob.svc`
- 若在 Alice Docker 容器外，那么需要把 Alice 节点的 80 端口暴露到宿主机上，然后通过 `curl -v http://127.0.0.1:{暴露在宿主机上的端口} -H "host:bob-pad.bob.svc"`

> Tips：通过上述方式，将 Service 暴露出来后，虽然 Kuscia 做了安全性的防护（只有授权后的节点才能访问到该 Service），但是毕竟是内部服务暴露出来给其他机构，请注意服务自身的安全性加强，比如越权漏洞等。