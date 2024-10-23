# Kuscia 端口介绍

在实际场景中，为了确保 Kuscia 运行在一个安全的网络环境中，用户需要根据本地网络防火墙，管理 Kuscia 对外暴露给合作机构的端口。

如果采用 Docker 方式部署，那么在部署的过程中，为了能够跨域和域内访问 Kuscia 节点，需要将 Kuscia 节点的部分内部端口通过端口映射的方式暴露在宿主机上。

下面是需要用户感知的端口信息，按是否需要暴露给合作方，分为两类：

* 是。该类端口需要通过公网或专线能够让合作方访问。
* 否。该类端口仅需局域网内使用，无需暴露给合作方。


| 协议       | 端口号 | 说明                                                                                                                                            | 是否需要暴露给合作方 |
| ------------ | -------- |-----------------------------------------------------------------------------------------------------------------------------------------------|------------|
| HTTP/HTTPS | 1080   | 节点之间的认证鉴权端口。在创建节点之间路由时需要指定，可参考[创建节点路由](../reference/apis/domainroute_cn.md#请求createdomainrouterequest)                                        | 是          |
| HTTP       | 80     | 访问节点中应用的端口。例如：可通过此端口访问 Serving 服务进行预测打分，可参考[使用 SecretFlow Serving 进行预测](../tutorial/run_sf_serving_with_api_cn.md#使用-secretflow-serving-进行预测) | 否          |
| HTTP/HTTPS | 8082   | 节点 KusciaAPI 的访问端口，可参考[如何使用 KusciaAPI](../reference/apis/summary_cn.md#如何使用-kuscia-api)                                                       | 否          |
| GRPC/GRPCS | 8083   | 节点 KusciaAPI 的访问端口，可参考[如何使用 KusciaAPI](../reference/apis/summary_cn.md#如何使用-kuscia-api)                                                       | 否          |
| HTTP       | 9091   | 节点 Metrics 指标采集端口，可参考 [Kuscia 监控](./kuscia_monitor)                                                                                           | 否          |