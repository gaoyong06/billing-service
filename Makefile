SERVICE_NAME := billing-service
SERVICE_DISPLAY_NAME := Billing Service
API_PROTO_PATH := api/billing/v1/billing.proto
API_PROTO_DIR := api/billing/v1
WIRE_DIRS := cmd/server cmd/cron
BUILD_OUTPUT := ./bin/server
RUN_MAIN := cmd/server/main.go cmd/server/wire_gen.go
CONFIG_FILE := configs/config.yaml
HTTP_PORT := 8107
GRPC_PORT := 9107
TEST_CONFIG := test/api/api-test-config.yaml

include ../devops-tools/Makefile.common

# 服务特定的目标

.PHONY: build-cron
# 构建 cron 服务
build-cron:
	mkdir -p bin/
	go build -ldflags "-X main.Version=$(VERSION)" -o ./bin/cron ./cmd/cron

.PHONY: build-all
# 构建所有服务
build-all: build build-cron

.PHONY: run-cron
# 运行 cron 服务
run-cron:
	./bin/cron -conf ./configs/config.yaml

.PHONY: run-all
# 同时运行所有服务（cron 后台，server 前台）
run-all:
	@echo "启动 cron 服务（后台）..."
	@mkdir -p logs
	@nohup ./bin/cron -conf ./configs/config.yaml > logs/cron.log 2>&1 & echo $$! > logs/cron.pid
	@sleep 1
	@if [ -f logs/cron.pid ]; then \
		CRON_PID=$$(cat logs/cron.pid); \
		if ps -p $$CRON_PID > /dev/null; then \
			echo "cron 服务已启动，PID: $$CRON_PID"; \
		else \
			echo "cron 服务启动失败!"; \
		fi \
	fi
	@echo "启动主服务（前台）..."
	@echo "========================================="
	@./bin/server -conf ./configs/config.yaml; \
	if [ -f logs/cron.pid ]; then \
		CRON_PID=$$(cat logs/cron.pid); \
		if ps -p $$CRON_PID > /dev/null; then \
			echo "停止 cron 服务..."; \
			kill $$CRON_PID; \
		fi; \
		rm -f logs/cron.pid; \
	fi

.PHONY: stop-all
# 停止所有服务
stop-all:
	@echo "停止所有服务..."
	@-pkill -f "bin/server" || true
	@-pkill -f "bin/cron" || true
	@-rm -f logs/cron.pid
	@echo "所有服务已停止"

# 覆盖 all 目标
.PHONY: all
all: api wire build-all
