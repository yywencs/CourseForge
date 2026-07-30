# CourseForge 选课接口压测工具

该工具压测 Redis-first 选课主链路：

```text
POST /api/v1/enrollments
```

它保留了独立的并发 Worker、HTTP 连接池和 QPS/P50/P95/P99 统计，并提供：

- `prepare`：创建专用轮次、教学班和高位 ID 压测学生，重置 MySQL 结果并预热 Redis 额度与名额。
- `run`：每个压测学生提交一次请求，为每次请求生成唯一 `request_id`，并校验 Broker Confirm 响应。

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
- MySQL DSN 必须直接连接 `courseforge`，不再使用旧抽奖分库模板。

```bash
export COURSEFORGE_BENCHMARK_MYSQL_DSN='root:密码@tcp(mysql:3306)/courseforge?charset=utf8mb4&parseTime=true&loc=Local&timeout=10s'
export COURSEFORGE_BENCHMARK_REDIS_ADDR='redis:6379'
export COURSEFORGE_BENCHMARK_REDIS_PASSWORD='密码'

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
  --timeout 5s
```

`duration` 是整轮最大执行时间，不会让同一学生重复选择同一课程。正常情况下请求总数
等于 `users`；当 `users` 大于 `capacity` 时，超出容量的请求应被统计为业务拒绝。

结果会输出总请求数、QPS、业务成功率、传输/HTTP/解码错误以及平均、
P50、P95、P99 和最大延迟。容量不足等选课拒绝会按业务码单独统计。
