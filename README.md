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
│   └── billing-service/
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

## License

MIT
