# GC 功能集成完成总结

## 完成时间
2025-12-15

## 已完成的集成步骤

### Step 1: 修改 http_server_bean.go ✅ (用户完成)
用户已经完成了此步骤,包括:
- 添加 GC 相关 imports
- 修改 httpServerBean 结构体
- 修改 NewHTTPServerBean 函数签名
- 添加 GC 路由组
- 添加 createGCHandler 辅助函数

### Step 2: 修改 controllers 模块 ✅ (已完成)

#### 2.1 修改 `pkg/controllers/controller.go`
- 在 `ControllerConfig` 结构体中添加了 `GCConfigManager interface{}` 字段

#### 2.2 修改 `pkg/controllers/options.go`
- 在 `Options` 结构体中添加了 `GCConfigManager interface{}` 字段

#### 2.3 修改 `pkg/controllers/server.go`
- 在创建 `ControllerConfig` 时传递 `GCConfigManager` 字段

#### 2.4 修改 `cmd/kuscia/modules/controllers.go`
- 添加了 GC ConfigMap 初始化代码
- 创建并启动了 GCConfigManager
- 将 GCConfigManager 存储到 ModuleRuntimeConfigs
- 在 Options 中传递 GCConfigManager

#### 2.5 修改 `cmd/kuscia/modules/runtime.go`
- 在 `ModuleRuntimeConfigs` 结构体中添加了 GC 管理器字段:
  ```go
  GCConfigManager  *garbagecollection.GCConfigManager
  GCTriggerManager *garbagecollection.GCTriggerManager
  ```

#### 2.6 修改 `pkg/controllers/garbagecollection/trigger_manager.go`
- 添加了全局 TriggerManager 注册机制:
  - `SetGlobalKusciaJobGCTriggerManager()`
  - `GetGlobalKusciaJobGCTriggerManager()`

#### 2.7 修改 `pkg/controllers/garbagecollection/kusciajob.go`
- 在 `NewKusciaJobGCController` 中接收 GCConfigManager
- 自动调用 `SetConfigManager()` 设置配置管理器
- 自动注册 TriggerManager 到全局变量

### Step 3: 修改 kusciaapi 模块 ✅ (已完成)

#### 3.1 修改 `pkg/kusciaapi/config/kusciaapi_config.go`
- 在 `KusciaAPIConfig` 结构体中添加了 `GCConfigManager interface{}` 字段

#### 3.2 修改 `cmd/kuscia/modules/kusciaapi.go`
- 在 `NewKusciaAPI` 函数中将 `d.GCConfigManager` 设置到 `kusciaAPIConfig.GCConfigManager`

#### 3.3 修改 `pkg/kusciaapi/commands/root.go`
- 添加了 garbagecollection 包的 import
- 从 `kusciaAPIConfig.GCConfigManager` 获取 ConfigManager
- 通过 `garbagecollection.GetGlobalKusciaJobGCTriggerManager()` 获取 TriggerManager
- 在创建 httpServerBean 时传递两个 GC 管理器

## 架构设计说明

### 数据流

```
ModuleRuntimeConfigs (controllers.go)
    ↓
    ├─→ GCConfigManager (初始化并启动)
    │       ↓
    │   传递到 ControllerConfig
    │       ↓
    │   KusciaJobGCController (构造时接收)
    │       ↓
    │   SetConfigManager() (自动调用)
    │
    └─→ TriggerManager
            ↓
        KusciaJobGCController.triggerManager (创建时自动生成)
            ↓
        SetGlobalKusciaJobGCTriggerManager() (自动注册到全局)

KusciaAPIConfig (kusciaapi.go)
    ↓
    从 ModuleRuntimeConfigs 获取 GCConfigManager
    ↓
commands.Run()
    ↓
    ├─→ gcConfigManager (从 kusciaAPIConfig 获取)
    └─→ gcTriggerManager (从全局变量获取)
    ↓
NewHTTPServerBean(config, cmConfigService, gcConfigManager, gcTriggerManager)
    ↓
GC Service + GC Handler
    ↓
GC API Endpoints
```

### 关键设计决策

1. **GCConfigManager 传递方式**: 通过 Options → ControllerConfig → KusciaJobGCController 构造函数
   - 优点: 清晰的依赖注入,便于测试
   - 实现: 在 controller 初始化时自动设置

2. **GCTriggerManager 共享方式**: 使用包级别的全局变量
   - 原因: Controllers 在 leader election 后异步创建,无法直接返回实例
   - 实现: 在 KusciaJobGCController 构造时自动注册到全局
   - 访问: KusciaAPI 通过 `GetGlobalKusciaJobGCTriggerManager()` 获取

3. **线程安全**:
   - GCConfigManager: 使用 RWMutex 保护配置访问
   - TriggerManager: 使用 atomic 操作防止并发执行
   - 全局 TriggerManager: 使用 RWMutex 保护读写

## 已修改的文件列表

