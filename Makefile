# Makefile for AI Sync Manager

.PHONY: help dev build run test lint clean install-tools

# 默认目标
help:
	@echo "AI Sync Manager - 可用命令:"
	@echo ""
	@echo "  make dev         - 启动开发模式"
	@echo "  make build       - 构建应用"
	@echo "  make run         - 运行应用"
	@echo "  make test        - 运行测试"
	@echo "  make lint        - 运行代码检查"
	@echo "  make clean       - 清理构建产物"
	@echo "  make install-tools - 安装开发工具"

# 开发模式
dev:
	@echo "启动开发模式..."
	@wails dev

# 构建
build:
	@echo "构建应用..."
	@wails build

# 运行
run:
	@echo "运行应用..."
	@wails build
	@./build/bin/ai-sync-manager.exe

# 测试
test:
	@echo "运行测试..."
	@go test -v ./...

# 代码检查
lint:
	@echo "运行代码检查..."
	@golangci-lint run

# 格式化代码
fmt:
	@echo "格式化代码..."
	@go fmt ./...
	@goimports -w .

# 清理
clean:
	@echo "清理构建产物..."
	@rm -rf build/
	@rm -rf frontend/dist/
	@rm -rf frontend/node_modules/.vite/

# 安装开发工具
install-tools:
	@echo "安装开发工具..."
	@go install github.com/wailsapp/wails/v2/cmd/wails@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install golang.org/x/tools/cmd/goimports@latest

# 安装 Wails 依赖
install-frontend:
	@echo "安装前端依赖..."
	@cd frontend && pnpm install

# 前端开发
frontend-dev:
	@echo "启动前端开发模式..."
	@cd frontend && pnpm dev

# 完整安装
install: install-tools install-frontend
