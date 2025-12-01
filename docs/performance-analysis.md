# DeductQuota 高频调用性能分析报告

**分析日期**：2024-12-01  
**分析目标**：评估 `DeductQuota` 在高频调用场景下的处理能力  
**场景**：Gateway 每次 API 调用都会调用 `DeductQuota`，属于高频接口

## 1. 当前实现分析

### 1.1 调用链路

```
Gateway (apisix-devshare-plugin-runner)
  ↓ gRPC 调用（单连接）
billing-service/internal/service/billing.go::DeductQuota
  ↓
billing-service/internal/biz/billing.go::DeductQuota
  ↓
billing-service/internal/data/billing.go::DeductQuota (事务)
  ↓
MySQL 数据库（事务 + 行锁）
```

### 1.2 数据库操作（每次调用）

**DeductQuota 事务中的操作**：

1. **查询 free_quota**（带行锁）
   ```sql
   SELECT * FROM free_quota 
   WHERE user_id = ? AND service_name = ? AND reset_month = ?
   FOR UPDATE
   ```

2. **更新 free_quota**（如果使用免费额度）
   ```sql
   UPDATE free_quota 
   SET used_quota = used_quota + ?
   WHERE user_id = ? AND service_name = ? AND reset_month = ?
   ```

3. **查询 user_balance**（带行锁，如果需要扣余额）
   ```sql
   SELECT * FROM user_balance 
   WHERE user_id = ?
   FOR UPDATE
   ```

4. **更新 user_balance**（如果需要扣余额）
   ```sql
   UPDATE user_balance 
   SET balance = balance - ?
   WHERE user_id = ?
   ```

5. **插入 billing_record**（1-2 条记录）
   ```sql
   INSERT INTO billing_record (...) VALUES (...)
   ```

**总计**：每次调用至少 **3-5 次数据库操作**（查询 + 更新 + 插入）

### 1.3 当前优化措施 ✅

1. **事务保证一致性**：使用数据库事务确保原子性
2. **行锁防止并发问题**：使用 `SELECT ... FOR UPDATE` 防止并发扣费
3. **Redis 缓存读取**：`GetUserBalance` 和 `GetFreeQuota` 有缓存（但 DeductQuota 中未使用）
4. **gRPC 连接复用**：使用单例模式，连接复用

### 1.4 性能瓶颈 ⚠️

#### 1.4.1 数据库连接池未配置

**问题**：
- GORM 默认连接池配置可能不适合高频场景
- 默认 `MaxOpenConns = 0`（无限制，但可能不够）
- 默认 `MaxIdleConns = 2`（空闲连接太少）

**影响**：
- 高并发时可能创建过多连接
- 连接复用效率低

**建议**：
```go
// 在 data.go 中配置连接池
sqlDB, _ := db.DB()
sqlDB.SetMaxOpenConns(100)  // 最大打开连接数
sqlDB.SetMaxIdleConns(20)   // 最大空闲连接数
sqlDB.SetConnMaxLifetime(time.Hour) // 连接最大生存时间
```

#### 1.4.2 每次调用都是独立事务

**问题**：
- 每次调用都开启新事务
- 事务开销：BEGIN + COMMIT/ROLLBACK
- 行锁持有时间长（整个事务期间）

**影响**：
- 事务开销累积
- 锁竞争严重（同一用户的并发请求会排队）

**建议**：
- 考虑批量处理（如果 Gateway 支持）
- 优化事务范围（减少锁持有时间）

#### 1.4.3 数据库索引检查

**当前索引**：
- ✅ `user_balance`: `uk_user_id` (唯一索引)
- ✅ `free_quota`: `uk_user_service_month` (唯一索引)
- ✅ `billing_record`: `idx_user_date` (普通索引)

**评估**：索引设计合理 ✅

#### 1.4.4 缓存未充分利用

