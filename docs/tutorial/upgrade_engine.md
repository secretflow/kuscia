# 如何在 Kuscia 中升级引擎镜像

Kuscia 支持在部署后升级引擎版本，本文档介绍如何在 Kuscia 中升级引擎镜像。

## 导入引擎镜像

Kuscia 提供脚本升级镜像和手动升级镜像两种方式，您可以根据自己的需求选择合适的方式。

### 脚本升级镜像

1. 获取工具脚本
```shell
docker cp root-kuscia-autonomy-alice:/home/kuscia/scripts .
```

2. 注册镜像

- 点对点模式

Autonomy 节点需要同时导入引擎镜像和注册 AppImage，下面以 root-kuscia-autonomy-alice 节点为例，其他 Autonomy 节点也需要进行导入
```shell
./scripts/deploy/register_app_image.sh -c root-kuscia-autonomy-alice -i secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/secretflow-lite-anolis8:latest -f scripts/templates/app_image.secretflow.yaml --import
```
- 中心化模式

Master 节点注册 AppImage 即可，下面以 root-kuscia-master 为例
```shell
./scripts/deploy/register_app_image.sh -c root-kuscia-master -i secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/secretflow-lite-anolis8:latest -f scripts/templates/app_image.secretflow.yaml
```

Lite 节点导入引擎镜像即可，下面以 root-kuscia-lite-alice 节点为例，其他 Lite 节点也需要进行导入
```shell
./scripts/deploy/register_app_image.sh -c root-kuscia-lite-alice -i secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/secretflow-lite-anolis8:latest --import
```

### 手动升级镜像
kuscia 命令支持在 RunC、RunP 模式中导入引擎镜像，使用示例如下：

1. 登录到 Autonomy、Lite 节点中
```shell
docker exec -it ${USER}-kuscia-autonomy-alice bash
```

2. 导入镜像
执行 kuscia image 导入镜像，此处以 sf 镜像为例
```shell
# 导入镜像
kuscia image pull secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/secretflow-lite-anolis8:1.7.0b0
```

如果您使用的是`私有仓库`，请加上 creds 参数指定账户密码，示例如下：
```shell
# 导入镜像
kuscia image pull --creds "user:password" private.registry.com/secretflow/secretflow-lite-anolis8:1.7.0b0
```

如果您的环境无法访问镜像仓库，您也可以将镜像打成 tar 包传到容器里，然后通过 kuscia image load 导入，示例如下：
```shell
# 导入镜像
kuscia image load -i secretflow-lite-anolis8.tar
```

验证镜像导入成功
```shell
# 查看镜像
kuscia image list
```

3. 注册 AppImage
镜像导入之后需要在 Autonomy 和 Master 节点上修改 AppImage，Lite 节点无需执行，示例如下：
```shell
# 进入 master 容器
docker exec -it ${USER}-kuscia-master bash

# appimage 以实际引擎名称为准，此处以 secretflow 默认名称为例
kubectl edit appimage secretflow-image

# 修改 image 字段中的 name、tag ,修改后保存退出
  image:
    name: xxx
    tag: xxx
```