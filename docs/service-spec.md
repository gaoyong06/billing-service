# Billing Service 技术设计文档

## 1. 服务概述
**服务名称**：billing-service
**职责**：管理用户资产（余额/配额），执行扣费逻辑。
**依赖**：MySQL, Redis

## 2. 接口定义 (gRPC & HTTP)

### 2.1 管理接口 (面向前端/开发者)
```protobuf
service BillingService {
    // 获取账户资产信息 (余额 + 剩余配额)
    // GET /api/v1/billing/account
    rpc GetAccount(GetAccountRequest) returns (GetAccountReply);

    // 发起充值 (返回支付链接)
    // POST /api/v1/billing/recharge
    rpc Recharge(RechargeRequest) returns (RechargeReply);

    // 获取消费流水
    // GET /api/v1/billing/records
    rpc ListRecords(ListRecordsRequest) returns (ListRecordsReply);
}
```

### 2.2 内部接口 (面向 Gateway/Payment)
```protobuf
service BillingInternalService {
    // 检查并预扣费 (Check & Reserve)
    // POST /internal/v1/billing/check
    rpc CheckQuota(CheckQuotaRequest) returns (CheckQuotaReply);

    // 确认扣费 (Commit) - 异步或同步
    // POST /internal/v1/billing/deduct
    rpc DeductQuota(DeductQuotaRequest) returns (DeductQuotaReply);

    // 充值回调 (来自 Payment Service)
    // POST /internal/v1/billing/callback
    rpc RechargeCallback(RechargeCallbackRequest) returns (RechargeCallbackReply);
}
```

## 3. 数据库设计

### 3.1 表结构

#### `user_balance` (账户余额表)
```sql
CREATE TABLE user_balance (
    user_balance_id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL UNIQUE COMMENT '用户ID',
    balance DECIMAL(10, 2) DEFAULT 0.00 COMMENT '余额',
    version INT DEFAULT 0 COMMENT '乐观锁版本号',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
```

#### `free_quota` (免费额度表)
```sql
CREATE TABLE free_quota (
    free_quota_id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    service_name VARCHAR(32) NOT NULL COMMENT '服务名: passport/payment/asset',
    total_quota INT DEFAULT 0 COMMENT '总额度',
    used_quota INT DEFAULT 0 COMMENT '已用额度',
    reset_month VARCHAR(7) NOT NULL COMMENT '重置月份: 2024-11',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_user_service_month (user_id, service_name, reset_month)
);
```

#### `billing_record` (消费流水表)
```sql
CREATE TABLE billing_record (
    billing_record_id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    service_name VARCHAR(32) NOT NULL,
    type TINYINT NOT NULL COMMENT '1:免费额度, 2:余额扣费',
    amount DECIMAL(10, 4) DEFAULT 0 COMMENT '扣费金额',
    count INT DEFAULT 1 COMMENT '调用次数',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_user_date (user_id, created_at)
);
```

## 4. 关键逻辑

### 4.1 扣费逻辑 (DeductQuota)
1.  **检查免费额度**：查询 `free_quota`。
    *   如果有剩余 -> 更新 `used_quota` -> 记录流水(Type=1)。
2.  **检查余额**：如果免费额度不足。
    *   计算所需金额 -> 检查 `user_balance` 余额 -> 扣减余额 (乐观锁) -> 记录流水(Type=2)。
3.  **事务保证**：上述操作需在 DB 事务中完成。

### 4.2 性能优化 (Redis)
*   为了减少 DB 压力，Gateway 的 `CheckQuota` 应该优先查 Redis。
*   **Redis 结构**：
    *   `balance:{user_id}` -> float
    *   `quota:{user_id}:{service}` -> int (remaining)
*   **同步策略**：DB 更新后，同步更新/失效 Redis。

## 5. Cron 定时任务服务

### 5.1 服务架构

Billing Service 包含一个独立的 Cron 服务，用于执行定时任务。

**服务结构**：
```
billing-service/
├── cmd/
│   ├── server/          # 主服务（gRPC + HTTP）
│   └── cron/            # Cron 定时任务服务
│       ├── main.go      # Cron 服务入口
│       ├── wire.go      # 依赖注入配置
│       └── wire_gen.go  # 生成的依赖注入代码
```

