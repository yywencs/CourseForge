# CourseForge 后端架构

## 组织原则

CourseForge 是按限界上下文纵向组织的模块化单体。目录层级先表达业务所有权，再在每个
上下文内部表达 DDD 分层；Go 的每个目录都是独立 package，不把领域模型和基础设施实现
放进同一个 package。

```text
internal/
  enrollment/
    domain/                 选课、正式选课、候补、轮次领域模型与规则
    application/            用例、应用端口、命令和查询模型
    infrastructure/         轮次管理及选课持久化适配器
    async/                  Redis Stream 消费者与 Asynq 任务适配器
    transport/http/         API 与 Admin HTTP 入站适配器
  catalog/
    domain/                 课程、教学班规划领域模型与规则
    application/            课程目录和教学班管理用例
    infrastructure/mysql/   Catalog MySQL 适配器
    transport/http/         API、Admin 和 DTO
  identity/
    domain/                 账号、登录凭据和认证规则
    application/            登录与当前会话用例
    infrastructure/         MySQL、JWT、bcrypt 和只读会话查询
    transport/http/         登录和当前会话 HTTP 入口
  platform/                 通用技术能力，不包含业务规则
  bootstrap/                唯一跨上下文组合根
```

依赖方向固定为：

```text
transport / async / infrastructure -> application -> domain
bootstrap -> 所有模块的公开构造入口
platform -> 不依赖 enrollment、catalog 或 identity
```

应用端口定义在消费端。只有领域服务直接消费的端口才放在 domain；Redis、MySQL、消息
发布、投影修复等用例能力放在 application。跨上下文不得复用对方聚合，允许通过稳定 ID、
只读投影或集成事件协作。

Application 只依赖 Domain 和标准库。ID 生成与业务流程观测分别通过 `IDGenerator`、
`EnrollmentObserver` 端口注入；Prometheus 适配器位于
`enrollment/infrastructure/observability`。异步任务类型由 `enrollment/async` 所有，Domain
不感知 Asynq 等调度协议。

Domain 对象不携带 JSON tag。Redis Stream、RabbitMQ、审计事件和 HTTP 使用各自 adapter
内的 payload/response，并显式映射到领域对象；现有外部 JSON 字段保持兼容。Enrollment
持久化的各 Store 只持有实际需要的 DB、Redis 或 ID 端口，共享的是连接池实例而不是一个
聚合全部能力的基础 Store。生产代码不提供聚合 Repository 门面。

## 上下文和数据所有权

| 数据/行为 | 写入所有者 | 其他上下文的使用方式 |
| --- | --- | --- |
| `course` | Catalog | Enrollment 读取课程 ID、学分等选课快照 |
| `teaching_class` 规划、容量、排课、状态 | Catalog | Enrollment 读取教学班快照执行资格判断 |
| `teaching_class.selected_count` | Enrollment | Catalog 只读，用于容量约束和展示 |
| `selection_round`、`selection_round_class` | Enrollment | Catalog 学生目录查询只读轮次投影 |
| 额度、申请、正式选课、候补和修复任务 | Enrollment | 不允许其他上下文直接写入 |
| `user_account`、学生登录凭据 | Identity | Enrollment 只接收认证后的学生 ID |
| `outbox_event` 及发布重试状态 | Platform Outbox | 业务上下文通过端口追加集成事件 |
| `student_notification` | Notification | RabbitMQ 消费选课落库事件后按 `event_id` 幂等写入 |

Catalog 的 `teachingClassRow.SelectedCount` 使用 GORM 只读字段声明，Catalog 的新增和更新
操作都不会写入该列。Enrollment 通过带条件的原子更新增加或归还名额，保持原有并发语义。

Identity 当前会话和学生课程目录需要组合轮次信息时，使用只读查询投影；这些查询不得改变
Enrollment 所有的轮次、额度或申请数据。后续拆分为独立服务时，可以将只读联表替换为事件
驱动投影，而不改变 application/domain 接口。

## 运行时装配

`internal/bootstrap` 创建数据库、Redis、RabbitMQ 和 Asynq 客户端，实例化各上下文的
应用服务，并启动选课 Redis Stream Consumer Group、HTTP 和定时任务适配器。RabbitMQ
保留给 MySQL Outbox 发布的跨业务集成事件，不再位于选课请求及核心落库链路。选课结果
与通知 Outbox 在同一 MySQL 事务提交；进程内常驻 Relay 连续驱动通用 Dispatcher，
Publisher Confirm 后更新发布状态，RabbitMQ 消费者按事件 ID 幂等写入学生站内通知。
Relay 空闲时每 100ms 轮询，有积压时连续排空，并每 10 秒采样 pending、publishing、failed
数量及最老待发布事件年龄。RabbitMQ 消费结果、通知新建/幂等重复/失败均记录 Prometheus
指标；Broker 队列、重试队列和 DLQ 深度由 RabbitMQ 自身监控采集。
`server/http/api` 和 `server/http/admin` 不持有具体业务用例，只遍历路由注册器；
`platform/taskqueue` 同样只消费通用任务注册项，不反向依赖任何业务上下文。
