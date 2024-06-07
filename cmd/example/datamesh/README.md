

## datamesh测试工具


### 如何编译

编译命令
```
go build -ldflags="-s -w" -o build/apps/datamesh/datamesh ./cmd/example/datamesh
```


### 如何测试

#### 启动DataMesh测试服务使用如下的命令
```
./datamesh --startDataMesh --log.level DEBUG
```
注： 对外支持的数据源是默认的`localfs`数据源， 文件目录是 `var/storage/data`


如果要测试OSS数据源，可以增加OSS相关的配置（具体的参数意义可以使用 `./datamesh --help` 查看）
```
./datamesh --startDataMesh --log.level DEBUG \
    --ossDataSource <data-source-id> --ossEndpoint <oss-endpoint> \
    --ossAccessKey <oss-access-key> \
    --ossAccessSecret <oss-access-secret> \
    --ossBucket <oss-bucket> \
    --ossPrefix <oss-path-prefix> \
    --ossType <oss-storage-type>
```


#### 测试客户端
建议使用 `python` 封装好的 `kuscia-sdk` 来进行测试， 参考文件 `<project-root>/python/test/test_datamesh.py`
