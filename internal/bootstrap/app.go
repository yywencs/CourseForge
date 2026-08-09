package bootstrap

import (
	"github.com/hibiken/asynq"
	danmakuredis "github.com/yywencs/courseforge/internal/danmaku/infrastructure/redis"
	danmakuws "github.com/yywencs/courseforge/internal/danmaku/transport/websocket"
	enrollmentasync "github.com/yywencs/courseforge/internal/enrollment/async"
	"github.com/yywencs/courseforge/internal/platform/config"
	outboxrelay "github.com/yywencs/courseforge/internal/platform/outbox/relay"
	"github.com/yywencs/courseforge/internal/platform/rabbitmq"
	"github.com/yywencs/courseforge/internal/platform/taskqueue"
	httpserver "github.com/yywencs/courseforge/server/http"
)

// HTTPApp holds the wired application dependencies.
//
// Admin 连接 MySQL、Redis 和 Asynq 以管理轮次预热；API 额外装配
// RabbitMQ、Asynq worker 与实时弹幕组件。
type HTTPApp struct {
	Config *config.Config

	apiServer         httpserver.Server
	adminServer       httpserver.Server
	asynqWorker       *taskqueue.AsynqWorker
	rabbitMQConsumer  *rabbitmq.RabbitMQConsumer
	outboxRelay       *outboxrelay.Relay
	selectionConsumer *enrollmentasync.SelectionStreamConsumer
	danmakuHub        *danmakuws.Hub
	danmakuSubscriber *danmakuredis.RealtimeSubscriber
	asynqClient       *asynq.Client
}

// Close 释放仅用于投递任务的 Asynq 客户端。
func (a *HTTPApp) Close() error {
	if a == nil || a.asynqClient == nil {
		return nil
	}
	return a.asynqClient.Close()
}

// APIServer returns the API HTTP server.
func (a *HTTPApp) APIServer() httpserver.Server { return a.apiServer }

// AdminServer returns the Admin HTTP server.
func (a *HTTPApp) AdminServer() httpserver.Server { return a.adminServer }

// AsynqWorker returns the API worker.
func (a *HTTPApp) AsynqWorker() *taskqueue.AsynqWorker { return a.asynqWorker }

// RabbitMQConsumer returns the API consumer.
func (a *HTTPApp) RabbitMQConsumer() *rabbitmq.RabbitMQConsumer { return a.rabbitMQConsumer }

// OutboxRelay returns the resident MySQL Outbox to RabbitMQ publisher.
func (a *HTTPApp) OutboxRelay() *outboxrelay.Relay { return a.outboxRelay }

// SelectionStreamConsumer returns the Redis Stream to MySQL projection worker.
func (a *HTTPApp) SelectionStreamConsumer() *enrollmentasync.SelectionStreamConsumer {
	return a.selectionConsumer
}

// DanmakuHub returns the API process-local realtime danmaku connection hub.
func (a *HTTPApp) DanmakuHub() *danmakuws.Hub { return a.danmakuHub }

// DanmakuSubscriber returns the cross-instance realtime danmaku subscriber.
func (a *HTTPApp) DanmakuSubscriber() *danmakuredis.RealtimeSubscriber {
	return a.danmakuSubscriber
}
