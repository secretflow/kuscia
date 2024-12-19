# 数据库连接数过多

## 问题描述

Kuscia 执行任务报错：

```
2024-08-30 17:44:15.584 WARN handler/pending_handler.go:128 Failed to update task resource group "xxx-xxx" spec parties info, rpc error: code = Unknown desc = Error 1040 (08004): Too many connections
```

## 解决方法

1. 检查 Mysql 数据库配置的最大连接数 max_connections
```bash
SHOW VARIABLES LIKE 'max_connections';
```

2. 查看当前数据库连接进程
```bash
SHOW PROCESSLIST;
```

3. 临时修改数据库配置的最大连接数，此处以 1000 为例
```bash
SET GLOBAL max_connections = 1000;
```

4. 修改 Mysql 数据库配置文件 my.cnf，增加最大连接数
```bash
[mysqld]
max_connections = 1000
```
5. 重启 Mysql 服务
```bash
systemctl restart mysqld
```

## 参考

- [MySQL Max Connections](https://dev.mysql.com/doc/refman/8.0/en/server-system-variables.html#sysvar_max_connections)