### Controllers 模块
1. `pkg/controllers/controller.go` - 添加 GCConfigManager 字段到 ControllerConfig
2. `pkg/controllers/options.go` - 添加 GCConfigManager 字段到 Options
3. `pkg/controllers/server.go` - 传递 GCConfigManager 到 ControllerConfig
4. `pkg/controllers/garbagecollection/trigger_manager.go` - 添加全局注册机制
5. `pkg/controllers/garbagecollection/kusciajob.go` - 自动设置 ConfigManager 和注册 TriggerManager
6. `cmd/kuscia/modules/controllers.go` - 初始化 GC ConfigManager 并传递
7. `cmd/kuscia/modules/runtime.go` - 添加 GC 管理器字段

### KusciaAPI 模块
8. `pkg/kusciaapi/config/kusciaapi_config.go` - 添加 GCConfigManager 字段
9. `pkg/kusciaapi/commands/root.go` - 获取并传递 GC 管理器
10. `cmd/kuscia/modules/kusciaapi.go` - 设置 GCConfigManager

## API 端点

GC 功能通过以下 HTTP API 端点暴露:

- `POST /api/v1/gc/trigger` - 触发 GC 执行
- `POST /api/v1/gc/config/update` - 更新 GC 配置
- `POST /api/v1/gc/config/query` - 查询 GC 配置
- `POST /api/v1/gc/status` - 查询 GC 状态

所有端点需要:
- Token 认证 (Header: `Token`)
- TLS/MTLS (cert, key, cacert)
- RBAC: 仅 master 角色可访问

## 配置说明

GC 配置存储在 Kubernetes ConfigMap 中:
- **Name**: `kuscia-gc-config`
- **Namespace**: `kuscia-system` 或 DomainID
- **Key**: `gc-config.json`

配置格式:
```json
{
  "kusciaJobGC": {
    "durationHours": 720,
    "batchSize": 100,
    "batchInterval": 5
  }
}
```

## 测试建议

### 1. 单元测试
```bash
go test ./pkg/controllers/garbagecollection/... -v
go test ./pkg/kusciaapi/service/... -v -run TestGC
```

### 2. 集成测试

#### 创建测试 Jobs
```bash
bash test/gc/create_test_jobs.sh
```

#### 查询配置
```bash
curl -X POST https://localhost:8082/api/v1/gc/config/query \
  --cacert /path/to/ca.crt \
  --cert /path/to/client.crt \
  --key /path/to/client.key \
  -H "Token: YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}'
```

#### 更新配置
```bash
curl -X POST https://localhost:8082/api/v1/gc/config/update \
  --cacert /path/to/ca.crt \
  --cert /path/to/client.crt \
  --key /path/to/client.key \
  -H "Token: YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "kusciaJobGC": {
      "durationHours": 168,
      "batchSize": 50,
      "batchInterval": 3
    }
  }'
```

#### 触发 GC (同步)
```bash
curl -X POST https://localhost:8082/api/v1/gc/trigger \
  --cacert /path/to/ca.crt \
  --cert /path/to/client.crt \
  --key /path/to/client.key \
  -H "Token: YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"async": false}'
```

#### 查询状态
```bash
curl -X POST https://localhost:8082/api/v1/gc/status \
  --cacert /path/to/ca.crt \
  --cert /path/to/client.crt \
  --key /path/to/client.key \
  -H "Token: YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}'
```

## 注意事项

1. **启动顺序**: Controllers 模块必须在 KusciaAPI 模块之前启动,以确保 TriggerManager 已注册到全局变量

2. **Leader Election**: GC Controller 只在 leader 节点运行,TriggerManager 在 controller 初始化时创建

3. **ConfigMap 权限**: 确保 Kuscia 服务账号有权限读写 ConfigMap

4. **RBAC 权限**: GC API 仅对 master 角色开放,已在 `casbin_policy.csv` 中配置

5. **动态配置**: 配置更改会通过 Informer 自动应用,无需重启

6. **并发控制**: TriggerManager 使用原子操作确保同一时间只有一个 GC 任务运行

## 故障排查

### ConfigMap 未创建
检查日志: `Failed to init GC ConfigMap`
解决: 手动创建或检查 K8s 权限

### GC API 返回 503
原因: GC Service 不可用
检查: TriggerManager 是否正确注册到全局变量

### Token 认证失败
检查: Token 文件路径和内容
位置: `/home/kuscia/var/certs/token`

### 配置更新不生效
原因: Informer 未启动或 ConfigManager 未正确初始化
检查日志: `Failed to start GC ConfigManager`

## 下一步工作

参考 `IMPLEMENTATION_GUIDE.md` 中的"后续工作"部分:
- [ ] 添加 Prometheus 指标监控
- [ ] 添加更详细的日志
- [ ] 支持 Dry-Run 模式
- [ ] 添加清理前的二次确认机制
- [ ] 支持按标签过滤清理

## 相关文档

- `IMPLEMENTATION_GUIDE.md` - 详细实现指南
- `GC_FEATURE_README.md` - 完整功能文档
- `test/gc/create_test_jobs.sh` - 测试脚本
