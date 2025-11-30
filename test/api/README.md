# Billing Service API 测试

本文档说明如何运行 Billing Service 的 API 测试。

## 前置条件

1. **安装 api-tester**
   ```bash
   go install github.com/gaoyong06/api-tester/cmd/api-tester@latest
   ```

2. **启动 billing-service**
   ```bash
   # 在项目根目录
   make run
   # 或者
   go run ./cmd/billing-service/main.go -conf ./configs/config.yaml
   ```

3. **初始化数据库**
   ```bash
   mysql -u root -p < docs/sql/billing_service.sql
   ```

## 运行测试

### 方式一：使用 Makefile（推荐）

```bash
# 在项目根目录运行
make test
```

### 方式二：直接使用 api-tester

```bash
cd test/api
api-tester run --config api-test-config.yaml --verbose
```

### 方式三：使用测试脚本

```bash
cd test/api
./run-tests.sh
```

## 测试场景

测试配置包含以下场景：

1. **基础功能测试**
   - 获取账户信息（新用户/已有用户）

2. **充值流程测试**
   - 正常充值流程
   - 异常场景（无效用户ID、负数金额、支付失败等）

3. **配额检查测试**
   - 免费额度充足
   - 余额充足
   - 余额不足
   - 异常场景

4. **扣费逻辑测试**
   - 使用免费额度扣费
   - 使用余额扣费
   - 混合扣费（免费额度+余额）
   - 余额不足
   - 异常场景

5. **消费流水查询测试**
   - 正常查询
   - 边界场景（空用户、负数页码、超大页码等）

6. **完整业务流程测试**
   - 从充值到扣费的完整流程

7. **并发测试**
   - 并发扣费场景

8. **数据一致性测试**
   - 扣费前后数据一致性验证

## 测试报告

测试完成后，报告会保存在 `./test-reports` 目录中。

## 配置说明

测试配置文件：`test/api/api-test-config.yaml`

主要配置项：
- `base_url`: API 基础 URL（默认：http://localhost:8107）
- `timeout`: 请求超时时间（秒）
- `variables`: 全局变量定义
- `scenarios`: 测试场景列表

## 注意事项

1. 确保服务已启动并运行在 `http://localhost:8107`
2. 确保数据库已初始化
3. 测试会创建测试数据，建议使用测试数据库
4. 某些测试场景有依赖关系，需要按顺序执行

## 故障排查

### 问题：api-tester 未找到

```bash
# 安装 api-tester
go install github.com/gaoyong06/api-tester/cmd/api-tester@latest

# 验证安装
which api-tester
```

### 问题：无法连接到服务

1. 检查服务是否运行：
   ```bash
   curl http://localhost:8107/api/v1/billing/account?user_id=test
   ```

2. 检查配置文件中的 `base_url` 是否正确

### 问题：数据库连接失败

1. 检查数据库是否运行
2. 检查 `configs/config.yaml` 中的数据库配置
3. 确保已执行数据库初始化脚本

