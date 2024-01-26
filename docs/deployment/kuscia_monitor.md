# Kuscia 监控
## Kuscia-monitor
Kuscia-monitor 是 Kuscia 的集群监控工具，可供快速体验 kuscia 暴露出的指标数据。Kuscia-monitor 收集 Prometheus 的 node_exporter 暴露的机器指标和envoy、ss的部分指标，利用 Prometheus 存储指标数据，利用 Grafana 完成可视化。主要分为两种模式：
- 中心化模式
    - node_exporter 部署在所有 master、lite 结点
    - Kuscia-monitor 暴露所有结点的 node_exporter 指标，仅暴露 lite 结点间的 envoy、ss 指标
    - 收集到的指标统一导入到容器 ${USER}-kuscia-monitor-center 下
- 点对点模式
    - node_exporter 部署在所有 autonomy 结点
    - 暴露所有结点间的指标
    - 各参与方仅可查看本方集群的指标
    - 各参与方的指标分别导入到容器 \${USER}-kuscia-monitor-\${DOMAIN_ID}下, 如 root-kuscia-monitor-alice。

### 编译 Kuscia-monitor 镜像
在 kuscia 目录下，
```
$ make build-monitor
```
### 部署 Kuscia-monitor
在通过 kuscia/scripts/deploy/start_standalone.sh 部署完毕 kuscia 后，利用 kuscia/scripts/deploy/start_monitor.sh 脚本部署 Kuscia-monitor
- 中心化模式
```
$ ./start_monitor center
```
1.2 p2p模式
```
$ ./start_monitor p2p
```
2. 浏览器打开 Granafa 的页面 localhost:3000, 账号密码均为 admin（登陆后可修改密码）。进入后，选择 Dashboard 界面的 machine-center 看板进入监控界面。

## 独立部署 Prometheus/Grafana
导入的 Grafana 模板可参考 scripts/templates/grafana-dashboard-machine.json

以 center 模式为例，获取机构某一方（如 root-kuscia-lite-alice）的指标数据，假设容器 IP 地址 container_ip = 172.18.0.3，可获取到容器暴露的指标，envoy 和 ss 的指标位于 9091 端口。
```
$ curl $(container_ip):9091/metrics
```
node_exporter 的指标位于 9100 端口
```
$ curl $(container_ip):9100/metrics
```
2. 加入prometheus获取指标数据
创建 prometheus.yml，放在 /home/${USER}/prometheus 路径下：
```
sudo docker run -d --name prometheus -p 9090:9090 -v /home/$USER/prometheus/prometheus.yml:/etc/prometheus/prometheus.yml prom/prometheus:latest --config.file=/etc/prometheus/prometheus.yml
```
prometheus.yml 的配置内容示例，将配置文件中的机构容器的ip地址（172.18.0.3）和端口号填入（端口号默认为9091）
```
global:

  scrape_interval:     5s # 默认 5 秒采集一次指标
  external_labels:
    monitor: 'Kuscia-monitor'
# Prometheus指标的配置
scrape_configs:
  - job_name: 'prometheus'
    # 覆盖全局默认采集周期
    scrape_interval: 5s
    static_configs:
      - targets: ['localhost:9090'] # 配置 Prometheus 采集指标暴露的地址
  - job_name: 'alice-network'
    scrape_interval: 5s
    static_configs:
      - targets: ['172.18.0.3:9091'] # 配置机构 IP 地址采集网络指标
    metrics_path: /metrics
    scheme: http
  - job_name: 'alice-node'
    scrape_interval: 5s
    static_configs:
      - targets: ['172.18.0.3:9100'] # 配置机构 IP 地址采集机器指标
    metrics_path: /metrics
    scheme: http
```
将prometheus加入容器网络：
```
sudo docker ps
```
找到 Prometheus 容器的ID $(prom_id)
```
sudo docker network ls
```
找到 Kuscia 容器网络的id $(net_id)
```
NETWORK ID     NAME              DRIVER    SCOPE

$(net_id)   kuscia-exchange   bridge    local

```

将加入 Prometheus 的容器加入Kuscia的容器网络

```

sudo docker network connect $(net_id) $(prom_id)
```

在浏览器中打开页面 prometheus localhost:9090


## 配置grafana可视化


加入grafana的容器

```
sudo docker run -itd --name=grafana --restart=always -p 3000:3000 grafana/grafana
```

查询Grafana的容器id $(graf_id)，将grafana的容器加入Kuscia的容器网络

```

sudo docker network connect $(net_id) $(graf_id)
```

