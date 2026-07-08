.ONESHELL:
SHELL := /bin/bash
BASE_DIR := $(shell pwd)
FRONTEND_DIR := $(BASE_DIR)/web/frontend
BACKEND_PORT := 8080
FRONTEND_PORT := 3000
DOCKER_CONTEXT_HOST := $(shell docker context inspect --format '{{json .Endpoints.docker.Host}}' 2>/dev/null | tr -d '"')
TESTCONTAINERS_DOCKER_HOST := $(if $(DOCKER_HOST),$(DOCKER_HOST),$(DOCKER_CONTEXT_HOST))
TESTCONTAINERS_RYUK := $(if $(TESTCONTAINERS_RYUK_DISABLED),$(TESTCONTAINERS_RYUK_DISABLED),$(if $(filter rancher-desktop,$(shell docker context show 2>/dev/null)),true,false))

.PHONY: all build run dev stop clean test test-unit test-api test-integration test-e2e lint fmt seed-dev seed-demo traffic-demo demo-data

## 默认目标
all: build

## ============================================================
## 构建
## ============================================================

## 构建后端
build-backend:
	@echo "==> Building backend..."
	go build -o bin/llm-gateway ./cmd/server

## 构建前端
build-frontend:
	@echo "==> Building frontend..."
	cd $(FRONTEND_DIR) && npm run build

## 构建全部
build: build-backend build-frontend

## ============================================================
## 运行（开发模式）
## ============================================================

## 启动后端（开发模式）
run-backend:
	@echo "==> Starting backend on :$(BACKEND_PORT)..."
	go run ./cmd/server/

## 启动前端（开发模式）
run-frontend:
	@echo "==> Starting frontend on :$(FRONTEND_PORT)..."
	cd $(FRONTEND_DIR) && npm run dev

## 一键启动前后端（后台运行）
dev:
	@echo "==> Starting all services..."
	@make -j2 run-backend run-frontend

## 分别后台启动
start-backend:
	@echo "==> Starting backend in background..."
	@$(SHELL) -c 'go run ./cmd/server/ > /tmp/llm-gateway-backend.log 2>&1 & echo "    Backend PID: $$!"'
	@echo "    Log: /tmp/llm-gateway-backend.log"

start-frontend:
	@echo "==> Starting frontend in background..."
	@$(SHELL) -c 'cd $(FRONTEND_DIR) && npm run dev > /tmp/llm-gateway-frontend.log 2>&1 & echo "    Frontend PID: $$!"'
	@echo "    Log: /tmp/llm-gateway-frontend.log"

## 一键后台启动前后端
start: start-backend start-frontend
	@echo ""
	@echo "==> All services started!"
	@echo "    Backend:  http://localhost:$(BACKEND_PORT)"
	@echo "    Frontend: http://localhost:$(FRONTEND_PORT)"
	@echo "    Backend log: /tmp/llm-gateway-backend.log"
	@echo "    Frontend log: /tmp/llm-gateway-frontend.log"

## ============================================================

## ===========================================================
## 重启
## ===========================================================

restart: stop
	@sleep 2
	@$(MAKE) start

## 停止
## ============================================================

stop:
	@echo "==> Stopping all services..."
	@lsof -ti:$(BACKEND_PORT) | xargs kill -9 2>/dev/null || true
	@lsof -ti:$(FRONTEND_PORT) | xargs kill -9 2>/dev/null || true
	@echo "    All services stopped."

## ============================================================
## 清理
## ============================================================

clean:
	@echo "==> Cleaning..."
	rm -rf bin/
	rm -f llm-gateway.db
	cd $(FRONTEND_DIR) && rm -rf .next node_modules/.cache

## ============================================================
## 测试 & 代码质量
## ============================================================

test: test-unit

test-unit:
	@echo "==> Running unit tests..."
	go test ./... -v -short -timeout 60s

test-api:
	@echo "==> Running API tests..."
	go test ./tests/api/... -v -timeout 30s

