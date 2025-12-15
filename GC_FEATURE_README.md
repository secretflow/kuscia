# Kuscia Job GC 功能实现总结

## 📋 功能概述

为 Kuscia 添加了 KusciaJob 垃圾回收(GC)的动态配置和手动触发功能,支持:
- ✅ **动态配置**: 运行时修改 GC 配置,无需重启
- ✅ **HTTP API**: 通过 API 手动触发清理
- ✅ **立即执行**: 触发后立即清理,无需等待定时任务
- ✅ **Token + TLS 认证**: 与现有 Job API 相同的安全机制
- ✅ **大规模测试**: 支持 10000+ Job 的压力测试

---

## 📦 已实现的代码文件

### 1. 核心组件 (pkg/controllers/garbagecollection/)
- ✅ `config_manager.go` - 配置管理器,监听 ConfigMap 变更
- ✅ `trigger_manager.go` - 触发管理器,协调手动和定时触发
- ✅ `kusciajob.go` - 增强的 GC Controller,支持动态配置
- ✅ `init_configmap.go` - ConfigMap 初始化工具

### 2. API 层 (pkg/kusciaapi/)
- ✅ `service/gc_types.go` - API 数据结构定义
- ✅ `service/gc_service.go` - Service 层实现
- ✅ `handler/httphandler/gc/gc_handler.go` - HTTP Handler

### 3. 配置和权限
- ✅ `handler/httphandler/middleware/rbac/casbin_policy.csv` - RBAC 权限配置

### 4. 文档和测试
- ✅ `IMPLEMENTATION_GUIDE.md` - 详细的集成指南
- ✅ `test/gc/create_test_jobs.sh` - 创建 10000 个测试 Job 的脚本
- ✅ `GC_FEATURE_README.md` - 本文档

---

## 🔌 HTTP API 接口

所有接口需要 Token 和 TLS 证书认证,仅 master 角色可访问。

### 1. 触发 GC
```bash
POST /api/v1/gc/trigger

Request:
{
  "async": false  # true=异步执行, false=同步执行
}

Response:
{
  "status": {
    "code": 0,
    "message": "GC completed successfully"
  },
  "result": {
    "controllerName": "kuscia-job-gc-controller",
    "deletedCount": 1523,
    "errorCount": 0,
    "duration": "2m15s",
    "startTime": "2024-12-15T10:30:00Z",
    "endTime": "2024-12-15T10:32:15Z"
  }
}
```

### 2. 更新配置
```bash
POST /api/v1/gc/config/update

Request:
{
  "config": {
    "kusciaJobGC": {
      "durationHours": 168,  # 保留期改为7天
      "batchSize": 200,      # 批处理大小
      "batchInterval": 3     # 批次间隔(秒)
    }
  }
}

Response:
{
  "status": {
    "code": 0,
    "message": "Config updated successfully"
  }
}
```

### 3. 查询配置
```bash
POST /api/v1/gc/config/query

Request: {}

Response:
{
  "status": {
    "code": 0,
    "message": "Success"
  },
  "config": {
    "kusciaJobGC": {
      "durationHours": 720,
      "batchSize": 100,
      "batchInterval": 5
    }
  }
}
```

### 4. 查询状态
```bash
POST /api/v1/gc/status

Request: {}

Response:
{
  "status": {
    "code": 0,
    "message": "Success"
  },
  "gcStatus": {
    "isRunning": false,
    "lastRunTime": "2024-12-15T10:30:00Z",
    "totalRuns": 15,
    "manualRuns": 3,
    "scheduledRuns": 12,
    "lastRunResult": {
      "controllerName": "kuscia-job-gc-controller",
      "deletedCount": 1523,
      "errorCount": 0,
      "duration": "2m15s"
    }
  }
}
```

---

## 🛠️ 集成步骤

详细的集成步骤请参考 **`IMPLEMENTATION_GUIDE.md`**,主要包括:

1. 修改 `pkg/kusciaapi/bean/http_server_bean.go` - 注册 HTTP 路由
2. 修改 `cmd/kuscia/modules/controllers.go` - 初始化 GC 管理器
3. 修改 `cmd/kuscia/modules/kusciaapi.go` - 传递 GC 管理器给 HTTP Server
4. 修改 `cmd/kuscia/modules/module.go` - 添加配置结构字段

---

## 🧪 测试方案

### 单元测试
```bash
go test ./pkg/controllers/garbagecollection/... -v
go test ./pkg/kusciaapi/service/... -v -run TestGC
```

### 集成测试 (10000 个 Job)
```bash
# 1. 创建 10000 个测试 Job
cd test/gc
bash create_test_jobs.sh

# 2. 验证创建
kubectl get kusciajobs -n cross-domain -l test-purpose=gc-test --no-headers | wc -l

# 3. 触发 GC (使用实际证书和 Token)
curl -X POST https://localhost:8082/api/v1/gc/trigger \
  --cacert /path/to/ca.crt \
  --cert /path/to/client.crt \
  --key /path/to/client.key \
  -H "Token: $(cat /path/to/token)" \
  -H "Content-Type: application/json" \
  -d '{"async": false}'

# 4. 验证清理结果
kubectl get kusciajobs -n cross-domain -l test-purpose=gc-test --no-headers | wc -l
```

