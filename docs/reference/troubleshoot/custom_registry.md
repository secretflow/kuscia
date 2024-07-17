# 使用自定义镜像仓库


Kuscia支持自动拉取远程的应用镜像（比如：secretflow等），这样可以不用手动导入镜像到容器中。可以在[Kuscia配置文件](../../deployment/kuscia_config_cn.md)中配置私有（or公开）镜像仓库地址。

## 如何配置使用自定义镜像仓库
配置文件中的 `image` 字段用来配置自定义仓库。相关含义参考 [Kuscia配置文件说明](../../deployment/kuscia_config_cn.md)

### 私有镜像仓库
如果有一个私有镜像仓库 （示例：`private.registry.com`）， 对应的配置如下：

```
- image:
  - defaultRegistry: private #随意，只需要对应到<image.registries[0].name>即可
  - registries:
    - name: private
      endpoint: private.registry.com/test
      username: testname
      password: testpass
```

### 公开镜像仓库
如果使用公开的镜像仓库 （示例：`secretflow-registry.cn-hangzhou.cr.aliyuncs.com`），对应的配置如下：

```
- image:
  - defaultRegistry: aliyun #随意，只需要对应到<image.registries[0].name>即可
  - registries:
    - name: aliyun
      endpoint: secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow
```


## 关于镜像仓库和AppImage的搭配使用

配置文件中有`image`字段，`AppImage` 中也存在image相关的配置，他们的搭配关系示例如下：

| 配置文件 | AppImage配置 | 实际镜像地址 | 备注 |
| - | - | - | - |
| 无配置 | secretflow/app:v1 | docker.io/secretflow/app:v1 | |
| 无配置 | private.registry.com/secretflow/app:v1 | private.registry.com/secretflow/app:v1 | |
| private.registry.com | secretflow/app:v1 | private.registry.com/app:v1 | |
| private.registry.com/secretflow | app:v1 | private.registry.com/secretflow/app:v1 | 推荐配置 |
| private.registry.com/secretflow | secretflow/app:v1 | private.registry.com/secretflow/app:v1 | |
| private.registry.com/secretflow | test/app:v1 | private.registry.com/secretflow/app:v1 | |
| private.registry.com/secretflow | private.registry.com/secretflow/app:v1 | private.registry.com/secretflow/app:v1 | |
| private.registry.com/secretflow | public.aliyun.com/secretflow/app:v1 | public.aliyun.com/secretflow/app:v1 | 强烈不推荐配置，未来可能会禁止这种配置 |


注：Kuscia推荐在`AppImage`中只配置镜像名（不带镜像仓库地址），否则切换仓库的时候，需要批量修改`AppImage`，所以不建议如此配置。

## 镜像拉取失败
当发现镜像拉取失败时，请确认 配置文件中仓库地址，以及账密相关配置是否正确， 以及参考上文，确保AppImage的镜像地址配置正确.

```
2024-06-06 13:33:00.534 ERROR framework/pod_workers.go:978 Error syncing pod "ant-test-0_ant(7fd5285b-2a5c-4a75-930a-2908e98c8799)", skipping: failed to "StartContainer" for "test" with ErrImagePull: "faile to pull image \"registry.xxxx.com/secretflow/nginx:v1\" with credentials, detail-> rpc error: code = Unknown desc = failed to pull and unpack image \"registry.xxxx.com/secretflow/nginx:v1\": failed to resolve reference \"registry.xxxx.com/secretflow/nginx:v1\": unexpected status from HEAD request to https://registry.xxxx.com/v2/secretflow/nginx/manifests/v1: 401 Unauthorized"
```