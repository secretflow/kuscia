# 部署要求

## 前言
kuscia 对操作系统、docker 版本、处理器型号等有一些要求，部署 kuscia 请确保您的环境符合以下要求。

## Docker 版本要求
我们推荐使用 Docker **20.10 或更高版本**。docker 的安装请参考[官方文档](https://docs.docker.com/engine/install/)，docker 部署包下载参考[docker软件包](https://download.docker.com/linux/centos/7/x86_64/stable/Packages/)。

## 处理器要求
目前 kuscia **不支持 arm 架构**处理器，建议使用基于 **x86** 架构的处理器，我们会在未来支持 arm 架构的处理器，敬请期待。

## 操作系统要求
支持的操作系统包括：
- **macOS**
- **CentOS 7**
- **CentOS 8**
- **Ubuntu 16.04 及以上版本**
- **Windows** (通过 [WSL2 上的 Ubuntu](https://docs.microsoft.com/en-us/windows/wsl/install-win10))

## 资源要求
部署 kuscia 的最低资源为：
- **1 核 CPU**
- **2 GB 内存**
- **20 GB 硬盘**

为了确保良好的性能，建议的系统资源为：
- **8 核 CPU**
- **16 GB 内存**
- **200 GB 硬盘**

## 网络要求

如果节点之间的入口网络存在网关时，为了确保节点之间通信正常，需要网关符合一些要求，详情请参考[网络要求](./networkrequirements.md)