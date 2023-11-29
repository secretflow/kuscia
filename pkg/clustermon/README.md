## 在机构的某一方（如 root-kuscia-lite-alice）运行集群测量
```
$ go run monitor.go
```
2. 加入prometheus获取指标数据
prometheus.yml 放在 /home/$USER/prometheus 路径下：
```
sudo docker run -d --name prometheus -p 9090:9090 -v /home/$USER/prometheus/prometheus.yml:/etc/prometheus/prometheus.yml prom/prometheus:latest --config.file=/etc/prometheus/prometheus.yml

```
prometheus.yml 的配置内容示例，将配置文件中的机构容器的ip地址（172.18.0.3）和端口号填入（端口号默认为9091）
```
global:

  scrape_interval:     5s # 默认 5 秒采集一次指标

  external_labels:

    monitor: 'codelab-monitor'



# Prometheus指标的配置

scrape_configs:

  - job_name: 'prometheus'



    # 覆盖全局默认采集周期

    scrape_interval: 5s

    static_configs:

      - targets: ['localhost:9090'] # 配置 Prometheus 采集指标暴露的地址

  

  - job_name: 'alice'

    scrape_interval: 5s

    static_configs:

      - targets: ['172.18.0.3:9091'] # 配置机构 IP 地址采集指标暴露的地址

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

浏览器打开Granafa的页面 localhost:3000, 账号密码均为admin（登陆后可修改密码）。进入后，添加数据源（Home页面的 Add your first data source），选择Prometheus的数据源，设置Connection中的地址为Prometheus容器的IP地址（如：172.18.0.6）和端口号（默认8080）（可通过 sudo docker network inspect $(net_id)查看），添加后可配置指标数据。


| 指标                                        | 类型 | 含义                                                         |
| ---------------------- | --------------------- | ------------------------------------------------------------ |
| **net_metrics**                             |                                                              |
| rto                       |      Gauge           |   ( 流平均/最大/最小)重新传输超时时间（Retransmission Timeout）                 |
| rtt                        |      Gauge           | （流平均/最大/最小）往返时延（Round Trip Time）                                |
| retrain_rate       |       Gauge             | （流平均/最大/最小）重传率                                                   |
| retrains       |      Counter             | （流平均/最大/最小）重传次数                                                   |
| bytes_sent         |         Counter       | （流平均/总/最大/最小）发送字节数                                                 |
| bytes_received   |         Counter                 | （流平均/总/最大/最小）接收总字节数                                                 |
| **clu_metrics**                             |                                                              |
| assignment_stale     |       Counter              | 接收的分配变陈旧之前，新分配到达的次数             |
| assignment_timeout_received       |    Counter      | 收到带有终端租约信息的总分配                                              |
| bind_errors                  |      Counter        | 绑定套接字到配置的源地址时发生的总错误的次数                                       |
| circuit_breakers.default.cx_open   |     Gauge    | 连接断路器是否达到并超出并发限制（0），或已达到容量不再接受（1）  |
| circuit_breakers.default.cx_pool_open  |  Gauge   | 连接池断路器是否达到并超出并发限制（0），或已达到容量不再接受（1）   |
| circuit_breakers.default.rq_open   |  Gauge       | 请求断路器是否达到并超出并发限制（0），或已达到容量不再接受（1）  |
| circuit_breakers.default.rq_pending_open  | Gauge | 待处理请求断路器是否达到并超出并发限制（0），或已达到容量不再接受（1）   |
| circuit_breakers.default.rq_retry_open   | Gauge  | 重试断路器是否达到并超出并发限制（0），或已达到容量不再接受（1） |
| circuit_breakers.high.cx_open     |   Gauge       | 连接断路器是否达到并超出并发限制（0），或已达到容量不再接受（1） |
| circuit_breakers.high.cx_pool_open   |   Gauge    | 连接池断路器是否达到并超出并发限制（0），或已达到容量不再接受（1）      |
| circuit_breakers.high.rq_open    |    Gauge       | 请求断路器是否达到并超出并发限制（0），或已达到容量不再接受（1）     |
| circuit_breakers.high.rq_pending_open  |  Gauge   | 待处理请求断路器是否达到并超出并发限制（0），或已达到容量不再接受（1）      |
| circuit_breakers.high.rq_retry_open  |   Gauge    | 重试断路器是否达到并超出并发限制（0），或已达到容量不再接受（1）  |
| health_check.attempt     |        Counter           | 健康检查次数     |
| health_check.degraded    |        Counter           | 健康检查：降级次数                                       |
| health_check.failure      |         Counter       | 立即失败的健康检查次数（例如 HTTP 503 错误）以及网络故障导致的失败次数 |
| health_check.healthy     |     Gauge              | 健康成员的数量     |
| health_check.network_failure    |    Counter         |由于网络错误导致的健康检查失败次数       |
| health_check.passive_failure      |    Counter       | 由于被动事件（例如 x-envoy-immediate-health-check-fail）导致的健康检查失败次数    |
| health_check.success      |      Counter            | 成功的健康检查次数                |
| health_check.verify_cluster    |    Counter         | 尝试进行群集名称验证的健康检查次数       |
| http1.dropped_headers_with_underscores   |  Counter  | 头部名称包含下划线的已丢弃头部的总次数  |
| http1.metadata_not_supported_error   |    Counter    | 在 HTTP/1 编码期间丢弃的元数据总次数      |
| http1.requests_rejected_with_underscores_in_headers | Counter | 由于头部名称包含下划线而被拒绝的请求总次数。可以通过设置 headers_with_underscores_action 配置来配置此操作           |
| http1.response_flood    |        Counter           | 由于响应泛滥而关闭的连接总次数。                  |
| lb_healthy_panic        |         Counter           | 在紧急模式下使用负载均衡器的总请求数    |
| lb_local_cluster_not_ok         |     Counter       | 本地主机集未设置或处于紧急模式    |
| lb_recalculate_zone_structures     |    Counter      | 重新生成区域感知路由结构的次数，以便快速选择上游区域  |
| lb_subsets_active        |     Gauge              | 当前可用子集的数量        |
| lb_subsets_created      |     Counter               |创建的子集数量
| lb_subsets_fallback        |      Counter           | 回退策略被触发的次数     |
| lb_subsets_fallback_panic     |   Counter           | 子集恐慌模式触发的次数          |
| lb_subsets_removed       |       Counter            | 由于没有主机而移除的子集数量           |
| lb_subsets_selected       |      Counter            | 为负载平衡选择任何子集的次数             |
| lb_zone_cluster_too_small      |     Counter        | 由于上游集群大小较小，未进行区域感知路由   |
| lb_zone_no_capacity_left       |     Counter        | 由于舍入误差而结束，随机选择区域  |
| lb_zone_number_differs     |        Counter         | 由于功能标志被禁用以及本地和上游集群中区域数量不同，未进行区域感知路由   |
| lb_zone_routing_all_directly    |    Counter        | 将所有请求直接发送到同一区域   |
| lb_zone_routing_cross_zone    |     Counter        | 区域感知路由模式，但必须跨区域发送请求     |
| lb_zone_routing_sampled    |     Counter           | 将部分请求发送到同一区域    |
| max_host_weight        |       Counter             | 群集中任何主机的最大权重                                               |
| membership_change     |       Counter              | 总集群成员变更次数     |
| membership_degraded    |      Gauge              | 当前集群的降级成员总数         |
| membership_excluded     |        Gauge           | 当前集群的排除成员总数         |
| membership_healthy     |    Gauge                | 当前集群的健康成员总数（包括健康检查和异常检测）     |
| membership_total        |       Gauge            | 当前集群的成员总数      |
| original_dst_host_invalid     |    Counter         | 传递给原始目标负载均衡器的无效主机总数          |
| retry_or_shadow_abandoned    |      Counter        |  由于缓冲区限制而取消的影子或重试缓冲的总次数      |
| update_attempt       |     Counter                 | 通过服务发现尝试的总群集成员更新                            |
| update_empty        |       Counter                | 结束时具有空群集加载分配并继续使用先前配置的总群集成员更新     |
| update_failure        |        Counter             | 通过服务发现失败的总群集成员更新   |
| update_no_rebuild     |        Counter             | 没有导致任何群集负载平衡结构重建的成功总群集成员更新        |
| update_success       |       Counter               | 通过服务发现成功的总群集成员更新      |
| upstream_cx_active       |       Gauge           | 活动连接总数                             |
| upstream_cx_close_notify      |    Counter         | 通过 HTTP/1.1 连接关闭标头或 HTTP/2 或 HTTP/3 GOAWAY 关闭的连接总数 |
| upstream_cx_connect_attempts_exceeded  |  Counter  | 连续连接失败次数超过配置的连接尝试总数  |
| upstream_cx_connect_fail     |      Counter        | 总连接失败次数     |
| upstream_cx_connect_timeout      |    Counter      | 总连接超时次数     |
| upstream_cx_connect_with_0_rtt    |   Counter      | 能够发送 0-RTT 请求（早期数据）的总连接数          |
| upstream_cx_destroy       |          Counter       | 上游总销毁连接数                                               |
| upstream_cx_destroy_local    |    Counter          | 上游本地销毁的连接总数                                           |
| upstream_cx_destroy_local_with_active_rq   | Counter | 带有 1 个或更多活动请求的销毁连接总数         |
| upstream_cx_destroy_remote        |     Counter    | 远程销毁的连接总数              |
| upstream_cx_destroy_remote_with_active_rq  | Counter | 带有 1 个或更多活动请求的本地销毁连接总数   |
| upstream_cx_destroy_with_active_rq  |    Counter   | 带有 1 个或更多活动请求的销毁连接总数                             |
| upstream_cx_http1_total     |        Counter       | 上游连接 HTTP/1 总数                                      |
| upstream_cx_http2_total      |       Counter       | 上游连接 HTTP/2 总数                                      |
| upstream_cx_http3_total      |      Counter        | 上游连接 HTTP/3 总数                                      |
| upstream_cx_idle_timeout   |      Counter          | 上游连接空闲超时                                           |
| upstream_cx_max_duration_reached    |   Counter     | 因达到最大持续时间而关闭的连接总数         |
| upstream_cx_max_requests    |       Counter        | 因最大请求数而关闭的连接总数          |
| upstream_cx_none_healthy     |      Counter         | 由于没有健康主机而未建立连接的总次数       |
| upstream_cx_overflow          |         Counter     | 群集的连接断路器溢出总次数        |
| upstream_cx_pool_overflow      |     Counter        | 群集的连接池断路器溢出总次数             |
| upstream_cx_protocol_error     |      Counter       | 连接协议错误总数      |
| upstream_cx_rx_bytes_buffered    |    Gauge       | 当前缓冲的接收连接字节总数         |
| upstream_cx_rx_bytes_total     |      Counter       | 总发送连接字节总数     |
| upstream_cx_total       |         Counter          | 上游连接总数                                               |
| upstream_cx_tx_bytes_buffered   |     Gauge       | 当前缓冲的发送连接字节总数              |
| upstream_cx_tx_bytes_total    |      Counter        | 总发送连接字节总数        |
| upstream_flow_control_backed_up_total  |   Counter  | 上游连接阻塞并暂停了从下游的读取的总次数          |
| upstream_flow_control_drained_total   |  Counter    | 上游连接排空并恢复了从下游的读取的总次数          |
| upstream_flow_control_paused_reading_total   | Counter | 流量控制暂停从上游读取的总次数             |
| upstream_flow_control_resumed_reading_total  |  Counter | 流量控制恢复从上游读取的总次数         |
| upstream_http3_broken         |   Counter           | 上游 HTTP/3 连接失败                                       |
| upstream_internal_redirect_failed_total   |  Counter | 失败的内部重定向导致重定向被传递给下游的总次数         |
| upstream_internal_redirect_succeeded_total  |  Counter | 内部重定向导致第二个上游请求的总次数       |
| upstream_rq_active          |       Gauge        | 活动请求总数        |
| upstream_rq_cancelled       |     Counter          | 在获取连接池连接之前取消的请求数总数           |
| upstream_rq_completed       |    Counter           | 总的上游请求完成次数    |
| upstream_rq_maintenance_mode      |   Counter       | 由于维护模式而导致立即返回 503 的请求数总数        |
| upstream_rq_max_duration_reached     |  Counter     | 由于达到最大持续时间而关闭的请求总数             |
| upstream_rq_pending_active      |     Gauge      | 等待连接池连接的活动请求总数       |
| upstream_rq_pending_failure_eject     |  Counter    | 由于连接池连接失败或远程连接终止而失败的请求数总数          |
| upstream_rq_pending_overflow      |    Counter      | 溢出连接池或请求（主要用于 HTTP/2 及更高版本）断路器并失败的请求数总数   |
| upstream_rq_pending_total       |   Counter        | 等待连接池连接的请求数总数      |
| upstream_rq_per_try_timeout      |    Counter      | 达到每次尝试超时的总请求数（当未启用request hedging时）             |
| upstream_rq_retry            |    Counter          | 总的请求重试次数            |
| upstream_rq_retry_backoff_exponential  | Counter    | 使用指数退避策略的重试总次数         |
| upstream_rq_retry_backoff_ratelimited   |  Counter  | 使用速率限制退避策略的重试总次数               |
| upstream_rq_retry_limit_exceeded    |   Counter    | 由于超出配置的最大重试次数而未重试的总请求次数           |
| upstream_rq_retry_overflow      |     Counter      | 由于断路或超出重试预算而未重试的总请求次数       |
| upstream_rq_retry_success      |    Counter        | 请求重试成功的总次数        |
| upstream_rq_rx_reset        |     Counter          | 被remote结点reset的连接的个数    |
| upstream_rq_timeout        |     Counter           | 等待响应超时的总请求次数             |
| upstream_rq_total             |    Counter         | 上游总请求数                                         |
| upstream_rq_tx_reset        |    Counter           | 被local结点reset的连接的个数                  |
| version                   |         Gauge       |上次成功API获取的内容的哈希值                                               |