# CourseForge Web

Vue 3 + TypeScript 的选课系统前端，学生端和教务端位于同一个工程。界面只消费
后端公开能力，不在浏览器中伪造学生身份、库存或最终选课结果。

## 本地开发

```bash
npm install
npm run dev
```

默认通过 Vite 将 `/api` 代理到 `http://127.0.0.1:8080`，将
`/admin-api` 代理到 `http://127.0.0.1:8081`。如 Go 服务运行在其他地址，
请复制 `.env.example` 为 `.env.local` 并调整其中的代理目标。

首次访问会进入 `/connect`。生产环境应粘贴认证服务签发的 JWT；本地开发模式也可
输入后端 JWT 密钥，在浏览器内临时生成调试令牌。密钥不会被持久化，学生身份由
JWT 声明提供，不会放进选课请求体。

## 路由

- `/connect`：连接学生身份和当前学期/轮次
- `/student/courses`：学生选课中心
- `/student/enrollments`：申请单、已选课程和候补队列
- `/student/schedule`：学生课表
- `/student/account`：会话和选课上下文
- `/admin`：教务概览
- `/admin/courses`：课程和教学班
- `/admin/enrollments`：选课申请监控

## 已接入的后端能力

学生端已覆盖后端当前公开的全部 8 个选课接口：

- 提交选课、按申请单查询异步处理结果
- 查询本人选课记录、退课
- 加入候补、查询候补单、查询本人候补队列、取消候补

选课使用稳定的 `request_id` 做幂等重试，并持续展示申请状态、
RabbitMQ Confirm 和 MySQL 异步落库状态。管理员端接入服务健康检查与就绪探针；
当前后端尚未提供课程目录 CRUD、全局申请列表和视频内容管理接口，因此对应页面
会明确展示能力边界，不使用假数据冒充真实管理结果。课程目录中的课程描述、课表
位置和视频链接是前端展示适配数据，实际选课资格、学分与名额均以后端为准。

## 质量检查

```bash
npm run typecheck
npm run test:run
npm run build
npm run test:e2e
```

端到端测试会启动 Vite，并用网络路由模拟后端响应，验证学生连接、发起选课和申请单
异步状态展示这条关键链路。
