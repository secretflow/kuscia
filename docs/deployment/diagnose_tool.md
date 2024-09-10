# Kuscia诊断工具

## 功能

检测双方节点的网络是否符合通信条件，以及网络当前的一些问题根因。

检测涵盖项：

- 带宽
- 传输延迟
- 网关最大请求包体大小配置
- 网关缓冲配置
- 网关超时配置


## 使用场景

- 可用于部署完成后，在实际执行算法作业前的前置网络环境检查；

- 可用于执行算法作业失败时，首先对双方网络环境进行诊断，定位（如果有）或排除网络环境的因素。

## 前置条件

用户已经在双方节点均完成Kuscia的部署，包括启动kuscia、创建Domain、双方互换证书、双方配置授权。

## 使用示例

假设双方节点为alice和bob，在某一方（alice或bob）计算节点容器内执行：

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
--Manual: false
--TestSpeed: true, Threshold: 10
--TestRTT: true, Threshold: 50
--TestProxyTimeout: false, Threshold: 600
--TestProxyBuffer: true
--TestRequestBodySize: true, Threshold: 1
--BidrectionMode: true
diagnose <alice-bob> network statitsics
diagnose crd config
waiting diagnose job <diagnose-alice-bob-fef293f8a68fe11b4a49> syncronize to peer...
waiting diagnose job <diagnose-alice-bob-fef293f8a68fe11b4a49> done, which may take several miniutes...
query job result...
query peer job result...
REPORT:
CRD CONFIG CHECK:
+-----------+------+--------+-------------------------------------------------+
|   NAME    | TYPE | RESULT |                   INFORMATION                   |
+-----------+------+--------+-------------------------------------------------+
| alice-bob | cdr  | [PASS] |                                                 |
| bob-alice | cdr  | [PASS] |                                                 |
+-----------+------+--------+-------------------------------------------------+

NETWORK STATSTICS(alice-bob):
+-------------------+---------------------+-------------+--------+-------------+
|       NAME        |   DETECTED VALUE    |  THRESHOLD  | RESULT | INFORMATION |
+-------------------+---------------------+-------------+--------+-------------+
| BANDWIDTH         | 7125.85938Mbits/sec | 10Mbits/sec | [PASS] |             |
| CONNECTION        | N/A                 |             | [PASS] |             |
| PROXY_BUFFER      | N/A                 |             | [PASS] |             |
| REQUEST_BODY_SIZE | >1MB                | 1MB         | [PASS] |             |
| RTT               | 1.58ms              | 50ms        | [PASS] |             |
+-------------------+---------------------+-------------+--------+-------------+

NETWORK STATSTICS(bob-alice):
+-------------------+---------------------+-------------+--------+-------------+
|       NAME        |   DETECTED VALUE    |  THRESHOLD  | RESULT | INFORMATION |
+-------------------+---------------------+-------------+--------+-------------+
| BANDWIDTH         | 6797.81129Mbits/sec | 10Mbits/sec | [PASS] |             |
| CONNECTION        | N/A                 |             | [PASS] |             |
| PROXY_BUFFER      | N/A                 |             | [PASS] |             |
| REQUEST_BODY_SIZE | >1MB                | 1MB         | [PASS] |             |
| RTT               | 0.88ms              | 50ms        | [PASS] |             |
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



## 报告字段说明
- CRD Config Check: 检查配置的ClusterDomainRoute是否有效，若为FAIL，则说明CDR配置有误或节点本身网络不通。
- NETWORK STATSTICS(alice-bob)：alice到bob的请求链路网络指标，包含：
    - BANDWIDTH：网络带宽指标，默认阈值为10Mbits/sec，可通过配置--speed_thres \<theshold\> 调整，当带宽检测值（DETECTED VALUE）小于10Mbits/sec时，结果为WARNING；
    - CONNECTION：联通性，检测Kuscia Job的服务网络联通；
    - PROXY_BUFFER：网关缓冲，结果为FAIL时表示网关存在缓冲，需要联系机构网关关闭网关缓冲；
    - REQUEST_BODY_SIZE：网关请求包体限制，默认阈值为1MB，可通过配置--size_thres \<threshold\>调整，当包体限制检测值（DETECTED VALUE）小于1MB时，结果为WARNING；
    - RTT：传输延迟，默认阈值为50ms，可通过配置--rtt_thres \<threshold\>调整，当传输延迟检测值（DETECTED VALUE）大于50ms时，结果为WARNING。
- NETWORK STATSTICS(bob-alice): bob到alice的请求链路网络指标。

## 手动模式（Manual Mode）

当运行kuscia diagnose network时，如果提示您以下内容：

~~~
waiting diagnose job <diagnose-alice-bob-ac8d8504f6b3a738d0a9> syncronize to peer...
fail to sync job to peer, err: wait job start diagnose-alice-bob-ac8d8504f6b3a738d0a9 reach timeout, fallback to manual mode
diagnose job can't syncronize from alice to bob, there might be some issues in your network. please enter the following command in bob's node to continue the diagnose procedure:

	kuscia diagnose network -m -e diagnose-alice-94c506fd57761f25eddf-0-test-server.alice.svc bob alice

waiting diagnose job <diagnose-alice-94c506fd57761f25eddf> done, which may take several miniutes...
~~~

则说明双方节点的网络存在一些未知异常（e.g. 存在网关缓冲），导致诊断工具无法将网络诊断任务同步到对方节点，此时当前命令会在alice侧一直等待检测任务同步，需要您根据终端提示，在对方节点容器内手动执行终端提供的命令，使得诊断工具能继续其网络诊断流程。

在bob侧节点容器内执行以下命令（以终端显示的命令为准），并继续在alice节点侧等待诊断命令的诊断结果：
~~~
kuscia diagnose network -m -e diagnose-alice-94c506fd57761f25eddf-0-test-server.alice.svc bob alice
~~~



## 其他说明

kuscia diagnose network参数说明：

~~~
bash-5.2# kuscia diagnose network -h
Diagnose the status of network between domains

Usage:
  kuscia diagnose network [flags]

Flags:
  -b, --bidirection                   Execute bidirection test (default true)
      --buffer                        Enable proxy buffer test (default true)
  -e, --endpoint string               Peer Endpoint, only effective in manual mode
  -h, --help                          help for network
  -m, --manual                        Initialize server/client manually
      --proxy-timeout                 Enable proxy timeout test
      --proxy-timeout-threshold int   Proxy timeout threshold, unit ms (default 600)
      --request-size-threshold int    Request size threshold, unit MB (default 1)
      --rtt                           Enable latency test (default true)
      --rtt-threshold int             RTT threshold, unit ms (default 50)
      --size                          Enable request body size test (default true)
      --speed                         Enable bandwidth test (default true)
      --speed-threshold int           Bandwidth threshold, unit Mbits/sec (default 10)
~~~