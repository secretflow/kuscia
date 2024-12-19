# 内核参数


Kuscia 运行小任务时对内核参数要求不高，但是在运行一些比较大的任务或者任务并发量较高时，如果内核参数配置的不太合理，会非常容易导致任务失败（常见的表现是网络通讯失败）。

Kuscia 内置了对内核参数的检查，如果内核参数不满足 Kuscia 推荐的高并发配置，会在节点实例的 `Node` 上进行 Condition 标记。日常运维时，可以通过该标记来排查任务失败是否可能是内核参数不满足要求导致的。

示例说明：
```bash
# 在 Kuscia Master/Autonomy 容器内执行
kubectl get node <node-instance-id> -o yaml
```

如果返回如下的示例，代表内核参数不符合 Kuscia 的推荐配置，可以参考脚本 [script/deploy/set_kernel_params.sh](https://github.com/secretflow/kuscia/blob/main/scripts/deploy/set_kernel_params.sh) 来进行内核参数的调整

> 注： 因为内核参数在容器内无法调整，需要在宿主机上进行调整（且需要 `root` 权限才可以调整）

```
...
  - lastHeartbeatTime: "2024-07-03T06:35:16Z"
    lastTransitionTime: "2024-07-03T06:35:16Z"
    message: tcp_max_syn_backlog=1024[ERR];somaxconn=4096[OK];tcp_retries2=15[ERR];tcp_slow_start_after_idle=1[ERR];tcp_tw_reuse=2[OK];file-max=913998[OK]
    reason: Kernel parameters not satisfy kuscia recommended requirements
    status: "False"
    type: Kernel-Params
...

```

如果 `status: "True"` 代表宿主机内核参数符合 Kuscia 的推荐内核参数配置，不再需要调整。