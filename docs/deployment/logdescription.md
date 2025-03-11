# Log Description

## Preface

Logs play a crucial role in application deployment, business operation, and troubleshooting. This document will provide a detailed description of the corresponding log paths.

## Kuscia Directory Structure

<pre>
/home/kuscia
    ├── bin
    ├── crds
    |   └── v1alpha1
    ├── etc
    |   ├── certs
    |   ├── cni
    |   ├── conf
    |   ├── kubeconfig
    |   ├── kuscia.kubeconfig
    |   └── kuscia.yaml
    ├── pause
    |   └──  pause.tar
    ├── scripts
    |   ├── deploy
    |   ├── templates
    |   ├── test
    |   ├── tools
    |   └── user
    └──  var
        ├── k3s
        ├── logs
        |   ├── envoy
        |   |   ├── envoy.log
        |   |   ├── envoy_admin.log
        |   |   ├── external.log
        |   |   ├── internal.log
        |   |   ├── kubernetes.log
        |   |   ├── prometheus.log
        |   |   └── zipkin.log
        |   ├── k3s.log
        |   ├── kusciaapi.log
        |   ├── containerd.log
        |   └── kuscia.log
        |
        ├── stdout
        └── storage
            └── data
</pre>

## Log File Description

| Path| Content   |
|:---------|:-------|
| `/home/kuscia/var/logs/kuscia.log` |  Records logs related to Kuscia startup status, node status, health checks, task scheduling, etc. When Kuscia startup or task execution fails, this log can be used to troubleshoot issues.  |
| `/home/kuscia/var/logs/k3s.log` |  Records logs related to k3s. When k3s startup fails, this log can be used to troubleshoot issues.  |
| `/home/kuscia/var/logs/containerd.log` | Records logs related to containerd. Issues related to containerd startup, image updates, and storage can be queried through this log. |
| `/home/kuscia/var/logs/kusciaapi.log` | Records all KusciaAPI request and response logs. |
| `/home/kuscia/var/logs/envoy/internal.log`   |  Records logs of requests initiated by the node (i.e., network requests from this node (+ internal applications) to other nodes). The log format is described below.  |
| `/home/kuscia/var/logs/envoy/external.log`  |  Records logs of requests received by the node (i.e., network requests from other nodes to this node). The log format is described below. |
| `/home/kuscia/var/logs/envoy/envoy.log`      |  The log file for the Envoy proxy, recording the runtime status, connection status, traffic information, and troubleshooting-related content of the Envoy gateway.        |
| `/home/kuscia/var/stdout/pods/alice_xxxx/xxx/*.log` |  The standard output (stdout) content of tasks.  |

:::{tip}
For the K8s RunK deployment mode, you can view the standard output logs of tasks by executing `kubectl logs ${engine_pod_name} -n xxx` in the K8s cluster where the Kuscia Pod is located.
:::

### Envoy Log Format

The format of `internal.log` is as follows:

```bash
%DOWNSTREAM_REMOTE_ADDRESS_WITHOUT_PORT% - [%START_TIME(%d/%b/%Y:%H:%M:%S %z)%] %REQ(Kuscia-Source)% %REQ(Kuscia-Host?:authority)% \"%REQ(:METHOD)% %REQ(X-ENVOY-ORIGINAL-PATH?:PATH)% %PROTOCOL%\" %REQ(x-b3-traceid)% %REQ(x-b3-spanid)% %RESPONSE_CODE% %RESPONSE_FLAGS% %REQ(content-length)% %DURATION% %REQUEST_DURATION% %RESPONSE_DURATION% %RESPONSE_TX_DURATION% %DYNAMIC_METADATA(envoy.kuscia:request_body)% %DYNAMIC_METADATA(envoy.kuscia:response_body)%
```

```bash
# Example:
1.2.3.4 - [23/Oct/2023:01:58:02 +0000] alice fgew-cwqearkz-node-4-0-fed.bob.svc "POST /org.interconnection.link.ReceiverService/Push HTTP/1.1" 743d0da7e6814c2e 743d0da7e6814c2e 200 - 1791 0 0 0 0 - -
1.2.3.4 - [23/Oct/2023:01:58:02 +0000] alice fgew-cwqearkz-node-4-0-fed.bob.svc "POST /org.interconnection.link.ReceiverService/Push HTTP/1.1" b2f636af87a047f8 b2f636af87a047f8 200 - 56 0 0 0 0 - -
1.2.3.4 - [23/Oct/2023:01:58:03 +0000] alice fgew-cwqearkz-node-4-0-fed.bob.svc "POST /org.interconnection.link.ReceiverService/Push HTTP/1.1" fdd0c66dfb0fbe45 fdd0c66dfb0fbe45 200 - 56 0 0 0 0 - -
1.2.3.4 - [23/Oct/2023:01:58:03 +0000] alice fgew-cwqearkz-node-4-0-fed.bob.svc "POST /org.interconnection.link.ReceiverService/Push HTTP/1.1" dc52437872f6e051 dc52437872f6e051 200 - 171 0 0 0 0 - -
```

The format description of `internal.log` is as follows:

