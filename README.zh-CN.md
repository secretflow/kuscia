# Kuscia

[![CircleCI](https://dl.circleci.com/status-badge/img/gh/secretflow/kuscia/tree/main.svg?style=svg)](https://dl.circleci.com/status-badge/redirect/gh/secretflow/kuscia/tree/main)

<p align="center">
<a href="./README.zh-CN.md">简体中文</a>｜<a href="./README.md">English</a>
</p>

Kuscia（Kubernetes-based Secure Collaborative InfrA）是一款基于 K3s 的轻量级隐私计算任务编排框架，旨在屏蔽异构基础设施和协议，并提供统一的隐私计算底座。通过 Kuscia：

- 轻量化部署：您可以用最低 1C2G 的资源完成 100W 级数据隐私求交(PSI)。
- 跨域网络安全通信：您可以实现多隐私计算任务并发执行时的端口复用（仅需一个公网端口）与安全通信。
- 统一的 API 接口：您可以使用 HTTP/GRPC API 接口集成隐私计算能力。
- 互联互通：你可以与行业内多种隐私计算系统进行互联互通。

更多 Kuscia 的能力介绍，请参考[ Kuscia 概述](./docs/reference/overview.md)。

![Kuscia](./docs/imgs/kuscia_architecture.png)

## 文档

- [Kuscia](https://www.secretflow.org.cn/docs/kuscia/latest/zh-Hans/)
- [准备开始](https://www.secretflow.org.cn/docs/kuscia/latest/zh-Hans/getting_started/index.html)
- [参考手册](https://www.secretflow.org.cn/docs/kuscia/latest/zh-Hans/reference/index.html)
- [教程](https://www.secretflow.org.cn/docs/kuscia/latest/zh-Hans/tutorial/index.html)
- [开发](https://www.secretflow.org.cn/docs/kuscia/latest/zh-Hans/development/index.html)

## 贡献代码

请查阅 [CONTRIBUTING.md](./CONTRIBUTING.md)

## 声明

非正式发布的 Kusica 版本仅用于演示，请勿在生产环境中使用。尽管此版本已涵盖 Kuscia 的基础功能，但由于项目存在功能不足和待完善项，可能存在部分安全问题和功能缺陷。因此，我们欢迎你积极提出建议，并期待正式版本的发布。
