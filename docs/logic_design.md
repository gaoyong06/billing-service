# Billing Service 业务逻辑设计

本文档通过 UML 图表展示 `billing-service` 的核心业务逻辑和流程。

## 1. 核心领域模型 (Domain Model)

展示核心实体 `UserBalance`, `FreeQuota`, `BillingRecord` 及其关系。

```mermaid
classDiagram
    class UserBalance {
        +String UserID
        +Decimal Balance
        +Int Version (乐观锁)
    }

    class FreeQuota {
        +String UserID
        +String ServiceName
        +Int TotalQuota
        +Int UsedQuota
        +String ResetMonth
    }

    class BillingRecord {
        +String RecordID
        +String UserID
        +String ServiceName
        +Type Type (Free/Balance)
        +Decimal Amount
        +Int Count
        +Time CreatedAt
    }

    UserBalance "1" -- "N" BillingRecord : 产生
    FreeQuota "1" -- "N" BillingRecord : 产生
```

**设计说明**：
- **混合支付**：用户同时拥有免费额度（FreeQuota）和余额（UserBalance）。
- **流水记录**：每次扣费（无论是扣额度还是扣余额）都会生成 BillingRecord。

## 2. 配额检查与扣减流程 (Check & Deduct)

展示业务服务调用本服务进行扣费的核心逻辑（优先抵扣免费额度）。

```mermaid
sequenceDiagram
    participant BizSvc as 业务服务 (Passport/Asset)
    participant Billing as Billing Service (Internal)
    participant DB as Database

    BizSvc->>Billing: 1. 请求扣费 (DeductQuota)
    Note right of BizSvc: UserID, Service, Count, Cost
    
    Billing->>DB: Begin Transaction
    
    Billing->>DB: SELECT * FROM free_quota FOR UPDATE
    
    alt 免费额度充足
        Billing->>DB: UPDATE free_quota SET used = used + count
        Billing->>DB: INSERT billing_record (Type=FREE)
    else 免费额度不足/无额度
        Billing->>DB: SELECT * FROM user_balance
        
        alt 余额充足
            Billing->>DB: UPDATE user_balance SET balance = balance - cost
            Billing->>DB: INSERT billing_record (Type=BALANCE)
        else 余额不足
            Billing->>DB: Rollback
            Billing-->>BizSvc: Error (Insufficient Balance)
        end
    end
    
    Billing->>DB: Commit
    Billing-->>BizSvc: Success
```

## 3. 充值流程 (Recharge)

展示用户充值的全流程，包括与 Payment Service 的交互。

```mermaid
sequenceDiagram
    participant User as 开发者
    participant Billing as Billing Service
    participant Payment as Payment Service
    participant DB as Database

    User->>Billing: 1. 发起充值 (Recharge)
    Billing->>Payment: 创建支付订单 (CreateOrder)
    Payment-->>Billing: PayURL
    Billing-->>User: PayURL

    User->>Payment: 2. 完成支付
    Payment->>Billing: 3. 支付回调 (Callback)
    
    Billing->>DB: Begin Transaction
    Billing->>DB: SELECT * FROM user_balance FOR UPDATE
    Billing->>DB: UPDATE user_balance SET balance = balance + amount
    Billing->>DB: Commit
    
    Billing-->>Payment: Success
```

## 4. 免费额度重置流程 (Quota Reset)

展示每月 1 日自动重置免费额度的逻辑。

```mermaid
sequenceDiagram
    participant Cron as Cron 服务
    participant Billing as Billing Service
    participant DB as Database

    Cron->>Billing: 1. 触发重置 (ResetFreeQuotas)
    Note right of Cron: 每月 1 日 00:00<br/>Cron: "0 0 0 1 * *"
    
    Billing->>DB: SELECT DISTINCT user_id FROM free_quota
    Billing->>DB: SELECT DISTINCT user_id FROM user_balance
    Note right of Billing: 合并两个表的用户ID<br/>确保所有用户都能获得免费额度
    
    Billing->>Billing: 计算下个月 (NextMonth)
    Note right of Billing: 格式: 2024-12
    
    loop 遍历每个用户
        loop 遍历每个服务 (passport/payment/asset)
            Billing->>DB: SELECT * FROM free_quota<br/>WHERE user_id=? AND service=? AND reset_month=?
            
            alt 记录已存在
                Note right of Billing: 跳过（幂等性保证）
            else 记录不存在
                Billing->>DB: INSERT INTO free_quota<br/>(user_id, service_name, total_quota, used_quota=0, reset_month)
                Note right of Billing: 创建下个月的新记录
            end
        end
    end
    
    Billing-->>Cron: 返回统计信息 (count, userIDs)
    Note right of Cron: 记录日志：成功数量、用户列表
```

**设计说明**：
- **执行时间**：每月 1 日 00:00 自动执行（Cron 表达式：`0 0 0 1 * *`）
- **用户获取策略**：从 `free_quota` 和 `user_balance` 两个表获取所有用户ID，确保：
  - 已有免费额度记录的用户能获得下个月的额度
  - 只有余额但还没有免费额度记录的新用户也能获得免费额度
- **幂等性保证**：如果下个月的记录已存在，自动跳过，避免重复创建
- **错误处理**：单个用户或服务的失败不影响其他用户，记录警告日志后继续处理
- **性能考虑**：使用批量查询和事务，支持大量用户的场景
