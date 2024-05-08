# 如何通过 Docker 命令对已部署的节点进行 Memory 扩容

## 背景
在使用脚本部署 Kuscia 时，可以使用 -m 或者 --memory-limit 参数给节点容器设置适当的内存限制。例如，"-m 4GiB 或 --memory-limit=4GiB" 表示限制最大内存 4GiB，"-m -1 或 --memory-limit=-1"表示没有限制，不设置默认 master 为 2GiB，lite 节点 4GiB，autonomy 节点 6GiB。如果当您的节点已经部署好了，但是遇到内存限制不符合需求时，您可以通过 Docker 命令对已部署的节点进行 Memory 扩容。

## 步骤
1. 运行以下命令来查看当前节点的内存限制：
```bash
docker inspect ${container_name} --format '{{.HostConfig.Memory}}'
```
2. 根据需要调整内存限制。例如，如果您需要增加内存限制到 20GiB，您可以运行以下命令：
```bash
docker update ${container_name} --memory=20GiB --memory-swap=20GiB
```