### 性能基准
- **创建 10000 个 Job**: < 10 分钟
- **GC 清理 10000 个 Job**: < 10 分钟
- **GC 吞吐量**: > 30 jobs/sec
- **内存增长**: < 500MB

---

## 🔐 安全机制

1. **Token 认证**: 从域私钥签名生成,长度 32 字符
2. **TLS/MTLS**: 双向证书认证
3. **RBAC**: 仅 master 角色可访问 GC API
4. **并发保护**: 原子操作防止重复执行

---

## 📊 配置示例

### ConfigMap 示例
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kuscia-gc-config
  namespace: kuscia-system
  labels:
    app: kuscia
    type: gc-config
data:
  gc-config.json: |
    {
      "kusciaJobGC": {
        "durationHours": 720,
        "batchSize": 100,
        "batchInterval": 5
      }
    }
```

### kuscia.yaml 配置 (未来支持)
```yaml
garbageCollection:
  kusciaJobGC:
    durationHours: 720  # 30天
    batchSize: 100
    batchInterval: 5
```

---

## 🎯 使用示例

### 示例 1: 清理 30 天前的 Job
```bash
# 使用默认配置(30天),直接触发
curl -X POST https://localhost:8082/api/v1/gc/trigger \
  --cacert /path/to/ca.crt \
  --cert /path/to/client.crt \
  --key /path/to/client.key \
  -H "Token: YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"async": false}'
```

### 示例 2: 修改保留期为 7 天并清理
```bash
# 1. 更新配置为 7 天
curl -X POST https://localhost:8082/api/v1/gc/config/update \
  --cacert /path/to/ca.crt \
  --cert /path/to/client.crt \
  --key /path/to/client.key \
  -H "Token: YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "config": {
      "kusciaJobGC": {
        "durationHours": 168,
        "batchSize": 100,
        "batchInterval": 5
      }
    }
  }'

# 2. 等待配置同步(约 1 分钟)
sleep 60

# 3. 触发清理
curl -X POST https://localhost:8082/api/v1/gc/trigger \
  --cacert /path/to/ca.crt \
  --cert /path/to/client.crt \
  --key /path/to/client.key \
  -H "Token: YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"async": false}'
```

### 示例 3: 异步清理大量 Job
```bash
# 异步执行,立即返回
curl -X POST https://localhost:8082/api/v1/gc/trigger \
  --cacert /path/to/ca.crt \
  --cert /path/to/client.crt \
  --key /path/to/client.key \
  -H "Token: YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"async": true}'

# 查询执行状态
curl -X POST https://localhost:8082/api/v1/gc/status \
  --cacert /path/to/ca.crt \
  --cert /path/to/client.crt \
  --key /path/to/client.key \
  -H "Token: YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}'
```

---

## 🐛 故障排查

### 问题 1: API 返回 "GC is already running"
**原因**: 上次 GC 还在执行中
**解决**: 等待执行完成,或通过 `/api/v1/gc/status` 查询状态

### 问题 2: API 返回 401 Unauthorized
**原因**: Token 错误或证书无效
**解决**: 检查 Token 和证书路径

### 问题 3: 配置更新后未生效
**原因**: ConfigMap 更新延迟
**解决**: 等待 1-2 分钟让 Informer 同步

### 问题 4: GC 删除速度慢
**原因**: 批次间隔过长
**解决**: 调整 `batchInterval` 为更小的值(如 1-2 秒)

---

## 📈 监控建议

1. **日志监控**: 搜索 "KusciaJob GC" 关键字
2. **指标监控** (未来支持):
   - `kuscia_gc_runs_total` - 总执行次数
   - `kuscia_gc_deleted_jobs_total` - 删除的 Job 总数
   - `kuscia_gc_duration_seconds` - GC 执行时长
   - `kuscia_gc_errors_total` - 错误次数

---

## 🚀 后续优化方向

- [ ] 添加 Prometheus 指标
- [ ] 支持 Dry-Run 模式(仅查询不删除)
- [ ] 支持按标签过滤清理
- [ ] 支持清理其他资源(Task、Deployment)
- [ ] 添加清理历史记录
- [ ] 支持定时清理策略配置

---

## 📝 代码规范

- ✅ 遵循 [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)
- ✅ 添加 Apache 2.0 License 头
- ✅ 使用 `nlog` 进行日志记录
- ✅ 使用 `klog` 进行 K8s 相关日志
- ✅ 错误处理完整
- ✅ 并发安全

---

## 👥 贡献者

本功能由 Claude Code 协助实现,符合 Kuscia 项目规范。

---

## 📄 许可证

Apache License 2.0

---

**注意**: 请在生产环境使用前进行充分测试!
