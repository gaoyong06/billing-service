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

## 5. 配置参数
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
