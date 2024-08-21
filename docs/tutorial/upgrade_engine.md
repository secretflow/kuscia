# 如何在 Kuscia 中升级引擎镜像

Kuscia 支持在部署后升级引擎版本，本文档介绍如何在 Kuscia 中升级引擎镜像。

## 导入引擎镜像

kuscia 命令支持在 RunC、RunP 模式中导入引擎镜像，使用示例如下：

登录到 autonomy、lite 节点中
```shell
docker exec -it ${USER}-kuscia-autonomy-alice bash
```

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


## 修改 AppImage

镜像导入之后需要在 autonomy 和 master 节点上修改 AppImage，lite 节点无需执行，示例如下：
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