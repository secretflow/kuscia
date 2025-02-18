# Kuscia

[![CircleCI](https://dl.circleci.com/status-badge/img/gh/secretflow/kuscia/tree/main.svg?style=svg)](https://dl.circleci.com/status-badge/redirect/gh/secretflow/kuscia/tree/main)

<p align="center">
<a href="./README.zh-CN.md">简体中文</a>｜<a href="./README.md">English</a>
</p>

Kuscia(Kubernetes-based Secure Collaborative InfrA) is a lightweight privacy-preserving computing task orchestration framework based on K3s.
It provides a unified privacy-preserving computing foundation that can abstract away heterogeneous infrastructure and protocols.
With Kuscia:

- Lightweight deployment: You can perform privacy set intersection (PSI) on datasets with 1 million records using minimal resources of 1 CPU and 2 GB RAM.
- Cross-domain network security communication: You can achieve port reuse (requiring only one public network port) and secure communication during the concurrent execution of multiple privacy computing tasks.
- Unified API interface: You can integrate privacy-preserving computing capabilities using HTTP/GRPC API interfaces.
- Interconnection: You can interconnect with various privacy-preserving computing systems within the industry.

For more information about Kuscia's capabilities, please refer to the [Kuscia Overview](./docs/reference/overview.md).

![Kuscia](./docs/imgs/kuscia_architecture.png)

## Documentation

Currently, we only provide detailed documentations in Chinese.

- [Kuscia](https://www.secretflow.org.cn/docs/kuscia/latest/zh-Hans/)
- [Getting Started](https://www.secretflow.org.cn/docs/kuscia/latest/zh-Hans/getting_started/index.html)
- [Reference](https://www.secretflow.org.cn/docs/kuscia/latest/zh-Hans/reference/index.html)
- [Tutorial](https://www.secretflow.org.cn/docs/kuscia/latest/zh-Hans/tutorial/index.html)
- [Development](https://www.secretflow.org.cn/docs/kuscia/latest/zh-Hans/development/index.html)

## Contributing

Please check [CONTRIBUTING.md](./CONTRIBUTING.md)

## Disclaimer

Non-release version of Kuscia is only for demonstration and should not be used in production environments.
Although this version of Kuscia covers the basic abilities, there may be some security issues and functional defects due to insufficient functionality and unfinished items in the project.
We welcome your active suggestions and look forward to the official release.
