# Kuscia 诊断工具

## kuscia diagnose network

### 功能

检测双方节点的网络是否符合通信条件，以及网络当前的一些问题根因。

检测涵盖项：

- 带宽
- 传输延迟
- 网关最大请求包体大小配置
- 网关缓冲配置
- 网关超时配置

### 使用场景

- 可用于部署完成后，在实际执行算法作业前的前置网络环境检查；

- 可用于执行算法作业失败时，首先对双方网络环境进行诊断，定位（如果有）或排除网络环境的因素。

### 前置条件

用户已经在双方节点均完成 Kuscia 的部署，包括启动 kuscia、创建 Domain、双方互换证书、双方配置授权。

### 使用示例

假设双方节点为 alice 和 bob，需要检测到 bob 的网络通信，可以在 alice 的计算节点容器内执行：

~~~
kuscia diagnose network alice bob
~~~

即会开始执行网络状态诊断流程，正常执行的结果如下：

~~~
Diagnose Config:
--Command: network
--Source: alice
--Destination: bob
--CRD:
--ReportFile:
--TestSpeed: true, Threshold: 10
--TestRTT: true, Threshold: 50
--TestProxyTimeout: false, Threshold: 600
--TestProxyBuffer: true
--TestRequestBodySize: true, Threshold: 1
--BidrectionMode: true
diagnose <alice-bob> network statitsics
diagnose crd config
Run CONNECTION task
Run BANDWIDTH task, threshold: 10Mbits/sec
Run RTT task, threshold: 50ms
Run REQUEST_BODY_SIZE task, threshold: 1MB
Run PROXY_BUFFER task
REPORT:
CRD CONFIG CHECK:
+-----------+------+--------+-------------+
|   NAME    | TYPE | RESULT | INFORMATION |
+-----------+------+--------+-------------+
| alice-bob | cdr  | [PASS] |             |
| bob-alice | cdr  | [PASS] |             |
+-----------+------+--------+-------------+

NETWORK STATSTICS(alice-bob):
+-------------------+---------------------+-------------+--------+-------------+
|       NAME        |   DETECTED VALUE    |  THRESHOLD  | RESULT | INFORMATION |
+-------------------+---------------------+-------------+--------+-------------+
| CONNECTION        | N/A                 |             | [PASS] |             |
| BANDWIDTH         | 22102.8125Mbits/sec | 10Mbits/sec | [PASS] |             |
| RTT               | 0.61ms              | 50ms        | [PASS] |             |
| REQUEST_BODY_SIZE | >1.0MB              | 1MB         | [PASS] |             |
| PROXY_BUFFER      | N/A                 |             | [PASS] |             |
+-------------------+---------------------+-------------+--------+-------------+

~~~

如果双方节点的网络状态存在异常，一个可能的报告如下：

~~~
REPORT:
CRD CONFIG CHECK:
+-----------+------+--------+----------------------------------+
|   NAME    | TYPE | RESULT |           INFORMATION            |
+-----------+------+--------+----------------------------------+
| alice-bob | cdr  | [FAIL] | cdr alice-bob status             |
|           |      |        | not succeeded, reason:           |
|           |      |        | LastUpdateAt=2024-08-14          |
|           |      |        | 14:52:03 +0800 CST,              |
|           |      |        | Message=TokenNotGenerate,        |
|           |      |        | Reason:DestinationIsNotAuthrized |
| bob-alice | cdr  | [PASS] |                                  |
+-----------+------+--------+----------------------------------+

NETWORK STATSTICS(alice-bob):
+-------------------+---------------------+-------------+-----------+--------------------------------+
|       NAME        |   DETECTED VALUE    |  THRESHOLD  |  RESULT   |          INFORMATION           |
+-------------------+---------------------+-------------+-----------+--------------------------------+
| BANDWIDTH         | 5.60921Mbits/sec    | 10Mbits/sec | [WARNING] | not satisfy threshold 10Mb/sec |
| CONNECTION        | N/A                 |             | [PASS]    |                                |
| PROXY_BUFFER      | 1828bytes           |             | [FAIL]    | proxy buffer exists, please    |
|                   |                     |             |           | check the proxy config         |
| REQUEST_BODY_SIZE | 0.39062~0.78125MB   | 1MB         | [WARNING] | not satisfy threshold 1MB      |
| RTT               | 56.51ms             | 50ms        | [WARNING] | not satisfy threshold 50ms     |
+-------------------+---------------------+-------------+-----------+--------------------------------+
~~~

