package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/yywencs/courseforge/internal/bootstrap"
	"github.com/yywencs/courseforge/internal/platform/observability/logger"
)

func main() {
	app, err := bootstrap.NewAPIApp()
	if err != nil {
		log.Fatalf("bootstrap API app: %v", err)
	}
	defer func() { _ = app.Close() }()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	apiServer := app.APIServer()

	// 先同步声明 RabbitMQ 的 Exchange、Queue 和 Binding，再启动 Outbox Relay 和
	// HTTP 服务，避免应用刚启动时消息先发布、队列尚未绑定而成为不可路由消息。
	logger.Info("starting RabbitMQ consumer")
	if err := app.RabbitMQConsumer().Start(ctx); err != nil {
		log.Fatalf("start RabbitMQ consumer: %v", err)
	}
	logger.Info("starting MySQL Outbox relay")
	if err := app.OutboxRelay().Start(ctx); err != nil {
		log.Fatalf("start MySQL Outbox relay: %v", err)
	}
	logger.Info("starting selection Redis Stream consumer")
	if err := app.SelectionStreamConsumer().Start(ctx); err != nil {
		log.Fatalf("start selection Redis Stream consumer: %v", err)
	}

	logger.Info("starting realtime danmaku hub")
	app.DanmakuHub().Start()
	logger.Info("starting realtime danmaku subscriber")
	if err := app.DanmakuSubscriber().Start(ctx); err != nil {
		log.Fatalf("start realtime danmaku subscriber: %v", err)
	}

	// 启动 API HTTP 服务
	go func() {
		logger.Info("starting API server", "addr", app.Config.Server.API.Addr)
		if err := apiServer.Run(); err != nil {
			logger.Error("API server stopped", "error", err)
		}
	}()

	// 启动 Asynq worker
	go func() {
		logger.Info("starting Asynq worker")
		if err := app.AsynqWorker().Start(ctx); err != nil {
			logger.Error("Asynq worker stopped", "error", err)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down...")

	// 优雅关闭：先关闭 HTTP 服务（停止接收新请求）
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := apiServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("API server shutdown error", "error", err)
	} else {
		logger.Info("API server shut down gracefully")
	}
	if err := app.DanmakuSubscriber().Stop(shutdownCtx); err != nil {
		logger.Error("realtime danmaku subscriber shutdown error", "error", err)
	} else {
		logger.Info("realtime danmaku subscriber shut down gracefully")
	}
	if err := app.DanmakuHub().Stop(shutdownCtx); err != nil {
		logger.Error("realtime danmaku hub shutdown error", "error", err)
	} else {
		logger.Info("realtime danmaku hub shut down gracefully")
	}
	if err := app.SelectionStreamConsumer().Stop(shutdownCtx); err != nil {
		logger.Error("selection Redis Stream consumer shutdown error", "error", err)
	} else {
		logger.Info("selection Redis Stream consumer shut down gracefully")
	}
	if err := app.OutboxRelay().Stop(shutdownCtx); err != nil {
		logger.Error("MySQL Outbox relay shutdown error", "error", err)
	} else {
		logger.Info("MySQL Outbox relay shut down gracefully")
	}

	app.AsynqWorker().Shutdown()
	app.RabbitMQConsumer().Shutdown()

	logger.Info("shutdown complete")
}
