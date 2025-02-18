# Kuscia K8s 部署模式下 SecretFlow 应用线程限制问题

## 问题描述

在 Kuscia K8s 部署模式下，SecretFlow 应用运行过程中出现创建线程失败问题：`pthread_create failed: Resource temporarily unavailable`。

## 原因分析

K8s 集群 Kubelet 组件的 `podPidsLimit` 参数被限制为某个特定值，而 SecretFlow 默认启动线程数为：物理机 CPU * 32 + Ray 的线程数 ，如果 SecretFlow 运行启动的线程数超过了 Kubelet 限制的线程数时则会出现以上报错。

## 解决方案

1、找到 kubelet.service 的配置文件 /var/lib/kubelet/config.yaml（此处以官网推荐路径为例），修改 `podPidsLimit` 参数，例如 `podPidsLimit: -1`。详情参考 [K8s 官方文档](https://kubernetes.io/docs/reference/config-api/kubelet-config.v1beta1/#kubelet-config-k8s-io-v1beta1-KubeletConfiguration)。

2、修改后重启 Kubelet 服务

    ```shell
    systemctl daemon-reload
    systemctl restart kubelet.service
    ```