浏览器打开Granafa的页面 localhost:3000, 账号密码均为 admin（登陆后可修改密码）。进入后，添加数据源（Home页面的 Add your first data source），选择 Prometheus的数据源，设置 Connection 中的地址为 Prometheus 容器的IP地址（如：172.18.0.6，可通过 docker inspect kuscia-exchange）和端口号（默认8080）（可通过 sudo docker network inspect $(net_id)查看），添加后可配置指标数据。

## 所支持的指标
| 模块 |指标 | 类型 | 含义 |
| -- | ---------------------- | --------------------- | ------------------------------------------------------------ |
| CPU | node_cpu_seconds_total | Counter | CPU 总使用时间(可计算cpu使用率)|
| CPU | node_cpu_frequency_max_hertz | Gauge | 可统计CPU核数(无直接核数指标，可用lspu替代) |
| MEM | node_memory_MemTotal_bytes | Gauge | 总内存字节数|
| MEM | node_memory_MemAvailable_bytes | Gauge | 可用内存字节数(可计算内存使用率)  |
| MEM | process_virtual_memory_max_bytes | Gauge | 最大虚拟内存字节数 |
| MEM | process_virtual_memory_bytes| Gauge | 当前虚拟内存字节数 |
| DISK | node_disk_io_now | Counter | 磁盘 io 次数 |
| DISK | node_disk_io_time | Counter | 磁盘 io 时间 |
| DISK | node_disk_read_bytes_total | Counter | 磁盘读取总字节数 |
| DISK | node_disk_read_time_seconds_total | Counter | 磁盘读取总时间 |
| DISK | node_disk_write_bytes_total | Counter | 磁盘写入总字节数 |
| DISK | node_disk_write_time_seconds_total | Counter | 磁盘写入总时间 |
| DISK | node_filesystem_avail_bytes | Gauge | 可用磁盘字节数 |
| DISK | node_filesystem_size_bytes | Gauge | 总磁盘字节数 |
| DISK | node_filesystem_files | Gauge | 系统总文件数（iNode） |
| DISK | node_filesystem_files_free | Gauge | 系统空闲文件数（iNode） |
| DISK |process_max_fds | Gauge | 最大开启文件描述符 |
| DISK |process_open_fds | Gauge | 开启文件描述符  |
| LOAD | node_load1/5/15 | Gauge | 结点1/5/15分钟内平均load |
| NET | node_network_receive_bytes_total | Counter | 网卡设备接收的总字节数|
| NET | node_network_receive_packets_total | Counter | 网卡设备接收的总包数 |
| NET | node_network_transmit_bytes_total | Counter | 网卡设备发送的总字节数|
| NET | node_network_transmit_packets_total | Counter | 网卡设备发送的总包数 |
| NET | node_netstat_Tcp_CurrEstab | Gauge | 当前 TCP 建立的总连接数 |
| NET | node_sockstat_TCP_tw | Gauge | 当前 TCP 处在 time_wait 的总连接数 |
| NET | node_netstat_Tcp_ActiveOpens | Gauge | 当前 TCP 处在active_open 的总连接数 |
| NET | node_netstat_Tcp_PassiveOpens | Gauge | 当前 TCP 处在passive_open 的总连接数 |
| NET | node_sockstat_TCP_alloc| Gauge | 当前 TCP 处在 allocate 状态的总连接数 |
| NET | node_sockstat_TCP_inuse| Gauge | 当前 TCP 处在inuse 状态的总连接数 |
| NET | avg_rtt | Gauge |tcp连接的流平均往返时延（Round Trip Tie） |
| NET | max_rtt | Gauge |tcp连接的流最大往返时延（Round Trip Tie） |
| NET | retrains | Counter | tcp重传次数 |
| NET | retrain_rate | Gauge | tcp重传率 （重传次数/总连接） |
| ENVOY | upstream_rq_total | Counter | 上游（envoy作为服务器端）请求总数 |
| ENVOY | upstream_cx_total | Counter | 上游（envoy作为服务器端））连接总数 |
| ENVOY | upstream_cx_tx_bytes_total | Counter | 上游（envoy作为服务器端）发送连接字节总数 |
| ENVOY | upstream_cx_rx_bytes_total | Counter | 上游（envoy作为服务器端）接收连接字节总数 |
| ENVOY | health_check.attempt | Counter | envoy 针对上游服务器集群健康检查次数 |
| ENVOY | health_check.failure | Counter | envoy 针对上游服务器集群立即失败的健康检查次数（如 HTTP 50 错误）以及网络故障导致的失败次数 |
| ENVOY | upstream_cx_connect_fail | Counter | 上游（envoy作为服务器端）总连接失败次数 |
| ENVOY | upstream_cx_connect_timeout | Counter | 上游（envoy作为服务器端）总连接超时次数 |
| ENVOY | upstream_rq_timeout | Counter | 上游（envoy作为服务器端）等待响应超时的总请求次数 | |