### 报告字段说明

- CRD Config Check: 检查配置的 ClusterDomainRoute 是否有效，若为 FAIL，则说明 CDR 配置有误或节点本身网络不通。
- NETWORK STATSTICS(alice-bob)：Alice 到 Bob 的请求链路网络指标，包含：
  - BANDWIDTH：网络带宽指标，默认阈值为 10Mbits/sec，可通过配置 `--speed_thres \<threshold\>` 调整，当带宽检测值（DETECTED VALUE）小于10Mbits/sec 时，结果为 WARNING；
  - CONNECTION：联通性，检测 Kuscia Job 的服务网络联通；
  - PROXY_BUFFER：网关缓冲，结果为FAIL时表示网关存在缓冲，需要联系机构网关关闭网关缓冲；
  - REQUEST_BODY_SIZE：网关请求包体限制，默认阈值为 1MB，可通过配置 `--size_thres \<threshold\>` 调整，当包体限制检测值（DETECTED VALUE）小于 1MB 时，结果为 WARNING；
  - RTT：传输延迟，默认阈值为 50ms，可通过配置 `--rtt_thres \<threshold\>`调整，当传输延迟检测值（DETECTED VALUE）大于 50ms 时，结果为 WARNING。
- NETWORK STATSTICS(bob-alice): Bob 到 Alice 的请求链路网络指标。

### 其他说明

kuscia diagnose network 参数说明：

~~~
bash-5.2# kuscia diagnose network -h
Diagnose the status of network between domains

Usage:
  kuscia diagnose network [flags]

Flags:
  -b, --bidirection                   Execute bidirection test (default true)
      --buffer                        Enable proxy buffer test (default true)
  -h, --help                          help for network
      --proxy-timeout                 Enable proxy timeout test
      --proxy-timeout-threshold int   Proxy timeout threshold, unit ms (default 600)
      --request-size-threshold int    Request size threshold, unit MB (default 1)
      --rtt                           Enable latency test (default true)
      --rtt-threshold int             RTT threshold, unit ms (default 50)
      --size                          Enable request body size test (default true)
      --speed                         Enable bandwidth test (default true)
      --speed-threshold int           Bandwidth threshold, unit Mbits/sec (default 10)
~~~

## kuscia diagnose log

### 功能

能够根据用户输入的 kuscia taskId，通过查询和解析对应的日志文件，获取任务运行期间节点间的网络访问是否有异常状况，帮助更好的排错。

### 使用场景

- 可用于执行任务失败之后，通过解析日志文件，定位具体节点间交互的异常情况。

### 前置条件

已经执行完成（成功或者失败）。

### 使用示例

假设用户任务 taskId 为 secretflow-task-20250617142539-single-psi，则需要执行日志解析流程，可以在任意一个参与方的master节点中容器内执行：

~~~
kuscia diagnose log secretflow-task-20250617142539-single-psi
~~~

即会开始执行任务日志解析流程，正常执行的结果如下：

~~~
Envoy log task <secretflow-task-20250617142539-single-psi> analysis
REPORT:
~~~

如果任务在执行的过程中，存在网络异常，一个可能的报告如下：

~~~
Envoy log task <secretflow-task-20250617142539-single-psi> analysis
REPORT:
DOMAIN(bob) TASK(secretflow-task-20250617142539-single-psi) NETWORK alice ---> bob
+------------+----------------------------+---------------------------------------------------------+------------------------------------------------+------------+------------------+------------+---------------+-------------+
|     IP     |         TIMESTAMP          |                       SERVICENAME                       |                 INTERFACEADDR                  | HTTPMETHOD |     TRACEID      | STATUSCODE | CONTENTLENGTH | REQUESTTIME |
+------------+----------------------------+---------------------------------------------------------+------------------------------------------------+------------+------------------+------------+---------------+-------------+
| 172.18.0.2 | 17/Jun/2025:06:25:53 +0000 | secretflow-task-20250617142539-single-psi-0-spu.bob.svc | /org.interconnection.link.ReceiverService/Push | POST       | a720674a69674f94 | 503        | 32            | -           |
+------------+----------------------------+---------------------------------------------------------+------------------------------------------------+------------+------------------+------------+---------------+-------------+

