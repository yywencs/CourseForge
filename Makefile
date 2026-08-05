GO ?= go
BUF ?= ./web/node_modules/.bin/buf
BUF_CACHE_DIR ?= /tmp/courseforge-buf-cache
COMPOSE ?= docker compose
LOCAL_COMPOSE_FILE ?= docker-compose.yaml
LOCAL_COMPOSE = $(COMPOSE) -f $(LOCAL_COMPOSE_FILE)
LOCAL_INFRA_SERVICES ?= mysql redis rabbitmq minio
BIN_DIR ?= bin
LDFLAGS ?= -s -w

.DEFAULT_GOAL := help

.PHONY: help
help: ## 显示可用命令
	@awk 'BEGIN {FS = ":.*## "; printf "Usage: make <target>\n\nTargets:\n"} /^[a-zA-Z0-9_.-]+:.*## / {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: fmt
fmt: ## 格式化 Go 代码
	$(GO) fmt ./...

.PHONY: proto
proto: ## 校验并生成 Go、TypeScript Protobuf 代码
	BUF_CACHE_DIR=$(BUF_CACHE_DIR) $(BUF) lint
	BUF_CACHE_DIR=$(BUF_CACHE_DIR) $(BUF) generate

.PHONY: proto-check
proto-check: proto
	BUF_CACHE_DIR=$(BUF_CACHE_DIR) $(BUF) format -d --exit-code
	@changed="$$(git status --porcelain -- gen web/src/gen)"; \
	if [ -n "$$changed" ]; then \
		printf '%s\n' "Protobuf 生成代码不是最新，请执行 make proto 并提交变更：" "$$changed"; \
		exit 1; \
	fi

.PHONY: fmt-check
fmt-check:
	@files="$$(gofmt -l $$(find . -type f -name '*.go' -not -path './vendor/*'))"; \
	if [ -n "$$files" ]; then \
		printf '%s\n' "以下文件需要执行 gofmt:" "$$files"; \
		exit 1; \
	fi

.PHONY: vet
vet:
	$(GO) vet ./...

.PHONY: test
test: ## 运行 Go 单元测试
	$(GO) test ./...

.PHONY: test-deploy
test-deploy:
	bash ./deploy/deploy_test.sh

.PHONY: check
check: fmt-check vet test test-deploy ## 执行后端格式、静态检查和测试

.PHONY: integration-test
integration-test: ## 使用临时 MySQL、Redis、RabbitMQ 运行集成测试
	bash ./tests/integration/run.sh

.PHONY: build
build: ## 构建 API 和 Admin
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/courseforge-api ./cmd/api
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/courseforge-admin ./cmd/admin

.PHONY: build-benchmark
build-benchmark: ## 构建选课和 WebSocket 压测工具
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/courseforge-benchmark ./cmd/benchmark/enrollment
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/courseforge-websocket-benchmark ./cmd/benchmark/websocket

.PHONY: run-api
run-api: ## 启动 API 服务
	$(GO) run ./cmd/api

.PHONY: run-admin
run-admin: ## 启动 Admin 服务
	$(GO) run ./cmd/admin

.PHONY: infra-up
infra-up: ## 启动 MySQL、Redis、RabbitMQ 和 MinIO
	$(LOCAL_COMPOSE) up -d --wait $(LOCAL_INFRA_SERVICES)
	$(LOCAL_COMPOSE) up -d minio-init

.PHONY: down
down: ## 停止本地 Docker 服务
	$(LOCAL_COMPOSE) --profile "*" down
