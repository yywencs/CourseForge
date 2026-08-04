# CourseForge 选课接口压测工具

该工具压测 Redis-first 选课主链路：

```text
POST /api/v1/enrollments
```

它保留了独立的并发 Worker、HTTP 连接池和 QPS/P50/P95/P99 统计，并提供：

- `prepare`：创建专用轮次、教学班和高位 ID 压测学生，重置 MySQL 结果并预热 Redis 额度与名额。
- `run`：预生成学生 JWT 与请求体后并发提交请求，校验 Broker Confirm，并等待 MySQL、Redis 最终状态收敛。

## 构建

```bash
go build -o ./bin/courseforge-benchmark ./cmd/benchmark

docker build \
  --file cmd/benchmark/Dockerfile \
  --tag courseforge-benchmark:local \
  .
```

## 准备数据

`prepare` 会修改指定的 courseforge 数据库和 Redis。为降低误操作风险：

- 必须显式传入 `--confirm-reset`。
- 轮次、教学班和学生 ID 必须不小于 `9000000000000`。
- 如果学生 ID 范围内存在学号不是 `BENCH-` 前缀的数据，工具会拒绝执行。
- MySQL DSN 必须直接连接 `courseforge`。

```bash
export COURSEFORGE_BENCHMARK_MYSQL_DSN='root:密码@tcp(mysql:3306)/courseforge?charset=utf8mb4&parseTime=true&loc=Local&timeout=10s'
export COURSEFORGE_BENCHMARK_REDIS_ADDR='redis:6379'
export COURSEFORGE_BENCHMARK_REDIS_PASSWORD='密码'
export COURSEFORGE_BENCHMARK_JWT_SIGNING_KEY='与 API COURSEFORGE_AUTH_JWT_SIGNING_KEY 相同的至少32字节密钥'

./bin/courseforge-benchmark prepare \
  --confirm-reset \
  --round-id 9000000000101 \
  --teaching-class-id 9000000000301 \
  --student-id-start 9100000000000 \
  --users 1000 \
  --capacity 500 \
  --credit-limit 20 \
  --course-limit 8
```

`capacity` 可以小于 `users`，用于验证高并发下不会超卖。

## 运行压测

`round-id`、`teaching-class-id`、`student-id-start` 和 `users` 必须与 `prepare` 一致：

```bash
./bin/courseforge-benchmark run \
  --url http://127.0.0.1:8080 \
  --round-id 9000000000101 \
  --teaching-class-id 9000000000301 \
 --student-id-start 9100000000000 \
  --users 1000 \
  --concurrency 100 \
  --duration 30s \
  --timeout 5s \
  --verify-timeout 30s
```

`duration` 是整轮最大执行时间，不会让同一学生重复选择同一课程。`selection` 和
`waitlist` 场景的 HTTP 请求数等于 `users`，`idempotency` 场景等于 `users * 2`；
当 `users` 大于 `capacity` 时，超出容量的请求应被统计为业务拒绝。

最终校验默认开启，复用数据准备阶段的 MySQL 和 Redis 环境变量。它会等待异步结果落库，
并校验以下不变量：

- MySQL 申请单、正式选课记录和教学班 `selected_count` 与客户端确认成功操作数一致。
- 正式选课学生与 `request_id` 唯一，不存在重复选课或重复申请。
- MySQL 选中人数不超过容量，Redis 剩余名额不小于零。
- Redis 剩余名额等于容量减去成功人数，且压测学生没有残留 pending。

仅调试 HTTP 驱动器、不连接 MySQL 和 Redis 时，可以显式添加 `--verify=false`；正式压测不应关闭最终校验。

结果会分别输出业务操作数和实际 HTTP 请求数，并统计 QPS、业务成功率、
传输/HTTP/解码错误以及平均、
P50、P95、P99 和最大延迟。容量不足等选课拒绝会按业务码单独统计。
未完成全部请求、出现传输/HTTP/解码错误、幂等结果不一致、非预期业务错误或最终状态校验失败时，命令会返回非零退出码。

`run` 支持三种场景：

- `--scenario selection`：每个学生发起一次正常选课，验证并发下不超卖。
- `--scenario idempotency`：每个学生使用同一个 `request_id` 同时发送两个请求，校验返回同一申请单；每个业务操作对应两个 HTTP 请求。
- `--scenario waitlist`：对已满教学班发起候补申请；运行前应先把准备数据的容量占满。
