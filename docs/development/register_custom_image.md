# 注册自定义算法镜像

在 Kuscia 中，使用 [AppImage](../reference/concepts/appimage_cn.md) 存储算法镜像模版信息。后续在运行任务时，必须在任务中指定 [AppImage](../reference/concepts/appimage_cn.md) 名称，从而实现算法应用 Pod 镜像启动模版的绑定，启动算法应用 Pod。

若你想使用自定义算法镜像运行任务，那么可参考下面步骤准备算法镜像 [AppImage](../reference/concepts/appimage_cn.md) 和将算法镜像加载到节点容器中。

## 准备工具脚本

### 获取工具脚本

```shell
# 中心化组网部署模式
docker cp ${USER}-kuscia-master:/home/kuscia/scripts/tools/register_app_image .

# 点对点组网部署模式
docker cp ${USER}-kuscia-autonomy-alice:/home/kuscia/scripts/tools/register_app_image .
```

### 工具脚本介绍

工具脚本的功能如下：

- 将自定义算法镜像加载到节点容器内
- 将自定义算法镜像的 AppImage 注册到 Kuscia 中

查看工具脚本帮助信息

```shell
./register_app_image/register_app_image.sh -h
```

工具脚本支持的 Flag 参数含义如下：

- `-h`：可选参数，查看工具脚本帮助信息
- `-m`：必填参数，指定 Kuscia 的部署模式，支持`[center, p2p]`。中心化组网模式为`center`和点对点组网模式为`p2p`
- `-i`：必填参数，指定需要注册的自定义算法的 Docker 镜像，包含镜像名称和 TAG 信息。可以通过命令`docker images`查询。 镜像示例: `secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/secretflow-lite-anolis8:latest`
- `-u`：可选参数，指定部署 Kuscia 的用户，默认为：`${USER}`。通过命令`echo ${USER}`可查看当前用户
- `-n`：可选参数，指定自定义算法镜像相关的 Kuscia AppImage 名称。若不指定，则工具脚本将根据算法镜像名称生成对应的 AppImage 名称。

## 准备自定义算法镜像的 AppImage

你可以在工具目录`register_app_image`下获取 Secretflow 算法镜像的 AppImage 模版`appimage.yaml`。若有需要，可参考 [AppImage](../reference/concepts/appimage_cn.md) 对模版进行修改。

在该模版中，以下占位符不建议修改，这些占位符实际内容由工具脚本动态填充。
- `{{APP_IMAGE_NAME}}`: 自定义算法镜像对应的 Kuscia AppImage 名称
- `{{IMAGE_NAME}}`: 自定义算法镜像名称
- `{{IMAGE_TAG}}`: 自定义算法镜像标签

## 注册镜像

### 中心化组网部署模式

注册自定义算法镜像

```shell
./register_app_image/register_app_image.sh -u {USER} -m center -n {APP_IMAGE_NAME} -i {IMAGE}

# 示例: ${USER} 用户注册 secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/secretflow-lite-anolis8:latest 镜像
./register_app_image/register_app_image.sh -u ${USER} -m center -n secretflow-image -i secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/secretflow-lite-anolis8:latest
=> register app image: secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/secretflow-lite-anolis8:latest
...
=> app_image_name: secretflow-image
```

- 在自定义算法镜像注册完成后，可以获取算法镜像对应的 AppImage 资源名称: `app_image_name: secretflow-image`

### 点对点组网部署模式

注册自定义算法镜像

```shell
./register_app_image/register_app_image.sh -u {USER} -m p2p -n {APP_IMAGE_NAME} -i {IMAGE}

# 示例: ${USER} 用户注册 secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/secretflow-lite-anolis8:latest 镜像
./register_app_image/register_app_image.sh -u ${USER} -m p2p -n secretflow-image -i secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/secretflow-lite-anolis8:latest
=> register app image: secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/secretflow-lite-anolis8:latest
...
=> app_image_name: secretflow-image
```
- 在自定义算法镜像注册完成后，可以获取算法镜像对应的 AppImage 资源名称: `app_image_name: secretflow-image`


## 使用自定义算法镜像运行作业

通过前面步骤注册完自定义算法镜像后，你可以获取算法镜像对应的 AppImage 资源名称。后续使用自定义算法镜像运行任务时，只需修改相应的字段即可。

下面以名称为`secretflow-image`的 AppImage 为例，使用自定义算法镜像运行 [KusciaJob](../reference/concepts/kusciajob_cn.md) 作业。

- 修改 KusciaJob 下 `spec.tasks[].appImage`字段的值。

