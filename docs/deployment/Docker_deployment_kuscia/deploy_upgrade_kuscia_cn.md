# 升级 Kuscia 版本

## 步骤

您可以参考 [Kuscia 部署](./deploy_p2p_cn.md)中了解如何获取 Kuscia 脚本，本文不做过多赘述。

1. 指定 Kuscia 使用的镜像版本，这里使用 1.1.0b0 版本。

   ```bash
   export KUSCIA_IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia:1.1.0b0
   ```

2. 升级 kuscia 版本只需要加上 upgrade 参数和容器名即可。示例如下：

   ```bash
   ./kuscia.sh upgrade root-kuscia-autonomy-alice
   ```

:::{tip}
注意：[体验模式](../../getting_started/quickstart_cn.md)因未挂载 Volume 到宿主机磁盘，无法使用以上方法升级 Kuscia。
:::
