# Network Requirements

## Preface

During the deployment process, you may encounter a complex network environment. Once problems occur, it will take a lot of time to troubleshoot, especially when an institutional gateway is introduced. The network topology of an institution may look like this:

It includes NAT gateways, firewalls, and HTTP proxy servers. It may also be a device with integrated functions. There may be policies on these devices that affect network connectivity:

- NAT and firewalls may be configured with an idle connection keep - alive duration. The situation is that if there is no traffic on a connection for a certain period of time, the connection will be closed. The symptom is that the packets sent from the sender to the institution are discarded. From the perspective of the sender's Envoy, TCP retransmissions are triggered, and from the perspective of the receiver's Envoy, the packets are directly discarded. 
- NAT and firewalls may be configured with an IP whitelist. If the whitelist is not configured, the TCP handshake requests may be directly rejected.
  - If the NAT and firewall send back a "Reset" response: From the perspective of the sender, the HTTP request will result in a 503 status code, and the TCP connection will be reset.  
  - If the NAT and firewall directly discard the packets: From the sender's perspective, the request will trigger a retransmission. 
- The firewall may be configured with security policies, which can cause requests that match the policies to be rejected, resulting in status codes such as 503 or 502 for the requests. 
- Gateway interception: When the gateway intercepts the request, it returns a 405 status code, which may lead to status codes such as 503, 502 or 405 for the request.  

## Parameter Requirements

If there is a gateway between nodes or between nodes and the Master, the gateway parameters need to meet the following requirements:

- Support the HTTP/1.1 protocol.
- The Keepalive timeout should be greater than 20 minutes.
  - TCP layer: Please confirm the firewall timeout.
  - HTTP layer: Please confirm the timeout of the institutional proxy (e.g., Nginx).
- The gateway should support sending content with a Body size of ≤ 2MB.
- Do not buffer the Request/Response to avoid poor performance; if it is an Nginx gateway, you can refer to the configuration below.
- A large amount of random number transmission in privacy computing may trigger some keyword rules of the firewall. Please ensure that keyword filtering is turned off in advance.
- Confirm the externally exposed IP and port, and whether the whitelist has been configured for the egress IP of the counterpart institution.

## Explanation of Network Connectivity

During the deployment process of Kuscia, there will probably be the following three different network mapping scenarios: 

- Scenario 1: Direct `Layer 4` communication between the institution and the external network. 
- Scenario 2: When accessing the institutional side from the outside, there is a front-end `Layer 7` gateway on the institutional side. Access to Kuscia needs to go through the `Layer 7` proxy; when Kuscia accesses the outside, it can normally go out directly at the `Layer 4`.  
- Scenario 3: There is a front-end `Layer 7` gateway on the institutional side. When the outside accesses Kuscia, it comes in through the `Layer 7` proxy, and when Kuscia accesses the external network, it also needs to go out through the `Layer 7` proxy. 

Among them:

- `Layer 4`: Generally refers to protocols such as TCP; proxy methods include Alibaba Cloud `Layer 4` SLB mapping, LVS, F5, etc.
- `Layer 7`: Generally refers to protocols such as HTTP/HTTPS/GRPC/GRPCS; proxy methods include the institutional Layer 7 firewall, Alibaba Cloud `Layer 7` SLB mapping, Nginx, Ingress, etc.

How should the authorized address be filled in:

- In Scenario 1, if there is a `Layer 4` SLB mapping to Kuscia, when configuring external nodes to access the Kuscia node of this institution, the authorized target address can use the IP address and front-end port exposed by the SLB to the outside. For example: 101.11.11.11:80; in the case of direct connection, the IP address of the host machine and the mapped port can be authorized. 
- In Scenario 2, if there is a `Layer 7` SLB or proxy mapping to Kuscia, when configuring external nodes to access the Kuscia node of this institution, the authorized target address can use the IP address and front-end port exposed by the Nginx proxy to the outside. For example: <https://101.11.11.11:443>. 
- In Scenario 3, if there are front-end `Layer 7` gateways configured both between the outside and the institutional side, when configuring external nodes to access the Kuscia node of this institution, the authorized target address should be filled in as <https://101.11.11.11:443>; when configuring the Kuscia node of this institution to access external nodes, the authorized target address should be filled in as <http://10.0.0.14>. 

> Tips: The HTTP/HTTPS protocols, IP addresses, and ports used above and in the pictures are only for example and reference purposes. Please adjust them according to actual requirements during the deployment. 

![network](../imgs/network.png)

## Examples of Nginx Proxy Parameter Configuration

- It is recommended to use Nginx release-1.15.3 and above versions. For details, please refer to [github/nginx](https://github.com/nginx/nginx). 

```bash
http {
    # Default is HTTP/1, keepalive is only enabled in HTTP/1.1
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
    #   server 0.0.0.1 weight=1 max_fails=5 fail_timeout=60s;
        server 0.0.0.2 weight=1 max_fails=5 fail_timeout=60s;
    #   Nginx_upstream_check_module can support upstream health check with Nginx
    #   Please refer to the document: https://github.com/yaoweibin/nginx_upstream_check_module/tree/master/doc
    #   check interval=3000 rise=2 fall=5 timeout=1000 type=http;

        keepalive 32;
        keepalive_timeout 600s;
        keepalive_requests 1000;
    }

    server {
        location / {
            proxy_read_timeout 10m;
            proxy_pass http://backend;
    #       Connect to kuscia with https
    #       proxy_pass https://backend;
    #       proxy_ssl_verify off;
    #       proxy_set_header Host $host;
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