**问题**：
- `DeductQuota` 中**没有使用缓存**
- 每次都直接查询数据库
- 缓存更新是异步的（事务提交后）

**影响**：
- 数据库压力大
- 响应时间可能较长

**建议**：
- 考虑使用 Redis 做配额缓存（但需要处理一致性问题）
- 或者使用 Redis 做分布式锁（减少数据库锁竞争）

#### 1.4.5 gRPC 连接池未配置

**问题**：
- 使用单连接（单例模式）
- 没有连接池配置
- 没有超时配置（使用默认值）

**影响**：
- 高并发时可能成为瓶颈
- 连接复用效率低

**建议**：
```go
// 在 billing_client.go 中配置连接池
conn, err := grpc.NewClient(
    address,
    grpc.WithTransportCredentials(insecure.NewCredentials()),
    grpc.WithDefaultCallOptions(
        grpc.MaxCallRecvMsgSize(4*1024*1024), // 4MB
        grpc.MaxCallSendMsgSize(4*1024*1024), // 4MB
    ),
    grpc.WithKeepaliveParams(keepalive.ClientParameters{
        Time:                10 * time.Second,
        Timeout:             3 * time.Second,
        PermitWithoutStream: true,
    }),
)
```

## 2. 性能评估

### 2.1 理论性能上限

**假设**：
- 数据库单次操作平均耗时：1ms
- 每次调用 4 次数据库操作
- 事务开销：0.5ms
- 网络延迟：1ms

**单次调用耗时**：约 **5.5ms**

**理论 QPS**：
- 单线程：~180 QPS
- 10 个并发：~1,800 QPS
- 100 个并发：~18,000 QPS（但受数据库连接数限制）

### 2.2 实际性能瓶颈

**主要瓶颈**：
1. **数据库连接数**：默认配置可能不够
2. **行锁竞争**：同一用户的并发请求会排队
3. **事务开销**：每次调用都开启事务
4. **网络延迟**：gRPC 调用延迟

**预估实际 QPS**：
- **小规模**（<100 并发）：~500-1,000 QPS
- **中规模**（100-500 并发）：~1,000-3,000 QPS（需要优化）
- **大规模**（>500 并发）：需要进一步优化

### 2.3 风险评估

| 风险项 | 风险等级 | 影响 | 建议 |
|--------|---------|------|------|
| 数据库连接池未配置 | ⚠️ 中 | 高并发时可能连接不足 | 立即配置 |
| 行锁竞争 | ⚠️ 中 | 同一用户并发请求排队 | 考虑优化锁策略 |
| 事务开销 | ⚠️ 低 | 累积开销 | 可接受 |
| 缓存未使用 | ⚠️ 中 | 数据库压力大 | 考虑引入缓存 |
| gRPC 连接池 | ⚠️ 低 | 单连接可能成为瓶颈 | 配置连接池 |

## 3. 优化建议

### 3.1 立即优化（高优先级）✅

#### 3.1.1 配置数据库连接池

```go
// 在 billing-service/internal/data/data.go 中
func NewData(c *conf.Data, logger log.Logger) (*Data, func(), error) {
    // ... 现有代码 ...
    
    // 配置连接池
    sqlDB, err := db.DB()
    if err != nil {
        return nil, nil, err
    }
    
    // 根据实际负载调整
    sqlDB.SetMaxOpenConns(100)        // 最大打开连接数
    sqlDB.SetMaxIdleConns(20)         // 最大空闲连接数
    sqlDB.SetConnMaxLifetime(time.Hour) // 连接最大生存时间
    sqlDB.SetConnMaxIdleTime(10 * time.Minute) // 空闲连接最大生存时间
    
    // ... 其他代码 ...
}
```

#### 3.1.2 配置 gRPC 连接参数

