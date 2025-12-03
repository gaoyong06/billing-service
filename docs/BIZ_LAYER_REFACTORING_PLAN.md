# Billing Service Biz Layer 重构方案

## 📋 当前状态分析

### 文件结构
- **文件**: `internal/biz/billing.go` (555 行)
- **内容**:
  - 7 个领域对象（UserBalance, FreeQuota, BillingRecord, RechargeOrder, Stats, ServiceStats, StatsSummary）
  - 2 个接口（BillingRepo, PaymentServiceClient）
  - 2 个 DTO（CreatePaymentRequest, CreatePaymentReply）
  - 1 个配置（BillingConfig）
  - 1 个 UseCase（BillingUseCase，包含 10 个方法）

### 问题分析
1. **单一文件过大**：555 行，包含多个职责
2. **领域对象混杂**：所有领域对象在一个文件中
3. **UseCase 职责过多**：一个 UseCase 包含所有业务逻辑
4. **不符合 DDD 原则**：没有按聚合根拆分

---

## 🎯 拆分方案

### 方案一：按领域对象拆分（推荐）✅

**参考**: `marketing-service` 的组织方式

#### 文件结构

```
internal/biz/
├── biz.go                    # ProviderSet（依赖注入）
├── config.go                 # BillingConfig 配置
├── repo.go                   # BillingRepo 接口定义
├── payment_client.go         # PaymentServiceClient 接口 + DTO
│
├── user_balance.go           # UserBalance 领域对象 + UseCase
├── free_quota.go             # FreeQuota 领域对象 + UseCase
├── billing_record.go         # BillingRecord 领域对象 + UseCase
├── recharge_order.go         # RechargeOrder 领域对象 + UseCase
├── stats.go                  # Stats/ServiceStats/StatsSummary 领域对象 + UseCase
│
└── billing.go                # BillingUseCase（组合 UseCase，协调各领域）
```

#### 详细说明

**1. `biz.go` - ProviderSet**
```go
// ProviderSet is biz providers.
var ProviderSet = wire.NewSet(
    NewBillingConfig,
    NewUserBalanceUseCase,
    NewFreeQuotaUseCase,
    NewBillingRecordUseCase,
    NewRechargeOrderUseCase,
    NewStatsUseCase,
    NewBillingUseCase, // 组合 UseCase
)
```

**2. `config.go` - 配置**
```go
// BillingConfig 计费配置
type BillingConfig struct {
    Prices     map[string]float64
    FreeQuotas map[string]int32
}

// NewBillingConfig 从配置创建 BillingConfig
func NewBillingConfig(c *conf.Bootstrap) *BillingConfig { ... }
```

**3. `repo.go` - 数据层接口（可选，如果保持统一接口）**
```go
// BillingRepo 定义数据层接口（统一接口，包含所有领域的方法）
// 注意：接口定义在 biz 层，实现在 data 层
type BillingRepo interface {
    // 余额相关
    GetUserBalance(ctx context.Context, userID string) (*UserBalance, error)
    Recharge(ctx context.Context, userID string, amount float64) error
    // ... 其他方法
}
```

**或者按领域拆分接口（推荐，更符合 DDD）**：
- 每个领域文件定义自己的 Repo 接口
- 例如：`user_balance.go` 中定义 `UserBalanceRepo interface`
- `data` 层分别实现这些接口

**4. `payment_client.go` - 支付服务客户端**
```go
// PaymentServiceClient payment-service 客户端接口
type PaymentServiceClient interface {
    CreatePayment(ctx context.Context, req *CreatePaymentRequest) (*CreatePaymentReply, error)
}

// CreatePaymentRequest 创建支付请求
type CreatePaymentRequest struct { ... }

// CreatePaymentReply 创建支付响应
type CreatePaymentReply struct { ... }
```

