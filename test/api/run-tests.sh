#!/bin/bash

# Billing Service API 测试运行脚本

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 配置
CONFIG_FILE="./api-test-config.yaml"
API_TESTER_CMD="api-tester"

# 检查 api-tester 是否安装
if ! command -v $API_TESTER_CMD &> /dev/null; then
    echo -e "${RED}错误: api-tester 未安装${NC}"
    echo "请运行: go install github.com/gaoyong06/api-tester/cmd/api-tester@latest"
    exit 1
fi

# 检查配置文件是否存在
if [ ! -f "$CONFIG_FILE" ]; then
    echo -e "${RED}错误: 配置文件不存在: $CONFIG_FILE${NC}"
    exit 1
fi

# 检查服务是否运行
echo -e "${YELLOW}检查服务是否运行...${NC}"
if ! curl -s http://localhost:8107/api/v1/billing/account?user_id=test > /dev/null 2>&1; then
    echo -e "${YELLOW}警告: 无法连接到服务 (http://localhost:8107)${NC}"
    echo -e "${YELLOW}请确保 billing-service 已启动${NC}"
    read -p "是否继续? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# 运行测试
echo -e "${GREEN}开始运行测试...${NC}"
echo "配置文件: $CONFIG_FILE"
echo ""

$API_TESTER_CMD run --config "$CONFIG_FILE" --verbose

# 检查测试结果
if [ $? -eq 0 ]; then
    echo ""
    echo -e "${GREEN}✓ 测试完成${NC}"
    echo -e "${GREEN}测试报告已生成到: ./test-reports${NC}"
else
    echo ""
    echo -e "${RED}✗ 测试失败${NC}"
    exit 1
fi

