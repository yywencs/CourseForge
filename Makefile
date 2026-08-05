GO ?= go
BUF ?= ./web/node_modules/.bin/buf
BUF_CACHE_DIR ?= /tmp/courseforge-buf-cache
DOCKER ?= docker
COMPOSE ?= docker compose
LOCAL_COMPOSE_FILE ?= $(if $(wildcard docker-compose-my.yaml),docker-compose-my.yaml,docker-compose.yaml)
LOCAL_COMPOSE ?= $(COMPOSE) -f $(LOCAL_COMPOSE_FILE)
LOCAL_INFRA_SERVICES ?= $(if $(filter docker-compose-my.yaml,$(LOCAL_COMPOSE_FILE)),redis rabbitmq,mysql redis rabbitmq)
INTEGRATION_COMPOSE ?= $(COMPOSE) -f compose.integration.yaml
INTEGRATION_MYSQL_PASSWORD ?= courseforge-integration
INTEGRATION_MYSQL_PORT ?= 13306
INTEGRATION_REDIS_PORT ?= 16379
INTEGRATION_RABBITMQ_PORT ?= 15673
INTEGRATION_RABBITMQ_USER ?= courseforge-integration
INTEGRATION_RABBITMQ_PASSWORD ?= courseforge-integration
export INTEGRATION_MYSQL_PASSWORD
export INTEGRATION_MYSQL_PORT
export INTEGRATION_REDIS_PORT
export INTEGRATION_RABBITMQ_PORT
export INTEGRATION_RABBITMQ_USER
export INTEGRATION_RABBITMQ_PASSWORD

BIN_DIR ?= bin
IMAGE_PREFIX ?= courseforge
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || printf 'dev')
LDFLAGS ?= -s -w
LOCAL_PROMETHEUS_CONFIG ?= monitoring/prometheus/prometheus.local.yml
LOCAL_PROMETHEUS_EXAMPLE ?= monitoring/prometheus/prometheus.local.example.yml

.DEFAULT_GOAL := help

.PHONY: help
help: ## 显示可用命令
	@awk 'BEGIN {FS = ":.*## "; printf "Usage: make <target>\n\nTargets:\n"} /^[a-zA-Z0-9_.-]+:.*## / {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: fmt
fmt: ## 格式化 Go 代码
	$(GO) fmt ./...

.PHONY: proto
proto: ## 校验协议并生成 Go、TypeScript Protobuf 代码
	BUF_CACHE_DIR=$(BUF_CACHE_DIR) $(BUF) lint
	BUF_CACHE_DIR=$(BUF_CACHE_DIR) $(BUF) generate

.PHONY: proto-format
proto-format: ## 格式化 Protobuf 协议文件
	BUF_CACHE_DIR=$(BUF_CACHE_DIR) $(BUF) format -w

.PHONY: proto-check
proto-check: proto ## 检查已提交的 Protobuf 生成代码是否为最新
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
test: ## 运行全部测试
	$(GO) test ./...

.PHONY: test-race
test-race: ## 使用竞态检测运行全部测试
	$(GO) test -race ./...

.PHONY: test-cover
test-cover: ## 生成测试覆盖率报告 coverage.html
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	$(GO) tool cover -func=coverage.out

.PHONY: integration-test
integration-test: ## 启动临时 MySQL、Redis、RabbitMQ，运行集成测试并自动销毁环境
	@set -eu; \
	INTEGRATION_MYSQL_PASSWORD='$(INTEGRATION_MYSQL_PASSWORD)' INTEGRATION_MYSQL_PORT='$(INTEGRATION_MYSQL_PORT)' INTEGRATION_REDIS_PORT='$(INTEGRATION_REDIS_PORT)' INTEGRATION_RABBITMQ_PORT='$(INTEGRATION_RABBITMQ_PORT)' INTEGRATION_RABBITMQ_USER='$(INTEGRATION_RABBITMQ_USER)' INTEGRATION_RABBITMQ_PASSWORD='$(INTEGRATION_RABBITMQ_PASSWORD)' $(INTEGRATION_COMPOSE) up -d --wait mysql redis rabbitmq; \
	trap '$(INTEGRATION_COMPOSE) down --volumes --remove-orphans' EXIT; \
	expected_courseforge_table_count="$$(grep -c '^CREATE TABLE' docs/sql/courseforge.sql)"; \
	courseforge_table_count="$$( $(INTEGRATION_COMPOSE) exec -T mysql sh -ec 'mysql -uroot -p"$$MYSQL_ROOT_PASSWORD" -Nse "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = '"'"'courseforge'"'"'"' )"; \
	if [ "$$courseforge_table_count" -ne "$$expected_courseforge_table_count" ]; then \
		printf '%s\n' \
			"CourseForge integration schema mismatch: expected $$expected_courseforge_table_count tables from docs/sql/courseforge.sql, found $$courseforge_table_count" \
			"Current CourseForge tables:" >&2; \
		$(INTEGRATION_COMPOSE) exec -T mysql sh -ec 'mysql -uroot -p"$$MYSQL_ROOT_PASSWORD" -Nse "SELECT table_name FROM information_schema.tables WHERE table_schema = '"'"'courseforge'"'"' ORDER BY table_name"' >&2; \
		exit 1; \
	fi; \
	printf '%s\n' "CourseForge integration MySQL schema is ready"; \
	redis_response="$$( $(INTEGRATION_COMPOSE) exec -T redis redis-cli ping )"; \
	test "$$redis_response" = "PONG"; \
	printf '%s\n' "integration Redis is ready"; \
	$(INTEGRATION_COMPOSE) exec -T rabbitmq rabbitmq-diagnostics -q ping >/dev/null; \
	printf '%s\n' "integration RabbitMQ is ready"; \
	COURSEFORGE_INTEGRATION_MYSQL_DSN='root:$(INTEGRATION_MYSQL_PASSWORD)@tcp(127.0.0.1:$(INTEGRATION_MYSQL_PORT))/courseforge?charset=utf8mb4&parseTime=True&loc=Local&timeout=5s' \
	COURSEFORGE_INTEGRATION_REDIS_ADDR='127.0.0.1:$(INTEGRATION_REDIS_PORT)' \
	COURSEFORGE_INTEGRATION_RABBITMQ_ADDR='127.0.0.1:$(INTEGRATION_RABBITMQ_PORT)' \
	COURSEFORGE_INTEGRATION_RABBITMQ_USER='$(INTEGRATION_RABBITMQ_USER)' \
	COURSEFORGE_INTEGRATION_RABBITMQ_PASSWORD='$(INTEGRATION_RABBITMQ_PASSWORD)' \
		$(GO) test -tags=integration ./tests/integration/... ./cmd/benchmark/enrollment -count=1