### 5.2 定时任务列表

| 任务名称 | Cron 表达式 | 执行时间 | 功能描述 |
|---------|------------|---------|---------|
| 免费额度重置 | `0 0 0 1 * *` | 每月1日 00:00 | 为所有用户创建下个月的免费额度记录 |

**Cron 表达式说明**（支持秒级调度）：
- 格式：`秒 分 时 日 月 周`
- `0 0 0 1 * *` 表示：每月1日 00:00:00 执行

### 5.3 免费额度重置实现

#### 5.3.1 核心方法

**Biz 层**：`internal/biz/billing.go`
```go
// ResetFreeQuotas 重置所有用户的免费额度（每月1日执行）
// 为所有用户创建下个月的免费额度记录
func (uc *BillingUseCase) ResetFreeQuotas(ctx context.Context) (int, []string, error)
```

**Data 层**：`internal/data/billing.go`
```go
// GetAllUserIDs 获取所有用户ID（用于重置免费额度）
// 从 free_quota 和 user_balance 表中获取所有不重复的 user_id
func (r *billingRepo) GetAllUserIDs(ctx context.Context) ([]string, error)
```

#### 5.3.2 重置流程

1. **获取所有用户ID**：
   - 从 `free_quota` 表获取所有不重复的 `user_id`
   - 从 `user_balance` 表获取所有不重复的 `user_id`
   - 合并去重，确保所有用户都能获得免费额度

2. **计算下个月**：
   - 使用 `time.Now().AddDate(0, 1, 0).Format("2006-01")` 获取下个月（格式：`2024-12`）

3. **为每个用户创建免费额度**：
   - 遍历所有用户
   - 遍历所有服务（passport/payment/asset）
   - 检查是否已存在下个月的记录
   - 如果不存在，创建新记录（`used_quota = 0`）

4. **幂等性保证**：
   - 如果下个月的记录已存在，自动跳过
   - 支持重复执行，不会产生重复记录

5. **错误处理**：
   - 单个用户或服务的失败不影响其他用户
   - 记录警告日志，继续处理下一个用户

#### 5.3.3 日志输出

Cron 服务会输出详细的执行日志：
```
[CRON] Starting free quota reset...
[CRON] Reset free quotas completed: count=150, users=50
[CRON] Reset users: [user-001, user-002, ...]
[CRON] Finished free quota reset
```

### 5.4 Cron 服务部署

#### 5.4.1 编译和运行

```bash
# 编译 Cron 服务
make build-cron

# 运行 Cron 服务（前台）
make run-cron

# 同时运行主服务和 Cron 服务（cron 后台，server 前台）
make run-all

# 停止所有服务
make stop-all
```

#### 5.4.2 独立部署

Cron 服务可以独立部署，只需要：
- 数据库连接配置
- Redis 连接配置（可选，用于缓存）
- Billing 配置（价格和免费额度）

**配置文件**：`configs/config.yaml`
```yaml
data:
  database:
    driver: mysql
    source: root:@tcp(127.0.0.1:3306)/billing_service?charset=utf8mb4&parseTime=True&loc=Local
  redis:
    addr: 127.0.0.1:6379

billing:
  prices:
    passport: 0.01
    payment: 0.10
    asset: 0.05
  free_quotas:
    passport: 10000
    payment: 1000
    asset: 1000
```

#### 5.4.3 生产环境建议

1. **高可用**：可以部署多个 Cron 服务实例，但需要确保同一时间只有一个实例执行任务（使用分布式锁）
2. **监控**：监控 Cron 服务的执行状态和日志
3. **告警**：如果重置任务执行失败，发送告警通知
4. **日志**：将日志输出到文件或日志系统，便于排查问题

### 5.5 技术实现

- **Cron 库**：`github.com/robfig/cron/v3`（支持秒级调度）
- **依赖注入**：使用 Wire 进行依赖注入
- **优雅退出**：支持 SIGINT/SIGTERM 信号，优雅停止定时任务

## 6. 配置参数
```yaml
billing:
  prices:
    passport: 0.01
    payment: 0.10
    asset: 0.05
  free_quotas:
    passport: 10000
    payment: 1000
    asset: 1000
```
