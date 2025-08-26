# Kuscia

Kuscia（Kubernetes-based Secure Collaborative InfrA）是一款基于 K3s 的轻量级隐私计算任务编排框架，旨在屏蔽异构基础设施和协议，并提供统一的隐私计算底座。通过 Kuscia：

- 轻量化部署：您可以用最低 1C2G 的资源完成 100W 级数据隐私求交(PSI)。
- 跨域网络安全通信：您可以实现多隐私计算任务并发执行时的端口复用（仅需一个公网端口）与安全通信。
- 统一的 API 接口：您可以使用 [HTTP/GRPC API 接口](./reference/apis/summary_cn.md)集成隐私计算能力。
- 互联互通：您可以与行业内多种隐私计算系统进行互联互通。

更多 Kuscia 的能力介绍，请参考 [Kuscia 概述](./reference/overview.md)。

## 入门

从零到一运行您的第一个 SecretFlow 作业！

1. [安装 Kuscia 并运行示例任务][quickstart]
2. [提交 KusciaJob][run-secretflow]

[quickstart]: ./getting_started/quickstart_cn.md
[run-secretflow]: ./getting_started/run_secretflow_cn.md

## 架构及设计

理解 Kuscia 架构以及重要概念。

- [架构总览][architecture]
- Kuscia 概念：[Domain][concept-domain] | [DomainRoute][concept-domainroute] | [DomainData][concept-domaindata] | [KusciaJob][concept-kusciajob] | [KusciaTask][concept-kusciatask] | [KusciaDeployment][concept-kusciadeployment] | [AppImage][concept-appimage] | [InteropConfig][concept-interopconfig]

[architecture]: ./reference/architecture_cn.md
[concept-domain]: ./reference/concepts/domain_cn.md
[concept-domainroute]: ./reference/concepts/domainroute_cn.md
[concept-domaindata]: ./reference/concepts/domaindata_cn.md
[concept-kusciajob]: ./reference/concepts/kusciajob_cn.md
[concept-kusciatask]: ./reference/concepts/kusciatask_cn.md
[concept-kusciadeployment]: ./reference/concepts/kusciadeployment_cn.md
[concept-appimage]: ./reference/concepts/appimage_cn.md
[concept-interopconfig]: ./reference/concepts/interopconfig_cn.md

## Kuscia API

- [Kuscia API 介绍][api-overview] | [教程：用 Kuscia API 运行 SecretFlow 作业][api-tutorial]
- API 参考：[请求和响应][api-request-and-response] | [Domain][api-domain] | [DomainRoute][api-domainroute] | [DomainData][api-domaindata] | [KusciaJob][api-kusciajob] | [Serving][api-serving] | [Health][api-health]

[api-overview]: ./reference/apis/summary_cn.md
[api-tutorial]: ./tutorial/run_sf_job_with_api_cn.md
[api-request-and-response]: ./reference/apis/summary_cn.md#请求和响应约定
[api-domain]: ./reference/apis/domain_cn.md
[api-domainroute]: ./reference/apis/domainroute_cn.md
[api-domaindata]: ./reference/apis/domaindata_cn.md
[api-kusciajob]: ./reference/apis/kusciajob_cn.md
[api-serving]: ./reference/apis/serving_cn.md
[api-health]: ./reference/apis/health_cn.md

## 部署

- [指南：部署指引][deploy-guide]
- [指南：部署要求][deploy-check]
- [指南：Docker 多机部署 Kuscia][deploy-kuscia-use-docker]
- [指南：K8s 集群部署 Kuscia][deploy-kuscia-use-k8s]
- [指南：使用 RunP 模式部署节点][deploy-with-runp]
- [指南：常见运维操作][ops-cheatsheet]
- [指南：Kuscia 网络要求][deploy-networkrequirements]
- [指南：Kuscia 日志说明][deploy-logdescription]
- [指南：Kuscia 监控][deploy-kuscia_monitor_cn]
- [指南：Kuscia 引擎指标监控][deploy-kuscia_engine_monitor_cn]
- [指南：Kuscia 配置文件][deploy-kuscia_config_cn]
- [指南：Kuscia 端口介绍][deploy-kuscia_ports_cn]

[deploy-guide]: ./deployment/kuscia_deployment_instructions.md
[deploy-check]: ./deployment/deploy_check.md
[deploy-kuscia-use-docker]: ./deployment/Docker_deployment_kuscia/index.rst
[deploy-kuscia-use-k8s]: ./deployment/K8s_deployment_kuscia/index.rst
[deploy-with-runp]: ./deployment/deploy_with_runp_cn.md
[ops-cheatsheet]: ./deployment/operation_cn.md
[deploy-networkrequirements]: ./deployment/networkrequirements.md
[deploy-logdescription]: ./deployment/logdescription.md
[deploy-kuscia_monitor_cn]: ./deployment/kuscia_monitor.md
[deploy-kuscia_engine_monitor_cn]: ./deployment/kuscia_engine_monitor.md
[deploy-kuscia_config_cn]: ./deployment/kuscia_config_cn.md
[deploy-kuscia_ports_cn]: ./deployment/kuscia_ports_cn.md

## 更多指南

- [如何使用 Kuscia API 运行一个 SecretFlow 作业][how-to-sf-job]
- [如何使用 Kuscia API 运行一个 SecretFlow Serving][how-to-serving]
- [如何使用 Kuscia API 部署 DataProxy][how-to-deploy-dp]
- [如何在 Kuscia 上运行 SCQL 联合分析任务][how-to-run-scql]
- [如何在 Kuscia 中使用自定义镜像仓库][how-to-use-custom-image]
- [如何在 Kuscia 中给自定义应用渲染配置文件][how-to-render-config]
- [如何在 Kuscia 中升级引擎镜像][how-to-upgrade-engine-image]
- [如何配置 Kuscia 对请求进行 Path Rewrite][how-to-path-rewrite]
- [如何给 Kuscia 自定义 Service 路由][how-to-custom-service-route]

[how-to-sf-job]: ./tutorial/run_sf_job_with_api_cn.md
[how-to-serving]: ./tutorial/run_sf_serving_with_api_cn.md
[how-to-deploy-dp]: ./tutorial/run_dp_on_kuscia_cn.md
[how-to-run-scql]: ./tutorial/run_scql_on_kuscia_cn.md
[how-to-use-custom-image]: ./tutorial/custom_registry.md
[how-to-render-config]: ./tutorial/config_render.md
[how-to-upgrade-engine-image]: ./tutorial/upgrade_engine.md
[how-to-path-rewrite]: ./tutorial/kuscia_gateway_with_path.md
[how-to-custom-service-route]: ./tutorial/user_defined_service_route.md

## 获得帮助

使用 Kuscia 时遇到问题？在这里找到获得帮助的方式。

- [常见问题（FAQ）][faq]
- Kuscia 的 [Issues] 和 [讨论区]

[faq]: ./troubleshoot/index.rst
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
troubleshoot/index
change_log
```

## 常见问题

- [概念答疑][concept_clarity]
- [启动部署][deploy_failed]
- [网络连接][network_failed]
- [任务运行][run_job_failed]
- [参数调优][parameter_tuning]

[concept_clarity]: ./troubleshoot/concept/index.rst
[deploy_failed]: ./troubleshoot/deployment/index.rst
[network_failed]: ./troubleshoot/network/index.rst
[run_job_failed]: ./troubleshoot/runtask/index.rst
[parameter_tuning]: ./troubleshoot/index.rst

## 版本更新日志

- [版本更新日志][change-log]

[change-log]: ./change_log.rst
