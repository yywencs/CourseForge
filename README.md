# CourseForge

CourseForge 是一个使用 Go 和 Vue 实现的高并发选课与课程展示系统，主要包含：

- 学生登录、课程查询、选课、退课和候补。
- Redis Lua 原子选课，防止高并发下超卖。
- RabbitMQ 异步落库、延迟重试和死信队列。
- 课程预览视频分片上传、断点续传、播放和无主对象清理。
- 历史弹幕分段查询，以及基于 WebSocket 和 Redis Pub/Sub 的多实例实时弹幕。
- Prometheus 指标和选课、WebSocket 压测工具。

## 本地启动

环境要求：Go 1.25+、Node.js 22+、Docker 和 Docker Compose。

### 1. 启动依赖

```bash
cp .env.example .env
cp configs/config.example.yaml configs/config.yaml

make infra-up
```

首次启动 MySQL 时会自动执行数据库结构和开发数据脚本。

### 2. 启动后端

分别打开两个终端：

```bash
make run-api
```

```bash
make run-admin
```

API 默认监听 `http://127.0.0.1:8080`，Admin 默认监听 `http://127.0.0.1:8081`。

### 3. 启动前端

```bash
cd web
npm install
npm run dev
```

浏览器访问 `http://127.0.0.1:5173`。

开发演示账号：

```text
学号：2026001001
密码：CourseForge@123
```

### 4. 停止服务

停止 API 和 Admin 进程后，关闭 Docker 服务：

```bash
make down
```