**5. `user_balance.go` - 余额领域**
```go
// UserBalance 账户余额领域对象
type UserBalance struct {
    UserID    string
    Balance   float64
    UpdatedAt time.Time
}

// UserBalanceRepo 余额数据层接口（定义在 biz 层）
type UserBalanceRepo interface {
    GetUserBalance(ctx context.Context, userID string) (*UserBalance, error)
    Recharge(ctx context.Context, userID string, amount float64) error
}

// UserBalanceUseCase 余额业务逻辑
type UserBalanceUseCase struct {
    repo UserBalanceRepo  // 使用领域特定的 Repo 接口
    log  *log.Helper
}

// NewUserBalanceUseCase 创建余额 UseCase
func NewUserBalanceUseCase(repo UserBalanceRepo, logger log.Logger) *UserBalanceUseCase { ... }

// GetBalance 获取余额
func (uc *UserBalanceUseCase) GetBalance(ctx context.Context, userID string) (*UserBalance, error) { ... }

// Recharge 充值
func (uc *UserBalanceUseCase) Recharge(ctx context.Context, userID string, amount float64) error { ... }
```

**对应的 data 层实现**：
```go
// internal/data/user_balance_repo.go

// userBalanceRepo 实现 biz.UserBalanceRepo 接口
type userBalanceRepo struct {
    data *Data
    log  *log.Helper
}

// NewUserBalanceRepo 创建余额 repo（返回 biz.UserBalanceRepo 接口）
func NewUserBalanceRepo(data *Data, logger log.Logger) biz.UserBalanceRepo {
    return &userBalanceRepo{...}
}

// GetUserBalance 实现接口方法
func (r *userBalanceRepo) GetUserBalance(ctx context.Context, userID string) (*biz.UserBalance, error) { ... }
```

**6. `free_quota.go` - 免费额度领域**
```go
// FreeQuota 免费额度领域对象
type FreeQuota struct {
    UserID      string
    ServiceName string
    TotalQuota  int
    UsedQuota   int
    ResetMonth  string
}

// FreeQuotaUseCase 免费额度业务逻辑
type FreeQuotaUseCase struct {
    repo BillingRepo
    conf *BillingConfig
    log  *log.Helper
}

// NewFreeQuotaUseCase 创建免费额度 UseCase
func NewFreeQuotaUseCase(repo BillingRepo, conf *BillingConfig, logger log.Logger) *FreeQuotaUseCase { ... }

// GetQuota 获取免费额度
func (uc *FreeQuotaUseCase) GetQuota(ctx context.Context, userID, serviceName, month string) (*FreeQuota, error) { ... }

// CreateQuota 创建免费额度
func (uc *FreeQuotaUseCase) CreateQuota(ctx context.Context, quota *FreeQuota) error { ... }

// UpdateQuota 更新免费额度
func (uc *FreeQuotaUseCase) UpdateQuota(ctx context.Context, quota *FreeQuota) error { ... }
```

**7. `billing_record.go` - 消费记录领域**
```go
// BillingRecord 消费记录领域对象
type BillingRecord struct {
    ID          string
    UserID      string
    ServiceName string
    Type        string // "free": 免费额度, "balance": 余额扣费
    Amount      float64
    Count       int
    CreatedAt   time.Time
}

// BillingRecordUseCase 消费记录业务逻辑
type BillingRecordUseCase struct {
    repo BillingRepo
    log  *log.Helper
}

// NewBillingRecordUseCase 创建消费记录 UseCase
func NewBillingRecordUseCase(repo BillingRepo, logger log.Logger) *BillingRecordUseCase { ... }

// CreateRecord 创建消费记录
func (uc *BillingRecordUseCase) CreateRecord(ctx context.Context, record *BillingRecord) error { ... }

// ListRecords 获取消费记录列表
func (uc *BillingRecordUseCase) ListRecords(ctx context.Context, userID string, page, pageSize int) ([]*BillingRecord, int64, error) { ... }
```

