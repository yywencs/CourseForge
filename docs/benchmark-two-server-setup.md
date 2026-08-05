# CourseForge 双机压测准备

这份文档只负责把两台 Ubuntu 24.04 LTS 服务器准备到可以开始压测的状态，不包含具体压测参数。

推荐拓扑：

```text
压测机（2 核 4G） ──私网──> 被压机（4 核 8G）
运行 benchmark                 运行 API、MySQL、Redis、RabbitMQ
```

## 一、在阿里云控制台配置

创建两台按量付费 ECS，并确保它们位于同一地域、同一 VPC，系统镜像均选择 Ubuntu 24.04 LTS 64 位。

记录两台机器的私网 IP，后文用以下占位符表示：

```text
<LOAD_PRIVATE_IP>    压测机私网 IP
<TARGET_PRIVATE_IP>  被压机私网 IP
```

安全组入方向规则：

| 机器 | 端口 | 来源 | 用途 |
| --- | --- | --- | --- |
| 两台机器 | TCP 22 | 你的公网 IP | SSH 登录 |
| 被压机 | TCP 8080 | 压测机私网 IP/32 | API 与 WebSocket 压测 |

不要向公网开放 MySQL 3306、Redis 6379 和 RabbitMQ 5672。被压机的 Compose 编排只将这些端口绑定在本机回环地址。

## 二、两台机器都执行

以下命令安装 Git、Docker Engine 和 Docker Compose。分别登录两台服务器后执行一次：

```bash
sudo apt-get update
sudo apt-get install -y ca-certificates curl git
sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc

. /etc/os-release
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu ${VERSION_CODENAME} stable" | sudo tee /etc/apt/sources.list.d/docker.list >/dev/null

sudo apt-get update
sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
sudo usermod -aG docker "$USER"
```

退出 SSH 后重新登录，让 Docker 用户组生效，然后验证：

```bash
docker version
docker compose version
```

## 三、被压机执行

### 1. 获取固定版本代码

```bash
git clone --branch v1.2.0 --depth 1 https://github.com/yywencs/CourseForge.git
cd CourseForge
```

如果仓库是私有的，请使用你已经配置好 SSH Key 的 SSH 地址克隆。

### 2. 登录阿里云 ACR

```bash
docker login crpi-sm40omlac0wou3kp.cn-hangzhou.personal.cr.aliyuncs.com
```

按提示输入 ACR 用户名和密码。凭据只保存在服务器，不要写进仓库。

### 3. 启动被压环境

```bash
./deploy/benchmark/env.sh up
```

该命令会启动 MySQL、Redis、RabbitMQ 和 CourseForge API，等待依赖健康后再检查 API 就绪状态。默认拉取 `v1.2.0` API 镜像。

检查服务：

```bash
docker compose -f deploy/benchmark/compose.yaml ps
curl -fsS http://127.0.0.1:8080/readyz
```

需要测试其他镜像版本时，先清理旧环境再指定镜像标签启动：

```bash
./deploy/benchmark/env.sh down
IMAGE_TAG=v1.2.1 ./deploy/benchmark/env.sh up
```

## 四、压测机执行

### 1. 获取相同版本代码

```bash
git clone --branch v1.2.0 --depth 1 https://github.com/yywencs/CourseForge.git
cd CourseForge
mkdir -p benchmark-results
```

### 2. 验证私网链路

将命令中的 `<TARGET_PRIVATE_IP>` 换成被压机的真实私网 IP：

```bash
curl -fsS http://<TARGET_PRIVATE_IP>:8080/readyz
```

能够返回成功结果，说明 VPC、安全组和 API 均已就绪。如果本机访问成功而这里超时，优先检查被压机安全组的 8080 入方向规则。

### 3. 构建两个压测镜像

```bash
docker build -f cmd/benchmark/enrollment/Dockerfile -t courseforge-enrollment-benchmark:v1.2.0 .
docker build -f cmd/benchmark/websocket/Dockerfile -t courseforge-websocket-benchmark:v1.2.0 .
```

至此压测机准备完成。运行压测时，HTTP 地址或 WebSocket `targets` 应填写：

```text
http://<TARGET_PRIVATE_IP>:8080
```

不要填写被压机的公网 IP，否则结果会混入公网链路波动和带宽限制。

## 五、压测时观察被压机

另开一个被压机 SSH 窗口观察容器资源：

```bash
cd CourseForge
docker stats
```

需要排查服务日志时：

```bash
docker compose -f deploy/benchmark/compose.yaml logs --tail=200 api
```

同时在阿里云控制台观察两台 ECS 的 CPU、内存和网络。若压测机 CPU 已接近满载，测到的是压测机上限，需要先提高压测机配置或减少单机连接规模。

## 六、结束后清理

被压机执行以下命令会停止容器并删除本次压测的 MySQL、Redis 和 RabbitMQ 数据卷：

```bash
cd CourseForge
./deploy/benchmark/env.sh down
```

确认结果已经保存后，在阿里云控制台释放两台按量付费 ECS，避免继续计费。
