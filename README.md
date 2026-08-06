# CourseForge

CourseForge 是一个面向高校选课场景的课程管理与高并发选课系统。项目覆盖课程维护、选课与候补、视频分片上传、实时弹幕等完整业务链路，并围绕并发控制、消息可靠性和可观测性进行工程化设计。

## 在线体验

| 入口 | 地址 | 账号 | 密码 |
| --- | --- | --- | --- |
| 学生端 | [http://223.109.143.131](http://223.109.143.131) | `2026001001` | `CourseForge@123` |
| 管理后台 | [http://223.109.143.131/admin/login](http://223.109.143.131/admin/login) | `admin` | `CourseForgeAdmin@123` |

这是公共演示环境，可以体验课程维护、选课、退课、候补、视频上传与播放以及实时弹幕。数据可能会不定期重置。

## 核心功能

- 学生登录、课程查询、选课、退课和候补。
- 管理员登录，以及课程、教学班和选课轮次维护。
- Redis Lua 原子选课，防止高并发下超卖。
- RabbitMQ 异步落库、延迟重试和死信队列。
- 课程预览视频分片上传、断点续传、播放和无主对象清理。
- 历史弹幕分段查询，以及基于 WebSocket 和 Redis Pub/Sub 的多实例实时弹幕。
- Prometheus 指标和选课、WebSocket 压测工具。

## 技术栈

- 后端：Go、Gin、GORM
- 前端：Vue 3、TypeScript、Vite
- 数据与中间件：MySQL、Redis、RabbitMQ、Asynq
- 文件与协议：MinIO、WebSocket、Protobuf
- 可观测性：Prometheus、Grafana

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

开发环境默认账号：

```text
学生：2026001001 / CourseForge@123
管理员：admin / CourseForgeAdmin@123
```

### 4. 停止服务

停止 API 和 Admin 进程后，关闭 Docker 服务：

```bash
make down
```
