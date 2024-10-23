# Kuscia 部署指引
目前 Kuscia 支持 Docker 和 K8s 两种部署方式，请根据部署环境选择相应的部署方式。

### 前置准备
Kuscia 对部署环境有一定要求，请参考[部署要求](./deploy_check.md)提前准备好环境

### Docker 多机部署 Kuscia
Docker 多机部署 Kusica 分为点对点模式和中心化模式，请根据需要选择相应的部署方式。
- 点对点模式：部署参考[多机部署点对点集群](./Docker_deployment_kuscia/deploy_p2p_cn.md)
- 中心化模式：部署参考[多机部署中心化集群](./Docker_deployment_kuscia/deploy_master_lite_cn.md)

### K8s 集群部署 Kuscia
在 K8s 集群部署中，我们将介绍点对点集群、中心化集群以及如何使用 RunP 运行时模式来训练任务。
- 点对点模式：部署参考[部署点对点集群](./K8s_deployment_kuscia/K8s_p2p_cn.md)
- 中心化模式：部署参考[部署中心化集群](./K8s_deployment_kuscia/K8s_master_lite_cn.md)
- 使用 RunP 运行时模式来训练任务：部署参考[使用 RunP 模式部署节点](./K8s_deployment_kuscia/deploy_with_runp_cn.md)

### 验证部署
Kuscia 部署完成后，可以通过以下几个方面来验证部署是否成功：
- 查看 Kuscia 容器或者 Pod 是否正常运行
- 查看 Kuscia 节点之间网络是否正常通行
- 验证 Kuscia 是否可以正常训练任务

### 故障排查
部署过程中遇到问题可以先参考[常见问题](./../reference/troubleshoot/deployfailed.md)进行排查

### 更多帮助
如需进一步帮助，请在 Kuscia [Issue](https://github.com/secretflow/kuscia/issues) 上提交问题。