| 属性                                       | 值                                                                                                                                                                                                                                              |
|------------------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `IP of the peer node` | 1.2.3.4                                                                                                                                                                                                                                        |
| `Time when the request was received`                             | 23/Oct/2023:01:58:02 +0000                                                                                                                                                                                                                     |
| `Sending node`                     | alice                                                                                                                                                                                                                                          |
| `Requested domain name`                       | fgew-cwqearkz-node-4-0-fed.bob.svc                                                                                                                                                                                                             |
| `URL`                                    | /org.interconnection.link.ReceiverService/Push                                                                                                                                                                                                 |
| `HTTP method/version`                             | HTTP/1.1                                                                                                                                                                                                                                       |
| `TRANCEID`                               | 743d0da7e6814c2e                                                                                                                                                                                                                               |
| `SPANID`                                 | 743d0da7e6814c2e                                                                                                                                                                                                                               |
| `HTTP response code`                               | 200                                                                                                                                                                                                                                            |
| `RESPONSE_FLAGS`                         | -，Indicates additional details about the response or connection; refer to the [envoy official documentation](https://www.envoyproxy.io/docs/envoy/v1.25.0/configuration/observability/access_log/usage#command-operators) for more information |
| `CONTENT-LENGTH`                         | 1791，indicating the length of the body.                                                                                                                                                                                                      |
| `DURATION`                               | 0，indicating the total request time.                                                                                                                                                                                                      |
| `REQ_META`                               | 0，indicating the meta information of the request body.                                                                                                                                                                                           |
| `RES_META`                               | 0，indicating the meta information of the request body.                                                                                                                                                                                         |
| `REQUEST_DURATION`                       | 0，the time to receive the downstream request message.                                                                                                                                                                                       |
| `RESPONSE_DURATION`                      | -，Time from request start to response start                                                                                                                                                                                                    |
| `RESPONSE_TX_DURATION`                   | -，Time to send the upstream response                                                                                                                                                                                                           |

The format of `external.log` is as follows:

```bash
%DOWNSTREAM_REMOTE_ADDRESS_WITHOUT_PORT% - [%START_TIME(%d/%b/%Y:%H:%M:%S %z)%] %REQ(Kuscia-Source)% %REQ(Kuscia-Host?:authority)% \"%REQ(:METHOD)% %REQ(X-ENVOY-ORIGINAL-PATH?:PATH)% %PROTOCOL%\" %REQ(x-b3-traceid)% %REQ(x-b3-spanid)% %RESPONSE_CODE% %RESPONSE_FLAGS% %REQ(content-length)% %DURATION% %DYNAMIC_METADATA(envoy.kuscia:request_body)% %DYNAMIC_METADATA(envoy.kuscia:response_body)%
```

```bash
1.2.3.4 - [23/Oct/2023:04:36:51 +0000] bob kuscia-handshake.alice.svc "GET /handshake HTTP/1.1" 01e87a178e05f967 01e87a178e05f967 200 - - 0 - -
1.2.3.4 - [23/Oct/2023:04:36:53 +0000] tee kuscia-handshake.alice.svc "GET /handshake HTTP/1.1" 65a07630561d3814 65a07630561d3814 200 - - 0 - -
1.2.3.4 - [23/Oct/2023:04:37:06 +0000] bob kuscia-handshake.alice.svc "GET /handshake HTTP/1.1" 8537c88b929fee67 8537c88b929fee67 200 - - 0 - -
1.2.3.4 - [23/Oct/2023:04:37:08 +0000] tee kuscia-handshake.alice.svc "GET /handshake HTTP/1.1" 875d64696b98c6fa 875d64696b98c6fa 200 - - 0 - -
```

The format description of `external.log` is as follows:

| Attribute               | Value                                                                                                                                                                                                                                       |
| ------------------ |---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `IP of the peer node`      | 1.2.3.4                                                                                                                                                                                                                                     |
| `Time when the request was received`       | 23/Oct/2023:01:58:02 +0000                                                                                                                                                                                                                  |
| `Sending node`           | alice                                                                                                                                                                                                                                       |
| `Requested domain name`         | fgew-cwqearkz-node-4-0-fed.bob.svc                                                                                                                                                                                                          |
| `URL`                | /org.interconnection.link.ReceiverService/Push                                                                                                                                                                                              |
| `HTTP method/version`          | HTTP/1.1                                                                                                                                                                                                                                    |
| `TRANCEID`            | 743d0da7e6814c2e                                                                                                                                                                                                                            |
| `SPANID`             | 743d0da7e6814c2e                                                                                                                                                                                                                            |
| `HTTP response code`        | 200                                                                                                                                                                                                                                         |
| `RESPONSE_FLAGS`     | -，indicating other detailed information about the response or connection. For details, refer to [envoy official documentation](https://www.envoyproxy.io/docs/envoy/v1.25.0/configuration/observability/access_log/usage#command-operators) |
| `CONTENT-LENGTH`     | 1791，indicating the length of the body.                                                                                                                                                                                                                        |
| `DURATION`           | 0，indicating the total request time.                                                                                                                                                                                                                     |
| `REQ_META`          | 0，indicating the meta information of the request body.                                                                                                                                                                                                             |
| `RES_META`           | 0，indicating the meta information of the request body.                                                                                                                                                                                                        |
