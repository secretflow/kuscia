# 作业运行失败

## 排查步骤

### 检查机器配置

若机器不满足[官网推荐配置](https://www.secretflow.org.cn/docs/kuscia/latest/zh-Hans/getting_started/quickstart_cn#id2)，可能会造成部分服务无法正常工作，从而导致作业运行失败。

### 检查网络授权

大部分任务失败问题都是网络问题，检查网络授权可以登录到容器（中心化模式在 master 容器、点对点模式在 automony 容器）并执行`kubectl get cdr`命令查看授权信息，READY 为 True 时表明通信正常，为空时表明通信异常，可以先看下 HOST 和端口是否正确或者执行 `kubectl get cdr ${cdr_name} -oyaml` 命令看下详细信息，参数确认无误仍无法正常通信请参考[授权错误排查](../network/network_authorization_check.md)。

### 检查内核参数

如果宿主机的内核参数配置过低，在运行一些比较大的任务或者并发多任务时，也容易出现任务失败的情况，可以参考 [内核参数](../parameter_tuning/kernel_params.md) 检查内核参数是否符合 Kuscia 的推荐配置。

### 查看作业失败原因

登陆查看作业的容器

- 若以中心化组网模式部署，请登陆到 Master 容器中

    ```shell
    docker exec -it ${USER}-kuscia-master bash
    ```

- 若以点对点组网模式部署，请登陆到下发作业的容器中，下面以 Alice 容器为例

    ```shell
    docker exec -it ${USER}-kuscia-autonomy-alice bash
    ```

查看作业信息：

```shell
kubectl get kj -n cross-domain
```

查看作业下任务的详细信息

- 具体失败原因可以查看 status 字段下的相关内容

    ```shell
    # list all jobs
    kubectl get kt -n cross-domain

    # View the detailed information of the task. The task name comes from the above command.
    kubectl get kt {任务名称} -n cross-domain -o yaml
    ```

查看任务 Pod 的详细信息

- 具体失败原因可以查看 status 字段下的相关内容

    ```shell
    # View the list of pods under the alice node
    kubectl get pod -n alice

    # View the detailed information of a certain pod under the alice node
    kubectl get pod xxxx -o yaml -n alice

    # View the list of pods under the bob node
    kubectl get pod -n bob

    # View the detailed information of a certain pod under the bob node
    kubectl get pod xxxx -o yaml -n bob

    # If the pod status is Pending, you can continue to view the detailed information of the corresponding node
    kubectl get node

    # View the detailed information of the node
    kubectl describe node xxxx
    ```

查看任务 Pod 详细日志

- 若以中心化组网模式部署，登陆到 Alice 和 Bob 容器命令如下

    ```shell
    # Log in to the alice container
    docker exec -it ${USER}-kuscia-lite-alice bash

    # View the task pod log on the alice node
    cat /home/kuscia/var/stdout/pods/podName_xxxx/xxxx/x.log

    # Log in to the bob container
    docker exec -it ${USER}-kuscia-lite-bob bash

    # View the task pod log on the bob node
    cat /home/kuscia/var/stdout/pods/podName_xxxx/xxxx/x.log
    ```

- 若以点对点组网模式部署，登陆到 Alice 和 Bob 容器命令如下

    ```shell
    # Log in to the alice container
    docker exec -it ${USER}-kuscia-autonomy-alice bash

    # View the task pod log on the alice node
    cat /home/kuscia/var/stdout/pods/podName_xxxx/xxxx/x.log

    # Log in to the bob container
    docker exec -it ${USER}-kuscia-autonomy-bob bash

    # View the task pod log on the bob node
    cat /home/kuscia/var/stdout/pods/podName_xxxx/xxxx/x.log
    ```
  
任务运行遇到网络错误时，可以参考[这里](../network/network_troubleshoot.md)排查。
