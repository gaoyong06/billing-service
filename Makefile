SERVICE_NAME := billing-service
SERVICE_DISPLAY_NAME := Billing Service
API_PROTO_PATH := api/billing/v1/billing.proto
API_PROTO_DIR := api/billing/v1
WIRE_DIRS := cmd/billing-service
BUILD_OUTPUT := ./bin/server
RUN_MAIN := cmd/billing-service/main.go cmd/billing-service/wire_gen.go
CONFIG_FILE := configs/config.yaml
HTTP_PORT := 8107
TEST_CONFIG := test/api/api-test-config.yaml

include ../devops-tools/Makefile.common
