# 部署失败日志排查

## 排查步骤

### 检查机器配置

若机器不满足[官网推荐配置](https://www.secretflow.org.cn/docs/kuscia/latest/zh-Hans/getting_started/quickstart_cn#id2)，可能会造成部分服务无法正常工作，从而导致部署失败。

### 查看服务日志

#### 中心化组网模式

查看 Master 日志

```shell
# 登陆到 master 容器中
docker exec -it ${USER}-kuscia-master bash

# 查看 kuscia 错误日志
cat /home/kuscia/var/logs/kuscia.log | grep -i error

# 查看 k3s 错误日志
cat /home/kuscia/var/logs/k3s.log | grep -i error
```

查看 Alice 节点日志

```shell
# 登陆到 alice 容器中
docker exec -it ${USER}-kuscia-lite-alice bash

# 查看 kuscia 错误日志
cat /home/kuscia/var/logs/kuscia.log | grep -i error
```

查看 Bob 节点日志

```shell
# 登陆到 bob 容器中
docker exec -it ${USER}-kuscia-lite-bob bash

# 查看 kuscia 错误日志
cat /home/kuscia/var/logs/kuscia.log | grep -i error
```

####  点对点组网模式

查看 Alice 节点日志

```shell
# 登陆到 alice 容器中
docker exec -it ${USER}-kuscia-autonomy-alice bash

# 查看 kuscia 错误日志
cat /home/kuscia/var/logs/kuscia.log | grep -i error

# 查看 k3s 错误日志
cat /home/kuscia/var/logs/k3s.log | grep -i error
```

查看 Bob 节点日志

```shell
# 登陆到 bob 容器中
docker exec -it ${USER}-kuscia-autonomy-bob bash

# 查看 kuscia 错误日志
cat /home/kuscia/var/logs/kuscia.log | grep -i error

# 查看 K3s 错误日志
cat /home/kuscia/var/logs/k3s.log | grep -i error
```

{#jon-run-failed}