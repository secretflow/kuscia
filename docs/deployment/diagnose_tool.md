# Kuscia Diagnostic Tool

## Function

To check whether the network between two nodes meets communication requirements and diagnose any existing network issues.

Detection items include:

- Bandwidth
- Transmission delay
- Gateway maximum request body size configuration
- Gateway buffer configuration
- Gateway timeout configuration

## Use Cases

- For pre-deployment network environment checks before executing algorithm tasks;

- For diagnosing network issues when algorithm tasks fail, to identify or rule out network-related factors.

## Prerequisites

Users must have completed the deployment of Kuscia on both nodes, including starting Kuscia, creating a Domain, exchanging certificates between the two parties, and configuring authorization for each other.

## Usage Example

Assume the two nodes are alice and bob. To test the network communication from alice to bob, you can execute the following command in alice's compute node container:

~~~
kuscia diagnose network alice bob
~~~

The normal execution result is as follows:

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

If there are network issues between the two nodes, a possible report might look like this:

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

## Report Field Explanation

- CRD Config Check: Checks whether the configured ClusterDomainRoute is valid. If the result is FAIL, it indicates that the CDR configuration is incorrect or the node's network is not accessible.
- NETWORK STATSTICS(alice-bob): Network metrics for the request link from Alice to Bob, including:
  - BANDWIDTH: Network bandwidth metric. The default threshold is 10Mbits/sec. You can adjust it using the `--speed_thres \<theshold\>` option. If the detected value is less than 10Mbits/sec, the result will be WARNING;
  - CONNECTION: Network connectivity of Kuscia Job services;
  - PROXY_BUFFER: Gateway buffer. A FAIL result indicates that the gateway buffer exists and needs to be disabled by contacting the gateway administrator;
  - REQUEST_BODY_SIZE: Gateway request body size limit. The default threshold is 1MB. You can adjust it using the `--size_thres \<threshold\>` option. If the detected value is less than 1MB, the result will be WARNING;
  - RTT: Transmission delay. The default threshold is 50ms. You can adjust it using the `--rtt_thres \<threshold\>` option. If the detected value exceeds 50ms, the result will be WARNING.
- NETWORK STATSTICS(bob-alice): Network metrics for the request link from Bob to Alice.

## Additional Notes

Explanation of the Kuscia diagnose network parameters:

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