test-integration:
	@echo "==> Running integration tests..."
	@if [ -d tests/integration ]; then DOCKER_HOST="$(TESTCONTAINERS_DOCKER_HOST)" TESTCONTAINERS_RYUK_DISABLED="$(TESTCONTAINERS_RYUK)" go test -tags=integration ./tests/integration/... -v -timeout 3m -count=1; else echo "    No integration tests yet."; fi

test-e2e:
	@echo "==> Running E2E tests..."
	@if [ -d tests/e2e ]; then DOCKER_HOST="$(TESTCONTAINERS_DOCKER_HOST)" TESTCONTAINERS_RYUK_DISABLED="$(TESTCONTAINERS_RYUK)" go test -tags=e2e ./tests/e2e/... -v -timeout 2m; else echo "    No E2E tests yet."; fi

test-coverage:
	@echo "==> Running tests with coverage..."
	go test ./... -coverprofile=/tmp/coverage.out
	go tool cover -html=/tmp/coverage.out -o /tmp/coverage.html
	@echo "    Coverage report: /tmp/coverage.html"

lint:
	@echo "==> Linting..."
	golangci-lint run ./... || true

fmt:
	@echo "==> Formatting code..."
	go fmt ./...
	goimports -w . || goimports -w . 2>/dev/null || true

## ============================================================
## 数据库
## ============================================================

db-migrate:
	@echo "==> Running database migrations..."
	@echo "    (GORM auto-migrate on server start)"

db-reset:
	@echo "==> Resetting database..."
	rm -f llm-gateway.db
	@echo "    Database reset complete."

## ============================================================
## 数据生成
## ============================================================

seed-dev:
	@echo "==> Seeding dev data..."
	go run ./cmd/seed --profile dev

seed-demo:
	@echo "==> Seeding demo data..."
	go run ./cmd/seed --profile demo

traffic-demo:
	@echo "==> Generating demo traffic..."
	go run ./cmd/generate-traffic --profile demo --requests $${REQUESTS:-20} --concurrency $${CONCURRENCY:-2}

demo-data: seed-demo traffic-demo

## ============================================================
## 安装依赖
## ============================================================

deps-backend:
	@echo "==> Installing backend dependencies..."
	go mod tidy
	go mod download

deps-frontend:
	@echo "==> Installing frontend dependencies..."
	cd $(FRONTEND_DIR) && npm install

deps: deps-backend deps-frontend

## ============================================================
## 帮助
## ============================================================

help:
	@echo ""
	@echo "LLM Gateway - Makefile 命令"
	@echo "=========================================="
	@echo ""
	@echo "  构建:"
	@echo "    make build          - 构建后端 + 前端"
	@echo "    make build-backend  - 仅构建后端"
	@echo "    make build-frontend  - 仅构建前端"
	@echo ""
	@echo "  运行（前台）:"
	@echo "    make dev             - 同时启动前后端（Ctrl+C 停止）"
	@echo "    make run-backend    - 仅启动后端"
	@echo "    make run-frontend   - 仅启动前端"
	@echo ""
	@echo "  运行（后台）:"
	@echo "    make start           - 后台启动前后端"
	@echo "    make stop            - 停止所有服务"
	@echo ""
	@echo "  其他:"
	@echo "    make test           - 运行测试"
	@echo "    make lint           - 代码检查"
	@echo "    make fmt            - 格式化代码"
	@echo "    make clean          - 清理构建产物"
	@echo "    make deps           - 安装所有依赖"
	@echo "    make db-reset      - 重置数据库"
	@echo "    make seed-dev       - 生成本地开发 seed 数据"
	@echo "    make seed-demo      - 生成 demo seed 数据和 manifest"
	@echo "    make traffic-demo   - 基于 demo manifest 生成真实代理流量"
	@echo "    make demo-data      - 生成 demo seed 数据并生成流量"
	@echo ""
