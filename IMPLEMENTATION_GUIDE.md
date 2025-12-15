# Kuscia Job GC 功能实现指南

本文档说明如何将 GC 功能集成到 Kuscia 项目中。

## 已完成的代码文件

### 核心组件
1. ✅ `pkg/controllers/garbagecollection/config_manager.go` - 配置管理器
2. ✅ `pkg/controllers/garbagecollection/trigger_manager.go` - 触发管理器
3. ✅ `pkg/controllers/garbagecollection/kusciajob.go` - 增强的 GC Controller
4. ✅ `pkg/controllers/garbagecollection/init_configmap.go` - ConfigMap 初始化

### API 层
5. ✅ `pkg/kusciaapi/service/gc_types.go` - API 数据结构
6. ✅ `pkg/kusciaapi/service/gc_service.go` - Service 层实现
7. ✅ `pkg/kusciaapi/handler/httphandler/gc/gc_handler.go` - HTTP Handler

### 配置
8. ✅ `pkg/kusciaapi/handler/httphandler/middleware/rbac/casbin_policy.csv` - RBAC 权限配置

---

## 需要手动集成的部分

### 1. 修改 `pkg/kusciaapi/bean/http_server_bean.go`

#### 步骤 1.1: 添加 import

在文件开头的 import 部分添加:

```go
import (
    // ... 现有 imports ...
    gchandler "github.com/secretflow/kuscia/pkg/kusciaapi/handler/httphandler/gc"
    "github.com/secretflow/kuscia/pkg/controllers/garbagecollection"
)
```

#### 步骤 1.2: 修改 httpServerBean 结构体

在 `httpServerBean` 结构体中添加字段:

```go
type httpServerBean struct {
    config          *apiconfig.KusciaAPIConfig
    externalGinBean *beans.GinBean
    internalGinBean *beans.GinBean
    cmConfigService cmservice.IConfigService

    // 新增: GC 管理器
    gcConfigManager  *garbagecollection.GCConfigManager
    gcTriggerManager *garbagecollection.GCTriggerManager
}
```

#### 步骤 1.3: 修改 NewHTTPServerBean 函数

```go
func NewHTTPServerBean(
    config *apiconfig.KusciaAPIConfig,
    cmConfigService cmservice.IConfigService,
    gcConfigManager *garbagecollection.GCConfigManager,   // 新增参数
    gcTriggerManager *garbagecollection.GCTriggerManager, // 新增参数
) *httpServerBean {
    return &httpServerBean{
        config: config,
        externalGinBean: &beans.GinBean{
            // ... 现有配置 ...
        },
        internalGinBean: &beans.GinBean{
            // ... 现有配置 ...
        },
        cmConfigService: cmConfigService,
        gcConfigManager: gcConfigManager,       // 新增
        gcTriggerManager: gcTriggerManager,     // 新增
    }
}
```

#### 步骤 1.4: 在 registerGroupRoutes 函数中添加 GC Service 和路由

在 `registerGroupRoutes` 函数中,找到创建 service 的部分,添加:

```go
func (s *httpServerBean) registerGroupRoutes(e framework.ConfBeanRegistry, bean *beans.GinBean) {
    // ... 现有 services ...
    logService := service.NewLogService(s.config)

    // 新增: 创建 GC Service
    var gcService service.IGCService
    if s.gcConfigManager != nil && s.gcTriggerManager != nil {
        gcService = service.NewGCService(s.gcConfigManager, s.gcTriggerManager)
    }

    // define router groups
    groupsRouters := []*router.GroupRouters{
        // ... 现有路由组 ...
```

然后在 `groupsRouters` 数组末尾(health 路由组之前)添加 GC 路由组:

```go
        // GC group routes (新增)
        {
            Group: "api/v1/gc",
            Routes: []*router.Router{
                {
                    HTTPMethod:   http.MethodPost,
                    RelativePath: "trigger",
                    Handlers:     []gin.HandlerFunc{createGCHandler(gcService, "trigger")},
                },
                {
                    HTTPMethod:   http.MethodPost,
                    RelativePath: "config/update",
                    Handlers:     []gin.HandlerFunc{createGCHandler(gcService, "update")},
                },
                {
                    HTTPMethod:   http.MethodPost,
                    RelativePath: "config/query",
                    Handlers:     []gin.HandlerFunc{createGCHandler(gcService, "query")},
                },
                {
                    HTTPMethod:   http.MethodPost,
                    RelativePath: "status",
                    Handlers:     []gin.HandlerFunc{createGCHandler(gcService, "status")},
                },
            },
        },
        // health group routes
        {
            Group: "",
            // ...
        },
```

#### 步骤 1.5: 添加辅助函数

在文件末尾添加:

```go
// createGCHandler 创建 GC Handler
func createGCHandler(gcService service.IGCService, handlerType string) gin.HandlerFunc {
    if gcService == nil {
        return func(c *gin.Context) {
            c.JSON(http.StatusServiceUnavailable, service.Status{
                Code:    -1,
                Message: "GC service not available",
            })
        }
    }

    handler := gchandler.NewGCHandler(gcService)

    switch handlerType {
    case "trigger":
        return handler.TriggerGC
    case "update":
        return handler.UpdateGCConfig
    case "query":
        return handler.QueryGCConfig
    case "status":
        return handler.QueryGCStatus
    default:
        return func(c *gin.Context) {
            c.JSON(http.StatusNotFound, service.Status{
                Code:    -1,
                Message: "Unknown handler type",
            })
        }
    }
}
```

