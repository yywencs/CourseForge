# syntax=docker/dockerfile:1

FROM golang:1.25-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

ARG TARGETOS=linux
ARG TARGETARCH

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/courseforge-api ./cmd/api

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/courseforge-admin ./cmd/admin

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/courseforge-cdc-sync ./cmd/cdc-sync

FROM debian:bookworm-slim AS runtime

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 10001 courseforge \
    && useradd --uid 10001 --gid courseforge --no-create-home --shell /usr/sbin/nologin courseforge \
    && mkdir -p /app/configs /app/logs \
    && chown -R courseforge:courseforge /app

WORKDIR /app

ENV TZ=Asia/Shanghai

USER courseforge
STOPSIGNAL SIGTERM

# CDC 从环境变量读取配置，不依赖 configs/config.yaml。
FROM runtime AS cdc-sync
COPY --from=builder --chown=courseforge:courseforge /out/courseforge-cdc-sync /usr/local/bin/courseforge-cdc-sync
ENTRYPOINT ["courseforge-cdc-sync"]

# Admin 从 /app/configs/config.yaml 读取配置。
FROM runtime AS admin
COPY --from=builder --chown=courseforge:courseforge /out/courseforge-admin /usr/local/bin/courseforge-admin
EXPOSE 8081
ENTRYPOINT ["courseforge-admin"]

# API 从 /app/configs/config.yaml 读取配置；最后一个 stage 也是默认构建目标。
FROM runtime AS api
COPY --from=builder --chown=courseforge:courseforge /out/courseforge-api /usr/local/bin/courseforge-api
EXPOSE 8080
ENTRYPOINT ["courseforge-api"]
