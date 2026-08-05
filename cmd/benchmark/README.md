# CourseForge 压测工具

## 构建

```bash
make build-benchmark
```

构建后会生成：

- `bin/courseforge-benchmark`：选课链路压测工具。
- `bin/courseforge-websocket-benchmark`：实时弹幕 WebSocket 压测工具。

完整参数可以通过以下命令查看：

```bash
./bin/courseforge-benchmark --help
./bin/courseforge-websocket-benchmark --help
```

## 选课压测

先配置 MySQL、Redis 和学生 JWT：

```bash
export COURSEFORGE_BENCHMARK_MYSQL_DSN='root:密码@tcp(127.0.0.1:3306)/courseforge?charset=utf8mb4&parseTime=true&loc=Local&timeout=10s'
export COURSEFORGE_BENCHMARK_REDIS_ADDR='127.0.0.1:6379'
export COURSEFORGE_BENCHMARK_REDIS_PASSWORD='Redis 密码'
export COURSEFORGE_BENCHMARK_JWT_SIGNING_KEY='与 API 相同的至少 32 字节密钥'
```

准备压测使用的轮次、教学班和高位 ID 学生：

```bash
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

运行选课压测：

```bash
./bin/courseforge-benchmark run \
  --url http://127.0.0.1:8080 \
  --scenario selection \
  --round-id 9000000000101 \
  --teaching-class-id 9000000000301 \
  --student-id-start 9100000000000 \
  --users 1000 \
  --concurrency 100 \
  --duration 30s \
  --timeout 5s \
  --verify-timeout 30s
```

`--scenario` 支持 `selection`、`idempotency` 和 `waitlist`。`prepare` 和 `run` 使用的轮次、教学班、学生起始 ID 和用户数量需要保持一致。

## 实时弹幕 WebSocket 压测

运行前需要满足以下条件：

- API、MySQL 和 Redis 已启动。
- `--video-id` 指向一个可播放的预览视频。
- 压测学生已经存在，可以复用选课压测 `prepare` 创建的学生。
- `COURSEFORGE_BENCHMARK_JWT_SIGNING_KEY` 与 API 使用的学生 JWT 密钥一致。

单实例压测：

```bash
./bin/courseforge-websocket-benchmark \
  --targets http://127.0.0.1:8080 \
  --video-id 1 \
  --student-id-start 9100000000000 \
  --clients 1000 \
  --publishers 4 \
  --publish-every 100ms \
  --warmup 10s \
  --duration 1m \
  --drain 5s
```

多实例压测时使用逗号分隔地址：

```bash
./bin/courseforge-websocket-benchmark \
  --targets http://127.0.0.1:8080,http://127.0.0.1:8081 \
  --video-id 1 \
  --clients 1000 \
  --publishers 4 \
  --duration 1m
```

运行期间会输出连接数、发布 QPS、广播交付 QPS 和 P99 延迟。最终结果保存在：

```text
benchmark-results/websocket/<运行时间>/summary.json
```
