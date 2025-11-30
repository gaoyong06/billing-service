# Billing Service Makefile
# 使用 devops-tools 的通用 Makefile

SERVICE_NAME=billing-service
API_PROTO_PATH=api/billing/v1/billing.proto
API_PROTO_DIR=api/billing/v1

# 服务特定配置（在引入通用 Makefile 之前设置）
WIRE_DIRS ?= cmd/billing-service
BUILD_OUTPUT ?= ./bin/billing-service
RUN_MAIN ?= cmd/billing-service/main.go cmd/billing-service/wire_gen.go
CONFIG_FILE ?= configs/config.yaml

# 测试数据库配置
TEST_DB_HOST ?= 127.0.0.1
TEST_DB_PORT ?= 3306
TEST_DB_USER ?= root
TEST_DB_NAME ?= billing_service

MYSQL_CMD = mysql -h $(TEST_DB_HOST) -P $(TEST_DB_PORT) -u $(TEST_DB_USER) -D $(TEST_DB_NAME)
ifneq ($(TEST_DB_PASSWORD),)
MYSQL_CMD += -p$(TEST_DB_PASSWORD)
endif

# 服务特定配置
SERVICE_DISPLAY_NAME=Billing Service
HTTP_PORT=8107
TEST_CONFIG=test/api/api-test-config.yaml

# 引入通用 Makefile（如果存在）
DEVOPS_TOOLS_DIR := $(shell cd .. && pwd)/devops-tools
ifneq ($(wildcard $(DEVOPS_TOOLS_DIR)/Makefile.common),)
include $(DEVOPS_TOOLS_DIR)/Makefile.common
endif

# 服务特定的目标（如果需要覆盖通用 Makefile 的目标，在这里定义）

.PHONY: conf
# 生成 conf 包的代码
conf:
	@echo "Generating conf proto files..."
	@protoc --proto_path=./internal/conf \
		--proto_path=$(shell go env GOPATH)/pkg/mod \
		--proto_path=$(shell go env GOPATH)/pkg/mod/github.com/go-kratos/kratos/v2@v2.7.2/third_party \
		--go_out=paths=source_relative:./internal/conf \
		internal/conf/conf.proto 2>/dev/null || protoc \
		--proto_path=./internal/conf \
		--proto_path=$(shell go env GOPATH)/pkg/mod \
		--go_out=paths=source_relative:./internal/conf \
		internal/conf/conf.proto
	@echo "Conf proto files generated successfully!"

# 确保 wire 之前先生成 conf 和 api
wire: conf api

.PHONY: test
# 运行 API 测试（自动清理测试数据，覆盖通用版本）
test: test-clean-data test-run

.PHONY: test-run
test-run:
	@echo "========================================="
	@echo "  Testing $(SERVICE_DISPLAY_NAME)"
	@echo "========================================="
	@echo "检查服务状态..."
	@curl -s http://localhost:$(HTTP_PORT)/api/v1/billing/account?user_id=test > /dev/null 2>&1 || echo "$(SERVICE_DISPLAY_NAME) 启动中..."
	@echo "\nRunning API tests..."
	@which api-tester > /dev/null || (echo "Error: api-tester not installed. Run: go install github.com/gaoyong06/api-tester/cmd/api-tester@latest" && exit 1)
	@api-tester run --config $(TEST_CONFIG) --verbose
	@echo "\nAPI tests completed. Reports saved to ./test-reports"

.PHONY: test-clean-data
test-clean-data:
	@echo "清理测试数据..."
	@$(MYSQL_CMD) -e "DELETE FROM billing_record WHERE user_id LIKE 'test-user-%';" || { echo "WARNING: failed to connect test database, skip cleanup"; }
	@$(MYSQL_CMD) -e "DELETE FROM free_quota WHERE user_id LIKE 'test-user-%';" || { echo "WARNING: failed to connect test database, skip cleanup"; }
	@$(MYSQL_CMD) -e "DELETE FROM user_balance WHERE user_id LIKE 'test-user-%';" || { echo "WARNING: failed to connect test database, skip cleanup"; }

.PHONY: unit-test
# 运行单元测试
unit-test:
	@go test -v ./...

.PHONY: api-test
# 运行 API 测试（使用 api-tester）- 别名
api-test: test

.PHONY: test-all
# 运行所有测试（单元测试 + API 测试）
test-all: unit-test test


# 覆盖 help 目标
help:
	@echo "$(SERVICE_DISPLAY_NAME) Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make init         - 安装所需工具并生成 API 代码"
	@echo "  make api          - 生成 API 代码（如果使用 devops-tools）"
	@echo "  make build         - 编译项目"
	@echo "  make run          - 运行服务"
	@echo "  make test         - 运行 API 测试（自动清理测试数据）"
	@echo "  make test-run     - 仅运行 API 测试（不清理数据）"
	@echo "  make test-clean-data - 清理测试数据"
	@echo "  make unit-test    - 运行单元测试"
	@echo "  make api-test     - 运行 API 测试（别名）"
	@echo "  make test-all     - 运行所有测试（单元测试 + API 测试）"
	@echo "  make clean        - 清理生成的文件"
	@if [ -f "$(DEVOPS_TOOLS_DIR)/Makefile.common" ]; then \
		echo "  make docker-build - 构建 Docker 镜像"; \
		echo "  make docker-run   - 运行 Docker 容器（需要设置 DOCKER_PORTS）"; \
		echo "  make all          - 生成代码并构建"; \
	fi
