# 部署失败日志排查

## 排查步骤

### 检查机器配置

若机器不满足[官网推荐配置](https://www.secretflow.org.cn/docs/kuscia/latest/zh-Hans/getting_started/quickstart_cn#id2)，可能会造成部分服务无法正常工作，从而导致部署失败。

### 查看服务日志

#### 中心化组网模式

查看 Master 日志

```shell
# Log in to the master container
docker exec -it ${USER}-kuscia-master bash

# View kuscia error logs
cat /home/kuscia/var/logs/kuscia.log | grep -i error

# View k3s error logs
cat /home/kuscia/var/logs/k3s.log | grep -i error
```

查看 Alice 节点日志

```shell
# Log in to the alice container
docker exec -it ${USER}-kuscia-lite-alice bash

# View kuscia error logs
cat /home/kuscia/var/logs/kuscia.log | grep -i error
```

查看 Bob 节点日志

```shell
# Log in to the bob container
docker exec -it ${USER}-kuscia-lite-bob bash

# View kuscia error logs
cat /home/kuscia/var/logs/kuscia.log | grep -i error
```

#### 点对点组网模式

查看 Alice 节点日志

```shell
# Log in to the alice container
docker exec -it ${USER}-kuscia-autonomy-alice bash

# View kuscia error logs
cat /home/kuscia/var/logs/kuscia.log | grep -i error

# View k3s error logs
cat /home/kuscia/var/logs/k3s.log | grep -i error
```

查看 Bob 节点日志

```shell
# Log in to the bob container
docker exec -it ${USER}-kuscia-autonomy-bob bash

# View kuscia error logs
cat /home/kuscia/var/logs/kuscia.log | grep -i error

# View k3s error logs
cat /home/kuscia/var/logs/k3s.log | grep -i error
```

{#jon-run-failed}
