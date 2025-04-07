# 注册自定义算法镜像

在 Kuscia 中，使用 [AppImage](../reference/concepts/appimage_cn.md) 存储算法镜像模版信息。后续在运行任务时，必须在任务中指定 [AppImage](../reference/concepts/appimage_cn.md) 名称，从而实现算法应用 Pod 镜像启动模版的绑定，启动算法应用 Pod。

若你想使用自定义算法镜像运行任务，那么可参考下面步骤准备算法镜像 [AppImage](../reference/concepts/appimage_cn.md) 和将算法镜像加载到节点容器中。

## 准备工具脚本

### 获取工具脚本

```shell
# Use Kuscia image, here we use the latest version
export KUSCIA_IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia

docker pull $KUSCIA_IMAGE && docker run --rm $KUSCIA_IMAGE cat /home/kuscia/scripts/deploy/register_app_image.sh > register_app_image.sh && chmod u+x register_app_image.sh
```

### 工具脚本介绍

工具脚本的功能如下：

- 将自定义算法镜像加载到节点容器内
- 将自定义算法镜像的 AppImage 注册到 Kuscia 中

查看工具脚本帮助信息

```shell
./register_app_image.sh -h
```

工具脚本支持的 Flag 参数含义如下：

- `-h`：可选参数，查看工具脚本帮助信息
- `-c`：必填参数，指定需要注册的自定义算法的 Docker 容器名
- `-i`：必填参数，指定需要注册的自定义算法的 Docker 容器镜像名
- `-f`：可选参数，指定自定义算法镜像相关的 Kuscia AppImage 模版文件。推荐在工具脚本同级目录下，以规则`{Kuscia AppImage 名称}.yaml`命名模版文件。否则必须通过该标志指定模版文件。
- `--import`：可选参数，将自定义算法镜像导入到节点容器中时指定该参数。

## 准备自定义算法镜像的 AppImage

你可以在引擎官网获取到 AppImage 模版

- `Secretflow` 引擎模版：[app_image.secretflow.yaml](https://github.com/secretflow/kuscia/blob/main/scripts/templates/app_image.secretflow.yaml)
- `Serving` 引擎模版：[app_image.serving.yaml](https://www.secretflow.org.cn/zh-CN/docs/serving/0.2.1b0/topics/deployment/serving_on_kuscia#appimage)
- `SCQL` 引擎模版：[app_image.scql.yaml](https://www.secretflow.org.cn/zh-CN/docs/scql/main/topics/deployment/run-scql-on-kuscia)
- 其他自定义算法镜像参考：[AppImage](../reference/concepts/appimage_cn)
- 自定义算法配置文件渲染参考： [配置文件渲染](../tutorial/config_render.md)

## 加载自定义算法镜像到节点容器

## 注册镜像

### 点对点模式

- Autonomy 节点需要同时导入引擎镜像和注册 Appimage，下面以 root-kuscia-autonomy-alice 节点为例，其他 autonomy 节点也需要进行导入

```shell
./register_app_image.sh -c root-kuscia-autonomy-alice -i secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/scql:latest -f appimage.yaml --import
```

### 中心化模式

- Master 节点注册 Appimage 即可，下面以 root-kuscia-master 为例

```shell
./register_app_image.sh -c root-kuscia-master -i secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/scql:latest -f appimage.yaml
```

- Lite 节点导入引擎镜像即可，下面以 root-kuscia-lite-alice 节点为例，其他 lite 节点也需要进行导入

```shell
./register_app_image.sh -c root-kuscia-lite-alice -i secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/scql:latest --import
```

## 使用自定义算法镜像运行作业

通过前面步骤注册完自定义算法镜像后，你可以获取算法镜像对应的 AppImage 资源名称。后续使用自定义算法镜像运行任务时，只需修改相应的字段即可。

以名称为`secretflow-image`的 AppImage 为例，使用自定义算法镜像运行 [KusciaJob](../reference/concepts/kusciajob_cn.md) 作业，修改[KusciaJob 示例](../reference/concepts/kusciajob_cn.md#创建-kusciajob) 中 `spec.tasks[].appImage`字段的值。
