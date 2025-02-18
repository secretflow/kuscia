# 部署要求

## 前言

Kuscia 对操作系统、Docker 版本、处理器型号等有一些要求，部署 Kuscia 请确保您的环境符合以下要求。

## Docker 版本要求

我们推荐使用 Docker **20.10.24 或更高版本**。Docker 的安装请参考[官方文档](https://docs.docker.com/engine/install/)，Docker 部署包下载参考[Docker软件包](https://download.docker.com/linux/centos/7/x86_64/stable/Packages/)。

## K8s 版本要求

我们推荐使用 K8s **1.20.0 或更高版本**。K8s 的安装请参考[官方文档](https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/install-kubeadm/)。

## MySQL 版本要求

我们推荐使用 MySQL **8.0.23 或者更高版本**，防止由于小版本(如 8.0.20) Bug 导致的连接异常。

## 处理器要求

支持的处理器包括：

- **x86_64**
- **arm64**

## 操作系统要求

支持的操作系统包括：

- **MacOS**
- **CentOS 7**
- **CentOS 8**
- **Ubuntu 16.04 及以上版本**
- **Windows** (通过 [WSL2 上的 Ubuntu](https://docs.microsoft.com/en-us/windows/wsl/install-win10))

## 内核版本要求

支持的 Linux 内核版本为：

- **Kernel 4.6 或更高版本**

## 资源要求

部署 Kuscia 的最低资源为：

- **1 核 CPU**
- **2 GB 内存**
- **20 GB 硬盘**

为了确保良好的性能，建议的系统资源为：

- **8 核 CPU**
- **16 GB 内存**
- **200 GB 硬盘**

## 网络要求

如果节点之间的入口网络存在网关时，为了确保节点之间通信正常，需要网关符合一些要求，详情请参考[网络要求](./networkrequirements.md)