**8. `recharge_order.go` - 充值订单领域**
```go
// RechargeOrder 充值订单领域对象
type RechargeOrder struct {
    OrderID        string
    UserID         string
    Amount         float64
    PaymentOrderID string
    Status         string
    CreatedAt      time.Time
    UpdatedAt      time.Time
}

// RechargeOrderUseCase 充值订单业务逻辑
type RechargeOrderUseCase struct {
    repo                BillingRepo
    paymentServiceClient PaymentServiceClient
    conf                *BillingConfig
    log                 *log.Helper
    metrics             *metrics.BillingMetrics
}

// NewRechargeOrderUseCase 创建充值订单 UseCase
func NewRechargeOrderUseCase(...) *RechargeOrderUseCase { ... }

// CreateRecharge 创建充值订单
func (uc *RechargeOrderUseCase) CreateRecharge(ctx context.Context, userID string, amount float64, method int32, returnURL, notifyURL string) (string, string, error) { ... }

// RechargeCallback 充值回调
func (uc *RechargeOrderUseCase) RechargeCallback(ctx context.Context, orderID string, amount float64) error { ... }
```

**9. `stats.go` - 统计领域**
```go
// Stats 统计对象
type Stats struct {
    UserID      string
    ServiceName string
    TotalCount  int
    TotalCost   float64
    FreeCount   int
    PaidCount   int
    Period      string
}

// ServiceStats 服务统计对象
type ServiceStats struct { ... }

// StatsSummary 汇总统计对象
type StatsSummary struct { ... }

// StatsUseCase 统计业务逻辑
type StatsUseCase struct {
    repo BillingRepo
    log  *log.Helper
}

// NewStatsUseCase 创建统计 UseCase
func NewStatsUseCase(repo BillingRepo, logger log.Logger) *StatsUseCase { ... }

// GetStatsToday 获取今日统计
func (uc *StatsUseCase) GetStatsToday(ctx context.Context, userID, serviceName string) (*Stats, error) { ... }

// GetStatsMonth 获取本月统计
func (uc *StatsUseCase) GetStatsMonth(ctx context.Context, userID, serviceName string) (*Stats, error) { ... }

// GetStatsSummary 获取汇总统计
func (uc *StatsUseCase) GetStatsSummary(ctx context.Context, userID string) (*StatsSummary, error) { ... }
```

**10. `billing.go` - 组合 UseCase（协调层）**
```go
// BillingUseCase 计费业务逻辑（组合 UseCase）
// 负责协调各个领域 UseCase，处理跨领域的业务逻辑
type BillingUseCase struct {
    userBalanceUseCase   *UserBalanceUseCase
    freeQuotaUseCase     *FreeQuotaUseCase
    billingRecordUseCase *BillingRecordUseCase
    rechargeOrderUseCase *RechargeOrderUseCase
    statsUseCase         *StatsUseCase
    
    repo                BillingRepo
    conf                *BillingConfig
    log                 *log.Helper
    metrics             *metrics.BillingMetrics
}

// NewBillingUseCase 创建计费 UseCase
func NewBillingUseCase(
    userBalanceUseCase *UserBalanceUseCase,
    freeQuotaUseCase *FreeQuotaUseCase,
    billingRecordUseCase *BillingRecordUseCase,
    rechargeOrderUseCase *RechargeOrderUseCase,
    statsUseCase *StatsUseCase,
    repo BillingRepo,
    conf *BillingConfig,
    logger log.Logger,
) *BillingUseCase { ... }

// GetAccount 获取账户信息（组合多个领域）
func (uc *BillingUseCase) GetAccount(ctx context.Context, userID string) (*UserBalance, []*FreeQuota, error) {
    balance, err := uc.userBalanceUseCase.GetBalance(ctx, userID)
    // ... 组合逻辑
}

// CheckQuota 检查配额（跨领域逻辑）
func (uc *BillingUseCase) CheckQuota(ctx context.Context, userID, serviceName string, count int) (bool, string, error) {
    // 1. 检查免费额度
    quota, err := uc.freeQuotaUseCase.GetQuota(ctx, userID, serviceName, month)
    // 2. 检查余额
    balance, err := uc.userBalanceUseCase.GetBalance(ctx, userID)
    // ... 组合逻辑
}

// DeductQuota 扣减配额（跨领域事务）
func (uc *BillingUseCase) DeductQuota(ctx context.Context, userID, serviceName string, count int) (string, error) {
    // 调用 repo 的 DeductQuota（事务操作）
    return uc.repo.DeductQuota(ctx, userID, serviceName, count, cost, month)
}

// ResetFreeQuotas 重置免费额度（定时任务）
func (uc *BillingUseCase) ResetFreeQuotas(ctx context.Context) (int, []string, error) {
    // 调用 freeQuotaUseCase 和 repo
}
```

