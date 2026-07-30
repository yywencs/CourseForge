# CourseForge

CourseForge 是一个使用 Go 实现的高并发选课系统。选课模块采用 DDD 分层：
`domain` 定义聚合、领域策略与仓储端口，`application` 负责编排，`infrastructure`
实现 MySQL、Redis 和消息队列端口。

## 快速开始

环境要求：Go 1.25+、Docker、Docker Compose。

启动临时 MySQL、Redis 和 RabbitMQ：

```bash
make integration-up
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

销毁临时依赖：

```bash
make integration-down
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
- `make monitoring-up` 启动 Prometheus、Grafana 和 MySQL Exporter。
- `make search-up` 启动 Elasticsearch、Kibana 和 CDC Sync。

## 项目结构

```text
cmd/                    API、Admin、CDC 与 benchmark 入口
internal/application/   选课应用编排
internal/domain/        选课与通用 Outbox 模型
internal/infrastructure/数据库、缓存、消息队列与仓储实现
internal/listener/      通用 RabbitMQ Consumer 与选课结果监听器
internal/job/           结果恢复与 Outbox 任务
server/http/            Gin 路由和 Handler
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

选课业务闭环及配套能力已经完成。课程运营后台、视频学习和弹幕是后续独立限界上下文；
通用 Outbox、CDC、消息队列和可观测基础设施可直接复用。
