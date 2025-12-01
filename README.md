# Billing Service

DevShare 平台的财务中心，负责管理开发者的钱包、配额与账单。

## 功能特性

- **资产管理**：支持免费额度和余额钱包的混合支付模式
- **配额管理**：自动管理每月免费额度，支持自动重置
- **扣费逻辑**：优先扣除免费额度，不足时扣除余额，支持混合扣费
- **账单记录**：记录每一笔 API 调用的扣费情况
- **性能优化**：使用 Redis 缓存优化配额检查和余额查询

## 技术栈

- **框架**：Kratos v2
- **数据库**：MySQL (GORM)
- **缓存**：Redis
- **协议**：gRPC + HTTP

## 快速开始

### 1. 初始化数据库

执行 SQL 脚本创建数据库表：

```bash
mysql -u root -p < docs/sql/billing_service.sql
```

### 2. 生成 Proto 代码

```bash
make init
```

### 3. 配置服务

修改 `configs/config.yaml` 中的数据库和 Redis 连接信息。

### 4. 运行服务

```bash
make run
```

或者：

```bash
go run ./cmd/billing-service/main.go -conf ./configs/config.yaml
```

## 项目结构

```
billing-service/
├── api/                    # API 定义 (proto 文件)
│   └── billing/v1/
├── cmd/                    # 入口文件
│   ├── server/            # 主服务入口
│   └── cron/              # Cron 定时任务服务入口
├── configs/                # 配置文件
├── docs/                   # 设计文档
├── internal/               # 内部代码
│   ├── biz/               # 业务逻辑层
│   ├── data/              # 数据访问层
│   ├── service/           # 服务层 (gRPC 实现)
│   ├── server/            # 服务器配置
│   └── conf/              # 配置结构
└── README.md
```

## API 接口

### 管理接口 (面向前端/开发者)

- `GET /api/v1/billing/account` - 获取账户资产信息
- `POST /api/v1/billing/recharge` - 发起充值
- `GET /api/v1/billing/records` - 获取消费流水

### 内部接口 (面向 Gateway/Payment)

- `CheckQuota` - 检查配额
- `DeductQuota` - 扣减配额
- `RechargeCallback` - 充值回调

## 设计文档

详细的设计文档请参考 `docs/` 目录：

- `product-demand.md` - 产品需求文档
- `service-spec.md` - 技术设计文档
- `logic_design.md` - 业务逻辑设计
- `sql/billing_service.sql` - 数据库设计

## Cron 定时任务服务

Billing Service 包含一个独立的 Cron 服务，用于执行定时任务。

### 定时任务

| 任务名称 | Cron 表达式 | 执行时间 | 功能描述 |
|---------|------------|---------|---------|
| 免费额度重置 | `0 0 0 1 * *` | 每月1日 00:00 | 为所有用户创建下个月的免费额度记录 |

### Cron 服务启动

```bash
# 编译 Cron 服务
make build-cron

# 启动 Cron 服务
make run-cron

# 或者直接运行
./bin/cron -conf ./configs/config.yaml
```

### 同时运行主服务和 Cron 服务

```bash
# 启动所有服务（cron 后台，server 前台）
make run-all

# 停止所有服务
make stop-all
```

### 日志

Cron 服务的日志会输出到标准输出，如果使用 `make run-all`，日志会保存到 `logs/cron.log`。

## License

MIT