```go
// 在 apisix-devshare-plugin-runner/pkg/client/billing_client.go 中
import (
    "google.golang.org/grpc/keepalive"
)

func InitBillingClient(address string) error {
    var err error
    billingClientOnce.Do(func() {
        conn, connErr := grpc.NewClient(
            address,
            grpc.WithTransportCredentials(insecure.NewCredentials()),
            grpc.WithKeepaliveParams(keepalive.ClientParameters{
                Time:                10 * time.Second,
                Timeout:             3 * time.Second,
                PermitWithoutStream: true,
            }),
        )
        // ... 其他代码 ...
    })
    return err
}
```

### 3.2 中期优化（中优先级）⚠️

#### 3.2.1 引入 Redis 分布式锁

**目的**：减少数据库行锁竞争

**方案**：
- 使用 Redis 做分布式锁（按 user_id + service_name）
- 只有获取锁的请求才访问数据库
- 其他请求等待或快速失败

**实现**：
```go
// 在 DeductQuota 开始时获取锁
lockKey := fmt.Sprintf("deduct:lock:%s:%s:%s", userID, serviceName, month)
lock := r.data.rdb.NewMutex(lockKey, 100*time.Millisecond)
if err := lock.Lock(); err != nil {
    return "", fmt.Errorf("failed to acquire lock: %w", err)
}
defer lock.Unlock()

// 然后执行数据库操作
```

#### 3.2.2 批量处理（如果 Gateway 支持）

**目的**：减少事务开销

**方案**：
- Gateway 收集一段时间内的扣费请求
- 批量调用 billing-service
- billing-service 批量处理

**注意**：需要 Gateway 支持批量接口

### 3.3 长期优化（低优先级）📋

#### 3.3.1 异步扣费

**目的**：减少响应时间

**方案**：
- Gateway 先放行请求
- 扣费操作异步执行
- 使用消息队列（如 RabbitMQ、Kafka）

**风险**：
- 可能超扣（需要补偿机制）
- 复杂度增加

#### 3.3.2 配额预扣

**目的**：减少数据库操作

**方案**：
- 在 Redis 中维护配额缓存
- 先扣 Redis，再异步同步到数据库
- 定期批量写入数据库

**风险**：
- 数据一致性需要保证
- 需要处理 Redis 故障

## 4. 性能测试建议

### 4.1 测试场景

1. **单用户高频调用**
   - 同一用户并发 100 请求
   - 观察锁竞争情况

2. **多用户并发调用**
   - 1000 用户，每用户 10 请求
   - 观察整体 QPS

3. **混合场景**
   - 免费额度充足用户
   - 免费额度不足用户
   - 余额不足用户

### 4.2 监控指标

- **QPS**：每秒请求数
- **P50/P95/P99 延迟**：响应时间分布
- **数据库连接数**：当前使用连接数
- **锁等待时间**：行锁等待时间
- **错误率**：失败请求比例

## 5. 总结

### 5.1 当前状态

**优点**：
- ✅ 事务保证一致性
- ✅ 行锁防止并发问题
- ✅ 索引设计合理

**问题**：
- ⚠️ 数据库连接池未配置
- ⚠️ gRPC 连接池未配置
- ⚠️ 缓存未充分利用
- ⚠️ 每次调用都是独立事务

### 5.2 性能预估

**当前配置**：
- 预估 QPS：**500-1,000**（小规模）
- 预估 QPS：**1,000-3,000**（中规模，需优化）

**优化后**：
- 预估 QPS：**2,000-5,000**（中规模）
- 预估 QPS：**5,000-10,000**（大规模，需进一步优化）

### 5.3 建议优先级

1. **立即**：配置数据库连接池 ✅
2. **立即**：配置 gRPC 连接参数 ✅
3. **本周**：引入 Redis 分布式锁 ⚠️
4. **本月**：性能测试和监控 📋
5. **长期**：考虑异步扣费或配额预扣 📋

---

**结论**：当前实现**基本满足中小规模场景**，但需要**立即配置连接池**以支持更高并发。大规模场景需要进一步优化。
