<!-- OPENSOURCE-CLEANUP DELETE_FILE -->
## 常用环境变量

目前支持下面环境变量配置


| 名称                                   | 值                       | 含义                                                                                                                                                      |
| --------------------------------------- | -------------------------- |---------------------------------------------------------------------------------------------------------------------------------------------------------|
| USE_KINE_MYSQL_V2                      | "true"/"false"           | 是否使用Kine Mysql V2版本，即内部调优后的版本，默认为"false"                                                                                                                |
| KINE_LOG_LEVEL                         | "trace", "debug", "info" | Kine日志级别                                                                                                                                                |
| KINE_METRICS_PORT                      | "11080"                  | Kine指标服务监听端口，默认为"11080"                                                                                                                                 |
| KINE_DISABLE_PROFILING                 | "true"/"false"           | 关闭 Kine 性能调试，默认为: "false"                                                                                                                               |
| KINE_SKIP_INIT_MYSQL                   | "true"/"false"           | 是否跳过初始化数据库步骤(创建数据库，表结构)，默认为"false"                                                                                                                      |
| KINE_MYSQL_ISOLATION_LEVEL             | "Serializable"           | 设置mysql事物隔离级别，支持两种"ReadCommitted"和"Serializable"。默认为"ReadCommitted"                                                                                     |
| KINE_LIST_TTL_EVENT_INTERVAL_MINUTE    | "10"                     | 检查 ttl event 是否过期间隔时间，默认为："10"                                                                                                                          |
| KINE_COMPACT_INTERVAL_MINUTE           | "5"                      | comact 间隔时间，默认为"5"                                                                                                                                      |
| KINE_COMPACT_BATCH_SIZE                | "1000"                   | compact 步长大小，默认为: "1000"                                                                                                                                |
| KINE_COMPACT_DELETE_SIZE               | "1000"                   | compact 删除大小，默认为: "1000"; value(KINE_COMPACT_DELETE_SIZE) <= value(KINE_COMPACT_BATCH_SIZE)                                                             |
| KINE_COMPACT_TIMEOUT_SECOND            | "10"                     | compact 超时时间，默认为: "10"                                                                                                                                  |
| KINE_COMPACT_RETENTION_MINUTE          | "5"                      | compact 时，跳过最近几分钟内的数据，默认为 "5"                                                                                                                           |
| KINE_DB_POLL_WAIT_MILLISECOND          | "1000"                   | 从DB Polll 数据的等待时间，默认为 "1000"                                                                                                                            |
| KINE_DB_MAX_IDLE_CONNS                 | "2"                      | 和DB的最大空闲连接数，默认为 "2"，参考:[https://pkg.go.dev/database/sql#DB.SetMaxIdleConns](https://pkg.go.dev/database/sql#DB.SetMaxIdleConns)                         |
| KINE_DB_MAX_OPEN_CONNS                 | "0"                      | 和DB的最大连接数，默认为"0"，不限制，参考:[https://pkg.go.dev/database/sql#DB.SetMaxOpenConns](https://pkg.go.dev/database/sql#DB.SetMaxOpenConns)                        |
| KINE_DB_MAX_LIFETIME_SECOND            | "0"                      | 和DB的连接超时时间，默认为"0"，不限制，依赖底层connection超时，参考[https://pkg.go.dev/database/sql#DB.SetConnMaxLifetime](https://pkg.go.dev/database/sql#DB.SetConnMaxLifetime) |
| CATTLE_NEW_SIGNED_CERT_EXPIRATION_DAYS | "3650"                   | 证书过期时间，默认为"365"                                                                                                                                         |


## 性能指标

### Metrics
```shell
curl -v http://127.0.0.1:11080/metrics
```

### Debug

- http://127.0.0.1:11080/debug/pprof/profile
- http://127.0.0.1:11080/debug/pprof/trace
- http://127.0.0.1:11080/debug/pprof/symbol
- http://127.0.0.1:11080/debug/pprof/cmdline