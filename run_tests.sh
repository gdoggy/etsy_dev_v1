#!/bin/bash

# Draft 模块测试脚本

set -e

echo "=========================================="
echo "  Draft 模块测试"
echo "=========================================="

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 检查依赖
check_deps() {
    echo -e "${YELLOW}检查依赖...${NC}"

    if ! go version > /dev/null 2>&1; then
        echo -e "${RED}错误: 未找到 Go${NC}"
        exit 1
    fi

    # 检查 SQLite 驱动
    go get gorm.io/driver/sqlite 2>/dev/null || true

    echo -e "${GREEN}依赖检查通过${NC}"
}

# 单元测试
unit_tests() {
    echo ""
    echo -e "${YELLOW}运行单元测试...${NC}"
    echo "----------------------------------------"

    # Service 层测试
    echo "Testing service layer..."
    go test ./internal/service/... -v -short -count=1 2>&1 | head -100

    # Controller 层测试
    echo ""
    echo "Testing controller layer..."
    go test ./internal/controller/... -v -short -count=1 2>&1 | head -100

    echo -e "${GREEN}单元测试完成${NC}"
}

# 集成测试
integration_tests() {
    echo ""
    echo -e "${YELLOW}运行集成测试...${NC}"
    echo "----------------------------------------"

    if [ -z "$GEMINI_API_KEY" ]; then
        echo -e "${YELLOW}警告: GEMINI_API_KEY 未设置，部分测试将跳过${NC}"
    fi

    if [ -z "$ONEBOUND_API_KEY" ]; then
        echo -e "${YELLOW}警告: ONEBOUND_API_KEY 未设置，部分测试将跳过${NC}"
    fi

    go test ./tests/... -v -count=1 2>&1 | head -150

    echo -e "${GREEN}集成测试完成${NC}"
}

# 覆盖率测试
coverage_tests() {
    echo ""
    echo -e "${YELLOW}生成覆盖率报告...${NC}"
    echo "----------------------------------------"

    go test ./internal/... -coverprofile=coverage.out -short
    go tool cover -func=coverage.out | tail -20

    # 生成 HTML 报告
    go tool cover -html=coverage.out -o coverage.html

    echo ""
    echo -e "${GREEN}覆盖率报告已生成: coverage.html${NC}"
}

# 快速测试（仅核心功能）
quick_tests() {
    echo ""
    echo -e "${YELLOW}运行快速测试...${NC}"
    echo "----------------------------------------"

    go test ./internal/service/ -run "TestOneBoundService_ParseURL|TestNewAIService_DefaultConfig|TestDraftService_Subscribe" -v

    echo -e "${GREEN}快速测试完成${NC}"
}

# 帮助信息
show_help() {
    echo "Usage: $0 [command]"
    echo ""
    echo "Commands:"
    echo "  unit        运行单元测试"
    echo "  integration 运行集成测试"
    echo "  coverage    生成覆盖率报告"
    echo "  quick       快速测试（核心功能）"
    echo "  all         运行所有测试"
    echo "  help        显示帮助信息"
    echo ""
    echo "环境变量:"
    echo "  GEMINI_API_KEY        Google Gemini API Key"
    echo "  ONEBOUND_API_KEY      万邦 API Key"
    echo "  ONEBOUND_API_SECRET   万邦 API Secret"
    echo "  AWS_BUCKET            S3 存储桶名称"
    echo "  AWS_REGION            S3 区域"
    echo "  AWS_ACCESS_KEY_ID     AWS Access Key"
    echo "  AWS_SECRET_ACCESS_KEY AWS Secret Key"
}

# 主逻辑
main() {
    case "${1:-all}" in
        unit)
            check_deps
            unit_tests
            ;;
        integration)
            check_deps
            integration_tests
            ;;
        coverage)
            check_deps
            coverage_tests
            ;;
        quick)
            check_deps
            quick_tests
            ;;
        all)
            check_deps
            unit_tests
            integration_tests
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            echo -e "${RED}未知命令: $1${NC}"
            show_help
            exit 1
            ;;
    esac
}

main "$@"