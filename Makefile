SERVICE_NAME := billing-service
SERVICE_DISPLAY_NAME := Billing Service
API_PROTO_PATH := api/billing/v1/billing.proto
API_PROTO_DIR := api/billing/v1
CONF_PROTO_PATH := internal/conf/conf.proto
WIRE_DIRS := cmd/server cmd/scheduler
BUILD_OUTPUT := ./bin/server
RUN_MAIN := cmd/server/main.go cmd/server/wire_gen.go
CONFIG_FILE := configs/config.yaml
HTTP_PORT := 8107
GRPC_PORT := 9107
TEST_CONFIG := test/api/api-test-config.yaml
RUN_MODE := debug

include ../devops-tools/Makefile.common

# 服务特定的目标

.PHONY: build-scheduler
# 构建 scheduler 服务
build-scheduler:
	mkdir -p bin/
	go build -ldflags "-X main.Version=$(VERSION)" -o ./bin/scheduler ./cmd/scheduler

.PHONY: build-all
# 构建所有服务
build-all: build build-scheduler

.PHONY: run-scheduler
# 运行 scheduler 服务
run-scheduler:
	./bin/scheduler -conf ./configs/config.yaml

.PHONY: run-all
# 同时运行所有服务（scheduler 后台，server 前台）
run-all:
	@echo "启动 scheduler 服务（后台）..."
	@mkdir -p logs
	@nohup ./bin/scheduler -conf ./configs/config.yaml > logs/scheduler.log 2>&1 & echo $$! > logs/scheduler.pid
	@sleep 1
	@if [ -f logs/scheduler.pid ]; then \
		SCHEDULER_PID=$$(cat logs/scheduler.pid); \
		if ps -p $$SCHEDULER_PID > /dev/null; then \
			echo "scheduler 服务已启动，PID: $$SCHEDULER_PID"; \
		else \
			echo "scheduler 服务启动失败!"; \
		fi \
	fi
	@echo "启动主服务（前台）..."
	@echo "========================================="
	@./bin/server -conf ./configs/config.yaml; \
	if [ -f logs/scheduler.pid ]; then \
		SCHEDULER_PID=$$(cat logs/scheduler.pid); \
		if ps -p $$SCHEDULER_PID > /dev/null; then \
			echo "停止 scheduler 服务..."; \
			kill $$SCHEDULER_PID; \
		fi; \
		rm -f logs/scheduler.pid; \
	fi

.PHONY: stop-all
# 停止所有服务
stop-all:
	@echo "停止所有服务..."
	@-pkill -f "bin/server" || true
	@-pkill -f "bin/scheduler" || true
	@-rm -f logs/scheduler.pid
	@echo "所有服务已停止"

# 覆盖 all 目标
.PHONY: all
all: api wire build-all