---

## 📊 方案对比

### 方案一：按领域对象拆分（推荐）✅

**优点**：
- ✅ 符合 DDD 设计原则（按聚合根拆分）
- ✅ 符合 Kratos 最佳实践（参考 marketing-service）
- ✅ 职责清晰，每个文件只负责一个领域
- ✅ 易于维护和扩展
- ✅ 支持独立测试

**缺点**：
- ⚠️ 文件数量增加（从 1 个到 10 个）
- ⚠️ 需要协调层（BillingUseCase）组合各领域 UseCase

**适用场景**：
- 领域边界清晰
- 需要独立扩展和维护各个领域
- 符合 DDD 设计原则

---

### 方案二：按功能模块拆分

**文件结构**：
```
internal/biz/
├── biz.go              # ProviderSet
├── domain.go           # 所有领域对象
├── repo.go             # BillingRepo 接口
├── payment_client.go   # PaymentServiceClient
├── config.go           # BillingConfig
├── account.go          # GetAccount, CheckQuota
├── quota.go            # DeductQuota, ResetFreeQuotas
├── recharge.go         # Recharge, RechargeCallback
├── record.go           # ListRecords
└── stats.go            # GetStatsToday, GetStatsMonth, GetStatsSummary
```

**优点**：
- ✅ 按功能拆分，逻辑清晰
- ✅ 文件数量适中

**缺点**：
- ❌ 不符合 DDD 原则（领域对象混杂）
- ❌ 领域边界不清晰

---

## 🎯 推荐方案

**推荐使用方案一：按领域对象拆分 + 按领域拆分 Repo 接口**

### 理由
1. **符合 DDD 设计原则**：每个领域对象独立管理，聚合根清晰
2. **符合 Kratos 最佳实践**：
   - ✅ Biz 层定义接口，Data 层实现接口（依赖倒置原则）
   - ✅ 参考 `marketing-service` 的组织方式（每个领域有自己的 Repo 接口）
3. **符合接口隔离原则**：每个领域只依赖自己需要的 Repo 接口
4. **职责单一**：每个文件只负责一个领域，符合单一职责原则
5. **易于扩展**：新增领域时只需添加新文件
6. **便于测试**：每个 UseCase 可以独立测试，可以轻松 Mock Repo 接口

### 实施步骤

#### 阶段一：创建基础设施文件
1. `biz.go` - ProviderSet（依赖注入配置）
2. `config.go` - BillingConfig 配置
3. `payment_client.go` - PaymentServiceClient 接口 + DTO

#### 阶段二：按领域拆分（Biz 层定义接口）
4. `user_balance.go` - UserBalance 领域对象 + `UserBalanceRepo` 接口 + `UserBalanceUseCase`
5. `free_quota.go` - FreeQuota 领域对象 + `FreeQuotaRepo` 接口 + `FreeQuotaUseCase`
6. `billing_record.go` - BillingRecord 领域对象 + `BillingRecordRepo` 接口 + `BillingRecordUseCase`
7. `recharge_order.go` - RechargeOrder 领域对象 + `RechargeOrderRepo` 接口 + `RechargeOrderUseCase`
8. `stats.go` - Stats 领域对象 + `StatsRepo` 接口 + `StatsUseCase`

#### 阶段三：Data 层实现接口
9. 更新 `data/user_balance_repo.go` - 实现 `biz.UserBalanceRepo` 接口
10. 更新 `data/free_quota_repo.go` - 实现 `biz.FreeQuotaRepo` 接口
11. 更新 `data/billing_record_repo.go` - 实现 `biz.BillingRecordRepo` 接口
12. 更新 `data/recharge_order_repo.go` - 实现 `biz.RechargeOrderRepo` 接口
13. 更新 `data/stats_repo.go` - 实现 `biz.StatsRepo` 接口
14. 更新 `data/billing_repo.go` - 组合所有 Repo，实现统一的 `BillingRepo` 接口（用于跨领域事务）

