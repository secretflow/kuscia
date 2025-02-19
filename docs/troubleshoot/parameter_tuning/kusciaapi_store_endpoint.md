# OSS（对象存储服务）Endpoint 配置问题

## 问题描述

Kuscia 访问百度云数据存储服务 BOS 报错，报错如下所示：

```bash
botocore.exceptions.ClientError: An error occurred (400) when calling the ListObjectsV2 operation: Bad Request
```

## 解决方案

使用 Kuscia API 注册数据源的时候，需要带上 `s3` 前缀来标明 client 将使用 `AWS s3` 标准协议连接 BOS。即如果 BOS 的地址为 `bcebos.xxx.com`，需要使用 `s3.bcebos.xxx.com` 作为 Endpoint。示例如下所示：

```bash
export CTR_CERTS_ROOT=/home/kuscia/var/certs
curl -k -X POST 'https://localhost:8082/api/v1/domaindatasource/create' \
 --header "Token: $(cat ${CTR_CERTS_ROOT}/token)" \
 --header 'Content-Type: application/json' \
 --cert ${CTR_CERTS_ROOT}/kusciaapi-server.crt \
 --key ${CTR_CERTS_ROOT}/kusciaapi-server.key \
 --cacert ${CTR_CERTS_ROOT}/ca.crt \
 -d '{
  "domain_id": "alice",
  "datasource_id":"demo-oss-datasource",
  "type":"oss",
  "name": "DemoDataSource",
  "info": {
      "oss": {
          "endpoint": "https://s3.bcebos.xxx.com",
          "bucket": "secretflow",
          "prefix": "kuscia/",
          "access_key_id":"ak-xxxx",
          "access_key_secret" :"sk-xxxx"
      }
  },
  "access_directly": true
}'
```

## 原因分析

AWS s3 服务在调用 ListObjectsV2 操作时报错 400 通常表示客户端发送了一个格式不正确或无效的请求。这可能是由于请求参数错误、缺失、或者请求的格式不符合 AWS 服务的要求。

## 问题总结

不同云平台提供的存储服务 Endpoint 有所差异，需要根据具体平台的要求进行适配。

百度云 BOS 关于兼容 AWS s3 文档可以参考[这里](https://cloud.baidu.com/doc/BOS/s/ojwvyq973)。

| 类型 | Java SDK 是否带 s3 前缀 | Python SDK 是否带 s3 前缀 |
| :---: | :---: | :---: |
| 阿里云 OSS | 带 | 带和不带前缀都支持 |
| 百度 BOS | 带 | 带 |
| 华为 OBS | 不带 | 不带 |
| 开源 minio | 不带 | 不带 |
