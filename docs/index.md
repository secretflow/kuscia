# Kuscia

Kuscia（Kubernetes-based Secure Collaborative InfrA）是一款基于 K8s 的隐私计算任务编排框架，旨在屏蔽异构基础设施和协议，并提供统一的隐私计算底座。通过 Kuscia：

- 你可以快速体验隐私计算功能。
- 你可以获得完整的隐私计算生产能力。
- 你可以与行业内多种隐私计算系统进行互联互通。
- 你可以使用不同的中心化或点对点业务组网模式。

## 入门

从零到一运行你的第一个 SecretFlow 作业！

1. [安装 Kuscia 并运行示例任务][quickstart]
2. [提交 KusciaJob][run-secretflow]

[quickstart]: ./getting_started/quickstart_cn.md
[run-secretflow]: ./getting_started/run_secretflow_cn.md

## 架构及设计

理解 Kuscia 架构以及重要概念。

- [架构总览][architecture]
- Kuscia 概念：[Domain][concept-domain] | [DomainRoute][concept-domainroute] | [DomainData][concept-domaindata] | [KusciaJob][concept-kusciajob] | [KusciaTask][concept-kusciatask] | [AppImage][concept-appimage] | [InteropConfig][concept-interopconfig]

[architecture]: ./reference/architecture_cn.md
[concept-domain]: ./reference/concepts/domain_cn.md
[concept-domainroute]: ./reference/concepts/domainroute_cn.md
[concept-domaindata]: ./reference/concepts/domaindata_cn.md
[concept-kusciajob]: ./reference/concepts/kusciajob_cn.md
[concept-kusciatask]: ./reference/concepts/kusciatask_cn.md
[concept-appimage]: ./reference/concepts/appimage_cn.md
[concept-interopconfig]: ./reference/concepts/interopconfig_cn.md

## Kuscia API

- [Kuscia API 介绍][api-overview] | [教程：用 Kuscia API 运行 SecretFlow 作业][api-tutorial]
- API 参考：[请求和响应][api-request-and-response] | [Domain][api-domain] | [DomainRoute][api-domainroute] | [DomainData][api-domaindata] | [KusciaJob][api-kusciajob] | [Health][api-health]

[api-overview]: ./reference/apis/summary_cn.md
[api-tutorial]: ./tutorial/run_secretflow_with_api_cn.md
[api-request-and-response]: ./reference/apis/summary_cn.md#请求和响应约定
[api-domain]: ./reference/apis/domain_cn.md
[api-domainroute]: ./reference/apis/domainroute_cn.md
[api-domaindata]: ./reference/apis/domaindata_cn.md
[api-kusciajob]: ./reference/apis/kusciajob_cn.md
[api-health]: ./reference/apis/health_cn.md

## 部署

- [指南：多机器部署中心化集群][deploy-p2p]
- [指南：多机器部署点对点集群][deploy-master-lite]
- [常见运维操作][ops-cheatsheet]

[deploy-master-lite]: ./deployment/deploy_master_lite_cn.md
[deploy-p2p]: ./deployment/deploy_p2p_cn.md
[ops-cheatsheet]: ./deployment/operation_cn.md

## 更多指南

- [如何运行一个互联互通银联 BFIA 协议作业][how-to-bfia]
- [如何运行一个 FATE 作业][how-to-fate]
- [安全加固方案][how-to-security-plan]

[how-to-bfia]: ./tutorial/run_bfia_job_cn.md
[how-to-fate]: ./tutorial/run_fate_cn.md
[how-to-security-plan]: ./tutorial/security_plan_cn.md

## 获得帮助

使用 Kuscia 时遇到问题？在这里找到获得帮助的方式。

- [常见问题（FAQ）][faq]
- Kuscia 的 [Issues] 和 [讨论区]

[faq]: ./reference/faq_cn.md
[Issues]: https://github.com/secretflow/kuscia/issues
[讨论区]: https://github.com/secretflow/kuscia/discussions

## 开发 Kuscia

- [构建 Kuscia][build-kuscia]
- [注册自定义算法镜像][custom-image]

[build-kuscia]: ./development/build_kuscia_cn.md
[custom-image]: ./development/register_custom_image.md

```{toctree}
:hidden:

getting_started/index
reference/index
deployment/index
tutorial/index
development/index
```