#### 阶段四：组合 UseCase
15. `billing.go` - `BillingUseCase`（组合各领域 UseCase，协调跨领域逻辑）

#### 阶段五：更新依赖注入
16. 更新 `biz/biz.go` 的 `ProviderSet`
17. 更新 `data/data.go` 的 `ProviderSet`
18. 重新生成 Wire 代码
19. 更新 `service` 层的依赖注入

---

## 📝 注意事项

### 1. Kratos 依赖倒置原则（关键！）

**✅ 正确做法**：
- **Biz 层定义接口**：`biz/user_balance.go` 中定义 `UserBalanceRepo interface`
- **Data 层实现接口**：`data/user_balance_repo.go` 中实现 `biz.UserBalanceRepo`
- **返回接口类型**：`NewUserBalanceRepo` 返回 `biz.UserBalanceRepo` 接口

**❌ 错误做法**：
- 在 data 层定义接口
- 在 biz 层实现接口

### 2. 接口组织方式

**推荐：按领域拆分接口**
- 每个领域有自己的 Repo 接口（如 `UserBalanceRepo`, `FreeQuotaRepo`）
- 符合接口隔离原则
- 参考 `marketing-service` 的做法

**备选：统一接口**
- 保持 `BillingRepo` 一个接口（包含所有领域的方法）
- 适合领域边界不清晰的场景

### 3. 组合 UseCase 的职责

- `BillingUseCase` 作为协调层，处理跨领域的业务逻辑
- 简单操作直接委托给对应的 UseCase
- 复杂事务操作（如 `DeductQuota`）直接调用 `repo`（因为涉及多个表的事务）

### 4. 依赖注入

- 各领域 UseCase 可以独立注入
- `BillingUseCase` 依赖所有领域 UseCase
- Data 层返回接口类型，便于 Mock 测试

### 5. 向后兼容

- `service` 层仍然使用 `BillingUseCase`
- 内部实现改为组合模式
- 对外接口保持不变

---

## 🔑 关键原则（Kratos 最佳实践）

### 依赖倒置原则
- ✅ **Biz 层定义接口**：业务逻辑层定义它需要的 Repo 接口
- ✅ **Data 层实现接口**：数据访问层实现这些接口
- ✅ **接口在 biz 层，实现在 data 层**

### 两种接口组织方式

#### 方式一：统一接口（当前方式）
- `biz/repo.go` 定义 `BillingRepo interface`（包含所有领域的方法）
- `data/billing_repo.go` 实现 `billingRepo struct`，实现所有方法
- **优点**：接口统一，便于管理
- **缺点**：接口过大，不符合接口隔离原则

#### 方式二：按领域拆分接口（推荐）✅
- `biz/user_balance.go` 定义 `UserBalanceRepo interface`
- `biz/free_quota.go` 定义 `FreeQuotaRepo interface`
- `data/user_balance_repo.go` 实现 `biz.UserBalanceRepo`
- `data/free_quota_repo.go` 实现 `biz.FreeQuotaRepo`
- **优点**：符合接口隔离原则，职责清晰
- **缺点**：接口数量增加

**推荐使用方式二**，参考 `marketing-service` 的做法。

---

## ❓ 讨论点

1. **Repo 接口的组织方式**？
   - 选项 A：统一接口 `BillingRepo`（当前方式）
   - 选项 B：按领域拆分接口（`UserBalanceRepo`, `FreeQuotaRepo` 等）✅ 推荐

2. **是否需要拆分 UseCase**？
   - 方案一：拆分多个 UseCase + 组合 UseCase ✅ 推荐
   - 方案二：保持单一 UseCase，只拆分文件

3. **`DeductQuota` 的归属**？
   - 选项 A：放在 `BillingUseCase`（跨领域事务）✅ 推荐
   - 选项 B：放在 `FreeQuotaUseCase`（主要涉及免费额度）
   - 选项 C：放在独立的 `QuotaDeductUseCase`

4. **统计对象的组织**？
   - 选项 A：所有统计对象放在 `stats.go` ✅ 推荐
   - 选项 B：按统计类型拆分（`stats_today.go`、`stats_month.go`）

请提供您的意见，我们讨论后确定最终方案！

