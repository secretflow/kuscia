# Kuscia 启动失败：Create fsnotify failed

## 问题描述
在 Kuscia 部署时 Lite 或者 Autonomy 节点启动失败，执行 `docker logs xxxx` 或者查看 Kuscia.log 日志打印如下报错：
```bash
WARN[2024-07-18T18:50:54.907210832+08:00] failed to load plugin io.containerd.grpc.v1.cri  error="failed to create CRI service: failed to create cni conf monitor for default: failed to create fsnotify watcher: too many open files"
```

## 解决方案
查看当前系统 inotify 的默认配置：
- max_user_watches 表示一个用户可以同时监视的最大文件数
- max_user_instances 表示一个用户可以同时使用的最大 inotify 实例数
- max_queued_events 表示内核队列中等待处理的事件的最大数量

```bash
cat /proc/sys/fs/inotify/max_user_instances

fs.inotify.max_queued_events = 16384
fs.inotify.max_user_instances = 128
fs.inotify.max_user_watches = 8192
```

出现上述报错时，推荐通过执行如下命令扩大一个用户能同时使用的 inotify 最大实例数来解决：
```bash
sudo sysctl fs.inotify.max_user_instances=8192
```