```yaml
apiVersion: kuscia.secretflow/v1alpha1
kind: KusciaJob
metadata:
  name: job-best-effort-linear
spec:
  initiator: alice
  scheduleMode: BestEffort
  maxParallelism: 2
  tasks:
    - taskID: job-psi
      alias: job-psi
      priority: 100
      taskInputConfig: '{"sf_datasource_config":{"bob":{"id":"default-data-source"},"alice":{"id":"default-data-source"}},"sf_cluster_desc":{"parties":["alice","bob"],"devices":[{"name":"spu","type":"spu","parties":["alice","bob"],"config":"{\"runtime_config\":{\"protocol\":\"REF2K\",\"field\":\"FM64\"},\"link_desc\":{\"connect_retry_times\":60,\"connect_retry_interval_ms\":1000,\"brpc_channel_protocol\":\"http\",\"brpc_channel_connection_type\":\"pooled\",\"recv_timeout_ms\":1200000,\"http_timeout_ms\":1200000}}"},{"name":"heu","type":"heu","parties":["alice","bob"],"config":"{\"mode\": \"PHEU\", \"schema\": \"paillier\", \"key_size\": 2048}"}],"ray_fed_config":{"cross_silo_comm_backend":"brpc_link"}},"sf_node_eval_param":{"domain":"preprocessing","name":"psi","version":"0.0.1","attr_paths":["input/receiver_input/key","input/sender_input/key","protocol","precheck_input","bucket_size","curve_type"],"attrs":[{"ss":["id1"]},{"ss":["id2"]},{"s":"ECDH_PSI_2PC"},{"b":true},{"i64":"1048576"},{"s":"CURVE_FOURQ"}],"inputs":[{"type":"sf.table.individual","meta":{"@type":"type.googleapis.com/secretflow.component.IndividualTable","schema":{"ids":["id1"],"features":["age","education","default","balance","housing","loan","day","duration","campaign","pdays","previous","job_blue-collar","job_entrepreneur","job_housemaid","job_management","job_retired","job_self-employed","job_services","job_student","job_technician","job_unemployed","marital_divorced","marital_married","marital_single"],"id_types":["str"],"feature_types":["float","float","float","float","float","float","float","float","float","float","float","float","float","float","float","float","float","float","float","float","float","float","float","float"]},"num_lines":"-1"},"data_refs":[{"uri":"alice.csv","party":"alice","format":"csv"}]},{"type":"sf.table.individual","meta":{"@type":"type.googleapis.com/secretflow.component.IndividualTable","schema":{"ids":["id2"],"features":["contact_cellular","contact_telephone","contact_unknown","month_apr","month_aug","month_dec","month_feb","month_jan","month_jul","month_jun","month_mar","month_may","month_nov","month_oct","month_sep","poutcome_failure","poutcome_other","poutcome_success","poutcome_unknown"],"labels":["y"],"id_types":["str"],"feature_types":["float","float","float","float","float","float","float","float","float","float","float","float","float","float","float","float","float","float","float"],"label_types":["int"]},"num_lines":"-1"},"data_refs":[{"uri":"bob.csv","party":"bob","format":"csv"}]}]},"sf_output_uris":["psi-output.csv"],"sf_output_ids":["psi-output"]}'
      appImage: secretflow-image
      parties:
        - domainID: alice
        - domainID: bob
    - taskID: job-split
      alias: job-split
      priority: 100
      dependencies: ['job-psi']
      taskInputConfig: '{"sf_datasource_config":{"bob":{"id":"default-data-source"},"alice":{"id":"default-data-source"}},"sf_cluster_desc":{"parties":["alice","bob"],"devices":[{"name":"spu","type":"spu","parties":["alice","bob"],"config":"{\"runtime_config\":{\"protocol\":\"REF2K\",\"field\":\"FM64\"},\"link_desc\":{\"connect_retry_times\":60,\"connect_retry_interval_ms\":1000,\"brpc_channel_protocol\":\"http\",\"brpc_channel_connection_type\":\"pooled\",\"recv_timeout_ms\":1200000,\"http_timeout_ms\":1200000}}"},{"name":"heu","type":"heu","parties":["alice","bob"],"config":"{\"mode\": \"PHEU\", \"schema\": \"paillier\", \"key_size\": 2048}"}],"ray_fed_config":{"cross_silo_comm_backend":"brpc_link"}},"sf_node_eval_param":{"domain":"preprocessing","name":"train_test_split","version":"0.0.1","attr_paths":["train_size","test_size","random_state","shuffle"],"attrs":[{"f":0.75},{"f":0.25},{"i64":1234},{"b":true}],"inputs":[{"type":"sf.table.vertical_table","meta":{"@type":"type.googleapis.com/secretflow.component.VerticalTable","schemas":[{"ids":["id1"],"features":["age","education","default","balance","housing","loan","day","duration","campaign","pdays","previous","job_blue-collar","job_entrepreneur","job_housemaid","job_management","job_retired","job_self-employed","job_services","job_student","job_technician","job_unemployed","marital_divorced","marital_married","marital_single"],"id_types":["str"],"feature_types":["float","float","float","float","float","float","float","float","float","float","float","float","float","float","float","float","float","float","float","float","float","float","float","float"]},{"ids":["id2"],"features":["contact_cellular","contact_telephone","contact_unknown","month_apr","month_aug","month_dec","month_feb","month_jan","month_jul","month_jun","month_mar","month_may","month_nov","month_oct","month_sep","poutcome_failure","poutcome_other","poutcome_success","poutcome_unknown","y"],"id_types":["str"],"feature_types":["float","float","float","float","float","float","float","float","float","float","float","float","float","float","float","float","float","float","float","int"]}]},"data_refs":[{"uri":"psi-output.csv","party":"alice","format":"csv"},{"uri":"psi-output.csv","party":"bob","format":"csv"}]}]},"sf_output_uris":["train-dataset.csv","test-dataset.csv"],"sf_output_ids":["train-dataset","test-dataset"]}'
      appImage: secretflow-image
      parties:
        - domainID: alice
        - domainID: bob
```
