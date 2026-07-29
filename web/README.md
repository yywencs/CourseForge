# CourseForge Web

Vue 3 + TypeScript 的选课系统前端，学生端和教务端位于同一个工程。

## 本地开发

```bash
npm install
npm run dev
```

默认通过 Vite 将 `/api` 代理到 `http://127.0.0.1:8080`。如 Go API
运行在其他地址，请复制 `.env.example` 为 `.env.local` 并修改
`VITE_DEV_PROXY_TARGET`。

## 路由

- `/student/courses`：学生选课中心
- `/student/enrollments`：学生选课记录
- `/student/schedule`：学生课表
- `/admin`：教务概览
- `/admin/courses`：课程和教学班
- `/admin/enrollments`：选课申请监控

当前只有 `POST /api/v1/enrollments` 已接入真实后端。课程列表、选课结果、
课表和管理端数据在对应查询接口完成前使用明确标记的演示数据。