DOMAIN(alice) TASK(secretflow-task-20250617142539-single-psi) NETWORK alice ---> bob
+-----------+----------------------------+---------------------------------------------------------+------------------------------------------------+------------+------------------+------------+---------------+-------------+
|    IP     |         TIMESTAMP          |                       SERVICENAME                       |                 INTERFACEADDR                  | HTTPMETHOD |     TRACEID      | STATUSCODE | CONTENTLENGTH | REQUESTTIME |
+-----------+----------------------------+---------------------------------------------------------+------------------------------------------------+------------+------------------+------------+---------------+-------------+
| 10.88.0.2 | 17/Jun/2025:06:25:53 +0000 | secretflow-task-20250617142539-single-psi-0-spu.bob.svc | /org.interconnection.link.ReceiverService/Push | POST       | a720674a69674f94 | 503        | 32            | 0           |
+-----------+----------------------------+---------------------------------------------------------+------------------------------------------------+------------+------------------+------------+---------------+-------------+

DOMAIN(alice) TASK(secretflow-task-20250617142539-single-psi) NETWORK bob ---> alice
+------------+----------------------------+-----------------------------------------------------------+------------------------------------------------+------------+------------------+------------+---------------+-------------+
|     IP     |         TIMESTAMP          |                        SERVICENAME                        |                 INTERFACEADDR                  | HTTPMETHOD |     TRACEID      | STATUSCODE | CONTENTLENGTH | REQUESTTIME |
+------------+----------------------------+-----------------------------------------------------------+------------------------------------------------+------------+------------------+------------+---------------+-------------+
| 172.18.0.3 | 17/Jun/2025:06:25:49 +0000 | secretflow-task-20250617142539-single-psi-0-fed.alice.svc | /org.interconnection.link.ReceiverService/Push | POST       | 097e8bcde693c4f1 | 503        | 48            | -           |
| 172.18.0.3 | 17/Jun/2025:06:25:50 +0000 | secretflow-task-20250617142539-single-psi-0-fed.alice.svc | /org.interconnection.link.ReceiverService/Push | POST       | ad00e7c10be9e47a | 503        | 48            | -           |
+------------+----------------------------+-----------------------------------------------------------+------------------------------------------------+------------+------------------+------------+---------------+-------------+

DOMAIN(bob) TASK(secretflow-task-20250617142539-single-psi) NETWORK bob ---> alice
+-----------+----------------------------+-----------------------------------------------------------+------------------------------------------------+------------+------------------+------------+---------------+-------------+
|    IP     |         TIMESTAMP          |                        SERVICENAME                        |                 INTERFACEADDR                  | HTTPMETHOD |     TRACEID      | STATUSCODE | CONTENTLENGTH | REQUESTTIME |
+-----------+----------------------------+-----------------------------------------------------------+------------------------------------------------+------------+------------------+------------+---------------+-------------+
| 10.88.0.2 | 17/Jun/2025:06:25:49 +0000 | secretflow-task-20250617142539-single-psi-0-fed.alice.svc | /org.interconnection.link.ReceiverService/Push | POST       | 097e8bcde693c4f1 | 503        | 48            | 0           |
| 10.88.0.2 | 17/Jun/2025:06:25:50 +0000 | secretflow-task-20250617142539-single-psi-0-fed.alice.svc | /org.interconnection.link.ReceiverService/Push | POST       | ad00e7c10be9e47a | 503        | 48            | 0           |
+-----------+----------------------------+-----------------------------------------------------------+------------------------------------------------+------------+------------------+------------+---------------+-------------+
~~~

### 报告字段说明

- DOMAIN(bob) TASK(secretflow-task-20250617142539-single-psi) NETWORK bob ---> alice  表示 bob 节点的任务 secretflow-task-20250617142539-single-psi 的网络访问情况，从 bob 到 alice 的网络访问情况。
  - IP：请求的目标 IP 地址。
  - TIMESTAMP：请求的时间戳。
  - SERVICENAME：请求的服务名称。
  - INTERFACEADDR：请求的接口地址。
  - HTTPMETHOD：请求的 HTTP 方法。
  - TRACEID：请求的跟踪 ID。
  - STATUSCODE：请求的状态码。
  - CONTENTLENGTH：请求的内容长度。
  - REQUESTTIME：请求耗时。
