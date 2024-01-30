## 在机构的某一方（如 root-kuscia-lite-alice）获取指标数据
假设容器 IP 地址 container_ip = 172.18.0.3
```
$ curl $(container_ip):9091/metrics
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


|编号| 指标                                        | 类型 | 含义                                                         |
|----------------------|---------------------- | --------------------- | ------------------------------------------------------------ |
| 1 |   upstream_rq_total             |    Counter         | 上游请求总数            |
| 2 |    upstream_cx_total       |         Counter          | 上游连接总数                                               |
| 3 |    upstream_cx_tx_bytes_total    |      Counter        | 上游发送连接字节总数        |
| 4 |    upstream_cx_rx_bytes_total     |      Counter       | 上游接收连接字节总数     |
| 5 |    avg_rtt                        |      Gauge           | 平均往返时延（Round Trip Time）                                |
| 6 |    max_rtt                        |      Gauge           | 最大往返时延（Round Trip Time）                                |
| 7 |    retrains       |      Counter             | 重传次数                                                   |
| 8 |    retrain_rate       |       Gauge             | 重传率 （重传次数/总连接数）                                                  |
| 9 |    health_check.attempt     |        Counter           | 健康检查次数     |
| 10 |    health_check.failure      |         Counter       | 立即失败的健康检查次数（例如 HTTP 503 错误）以及网络故障导致的失败次数 |
| 11 |    upstream_cx_connect_fail     |      Counter        | 总连接失败次数     |
| 12  |     upstream_cx_connect_timeout      |    Counter      | 总连接超时次数     |
| 13 |    upstream_rq_timeout        |     Counter           | 等待响应超时的总请求次数             |                             |
