# MTU 配置导致的网络异常

## MTU 简述
MTU（Maximum Transmission Unit）是网络中数据包的最大传输单元，即数据链路层一次能传输的最大数据量。MTU 的配置不当会导致网络传输效率低下，甚至导致网络超时。

## 问题描述
Kuscia 内引擎 Pod 网卡 MTU 配置默认为 1500，如果机构的网络网卡 MTU 限制与 Kuscia 默认配置不匹配时（例如，机构网卡 MTU 为 1350，Kuscia Pod MTU 1500），会导致任务网络超时。报错如下：

```bash
botocore.exceptions.ReadTimeoutError: Read timeout on endpoint URL: \"http://1.2.3.4:9900/xxx/data/linear_b_label_1M_1D.csv\
```

## 解决方案

### Kuscia 容器修改 MTU

1. 所有 docker 容器统一修改 MTU

```bash
sudo tee /etc/docker/daemon.json <<-'EOF'
{
  "mtu": 1350
}
EOF
sudo systemctl daemon-reload
sudo systemctl restart docker
```

2. 单个容器修改 MTU

与 kuscia-exchange 网络断开连接

```bash
docker network disconnect kuscia-exchange <kuscia_container_id>
```

删除 kuscia-exchange 网络

```bash
docker network rm kuscia-exchange
```

重新创建 kuscia-exchange 网络，并设置 MTU

```bash
docker network create --opt com.docker.network.driver.mtu=1350 kuscia-exchange
```

将 kuscia 容器重新连接到 kuscia-exchange 网络

```bash
docker network connect kuscia-exchange <kuscia_container_id>
```

### Kuscia 内引擎 Pod 修改 MTU

1. 临时方案

登录到 kuscia 容器

```bash
docker exec -it <kuscia_container_id> bash
```

修改 cni 配置文件 10-containerd-net.conflist，示例如下：

```bash
vi /home/kuscia/etc/cni/net.d/10-containerd-net.conflist
```
```json
{
  "cniVersion": "1.0.0",
  "name": "containerd-net",
  "plugins": [
    {
      "type": "bridge",
      "bridge": "cni0",
      "isGateway": true,
      "ipMasq": true,
      "promiscMode": true,
      "ipam": {
        "type": "host-local",
        "ranges": [
          [{
            "subnet": "10.88.0.0/16"
          }]
        ],
        "routes": [
          { "dst": "0.0.0.0/0" }
        ]
      },
     "mtu": 1350
    },
    {
      "type": "portmap",
      "capabilities": {"portMappings": true}
    }
  ]
}
```

重启 kuscia 容器

```bash
docker restart <kuscia_container_id>
```

2. 长期方案

可以通过修改 kuscia 代码并重新编译镜像生效，详情参考[github](https://github.com/secretflow/kuscia/blob/main/etc/cni/net.d/10-containerd-net.conflist)
