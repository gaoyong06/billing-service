# 免费应用白名单功能

## 功能说明

免费应用白名单功能允许您配置某些应用（`app_id`）的所有调用都不扣费。这通常用于官方开发的应用，如：
- 官方 Web 管理后台
- 官方移动应用
- 内部工具应用

## 配置方式

在 `billing-service/configs/config.yaml` 中配置 `free_app_ids`：

```yaml
billing:
  # ... 其他配置 ...
  
  # 免费应用白名单（app_id 列表）
  # 这些应用的所有调用都不扣费，用于官方应用等
  free_app_ids:
    - "00000000-0000-0000-0000-000000000001"  # 官方 Web 应用
    - "00000000-0000-0000-0000-000000000002"  # 官方移动应用
```

## 工作原理

1. **CheckQuota（配额检查）**：
   - 如果 `app_id` 在白名单中，直接返回 `allowed=true`，跳过配额检查
   - 返回原因：`"free_app"`

2. **DeductQuota（扣费）**：
   - 如果 `app_id` 在白名单中，**不扣费**，但会**按次记录用量**到免费额度表（`user_id + app_id + service + month`）
   - 免费应用的配额行总额度为「无限」（内部使用 `UnlimitedQuota` 常量），已用额度随调用递增，便于统计与前端展示
   - 生成一个特殊的 `record_id`（格式：`free_{timestamp}_{app_id}`）用于日志追踪
   - 记录指标：`DeductQuotaTotal` 标签为 `free`

3. **配额查询接口（已拆分为两个独立 API）**：
   - **GetAccountQuota**（开发者维度）：`GET /billing/v1/billing/account-quota?userId=xxx`，返回账户余额与汇总配额（app_id 为空）。
   - **GetAppQuota**（应用维度）：`GET /billing/v1/billing/app-quota?userId=xxx&appId=yyy`，返回指定应用的配额与用量；当该应用在白名单时，响应 `isFreeApp=true`，各配额 `isUnlimited=true`，前端可展示「已用 xxx / 无限制」及「免费应用」标识。
   - 在官方后台（如 atseeker.com）费用中心应调用 **GetAppQuota** 并传入当前应用 `appId`，即可看到正确的调用量与「无限制」展示。

## 行业最佳实践

### 1. 配置白名单（推荐，已实现）
- ✅ **优点**：
  - 简单直接，不需要修改数据库
  - 可以通过配置文件快速调整
  - 符合"配置优于代码"的原则
  - 支持热更新（重启服务即可生效）
- ❌ **缺点**：
  - 需要重启服务才能生效
  - 配置文件中维护大量 app_id 可能不够优雅

### 2. 数据库标记（备选方案）
在 `app` 表中增加 `is_official` 或 `is_free` 字段：
- ✅ **优点**：
  - 可以通过管理界面动态调整
  - 不需要重启服务
  - 可以记录更多元数据（如标记原因、标记时间等）
- ❌ **缺点**：
  - 需要数据库迁移
  - 需要修改 `api-key-service` 的数据库结构
  - 每次检查需要查询数据库（性能开销）

### 3. 特殊用户（备选方案）
给官方应用分配特殊的 `user_id`，然后对特定 `user_id` 不扣费：
- ✅ **优点**：
  - 实现简单
  - 不需要修改数据库结构
- ❌ **缺点**：
  - 不够灵活（一个用户可能有多个应用）
  - 逻辑不够清晰（用户级别 vs 应用级别）

## 推荐方案

**当前实现采用配置白名单方案**，原因：
1. 官方应用数量通常较少，配置文件中维护即可
2. 不需要修改数据库结构，降低复杂度
3. 符合"简单至上"的原则
4. 如果未来需要更灵活的管理，可以升级为数据库标记方案

## 使用示例

### 添加官方应用到白名单

1. 编辑 `billing-service/configs/config.yaml`：
```yaml
billing:
  free_app_ids:
    - "00000000-0000-0000-0000-000000000001"  # dev-share-web
```

2. 重启 `billing-service` 服务

3. 验证：调用该应用的 API，应该不会扣费

### 从白名单移除应用

1. 编辑配置文件，删除对应的 `app_id`
2. 重启服务

## 前端对接说明（Dashboard 展示）

- **开发者维度**（总览）：调用 `GET /billing/v1/billing/account-quota?userId=xxx`。
- **应用维度**（当前应用费用中心）：调用 `GET /billing/v1/billing/app-quota?userId=xxx&appId=yyy`，其中 `appId` 为当前登录应用（如从 getAppConfig().appId 获取）。
- **GetAppQuota 响应字段**：
  - `isFreeApp`：是否为免费应用（白名单），可展示「免费应用」标签
  - `quotas[].isUnlimited`：该服务是否为无限额度（免费应用下为 true）
  - `quotas[].usedQuota`：已使用调用量（免费应用也会正确累加）
  - `quotas[].totalQuota`：总配额；当 `isUnlimited=true` 时前端宜展示为「无限制」而非具体数字

## 注意事项

1. **白名单检查优先级**：
   - 白名单检查在 `CheckQuota` 和 `DeductQuota` 的最开始执行
   - 如果 `app_id` 在白名单中，直接跳过所有扣费逻辑

2. **日志追踪**：
   - 白名单应用的调用会生成特殊的 `record_id`（格式：`free_{timestamp}_{app_id}`）
   - 可以通过日志和指标追踪白名单应用的使用情况

3. **指标监控**：
   - `DeductQuotaTotal` 指标会记录白名单应用的调用次数（标签：`free`）
   - 可以用于监控官方应用的使用情况

4. **安全性**：
   - 确保配置文件的安全性，避免泄露官方应用的 `app_id`
   - 建议使用环境变量或配置中心管理敏感配置