.PHONY: check
check: fmt-check vet test test-deploy ## 执行格式、静态检查和测试

.PHONY: test-deploy
test-deploy:
	bash ./deploy/deploy_test.sh

.PHONY: build
build: build-api build-admin build-cdc ## 构建全部服务

.PHONY: build-api
build-api:
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/courseforge-api ./cmd/api

.PHONY: build-admin
build-admin:
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/courseforge-admin ./cmd/admin

.PHONY: build-cdc
build-cdc:
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/courseforge-cdc-sync ./cmd/cdc-sync

.PHONY: build-benchmark
build-benchmark: build-enrollment-benchmark build-websocket-benchmark ## 构建全部压测工具

.PHONY: build-enrollment-benchmark
build-enrollment-benchmark: ## 构建 CourseForge 选课压测工具
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/courseforge-benchmark ./cmd/benchmark/enrollment

.PHONY: build-websocket-benchmark
build-websocket-benchmark: ## 构建 CourseForge 实时弹幕 WebSocket 压测工具
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/courseforge-websocket-benchmark ./cmd/benchmark/websocket

.PHONY: run-api
run-api: ## 本地启动 API 服务
	$(GO) run ./cmd/api

.PHONY: run-admin
run-admin: ## 本地启动 Admin 服务
	$(GO) run ./cmd/admin

.PHONY: run-cdc
run-cdc: ## 本地启动 CDC Sync 服务（配置来自 CDC_* 环境变量）
	$(GO) run ./cmd/cdc-sync

.PHONY: docker-build
docker-build: docker-build-api docker-build-admin docker-build-cdc ## 构建全部 Docker 镜像

.PHONY: docker-build-api
docker-build-api:
	$(DOCKER) build --target api -t $(IMAGE_PREFIX)-api:$(VERSION) .

.PHONY: docker-build-admin
docker-build-admin:
	$(DOCKER) build --target admin -t $(IMAGE_PREFIX)-admin:$(VERSION) .

.PHONY: docker-build-cdc
docker-build-cdc:
	$(DOCKER) build --target cdc-sync -t $(IMAGE_PREFIX)-cdc-sync:$(VERSION) .

.PHONY: docker-build-benchmark
docker-build-benchmark: docker-build-enrollment-benchmark docker-build-websocket-benchmark ## 构建全部压测镜像

.PHONY: docker-build-enrollment-benchmark
docker-build-enrollment-benchmark: ## 构建 CourseForge 选课压测镜像
	$(DOCKER) build -f cmd/benchmark/enrollment/Dockerfile -t courseforge-benchmark:$(VERSION) .

.PHONY: docker-build-websocket-benchmark
docker-build-websocket-benchmark: ## 构建 CourseForge 实时弹幕 WebSocket 压测镜像
	$(DOCKER) build -f cmd/benchmark/websocket/Dockerfile -t courseforge-websocket-benchmark:$(VERSION) .

.PHONY: monitoring-config
monitoring-config:
	@if [ ! -f "$(LOCAL_PROMETHEUS_CONFIG)" ]; then \
		cp "$(LOCAL_PROMETHEUS_EXAMPLE)" "$(LOCAL_PROMETHEUS_CONFIG)"; \
		printf '%s\n' "generated $(LOCAL_PROMETHEUS_CONFIG)"; \
	fi

.PHONY: compose-config
compose-config: monitoring-config
	$(LOCAL_COMPOSE) --profile "*" config

.PHONY: infra-up
infra-up: ## 从本地 Compose 启动并等待基础设施就绪
	$(LOCAL_COMPOSE) up -d --wait $(LOCAL_INFRA_SERVICES)

.PHONY: monitoring-up
monitoring-up: monitoring-config ## 启动 Prometheus 和 Grafana
	$(LOCAL_COMPOSE) --profile observability up -d prometheus grafana

.PHONY: search-up
search-up: ## 启动 Elasticsearch 和 Kibana
	$(COMPOSE) --profile search up -d elasticsearch kibana

.PHONY: cdc-up
cdc-up: ## 启动 MySQL、Elasticsearch 和 CDC Sync
	$(COMPOSE) --profile cdc up -d elasticsearch cdc-sync

.PHONY: down
down: ## 停止 Compose 服务
	$(LOCAL_COMPOSE) --profile "*" down

.PHONY: logs
logs: ## 跟踪 Compose 日志
	$(LOCAL_COMPOSE) --profile "*" logs --tail=200 -f

.PHONY: clean
clean: ## 清理本地构建产物和覆盖率报告
	rm -rf $(BIN_DIR) coverage.out coverage.html
