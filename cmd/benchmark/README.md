# CourseForge 压测环境

服务器安装 Docker 并登录阿里云 ACR：

```bash
docker login crpi-sm40omlac0wou3kp.cn-hangzhou.personal.cr.aliyuncs.com
```

启动压测环境：

```bash
./deploy/benchmark/env.sh up
```

环境启动后可以直接访问：

```text
API:      http://127.0.0.1:8080
MySQL:   root:courseforge-benchmark@tcp(127.0.0.1:3306)/courseforge
Redis:   127.0.0.1:6379
RabbitMQ: 127.0.0.1:5672
```

默认使用 `v1.2.0` API 镜像。测试其他版本时：

```bash
IMAGE_TAG=v1.2.1 ./deploy/benchmark/env.sh up
```

停止环境并删除压测数据卷：

```bash
./deploy/benchmark/env.sh down
```