---

### 2. 修改 `cmd/kuscia/modules/controllers.go`

#### 步骤 2.1: 查找 NewControllersModule 函数

找到 controllers 模块的初始化函数。

#### 步骤 2.2: 添加 GC 配置管理器初始化

在函数开头添加:

```go
func NewControllersModule(i *ModuleRuntimeConfigs) (Module, error) {
    ctx := context.Background()

    // 初始化 GC ConfigMap
    namespace := "kuscia-system"  // 根据实际配置调整
    if err := garbagecollection.InitGCConfigMap(ctx, i.Clients.KubeClient, namespace); err != nil {
        nlog.Warnf("Failed to init GC ConfigMap: %v", err)
    }

    // 创建 GC ConfigManager
    gcConfigManager, err := garbagecollection.NewGCConfigManager(
        i.Clients.KubeClient,
        namespace,
        garbagecollection.DefaultConfigMapName,
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create GC ConfigManager: %v", err)
    }

    // 启动 ConfigManager
    go func() {
        if err := gcConfigManager.Start(ctx); err != nil {
            nlog.Errorf("Failed to start GC ConfigManager: %v", err)
        }
    }()

    // 存储到模块中,供 KusciaAPI 使用
    // 注意:需要在 ModuleRuntimeConfigs 中添加相应字段:
    // GCConfigManager  *garbagecollection.GCConfigManager
    // GCTriggerManager *garbagecollection.GCTriggerManager

    // ... 继续现有的初始化逻辑 ...
}
```

#### 步骤 2.3: 修改 KusciaJob GC Controller 创建

找到创建 KusciaJobGCController 的代码,修改为:

```go
// 创建 KusciaJob GC Controller
kusciaJobGC := garbagecollection.NewKusciaJobGCController(ctx, controllerConfig)

// 类型断言获取具体类型
if kgc, ok := kusciaJobGC.(*garbagecollection.KusciaJobGCController); ok {
    // 设置 ConfigManager
    kgc.SetConfigManager(gcConfigManager)

    // 获取 TriggerManager 并存储
    i.GCTriggerManager = kgc.GetTriggerManager()
}

i.GCConfigManager = gcConfigManager
```

---

### 3. 修改 `cmd/kuscia/modules/kusciaapi.go`

#### 步骤 3.1: 修改 NewKusciaAPIModule 函数

找到创建 `httpServerBean` 的代码,修改为:

```go
func NewKusciaAPIModule(i *ModuleRuntimeConfigs) (Module, error) {
    // ... 现有初始化逻辑 ...

    // 创建 HTTP Server Bean时传入 GC 管理器
    httpServerBean := bean.NewHTTPServerBean(
        kusciaAPIConfig,
        i.CMConfigService,
        i.GCConfigManager,   // 新增
        i.GCTriggerManager,  // 新增
    )

    // ... 其他逻辑 ...
}
```

---

### 4. 修改 `cmd/kuscia/modules/module.go`

#### 步骤 4.1: 在 ModuleRuntimeConfigs 结构体中添加字段

```go
type ModuleRuntimeConfigs struct {
    // ... 现有字段 ...

    // GC 管理器
    GCConfigManager  *garbagecollection.GCConfigManager
    GCTriggerManager *garbagecollection.GCTriggerManager
}
```

---

## 编译和测试

### 1. 编译项目

```bash
make build
```

### 2. 运行测试

```bash
# 单元测试
go test ./pkg/controllers/garbagecollection/... -v

# 集成测试
go test ./pkg/kusciaapi/service/... -v -run TestGC
```

### 3. 启动 Kuscia

```bash
./bin/kuscia start -c etc/conf/kuscia.yaml
```

### 4. 验证 GC API

```bash
# 查询配置
curl -X POST http://localhost:8082/api/v1/gc/config/query \
  --cacert /path/to/ca.crt \
  --cert /path/to/client.crt \
  --key /path/to/client.key \
  -H "Token: YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}'

# 触发 GC
curl -X POST http://localhost:8082/api/v1/gc/trigger \
  --cacert /path/to/ca.crt \
  --cert /path/to/client.crt \
  --key /path/to/client.key \
  -H "Token: YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"async": false}'
```

---

## 故障排查

### 问题 1: ConfigMap 未创建

**症状**: 启动时提示 "ConfigMap not found"

**解决**:
```bash
# 手动创建 ConfigMap
kubectl apply -f - <<EOF
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
EOF
```

### 问题 2: GC API 返回 404

**原因**: 路由未正确注册

**检查**:
1. 确认 `http_server_bean.go` 中添加了 GC 路由组
2. 确认 `gcService` 不为 nil
3. 检查日志中是否有路由注册错误

### 问题 3: Token 认证失败

**原因**: Token 不正确

**解决**:
```bash
# 查看 Token
cat /home/kuscia/var/certs/token

# 或从域私钥生成
# Token 是域 ID 使用域私钥签名后的前 32 个字符
```

---

## 性能优化建议

1. **批处理大小**: 根据 Job 数量调整,建议 100-200
2. **批次间隔**: 避免 API Server 过载,建议 1-5 秒
3. **保留期**: 根据业务需求调整,默认 30 天
4. **清理时间**: 建议在业务低峰期执行手动清理

---

## 后续工作

- [ ] 添加 Prometheus 指标监控
- [ ] 添加更详细的日志
- [ ] 支持 Dry-Run 模式
- [ ] 添加清理前的二次确认机制
- [ ] 支持按标签过滤清理
