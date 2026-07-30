# CourseForge 工程问题与解决记录

> 更新时间：2026-07-30

## 1. Redis-first 选课结果的可靠落库

同步写 MySQL 会把数据库事务放在高并发请求热路径；只在 Redis 扣减名额后异步写库，
又会产生进程在消息发布前退出的丢失窗口。

当前链路将正确性边界拆分为：

1. Redis Lua 原子校验学生额度、重复课程和教学班名额；
2. 同一段 Lua 保存标准申请结果并写入 Redis Stream；
3. RabbitMQ 发布使用持久化消息、mandatory 路由检查和 Publisher Confirm；
4. 消费者使用 MySQL 事务与唯一约束幂等落库；
5. Asynq 定时扫描 Stream，补发尚未确认的结果。

真实依赖集成测试覆盖完整链路、重复消费、恢复投递和多人并发抢同一教学班时不超卖。

## 2. Publisher 全局锁导致消息发布串行化

RabbitMQ Confirm 与 mandatory return 必须正确关联，但如果所有发布共用一个 Channel
并在等待网络确认时持有全局锁，并发请求会在 Publisher 处串行排队。

当前 Publisher 使用有界 Channel 池。每次发布独占一个 slot，同一 slot 内顺序等待
Confirm，不同 slot 可以并行；池满时遵守调用方 Context。单元测试和真实 RabbitMQ
集成测试共同覆盖并发发布、Broker Confirm 和不可路由消息。

## 3. 单消费者限制异步落库吞吐

一个 Topic 只有一个 Channel、一个消费 goroutine 且 `prefetch=1` 时，异步落库吞吐
容易低于入口流量。

通用 Consumer 支持队列级并发映射，每个消费者使用独立 AMQP Channel：

```yaml
rabbitmq:
  listener:
    simple:
      prefetch: 1
      default_concurrency: 1
      concurrency:
        selection_result_queue: 8
```

集成测试验证同一队列的三个独立 Channel 能同时进入阻塞 Listener；重试测试验证临时
错误 Nack 后重新入队，永久错误则 Reject。

## 4. 通用 Outbox 与业务解耦

后续视频、弹幕等模块同样需要“事务提交与事件发布最终一致”。因此 Outbox 不绑定选课
事件：`outbox_event` 保存 topic、聚合标识和 JSON payload，通用 Dispatcher 负责抢占、
Publisher Confirm、失败退避以及过期 publishing 租约恢复。新模块只需在自己的事务中
调用 `Append`，无需复制消息可靠性代码。

## 5. 当前待解决边界

- 相同 `request_id` 的完整重试应在可变校验之前返回原结果；
- 学生身份应来自认证上下文，不能直接信任请求体；
- 需要提供申请状态和本人选课列表查询；
- Redis 数据丢失且消息尚未落库时，基于旧 MySQL 快照重建存在窗口；
- 退课、候补、先修课程、专业年级和时间冲突规则尚未接入。

这些问题属于下一阶段扩展，不应在当前简历或文档中描述为已完成。
