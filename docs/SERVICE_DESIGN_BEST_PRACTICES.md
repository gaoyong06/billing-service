# 服务设计最佳实践：BillingService vs BillingInternalService

## 问题分析

在微服务架构中，我们经常需要区分**面向外部用户的服务**和**面向内部系统的服务**。对于 `BillingService` 和 `BillingInternalService` 的设计，有以下几种方案：

## 方案对比

### 方案1：合并服务（推荐）✅

**实现方式**：
- 使用同一个 `BillingService` 结构体实现两个接口
- 在 HTTP 服务器中注册两次（不同的路由前缀）

**优点**：
- ✅ 代码简洁，避免重复
- ✅ 共享相同的业务逻辑（UseCase）
- ✅ 维护成本低
- ✅ 符合 DRY（Don't Repeat Yourself）原则

**适用场景**：
- 两个服务共享相同的业务逻辑
- 认证/授权策略相同
- 中间件需求相同
- 仅路由路径不同（`/api/v1/` vs `/internal/v1/`）

**代码示例**：
```go
// service/billing.go
type BillingService struct {
    pb.UnimplementedBillingServiceServer
    pb.UnimplementedBillingInternalServiceServer  // 同时实现两个接口
    
    uc  *biz.BillingUseCase
    log *log.Helper
}

// server/http.go
func NewHTTPServer(c *conf.Server, billing *service.BillingService, logger log.Logger) *http.Server {
    srv := http.NewServer(opts...)
    
    // 注册外部服务路由
    v1.RegisterBillingServiceHTTPServer(srv, billing)
    
    // 注册内部服务路由（使用同一个实例）
    v1.RegisterBillingInternalServiceHTTPServer(srv, billing)
    
    return srv
}
```

### 方案2：分离服务

**实现方式**：
- 创建两个独立的结构体：`BillingService` 和 `BillingInternalService`
- 分别注册到 HTTP 服务器

**优点**：
- ✅ 安全边界清晰
- ✅ 可以应用不同的中间件（认证、限流、日志）
- ✅ 可以独立扩展和部署（如果需要）

**缺点**：
- ❌ 代码重复（如果业务逻辑相同）
- ❌ 维护成本高
- ❌ 违反 DRY 原则

**适用场景**：
- 内部服务需要不同的认证机制（如服务间认证 vs 用户认证）
- 需要不同的限流策略（内部服务可能需要更高的限流阈值）
- 需要不同的日志级别或审计策略
- 未来可能需要独立部署

**代码示例**：
```go
// service/billing.go
type BillingService struct {
    pb.UnimplementedBillingServiceServer
    uc  *biz.BillingUseCase
    log *log.Helper
}

// service/billing_internal.go
type BillingInternalService struct {
    pb.UnimplementedBillingInternalServiceServer
    uc  *biz.BillingUseCase
    log *log.Helper
}

// server/http.go
func NewHTTPServer(
    c *conf.Server, 
    billing *service.BillingService,
    billingInternal *service.BillingInternalService,
    logger log.Logger,
) *http.Server {
    srv := http.NewServer(opts...)
    
    // 外部服务：应用用户认证中间件
    externalRouter := srv.Route("/api/v1")
    externalRouter.Use(auth.UserAuthMiddleware())
    v1.RegisterBillingServiceHTTPServer(externalRouter, billing)
    
    // 内部服务：应用服务间认证中间件
    internalRouter := srv.Route("/internal/v1")
    internalRouter.Use(auth.ServiceAuthMiddleware())
    v1.RegisterBillingInternalServiceHTTPServer(internalRouter, billingInternal)
    
    return srv
}
```

## 行业最佳实践

### 1. **Google Cloud API Design Guide**
- 建议：如果 API 只是路由不同，业务逻辑相同，应该合并
- 通过不同的路由前缀区分外部和内部 API
- 使用中间件处理不同的认证/授权需求

### 2. **AWS API Gateway Best Practices**
- 建议：使用同一个后端服务，通过 API Gateway 的路由规则区分
- 内部 API 可以通过不同的网关端点或 VPC 访问控制

### 3. **微服务架构模式（Martin Fowler）**
- 建议：**服务应该按业务能力划分，而不是按技术层次划分**
- 如果两个服务共享相同的业务逻辑，应该合并
- 分离应该基于**业务边界**，而不是**技术边界**

### 4. **Kubernetes Service Mesh（Istio）**
- 实践：使用同一个服务实例，通过 VirtualService 和 DestinationRule 区分路由
- 通过不同的 Service 和 Endpoint 暴露不同的访问路径

## 当前项目建议

基于当前代码分析：

1. ✅ **两个服务共享相同的 `BillingUseCase`**
2. ✅ **没有看到不同的认证/授权需求**
3. ✅ **没有看到不同的中间件需求**
4. ✅ **仅路由路径不同**

**结论：推荐使用方案1（合并服务）**

## 实施建议

### 如果未来需要分离（渐进式演进）

如果未来需要不同的认证/授权策略，可以采用以下方式：

1. **保持合并，通过中间件区分**：
   ```go
   // 外部路由：应用用户认证
   externalRouter := srv.Route("/api/v1")
   externalRouter.Use(auth.UserAuthMiddleware())
   
   // 内部路由：应用服务间认证
   internalRouter := srv.Route("/internal/v1")
   internalRouter.Use(auth.ServiceAuthMiddleware())
   ```

2. **使用不同的中间件链**：
   ```go
   // 外部服务中间件链
   externalMiddleware := []http.Middleware{
       recovery.Recovery(),
       auth.UserAuthMiddleware(),
       rateLimit.UserRateLimitMiddleware(),
   }
   
   // 内部服务中间件链
   internalMiddleware := []http.Middleware{
       recovery.Recovery(),
       auth.ServiceAuthMiddleware(),
       rateLimit.ServiceRateLimitMiddleware(),
   }
   ```

3. **如果确实需要完全分离**，再重构为两个独立的服务

## 总结

**当前阶段**：使用方案1（合并服务）
- 代码简洁
- 维护成本低
- 符合 DRY 原则

**未来演进**：如果需要不同的安全策略，通过中间件区分，而不是分离服务

**原则**：
- ✅ 按业务能力划分服务，而不是按技术层次
- ✅ 避免过早优化
- ✅ 保持代码简洁，易于维护
- ✅ 渐进式演进，而不是一次性重构

