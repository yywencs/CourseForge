# CourseForge

CourseForge 是一个使用 Go 实现的高并发选课系统。后端采用限界上下文优先的模块化
单体结构：`enrollment`、`catalog`、`identity` 各自在模块内部划分 `domain`、
`application`、`infrastructure` 与入站适配器，共享技术能力统一放在 `platform`。
详细依赖和数据所有权见 [架构说明](docs/architecture.md)。

## 快速开始

环境要求：Go 1.25+、Docker、Docker Compose。

复制本地环境变量示例，并启动本地基础设施：

```bash
cp .env.example .env
make infra-up
```

`make infra-up` 会优先使用已被 Git 忽略的 `docker-compose-my.yaml`。该个人配置只启动
Redis 和 RabbitMQ，MySQL 使用宿主机服务或 SSH 隧道；没有该文件时会回退到仓库中的
`docker-compose.yaml`，同时启动 MySQL、Redis 和 RabbitMQ。

使用仓库默认 Compose 首次启动 MySQL 时，会依次执行 `docs/sql/courseforge.sql` 和
`docs/sql/dev_seed.sql`，
自动创建 `courseforge` 库、表结构和开发演示数据。默认连接信息与
`configs/config.example.yaml` 一致。

开发演示账号：

```text
学号：2026001001
密码：CourseForge@123
```

MySQL 初始化脚本只会在数据卷为空时自动执行。已有数据库或外部数据库需要手动导入：

```bash
mysql -h 127.0.0.1 -P 3306 -u root -p < docs/sql/split_student_account.sql
mysql -h 127.0.0.1 -P 3306 -u root -p < docs/sql/dev_seed.sql
```

复制并调整本地配置：

```bash
cp configs/config.example.yaml configs/config.yaml
```

启动服务：

```bash
make run-api
make run-admin
```

默认接口：

选课业务接口均要求 `Authorization: Bearer <JWT>`；学生 ID 从
`student_id`（兼容 `user_id`/`sub`）Claim 获取，不接受请求体传入身份。

- `POST /api/v1/enrollments`
- `GET /api/v1/enrollments/applications/:application_id`
- `GET /api/v1/enrollments/me?term_id=:term_id`
- `DELETE /api/v1/enrollments/:enrollment_id`
- `POST /api/v1/enrollments/waitlist`
- `GET /api/v1/enrollments/waitlist/me?term_id=:term_id`
- `GET /api/v1/enrollments/waitlist/:waitlist_id`
- `DELETE /api/v1/enrollments/waitlist/:waitlist_id`
- `GET /healthz`
- `GET /readyz`
- Admin：`GET /admin/v1/status`

停止本地依赖：

```bash
make down
```

## 测试与压测

```bash
# 格式、静态检查、单元测试和部署脚本测试
make check

# 真实 MySQL、Redis、RabbitMQ 集成测试
make integration-test

# 构建选课压测工具
make build-benchmark
./bin/courseforge-benchmark prepare --help
./bin/courseforge-benchmark run --help
```

压测工具会准备隔离的高位 ID 测试数据，支持正常选课、幂等重试和候补三种场景。
完整参数见 [压测工具说明](cmd/benchmark/README.md)。

## 可观测性与数据同步

- Prometheus 指标使用 `courseforge` namespace，覆盖 HTTP、选课、候补、投影修复、
  异步落库、Redis、RabbitMQ、Asynq、Outbox 和 MySQL 连接池。
- Prometheus 规则覆盖投影修复积压/失败、Asynq 重试积压和选课内部错误率。
- Grafana 自动加载 `CourseForge Overview` Dashboard。
- CDC 默认订阅 `courseforge\..*`，并写入 `courseforge_<table>` Elasticsearch 索引。
- `make monitoring-up` 首次运行时会从示例生成已被 Git 忽略的本地 Prometheus 配置，
  随后启动 Prometheus 和 Grafana。本地需要自定义抓取目标时，修改
  `monitoring/prometheus/prometheus.local.yml`。
- ECS 部署使用 `monitoring/prometheus/prometheus.deploy.yml`，在仓库根目录执行
  `docker compose --env-file deploy/.env -f deploy/compose.yaml --profile observability up -d prometheus grafana`
  可独立启动生产监控，不影响 API、Admin 和 Web 的版本回滚。
- `make search-up` 启动 Elasticsearch 和 Kibana。
- `make cdc-up` 启动 MySQL、Elasticsearch 和 CDC Sync。

## 项目结构

```text
cmd/                    API、Admin、CDC 与 benchmark 入口
internal/enrollment/    选课、候补、轮次、异步结果与对应适配器
internal/catalog/       课程、教学班及对应应用和适配器
internal/identity/      学生认证、账号查询、JWT 与 HTTP 入口
internal/platform/      数据库、缓存、任务队列、RabbitMQ、Outbox、CDC 与可观测能力
internal/bootstrap/     进程级组合根，只负责装配上下文和平台能力
server/http/            通用 Gin 服务外壳、健康检查与路由注册机制
tests/integration/      真实依赖集成测试
monitoring/             Prometheus 与 Grafana
deploy/                 生产 Compose 与部署脚本
docs/sql/               CourseForge 数据库初始化脚本
```

## 核心能力

- **原子选课决策**：Redis Lua 在一次执行中校验学生额度、课程重复选择和教学班名额，
  同时保存申请结果并写入 Redis Stream，避免并发超卖。
- **可靠异步落库**：请求成功前等待 RabbitMQ Publisher Confirm；消费者使用 MySQL
  事务和唯一约束完成幂等持久化。
- **失败恢复**：Asynq 定时扫描未确认的 Redis Stream 结果并重新投递。
- **完整业务闭环**：支持 JWT 学生身份、请求级幂等、申请状态与本人课表查询、
  退课、候补排队及空位自动晋级。
- **资格领域策略**：学生状态、年级、专业、先修课程和课表冲突在纯领域策略中判断，
  基础设施层一次性装配资格快照。
- **一致性修复**：退课事务同步写入可靠修复任务；Redis 回补失败时由 Asynq
  幂等重放，并暴露 Prometheus 指标和告警。
- **通用 Outbox**：业务事务内写入 `outbox_event`，后台任务负责抢占、发布确认、
  失败退避和过期租约恢复，可直接复用于后续视频、弹幕等模块。
- **工程化设施**：保留 DBRouter、RabbitMQ、Asynq、CDC、Elasticsearch、
  Prometheus、Grafana、真实依赖集成测试和多场景选课 benchmark。
- **可扩展入口**：API 当前提供选课接口；Admin 保留独立服务与路由注册骨架，
  便于后续接入课程、视频和弹幕管理。

## 当前边界

选课业务闭环及配套能力已经完成。选课轮次由 `enrollment` 管理，课程和教学班规划由
`catalog` 管理，账号认证由 `identity` 管理。课程运营扩展、视频学习和弹幕是后续独立
限界上下文；通用 Outbox、CDC、消息队列和可观测基础设施可直接复用。
