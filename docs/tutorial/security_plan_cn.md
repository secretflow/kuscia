# 安全加固方案

## 说明

在面对不可知任务场景或未知代码执行的场景中，建议使用进程安全沙箱 [NsJail](https://github.com/google/nsjail) 保障运行时安全以及使用安全容器 [Kata](https://github.com/kata-containers/kata-containers)  保障容器安全。

SecretFlow 支持通过 NsJail 启动，这使得 SecretFlow 任务可以在安全沙箱中运行，增强任务运行时的安全性。

## 在安全沙箱中运行 SecretFlow 任务

基于 Kuscia 框架， 您可以快速体验在安全沙箱中运行 SecretFlow 任务，这里以部署中心化组网模式为例，关于更多的部署细节请参考[快速入门](../getting_started/quickstart_cn.md)。

1. 配置 Kuscia 镜像，以下示例选择使用 latest 版本镜像：

    ```bash
    export KUSCIA_IMAGE=secretflow/kuscia
    ```

2. 获取 Kuscia 安装脚本，安装脚本会下载到当前目录：

    ```bash
    docker pull ${KUSCIA_IMAGE} && docker run --rm -v $(pwd):/tmp/kuscia ${KUSCIA_IMAGE} cp -f /home/kuscia/scripts/deploy/start_standalone.sh /tmp/kuscia
    ```

3. 开启特权配置，并以中心化组网模式启动集群（开启容器特权时，会带来容器逃逸的风险，建议使用安全容器 kata)：

    ```bash
    # 开启特权配置，使得 Kuscia 可以通过特权方式启动 Secretflow 容器。
    export ALLOW_PRIVILEGED=true

    # 启动集群，会拉起 3 个 docker 容器，包括一个控制平面 master 和两个 Lite 节点 alice 和 bob。
    ./start_standalone.sh center
    ```

4. 执行作业：

    ```bash
    # 登入 master 容器。
    docker exec -it ${USER}-kuscia-master bash

    # 创建并启动作业（两方 PSI 任务），作业会在安全沙箱中执行。
    scripts/user/create_example_job.sh NSJAIL_PSI

    # 查看作业状态。
    kubectl get kj -n cross-domain
    ```
