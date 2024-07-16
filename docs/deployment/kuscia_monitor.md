# Kuscia 监控
Kuscia 暴露了一些指标数据，可作为数据源供外部观测工具采集（如 Prometheus）。目前已通过 [node_exporter](https://github.com/prometheus/node_exporter) 暴露机器指标、通过 [Envoy](https://www.envoyproxy.io/docs/envoy/v1.29.0/configuration/upstream/cluster_manager/cluster_stats#general) 和 [ss](https://man7.org/linux/man-pages/man8/ss.8.html) 暴露网络指标。后续预计集成包括引擎、Kuscia-API、跨机构的指标数据。
## 1 监控能力
| 指标 |来源模块 | 集成 | 介绍 |
| -- | ---------------------- | --------------------- | ------------------------------------------------------------ |
| 机器指标 | node_exporter | 已集成 | Kuscia 所在容器的 CPU/MEM/DISK/LOAD 等核心指标 |
|   网络指标   |    envoy/ss    |    已集成      |   网络收发，QPS等指标    |
|   引擎指标   |    -    |   未集成    |     运行在kuscia上各引擎的指标，如： secretflow/serving/psi/scql/...等 |
|    Kuscia-API指标  |    kuscia-api    |      未集成     |    kuscia-api 错误/QPS等指标         |
|     跨机构指标 |    kuscia    |      未集成     |      在允许的情况下采集其他机构指标       |

## 2 配置监控
### 2.1 Kuscia 暴露的指标采集端口
- 指标端口位于 9091 端口的 /metrics, container_ip 请赋值为机构容器的 IP 地址
```
$ curl $(container_ip):9091/metrics
```

### 2.2 Prometheus/Grafana 监控 Kuscia
可通过配置Prometheus/Grafana 监控 Kuscia。以中心化模式为例，获取机构某一方（如 root-kuscia-lite-alice）的指标数据，假设容器 IP 地址 container_ip = 172.18.0.3，可获取到容器暴露的指标。创建 prometheus.yml [示例文件](https://github.com/secretflow/kuscia/blob/main/scripts/templates/prometheus.yml)，将配置文件中的机构容器的ip地址（172.18.0.3）和端口号填入（端口号默认为9091）。
启动 Promethus、Grafana 后，在 Grafana 创建指标数据源，可导入 [Grafana 模板文件](https://github.com/secretflow/kuscia/blob/main/scripts/templates/grafana-dashboard-machine.json)，注意将数据源{{Kuscia-datasource}}替换为创建数据源 ID（可通过可视化界面查看，也可通过 curl -s http://admin:admin@localhost:3000/api/datasources 查询）。

### 2.4 部署 Kuscia-monitor 快速体验监控
Kuscia-monitor 是 Kuscia 的集群监控工具，中心化模式指标导入到容器 \${USER}-kuscia-monitor-center 下，点对点模式各参与方的指标分别导入到容器 \${USER}-kuscia-monitor-\${DOMAIN_ID}下。
在通过 kuscia/scripts/deploy/start_standalone.sh 部署完毕 kuscia 后，利用 kuscia/scripts/deploy/start_monitor.sh 脚本部署 Kuscia-monitor
在 kuscia 目录下，
```
$ make build-monitor
```
- 中心化模式
```
$ ./start_monitor.sh center
```
- p2p模式
```
$ ./start_monitor.sh p2p
```
浏览器打开 Granafa 的页面 localhost:3000, 账号密码均为 admin（登陆后可修改密码）。进入后，选择 Dashboard 界面的 machine-center 看板进入监控界面。

## 3 Kuscia 监控指标项
Kuscia 暴露的监控指标项
| 模块 |指标 | 类型 | 含义 |
| -- | ---------------------- | --------------------- | ------------------------------------------------------------ |
| CPU | node_cpu_seconds_total | Counter | CPU 总使用时间(可计算cpu使用率)|
| MEM | node_memory_MemTotal_bytes | Gauge | 总内存字节数|
| MEM | node_memory_MemAvailable_bytes | Gauge | 可用内存字节数(可计算内存使用率)  |
| MEM | process_virtual_memory_max_bytes | Gauge | 最大虚拟内存字节数 |
| MEM | process_virtual_memory_bytes| Gauge | 当前虚拟内存字节数 |
| DISK | node_disk_io_now | Counter | 磁盘 io 次数 |
| DISK | node_disk_io_time_seconds_total | Counter | 磁盘 io 时间 |
| DISK | node_disk_read_bytes_total | Counter | 磁盘读取总字节数 |
| DISK | node_disk_read_time_seconds_total | Counter | 磁盘读取总时间 |
| DISK | node_disk_written_bytes_total | Counter | 磁盘写入总字节数 |
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
| NET | ss_rtt | Gauge |tcp连接的流平均往返时延（Round Trip Tie） |
| NET | ss_retrans | Counter | tcp重传次数 |
| NET | ss_retran_rate | Gauge | tcp重传率 （重传次数/总连接） |
| NET | ss_total_connections | Counter | 与各个Domain的 TCP 连接数 |
| ENVOY | envoy_cluster_upstream_rq_total | Counter | 上游（envoy作为服务器端）请求总数 |
| ENVOY | envoy_cluster_upstream_cx_total | Counter | 上游（envoy作为服务器端））连接总数 |
| ENVOY | envoy_cluster_upstream_cx_tx_bytes_total | Counter | 上游（envoy作为服务器端）发送连接字节总数 |
| ENVOY | envoy_cluster_upstream_cx_rx_bytes_total | Counter | 上游（envoy作为服务器端）接收连接字节总数 |
| ENVOY | envoy_cluster_health_check_attempt | Counter | envoy 针对上游服务器集群健康检查次数 |
| ENVOY | envoy_cluster_health_check_failure | Counter | envoy 针对上游服务器集群立即失败的健康检查次数（如 HTTP 50 错误）以及网络故障导致的失败次数 |
| ENVOY | envoy_cluster_upstream_cx_connect_fail | Counter | 上游（envoy作为服务器端）总连接失败次数 |
| ENVOY | envoy_cluster_upstream_cx_connect_timeout | Counter | 上游（envoy作为服务器端）总连接超时次数 |
| ENVOY | envoy_cluster_upstream_rq_timeout | Counter | 上游（envoy作为服务器端）等待响应超时的总请求次数 |