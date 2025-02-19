# Docker 集群部署-组网模式

## 前言

本教程帮助您使用 Docker 组网模式来完成部署 Kuscia 集群。

<span style="color: red;">注：</span>目前只支持 Kuscia 以 `runp` 模式以此方式组网。

## 前置准备

在部署 Kuscia 之前，请确保环境准备齐全，包括所有必要的软件、资源、操作系统版本和网络环境等满足要求，以确保部署过程顺畅进行，详情参考[部署要求](../deploy_check.md)。

## 结构图示

> work 127.0.0.1 &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;(示例 IP 以实际为准)  
> manager 127.0.0.2 &nbsp;&nbsp;(示例 IP 以实际为准)

![image.png](../../imgs/deploy_clustered_structure.png)  
注：实际生产环境中 Alice 应该对外暴露一个统一的 LB 地址，由 LB 将请求代理至 Alice1 或 Alice2 节点实例。

## 部署流程

### 完成 Docker Swarm 组网

> Docker Swarm 是 Docker 官方提供的容器编排工具，用于管理和编排多个 Docker 容器，构建和管理容器集群。

**相关描述**

[Docs 阿里云](https://developer.aliyun.com/article/1532136?spm=5176.26934562.main.1.73d24271TEV7gG)

[Docs Docker](https://docs.docker.com/reference/cli/docker/swarm/)

#### 初始化 swarm

```shell
# 选择其中一台主机初始化为 Docker manager 节点，IP 为当前主机宿主机端口，Docker 集群管理监听使用 IP，具体以 Docker 官方描述为准
# 管理端口使用默认端口（2377），可根据 Docker 官网文档进行设置 (format: <ip|interface>[:port])
docker swarm init --advertise-addr 127.0.0.2
```

执行完上述命令可得到以下描述信息，以及 Token，需要记录该 Token 字符串，在 worker 节点宿主机执行可加入该 docker swarm
集群。  
Token 遗忘丢失也可以通过`docker swarm join-token manager`命令进行查询

```shell
[root@node-01 ~]# docker swarm init --advertise-addr 127.0.0.2
Swarm initialized: current node (52l3w8qo6drdmvjl6t1z8bf1g) is now a manager.

To add a worker to this swarm, run the following command:

    docker swarm join --token SWMTKN-1-37xpluc9bo2sw3ez8yslcgooo8dq082pd5ao0zmtbmuqjcip51-cki6vjrdm931lnvkc5edj075s 127.0.0.2:2377

To add a manager to this swarm, run 'docker swarm join-token manager' and follow the instructions.
```

#### 初始化 Docker Network Create

[[Docs Docker] Docker Network Create](https://docs.docker.com/reference/cli/docker/network/create/)  
在 docker manager 节点主机上执行：

```shell
# -d, --driver: Driver to manage the Network, default is bridge. Need to use overlay here
# --attachable: Enable manual container attachment
docker network create -d overlay --subnet 16.0.0.0/8 --attachable kuscia-exchange-cluster
```

`--subnet`按需（最多多少个容器使用该网段）进行设置，可不设置。
`kuscia-exchange-cluster`network 名字，需要在脚本部署时使用，**必须是这个名字**。

#### Worker 节点加入 Swarm

在 worker 节点的宿主机执行 docker swarm init 得到的 join 命令

```shell
docker swarm join --token SWMTKN-1-37xpluc9bo2sw3ez8yslcgooo8dq082pd5ao0zmtbmuqjcip51-cki6vjrdm931lnvkc5edj075s 127.0.0.2:2377
```

执行完 join 命令可得到以下执行结果。

```shell
[root@node-02 ~]# docker swarm join --token SWMTKN-1-37xpluc9bo2sw3ez8yslcgooo8dq082pd5ao0zmtbmuqjcip51-cki6vjrdm931lnvkc5edj075s 127.0.0.2:2377
This node joined a swarm as a worker.
```

也可以在 manager 节点执行 `docker node ls` 查询已加入集群的节点状态，并且通过 `STATUS` 字段来判断节点连接是否正常。

```shell
[root@node-01 ~]# docker node ls
ID                            HOSTNAME        STATUS    AVAILABILITY   MANAGER STATUS   ENGINE VERSION
52l3w8qo6drdmvjl6t1z8bf1g *   node-01         Ready     Active         Leader           20.10.24
2af71qccfr8p4po7zqhimkrjr     node-02         Ready     Active                          20.10.24
```

注：在实际生产中，设置多个管理节点（manager nodes）对于高可用性和容错是非常关键的。详情请参考：

- [Promote or demote a node](https://docs.docker.com/engine/swarm/manage-nodes/#promote-or-demote-a-node)
- [Administer and maintain a swarm of Docker Engines](https://docs.docker.com/engine/swarm/admin_guide/ )

### Kuscia 部署实例

> 这里使用 kuscia.sh 实现 P2P 模式中`alice`节点的双机双副本部署。
> 部署参考：[多机部署点对点集群](./deploy_p2p_cn.md)

```shell
# 指定 Kuscia 使用的镜像版本，这里使用 0.14.0b0 版本
export KUSCIA_IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia:0.14.0b0
```

```shell
docker pull $KUSCIA_IMAGE && docker run --rm $KUSCIA_IMAGE cat /home/kuscia/scripts/deploy/kuscia.sh > kuscia.sh && chmod u+x kuscia.sh
```

#### 修改配置文件创建使用的 DB

验证使用的示例，仅供参考：

```shell
# 拉起 MySQL 服务，并且不挂载磁盘，仅用于测试示例
# 拉起一个 MySQL 8.0 的容器，使用 password 作为密码，创建 database kine
docker run -d --name alice-mysql -p 3306:3306 -e MYSQL_ROOT_PASSWORD=password -e MYSQL_DATABASE=kine mysql:8
```

#### 初始化 Kuscia 配置文件

Kuscia init 参数请参考[配置文件](../kuscia_config_cn.md#快速生成配置文件)，命令示例如下：

```shell
docker run -it --rm ${KUSCIA_IMAGE} kuscia init --mode autonomy --domain "alice" --runtime "runp" --datastore-endpoint "mysql://root:password@tcp(xx.xx.xx.xx:3306)/kine" > autonomy_alice.yaml
```

#### 部署 Kuscia

部署端口请参考[这里](../kuscia_ports_cn.md)，命令示例如下：

```shell
./kuscia.sh start -c ./autonomy_alice.yaml -p 20000 -q 20001 -k 20010 -g 20011 -a none -m 8G --cluster
```

如果部署时报错 `kuscia-exchange-cluster 已经存在且不是预期的类型` 等类似错误，需要将 network 手动删除后重新部署：

```bash
# 删除 network
docker network rm kuscia-exchange-cluster
```

后续部署其它实例需要将 autonomy_alice.yaml 拷贝过去而不是重新生成<br>
后续步骤可参考官网进行配置：[多机部署点对点集群](./deploy_p2p_cn.md)

按照顺序完成：配置证书 > 配置路由授权 > 拉起示例任务<br>
注：使用 LB 代理时，路由授权地址使用代理服务地址建立。

### LB 示例（Nginx）

> 以 Nginx 为例  
> 拉起 Nginx 服务通过 8080 端口代理多副本中 alice 的宿主机地址与端口。

```shell
# 拉取 nginx latest 镜像
docker pull nginx:latest
```

#### 修改配置文件

1. 从 NGINX 镜像中拷贝配置文件至宿主机当前命令目录

   ```shell
   docker run --rm nginx:latest cat /etc/nginx/nginx.conf > ./nginx.conf
   ```

2. 修改配置文件

   参考官网中 Nginx 配置示例修改配置文件中 http 代理块，如果 Kuscia 需要使用 https 访问，在修改的配置中 `server` 块中使用
   https，并注释原有 http 和打开 https 注释部分

## nginx 代理参数配置示例

- nginx 代理参数配置示例,详情请参考[这里](../networkrequirements.md#nginx)。

### 启动并挂载配置

使用 Docker 拉起 Nginx 服务，并把修改的配置文件挂载至容器内

```shell
docker run -it -d --name lb-nginx -p 8080:80 -v /path/to/nginx.conf:/etc/nginx/nginx.conf nginx
```

#### 验证代理服务

多次请求代理服务，返回的 `kuscia-error-message` 信息是随机的。

```shell
curl -kv http://127.0.0.1:8080
```

#### 完整配置文件参考

```nginx
user  nginx;
worker_processes  auto;

error_log  /var/log/nginx/error.log notice;
pid        /var/run/nginx.pid;


events {
    worker_connections  1024;
}

http {
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_set_header Host $http_host;
    proxy_pass_request_headers on;

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
    #   If kuscia is deployed to multiple machines, use the ip of each kuscia here
        server 127.0.0.2:11080 weight=1 max_fails=5 fail_timeout=60s;
        server 127.0.0.1:11080 weight=1 max_fails=5 fail_timeout=60s;
    #   Nginx_upstream_check_module can support upstream health check with Nginx
    #   Please refer to the document: https://github.com/yaoweibin/nginx_upstream_check_module/tree/master/doc
    #   check interval=3000 rise=2 fall=5 timeout=1000 type=http;

        keepalive 32;
        keepalive_timeout 600s;
        keepalive_requests 1000;
    }

    server {
        location / {
    #   Change the content of the comment based on the http/https mode that the proxy service needs to access
    #        proxy_read_timeout 10m;
    #        proxy_pass http://backend;
    #       Connect to kuscia with https
            proxy_pass https://backend;
            proxy_ssl_verify off;
            proxy_set_header Host $host;
        }
    }

    # This corresponds to case 3 above, kuscia needs to configure a proxy when accessing the internet
    # The port must be different with the reverse proxy port
    # server {
    #    resolver $dns_host_ip;
    #    location / {
    #    proxy_pass ${The address provided by the other organization};
    #    }
    # }
}
```
