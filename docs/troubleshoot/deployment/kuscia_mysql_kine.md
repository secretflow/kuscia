# Kine 表问题导致 Kuscia 启动失败

### Kine 表不存在

Kuscia 启动失败可能是 Kine 表在数据库中不存在导致的，可以使用如下方法解决：

1. 部署 Kuscia 前先手动创建好 Kine 表，建表语句参考[kine](https://github.com/secretflow/kuscia/blob/main/hack/k8s/kine.sql)。
2. 提供的数据库账号有建表权限（账号具有`DDL+DML`权限），并且数据表不存在，kuscia 会尝试自动建表，如果创建失败 kuscia 会启动失败。

### Kine 多节点混用导致启动失败

Kuscia 启动失败可能是多个节点使用同一张表导致的，报错示例如下：
```bash
2024-06-27 19:22:53.070 ERROR modules/k3s.go:283 context canceled
2024-06-27 19:22:53.070 FATAL modules/modules.go:181 error building kubernetes client config from token, detail-> build config from flags failed, detail-> stat /home/kuscia/etc/kubeconfig: no such file or directory
```
可以使用如下方法解决：

1. 确保一个节点（Domain） 使用一张 Kine 表。
2. 复用 Kine 表需保证 DomainID 与之前的节点保持一致。