package bootstrap

import (
	danmakuws "github.com/yywencs/courseforge/internal/danmaku/transport/websocket"
	"github.com/yywencs/courseforge/internal/platform/config"
	"github.com/yywencs/courseforge/internal/platform/rabbitmq"
	"github.com/yywencs/courseforge/internal/platform/taskqueue"
	httpserver "github.com/yywencs/courseforge/server/http"
)

// HTTPApp holds the wired application dependencies.
//
// Admin 只连接 CourseForge MySQL；API 额外装配选课所需的 Redis、
// RabbitMQ、Asynq worker 和 RabbitMQ consumer。
type HTTPApp struct {
	Config *config.Config

	apiServer        httpserver.Server
	adminServer      httpserver.Server
	asynqWorker      *taskqueue.AsynqWorker
	rabbitMQConsumer *rabbitmq.RabbitMQConsumer
	danmakuHub       *danmakuws.Hub
}

// APIServer returns the API HTTP server.
func (a *HTTPApp) APIServer() httpserver.Server { return a.apiServer }

// AdminServer returns the Admin HTTP server.
func (a *HTTPApp) AdminServer() httpserver.Server { return a.adminServer }

// AsynqWorker returns the API worker.
func (a *HTTPApp) AsynqWorker() *taskqueue.AsynqWorker { return a.asynqWorker }

// RabbitMQConsumer returns the API consumer.
func (a *HTTPApp) RabbitMQConsumer() *rabbitmq.RabbitMQConsumer { return a.rabbitMQConsumer }

// DanmakuHub returns the API process-local realtime danmaku connection hub.
func (a *HTTPApp) DanmakuHub() *danmakuws.Hub { return a.danmakuHub }
